package indexer

import (
	"context"
)

type indexer struct {
}

// Start is the entry point of the indexer service.
// It sets up and starts a worker
func Start(ctx context.Context) error {
	worker, err := newWorker(ctx)
	if err != nil {
		return err
	}

	return worker.start(ctx)
}
