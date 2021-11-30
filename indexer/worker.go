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

type Worker struct {
	celoClient         *client.CeloClient
	db                 *rdb.RedisDB
	logger             log.Logger
	blockRetryAttempts int
	blockRetryDelay    time.Duration
	output             Output
	input              rdb.Queue
	nodeURI            string
	dequeueTimeout     time.Duration
	concurrency        int
	collectMetrics     bool
	collectData        bool
	debugEnabled       bool
}

// NewWorker sets up the struct responsible for a worker process.
// The workers process blocks in parallel and have their own
// connections to nodes (ideally different) and redis.
func NewWorker(ctx context.Context) (*Worker, error) {
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
		return nil, fmt.Errorf("indexer.destination invalid, expecting: stdout, bigquery")
	}

	input, err := ParseInput(viper.GetString("indexer.source"))
	if err != nil {
		return nil, err
	}

	mode, err := newMode(viper.GetString("indexer.mode"))
	if err != nil {
		return nil, err
	}

	logger.Info("Starting Worker", "nodeURI", nodeURI, "source", input, "destination", dest, "mode", mode)

	supported, err := celoClient.Rpc.SupportedModules()
	if err != nil {
		return nil, err
	}
	_, debugEnabled := supported["debug"]

	return &Worker{
		logger:             logger,
		dequeueTimeout:     viper.GetDuration("indexer.dequeueTimeoutMilliseconds") * time.Millisecond,
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
		debugEnabled:       debugEnabled,
	}, nil
}

func (w *Worker) isTip() bool {
	return w.input == rdb.TipQueue
}

// start starts a worker's main loop which consists of
// trying to dequeue a block, checking if it's already
// processed and firing a handler for it.
func (w *Worker) start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			block, err := w.db.PopBlock(ctx, w.dequeueTimeout)
			if err != nil && err != redis.Nil {
				return err
			} else if err == redis.Nil {
				continue
			}

			w.logger.Info("Dequeued block", "block", block, "queue", w.input)
			indexed, err := w.db.HGet(ctx, rdb.BlocksMap, fmt.Sprintf("%d", block)).Bool()
			if err != nil && err != redis.Nil {
				return err
			}

			if indexed {
				w.logger.Info("Skipping block: already indexed", "number", block)
				continue
			}

			handler, duration, err := w.indexBlockWithRetry(ctx, block)

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
				metrics.ProcessBlockDuration.Observe(float64(duration) / float64(time.Second))
				metrics.LastBlockProcessed.WithLabelValues(w.input).Set(float64(block))
				metrics.BlockFinished.WithLabelValues(w.input, "success").Add(1)

				if w.collectData {
					// Only mark block as done if data is getting collected
					// Don't mark it if we're just in metrics mode
					w.db.HSet(ctx, rdb.BlocksMap, fmt.Sprintf("%d", block), true)
				}
				handler.logger.Info("Block done")
			}
		}
	}
}

// indexBlockWithRetry attempts to process the block for
// a configurable amount of times and then returns the
// handler, duration and err
func (w *Worker) indexBlockWithRetry(ctx context.Context, block uint64) (*blockHandler, time.Duration, error) {
	var handler *blockHandler
	var blockProcessStartedAt time.Time

	err := try.Do(func(attempt int) (retry bool, err error) {
		retry = attempt < w.blockRetryAttempts
		err = func() error {
			blockProcessStartedAt = time.Now()
			handler, err = w.NewBlockHandler(block)
			if err != nil {
				return err
			}
			return handler.Run(ctx)
		}()

		if err != nil && retry {
			handler.logger.Warn("Retrying block", "err", err.Error(), "attempt", attempt)
			time.Sleep(w.blockRetryDelay)
		}
		return
	})

	return handler, time.Since(blockProcessStartedAt), err
}
