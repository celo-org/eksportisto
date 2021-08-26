package publisher

import (
	"context"
	"os"
	"time"

	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/rdb"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

type backfillService struct {
	db            *rdb.RedisDB
	sleepInterval time.Duration
	logger        log.Logger
	cursor        uint64
	batchSize     uint64
}

func newBackfill(ctx context.Context) (*backfillService, error) {
	db := rdb.NewRedisDatabase()
	cursor, err := db.Get(ctx, rdb.BackfillCursor).Uint64()
	if err == redis.Nil {
		cursor = viper.GetUint64("publisher.backfill.startBlock")
	} else if err != nil {
		return nil, err
	}

	handler := log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.JSONFormat()))
	logger := log.New()
	logger.SetHandler(handler)

	return &backfillService{
		db:            db,
		logger:        logger,
		cursor:        cursor,
		batchSize:     viper.GetUint64("publisher.backfill.batchSize"),
		sleepInterval: viper.GetDuration("publisher.backfill.sleepIntervalMilliseconds") * time.Millisecond,
	}, nil
}

func (svc *backfillService) isQueueEmpty(ctx context.Context) (bool, error) {
	length, err := svc.db.LLen(ctx, rdb.BackfillQueue).Result()
	return length == 0, err
}

func (svc *backfillService) updateCursor(ctx context.Context) error {
	batch, err := svc.db.GetBlocksBatch(ctx, svc.cursor, svc.batchSize*2)
	if err != nil {
		return err
	}

	for i := svc.cursor; i < svc.cursor+svc.batchSize; i++ {
		if batch[i] {
			svc.cursor = i + 1
		} else {
			break
		}
	}

	svc.logger.Info("Cursor updated", "cursor", svc.cursor)
	return svc.db.Set(ctx, rdb.BackfillCursor, svc.cursor, 0).Err()
}

func (svc *backfillService) queueBatch(ctx context.Context) error {
	batch, err := svc.db.GetBlocksBatch(ctx, svc.cursor, svc.batchSize*2)
	if err != nil {
		return err
	}

	queued := uint64(0)

	for block := svc.cursor; block < svc.cursor+2*svc.batchSize; block++ {
		if !batch[block] {
			queued += 1
			err := svc.db.RPush(ctx, rdb.BackfillQueue, block).Err()
			if err != nil {
				return err
			}
		}

		if queued >= svc.batchSize {
			return nil
		}
	}

	svc.logger.Info("Queued backfill blocks", "cursor", svc.cursor)
	return nil
}

func (svc *backfillService) start(ctx context.Context) error {
	svc.logger.Info("Starting backfill process")
	for {
		isEmpty, err := svc.isQueueEmpty(ctx)
		if err != nil {
			return err
		}
		if isEmpty {
			if err := svc.updateCursor(ctx); err != nil {
				return err
			}
			if err := svc.queueBatch(ctx); err != nil {
				return err
			}
		}

		queueSize, err := svc.db.LLen(ctx, rdb.BackfillQueue).Uint64()
		if err != nil {
			return err
		}
		metrics.BlockQueueSize.WithLabelValues(rdb.BackfillQueue).Set(float64(queueSize))

		time.Sleep(svc.sleepInterval)
	}
}
