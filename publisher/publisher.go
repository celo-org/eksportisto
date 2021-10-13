package publisher

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type publisher interface {
	start(context.Context) error
}

type publisherFactory = func(context.Context) (publisher, error)

var factories []publisherFactory

func init() {
	factories = []publisherFactory{
		newBackfillPublisher,
		newChainTipPublisher,
		newManualPublisher,
	}
}

func Start(ctx context.Context) error {
	group, ctx := errgroup.WithContext(ctx)

	for _, factory := range factories {
		publisher, err := factory(ctx)
		if err != nil {
			return err
		}
		if publisher != nil {
			group.Go(func() error { return publisher.start(ctx) })
		}
	}

	return group.Wait()
}
