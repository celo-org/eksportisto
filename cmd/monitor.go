package cmd

import (
	"context"
	"os"

	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/monitor"
	"github.com/celo-org/kliento/utils/service"
	"github.com/prometheus/common/log"
	"golang.org/x/sync/errgroup"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Publish metrics extracted from blocks",
	Long:  `Connects to a node and publishes extracted metrics for every block.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := service.WithExitSignals(context.Background())
		group, ctx := errgroup.WithContext(ctx)

		group.Go(func() error { return monitor.Start(ctx) })
		group.Go(func() error { return metrics.Start(ctx) })

		err := group.Wait()
		if err != nil && err != context.Canceled {
			log.Error("Error while running: ", err)
			os.Exit(1)
		}
	},
}

func init() {
	viper.SetDefault("indexer.destination", "stdout")
	viper.SetDefault("indexer.source", "tip")
	viper.SetDefault("indexer.mode", "metrics")
}
