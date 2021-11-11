package cmd

import (
	"context"
	"os"

	"github.com/celo-org/eksportisto/indexer"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/publisher"
	"github.com/celo-org/kliento/utils/service"
	"github.com/prometheus/common/log"
	"golang.org/x/sync/errgroup"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var publisherCmd = &cobra.Command{
	Use:   "publisher",
	Short: "Services responsible for publishing blocks for processing",
	Long: `The service will handle both getting a stream of new block 
headers from a node, and backfilling data when needed. It will 
also ensure that blocks are retried when errors occur.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := service.WithExitSignals(context.Background())
		group, ctx := errgroup.WithContext(ctx)

		group.Go(func() error { return publisher.Start(ctx) })
		group.Go(func() error { return metrics.Start(ctx) })

		err := group.Wait()
		if err != nil && err != context.Canceled {
			log.Error("Error while running: ", err)
			os.Exit(1)
		}
	},
}

func init() {
	viper.SetDefault("publisher.backfill", false)
	viper.SetDefault("publisher.chainTip.queue", indexer.Tip)
}
