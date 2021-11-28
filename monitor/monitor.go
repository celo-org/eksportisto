package monitor

import (
	"context"
	"os"
	"time"

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
        // A single slot for blocks waiting to be processed.
	blockNumbers := make(chan uint64, 1)

	logger.Info("Latest block number.", "startBlock", startBlock)

	worker, err := indexer.NewWorker(ctx)
	if err != nil {
		return err
	}

        // Kliento's header listener -- enqueues headers to the 'headers' channel.
	group.Go(func() error { return kliento_mon.HeaderListener(ctx, headers, client, logger, startBlock) })
        // Dequeue headers from the 'headers' channel and enqueue block numbers to the 'blockNumbers' channel.
	group.Go(func() error {
		for {
			var header *types.Header
			select {
			case <-ctx.Done():
				return ctx.Err()
			case header = <-headers:
			}

			logger.Info("Header received.", "number", header.Number.Uint64())

			select {
			case blockNumbers <- header.Number.Uint64():
			default:
                          // Drop blocks that arrive while the previous one is being processed.
			}
		}
	})
        // Dequeue block numbers from the 'blockNumbers' channel and process these blocks.
	group.Go(func() error {
		for {
			var blockNumber uint64
			select {
			case <-ctx.Done():
				return ctx.Err()
			case blockNumber = <-blockNumbers:
			}

			logger.Info("Processing block", "number", blockNumber)

			blockHandler, err := worker.NewBlockHandler(blockNumber)
			if err != nil {
				return err
			}
			blockProcessingStartTime := time.Now()
			err = blockHandler.Run(ctx)
			blockProcessingDuration := time.Since(blockProcessingStartTime)
			metrics.ProcessBlockDuration.Observe(float64(blockProcessingDuration) / float64(time.Second))

			if err != nil {
				logger.Error("Failed to process block.", "number", blockNumber, "err", err)
			} else {
				logger.Info("Finished processing block.", "number", blockNumber, "processing_time", blockProcessingDuration)
				metrics.LastBlockProcessed.WithLabelValues("monitor").Set(float64(blockNumber))
			}
		}
	})

	return group.Wait()
}
