package indexer

import (
	"context"
	"os"
	"time"

	"github.com/celo-org/celo-blockchain/log"
	"golang.org/x/sync/errgroup"

	"github.com/spf13/viper"
)

type indexer struct {
	workers       int
	logger        log.Logger
	sleepInterval time.Duration
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

// newIndexer initializes the struct responsible for the indexing process.
// It reads some configuration and sets up logging
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
