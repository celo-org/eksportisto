package indexer

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/rdb"
	"github.com/celo-org/kliento/client"
	"github.com/go-errors/errors"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

type worker struct {
	celoClient       *client.CeloClient
	db               *rdb.RedisDB
	logger           log.Logger
	index            int
	baseBlockHandler *baseBlockHandler
	parent           *indexer
	output           Output
	nodeURI          string
}

type block struct {
	number uint64
	source string
}

func (b block) fromTip() bool {
	return b.source == rdb.TipQueue
}

// newWorker sets up the struct responsible for a worker process.
// The workers process blocks in parallel and have their own
// connections to nodes (ideally different) and redis.
func (svc *indexer) newWorker(ctx context.Context, index int) (*worker, error) {
	logger := svc.logger.New("workerIndex", index)
	logger.Info("Starting worker")

	nodeURI := viper.GetString("indexer.celoNodeURI")
	nodeURIOverride := viper.GetString(fmt.Sprintf("indexer.workerCeloNodeURIOverride.%d", index))
	if nodeURIOverride != "" {
		nodeURI = nodeURIOverride
	}

	logger.Info("Connecting to node", "nodeURI", nodeURI)

	celoClient, err := client.Dial(nodeURI)
	if err != nil {
		return nil, err
	}

	var output Output
	dest := viper.GetString("indexer.dest")
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

	w := &worker{
		celoClient: celoClient,
		db:         rdb.NewRedisDatabase(),
		index:      index,
		logger:     logger,
		parent:     svc,
		output:     output,
		nodeURI:    nodeURI,
	}

	w.baseBlockHandler, err = w.newBaseBlockHandler()
	if err != nil {
		return nil, err
	}

	return w, nil
}

// start starts a worker's main loop which consists of
// trying to dequeue a block, checking if it's already
// processed and firing a handler for it.
func (w *worker) start(ctx context.Context) error {
	blocks := make(chan block, 2)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case block := <-blocks:
			indexed, err := w.db.HGet(ctx, rdb.BlocksMap, fmt.Sprintf("%d", block.number)).Bool()
			if err != nil && err != redis.Nil {
				return err
			}
			if indexed {
				w.logger.Info("Skipping block: already indexed", "number", block.number)
			} else {
				// Reconnect the node 25% of the time as an experiment
				// in dealing with our full nodes that seam to act weird
				if rand.Intn(4) == 0 {
					w.celoClient, err = client.Dial(w.nodeURI)
					if err != nil {
						return err
					}
				}

				blockProcessStartedAt := time.Now()
				handler, err := w.newBlockHandler(block)
				if err != nil {
					return err
				}

				if err := handler.run(ctx); err != nil {
					if err, ok := err.(*errors.Error); ok {
						handler.logger.Error("Failed block", "err", err.Error(), "stack", err.ErrorStack())
					} else {
						handler.logger.Error("Failed block", "err", err)
					}
				} else {
					metrics.LastBlockProcessed.WithLabelValues(block.source).Set(float64(block.number))
					metrics.ProcessBlockDuration.Observe(float64(time.Since(blockProcessStartedAt)) / float64(time.Second))
					w.db.HSet(ctx, rdb.BlocksMap, fmt.Sprintf("%d", block.number), true)
					handler.logger.Info("Block done")
				}
			}
		default:
			foundBlock, err := w.dequeueBlock(ctx, blocks)
			if err != nil {
				return err
			}
			if !foundBlock {
				time.Sleep(w.parent.sleepInterval)
			}
		}
	}
}

// dequeueBlock is used to get the next block to process. It will first
// try the TipQueue (which follows the tip of the chain) and if that's
// empty it will try the BackfillQueue which is responsible for
// backfilling and missing blocks
func (w *worker) dequeueBlock(ctx context.Context, blocks chan block) (bool, error) {
	result, err := w.db.LPop(ctx, rdb.TipQueue).Uint64()
	if err != nil && err != redis.Nil {
		return false, err
	}

	if err == nil {
		blocks <- block{result, rdb.TipQueue}
		w.logger.Info("Dequeued block", "block", result, "queue", "tip")
		return true, nil
	}

	result, err = w.db.LPop(ctx, rdb.BackfillQueue).Uint64()
	if err != nil && err != redis.Nil {
		return false, err
	}

	if err == nil {
		w.logger.Info("Dequeued block", "block", result, "queue", "backfill")
		blocks <- block{result, rdb.BackfillQueue}
		return true, nil
	}

	return false, nil
}
