package indexer

import (
	"context"
)

// Start is the entry point of the indexer service.
// It sets up and starts a worker
func Start(ctx context.Context) error {
	worker, err := NewWorker(ctx)
	if err != nil {
		return err
	}

	return worker.start(ctx)
}
