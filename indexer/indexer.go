package indexer

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/rdb"
	"github.com/celo-org/kliento/client"
	"golang.org/x/sync/errgroup"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

var EpochSize = uint64(17280)   // 17280 = 12 * 60 * 24
var BlocksPerHour = uint64(720) // 720 = 12 * 60

type indexer struct {
	workers       int
	logger        log.Logger
	sleepInterval time.Duration
}

type worker struct {
	celoClient       *client.CeloClient
	db               *rdb.RedisDB
	logger           log.Logger
	index            int
	baseBlockHandler *baseBlockHandler
	parent           *indexer
}

func newIndexer() (*indexer, error) {
	handler := log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.JSONFormat()))
	logger := log.New()
	logger.SetHandler(handler)

	return &indexer{
		logger:        logger,
		sleepInterval: viper.GetDuration("indexer.sleepIntervalMilliseconds") * time.Millisecond,
		workers:       viper.GetInt("indexer.workers"),
	}, nil
}

func (svc *indexer) newWorker(ctx context.Context, index int) (*worker, error) {
	logger := svc.logger.New("workerIndex", index)
	logger.Info("Starting worker")

	nodeURI := viper.GetString(fmt.Sprintf("indexer.workerNodeURIs.%d", index))
	logger.Info("Connecting to node", "nodeURI", nodeURI)

	celoClient, err := client.Dial(nodeURI)
	if err != nil {
		return nil, err
	}

	w := &worker{
		celoClient: celoClient,
		db:         rdb.NewRedisDatabase(),
		index:      index,
		logger:     logger,
		parent:     svc,
	}

	w.baseBlockHandler, err = w.newBaseBlockHandler()
	if err != nil {
		return nil, err
	}

	return w, nil
}

type block struct {
	number  uint64
	fromTip bool
}

func (w *worker) dequeueBlock(ctx context.Context, blocks chan block) (bool, error) {
	result, err := w.db.LPop(ctx, rdb.TipQueue).Uint64()
	if err != nil && err != redis.Nil {
		return false, err
	}

	if err == nil {
		blocks <- block{result, true}
		w.logger.Info("Dequeued block", "block", result, "queue", "tip")
		return true, nil
	}

	result, err = w.db.LPop(ctx, rdb.BackfillQueue).Uint64()
	if err != nil && err != redis.Nil {
		return false, err
	}

	if err == nil {
		w.logger.Info("Dequeued block", "block", result, "queue", "backfill")
		blocks <- block{result, false}
		return true, nil
	}

	return false, nil
}

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
				handler, err := w.newBlockHandler(block.number, block.fromTip)
				if err != nil {
					return err
				}

				if err := handler.run(ctx); err != nil {
					handler.logger.Error("Failed block", "err", err)
				} else {
					handler.logger.Info("Block done")
					w.db.HSet(ctx, rdb.BlocksMap, fmt.Sprintf("%d", block.number), true)
				}
			}
		default:
			queued, err := w.dequeueBlock(ctx, blocks)
			if err != nil {
				return err
			}
			if !queued {
				time.Sleep(w.parent.sleepInterval)
			}
		}
	}
}

// Start is the entry point of the indexer service.
// It sets up and starts a number of workers which
// will process blocks in parallel
func Start(ctx context.Context) error {
	indexer, err := newIndexer()
	if err != nil {
		return err
	}

	group, ctx := errgroup.WithContext(ctx)
	for i := 0; i < indexer.workers; i++ {
		worker, err := indexer.newWorker(ctx, i)
		if err != nil {
			return err
		}
		group.Go(func() error { return worker.start(ctx) })
	}

	return group.Wait()
}
