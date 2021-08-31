package indexer

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/rdb"
	"github.com/celo-org/kliento/client"
	"github.com/go-errors/errors"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
	"gopkg.in/matryer/try.v1"
)

type worker struct {
	celoClient         *client.CeloClient
	db                 *rdb.RedisDB
	logger             log.Logger
	baseBlockHandler   *baseBlockHandler
	blockRetryAttempts int
	blockRetryDelay    time.Duration
	output             Output
	input              rdb.Queue
	nodeURI            string
	sleepInterval      time.Duration
	concurrency        int
	collectMetrics     bool
	collectData        bool
}

// newWorker sets up the struct responsible for a worker process.
// The workers process blocks in parallel and have their own
// connections to nodes (ideally different) and redis.
func newWorker(ctx context.Context) (*worker, error) {
	handler := log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.JSONFormat()))
	logger := log.New()
	logger.SetHandler(handler)

	nodeURI := viper.GetString("celoNodeURI")
	celoClient, err := client.Dial(nodeURI)
	if err != nil {
		return nil, err
	}

	var output Output
	dest := viper.GetString("indexer.destination")
	if dest == "stdout" {
		output = newStdoutOutput()
	} else if dest == "bigquery" {
		output, err = newBigQueryOutput(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("indexer.dest invalid, expecting: stdout, bigquery")
	}

	input, err := parseInput(viper.GetString("indexer.source"))
	if err != nil {
		return nil, err
	}

	mode, err := newMode(viper.GetString("indexer.mode"))
	if err != nil {
		return nil, err
	}

	logger.Info("Starting Worker", "nodeURI", nodeURI, "source", input, "destination", dest, "mode", mode)

	w := &worker{
		logger:             logger,
		sleepInterval:      viper.GetDuration("indexer.sleepIntervalMilliseconds") * time.Millisecond,
		celoClient:         celoClient,
		db:                 rdb.NewRedisDatabase(),
		output:             output,
		input:              input,
		nodeURI:            nodeURI,
		blockRetryAttempts: viper.GetInt("indexer.blockRetryAttempts"),
		blockRetryDelay:    viper.GetDuration("indexer.blockRetryDelayMilliseconds") * time.Millisecond,
		concurrency:        viper.GetInt("indexer.concurrency"),
		collectMetrics:     mode.shouldCollectMetrics(),
		collectData:        mode.shouldCollectData(),
	}

	w.baseBlockHandler, err = w.newBaseBlockHandler()
	if err != nil {
		return nil, err
	}

	return w, nil
}

func (w *worker) isTip() bool {
	return w.input == rdb.TipQueue
}

// start starts a worker's main loop which consists of
// trying to dequeue a block, checking if it's already
// processed and firing a handler for it.
func (w *worker) start(ctx context.Context) error {
	blocks := make(chan uint64, 2)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case block := <-blocks:
			indexed, err := w.db.HGet(ctx, rdb.BlocksMap, fmt.Sprintf("%d", block)).Bool()
			if err != nil && err != redis.Nil {
				return err
			}
			if indexed {
				w.logger.Info("Skipping block: already indexed", "number", block)
			} else {
				var blockProcessStartedAt time.Time
				var handler *blockHandler

				err := try.Do(func(attempt int) (retry bool, err error) {
					retry = attempt < w.blockRetryAttempts
					blockProcessStartedAt = time.Now()
					err = func() error {
						handler, err = w.newBlockHandler(block)
						if err != nil {
							return err
						}
						return handler.run(ctx)
					}()

					if err != nil && retry {
						handler.logger.Warn("Retrying block", "err", err.Error(), "attempt", attempt)
						time.Sleep(w.blockRetryDelay)
					}
					return
				})

				if err != nil {
					if errWithStack, ok := err.(*errors.Error); ok {
						handler.logger.Error(
							"Failed block",
							"err", errWithStack.Error(),
							"stack", errWithStack.ErrorStack(),
						)
					} else {
						handler.logger.Error("Failed block", "err", err.Error())
					}
					metrics.BlockFinished.WithLabelValues(w.input, "fail").Add(1)
				} else {
					metrics.LastBlockProcessed.WithLabelValues(w.input).Set(float64(block))
					metrics.ProcessBlockDuration.Observe(float64(time.Since(blockProcessStartedAt)) / float64(time.Second))
					metrics.BlockFinished.WithLabelValues(w.input, "success").Add(1)

					w.db.HSet(ctx, rdb.BlocksMap, fmt.Sprintf("%d", block), true)
					handler.logger.Info("Block done")
				}
			}
		default:
			block, err := w.db.LPop(ctx, w.input).Uint64()
			if err != nil && err != redis.Nil {
				return err
			}

			if err == nil {
				blocks <- block
				w.logger.Info("Dequeued block", "block", block, "queue", w.input)
			} else {
				time.Sleep(w.sleepInterval)
			}
		}
	}
}
