package publisher

import (
	"context"
	"os"
	"time"

	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/rdb"
	"github.com/celo-org/kliento/client"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

type backfillPublisher struct {
	db            *rdb.RedisDB
	celoClient    *client.CeloClient
	sleepInterval time.Duration
	logger        log.Logger
	cursor        uint64
	batchSize     uint64
	tipBuffer     uint64
}

func newBackfillPublisher(ctx context.Context) (publisher, error) {
	if !viper.GetBool("publisher.backfill.enabled") {
		return nil, nil
	}

	db := rdb.NewRedisDatabase()

	celoClient, err := client.Dial(viper.GetString("celoNodeURI"))
	if err != nil {
		return nil, err
	}

	cursor, err := db.Get(ctx, rdb.BackfillCursor).Uint64()
	if err == redis.Nil {
		cursor = viper.GetUint64("publisher.backfill.startBlock")
	} else if err != nil {
		return nil, err
	}

	handler := log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.JSONFormat()))
	logger := log.New()
	logger.SetHandler(handler)

	return &backfillPublisher{
		db:            db,
		celoClient:    celoClient,
		logger:        logger,
		cursor:        cursor,
		batchSize:     viper.GetUint64("publisher.backfill.batchSize"),
		sleepInterval: viper.GetDuration("publisher.backfill.sleepIntervalMilliseconds") * time.Millisecond,
		tipBuffer:     viper.GetUint64("publisher.backfill.tipBuffer"),
	}, nil
}

func (svc *backfillPublisher) updateCursor(ctx context.Context, batchSize uint64) error {
	batch, err := svc.db.GetBlocksBatch(ctx, svc.cursor, batchSize*2)
	if err != nil {
		return err
	}

	for i := svc.cursor; i < svc.cursor+batchSize; i++ {
		if batch[i] {
			svc.cursor = i + 1
		} else {
			break
		}
	}

	err = svc.db.Set(ctx, rdb.BackfillCursor, svc.cursor, 0).Err()
	if err == nil {
		svc.logger.Info("Cursor updated", "cursor", svc.cursor)
		metrics.BackfillCursor.Set(float64(svc.cursor))
	}
	return err
}

func (svc *backfillPublisher) queueBatch(ctx context.Context, batchSize uint64) error {
	latestBlockHeader, err := svc.celoClient.Eth.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil
	}
	// Allow a buffer from the tip of the chain so we don't
	// start queuing for backfill blocks that are already
	// being processed at the tip.
	maxBlock := latestBlockHeader.Number.Uint64() - svc.tipBuffer

	queued := uint64(0)
	searchWindow := batchSize * 2

	for searchStart := svc.cursor; searchStart < maxBlock && queued < batchSize; searchStart += searchWindow {
		searchEnd := searchStart + searchWindow
		if searchEnd > maxBlock {
			searchEnd = maxBlock
		}

		batch, err := svc.db.GetBlocksBatch(ctx, searchStart, searchWindow)
		if err != nil {
			return err
		}

		for block := searchStart; block < searchEnd && queued < batchSize; block++ {
			if !batch[block] {
				queued += 1
				err := svc.db.RPush(ctx, rdb.BackfillQueue, block).Err()
				if err != nil {
					return err
				}
			}
		}
	}

	if queued > 0 {
		svc.logger.Info("Queued backfill blocks", "cursor", svc.cursor, "blocks", queued)
	} else {
		svc.logger.Info("No backfill blocks found", "cursor", svc.cursor, "maxBlock", maxBlock)
	}

	return nil
}

func (svc *backfillPublisher) start(ctx context.Context) error {
	svc.logger.Info("Starting backfill process")
	for {
		queueSize, err := svc.db.LLen(ctx, rdb.BackfillQueue).Uint64()
		if err != nil {
			return err
		}

		if queueSize < svc.batchSize {
			if err := svc.updateCursor(ctx, svc.batchSize); err != nil {
				return err
			}
			if err := svc.queueBatch(ctx, svc.batchSize); err != nil {
				return err
			}
		}

		queueSize, err = svc.db.LLen(ctx, rdb.BackfillQueue).Uint64()
		if err != nil {
			return err
		}
		metrics.BlockQueueSize.WithLabelValues(rdb.BackfillQueue).Set(float64(queueSize))

		blocksIndexedQueueSize, err := svc.db.HLen(ctx, rdb.BlocksMap).Uint64()
		if err != nil {
			return err
		}
		metrics.BlockQueueSize.WithLabelValues(rdb.BlocksMap).Set(float64(blocksIndexedQueueSize))

		time.Sleep(svc.sleepInterval)
	}
}
