package cmd

import (
	"context"
	"os"

	"github.com/celo-org/eksportisto/indexer"
	"github.com/celo-org/eksportisto/server"
	"github.com/celo-org/kliento/utils/service"
	"github.com/prometheus/common/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var indexerCmd = &cobra.Command{
	Use:   "indexer",
	Short: "Worker that indexes block data",
	Long: `The indexer reads blocks from a few queues with
different priority and processes and stores them in a data store.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := service.WithExitSignals(context.Background())
		group, ctx := errgroup.WithContext(ctx)

		group.Go(func() error { return indexer.Start(ctx) })
		group.Go(func() error { return server.Start(ctx) })

		err := group.Wait()
		if err != nil && err != context.Canceled {
			log.Error("Error while running: ", err)
			os.Exit(1)
		}
	},
}

func init() {
	indexerCmd.PersistentFlags().String("indexer-source", "", "What queue to read blocks from")
}
