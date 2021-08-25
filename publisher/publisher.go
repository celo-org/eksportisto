package publisher

import (
	"context"

	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

func Start(ctx context.Context) error {
	group, ctx := errgroup.WithContext(ctx)

	if viper.GetBool("publisher.backfill.enabled") {
		backfill, err := newBackfill(ctx)
		if err != nil {
			return err
		}
		group.Go(func() error { return backfill.start(ctx) })
	}

	if viper.GetBool("publisher.chainFollower.enabled") {
		chainFollower, err := newChainFollower()
		if err != nil {
			return err
		}
		group.Go(func() error { return chainFollower.start(ctx) })
	}

	return group.Wait()
}
