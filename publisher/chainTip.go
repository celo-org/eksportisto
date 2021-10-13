package publisher

import (
	"context"
	"os"

	"github.com/celo-org/celo-blockchain/core/types"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/rdb"
	"github.com/celo-org/kliento/client"
	kliento_mon "github.com/celo-org/kliento/monitor"
	"github.com/spf13/viper"

	"golang.org/x/sync/errgroup"
)

type chainTipPublisher struct {
	celoClient *client.CeloClient
	db         *rdb.RedisDB
	logger     log.Logger
}

func newChainTipPublisher(_ context.Context) (publisher, error) {
	if !viper.GetBool("publisher.chainTip.enabled") {
		return nil, nil
	}

	db := rdb.NewRedisDatabase()
	celoClient, err := client.Dial(viper.GetString("celoNodeURI"))
	if err != nil {
		return nil, err
	}

	handler := log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.JSONFormat()))
	logger := log.New()
	logger.SetHandler(handler)

	return &chainTipPublisher{celoClient, db, logger}, nil
}

func (svc *chainTipPublisher) start(ctx context.Context) error {
	svc.logger.Info("Starting chainTipPublisher process")
	group, ctx := errgroup.WithContext(ctx)

	latestHeader, err := svc.celoClient.Eth.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}

	startBlock := latestHeader.Number
	headers := make(chan *types.Header, 10)

	group.Go(func() error { return kliento_mon.HeaderListener(ctx, headers, svc.celoClient, svc.logger, startBlock) })
	group.Go(func() error {
		for {
			var header *types.Header
			select {
			case <-ctx.Done():
				return ctx.Err()
			case header = <-headers:
			}

			err := svc.db.RPush(ctx, rdb.TipQueue, header.Number.Uint64()).Err()
			if err != nil {
				return err
			}
			svc.logger.Info("Queued block at tip", "number", header.Number.Uint64())

			queueSize, err := svc.db.LLen(ctx, rdb.TipQueue).Uint64()
			if err != nil {
				return err
			}
			metrics.BlockQueueSize.WithLabelValues(rdb.TipQueue).Set(float64(queueSize))
		}
	})

	return group.Wait()
}
