package monitor

import (
	"context"
	"os"

	"github.com/celo-org/celo-blockchain/core/types"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/indexer"
	"github.com/celo-org/eksportisto/metrics"
	kliento_client "github.com/celo-org/kliento/client"
	kliento_mon "github.com/celo-org/kliento/monitor"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

func setupLogger() log.Logger {
	handler := log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.JSONFormat()))
	logger := log.New()
	logger.SetHandler(handler)

	return logger
}

func Start(ctx context.Context) error {
	logger := setupLogger()
	logger.Info("Starting monitor process")
	group, ctx := errgroup.WithContext(ctx)

	client, err := kliento_client.Dial(viper.GetString("celoNodeURI"))
	if err != nil {
		return err
	}

	latestHeader, err := client.Eth.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}

	startBlock := latestHeader.Number
	headers := make(chan *types.Header)

	logger.Info("Latest block number.", "startBlock", startBlock)

	worker, err := indexer.NewWorker(ctx)
	if err != nil {
		return err
	}

	group.Go(func() error { return kliento_mon.HeaderListener(ctx, headers, client, logger, startBlock) })
	group.Go(func() error {
		for {
			var header *types.Header
			select {
			case <-ctx.Done():
				return ctx.Err()
			case header = <-headers:
			}

			blockNumber := header.Number.Uint64()

			logger.Info("Header received.", "number", blockNumber)

			blockHandler, err := worker.NewBlockHandler(blockNumber)
			if err != nil {
				return err
			}
			err = blockHandler.Run(ctx)
			if err != nil {
				logger.Error("Failed to process block.", "number", blockNumber, "err", err)
			} else {
				metrics.LastBlockProcessed.WithLabelValues("monitor").Set(float64(blockNumber))
			}
		}
	})

	return group.Wait()
}
