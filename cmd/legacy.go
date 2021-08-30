package cmd

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/legacy/monitor"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/kliento/utils/service"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var (
	httpConfig    metrics.HttpServerConfig
	monitorConfig monitor.Config
	cpuprofile    string
)

var legacyCmd = &cobra.Command{
	Use:   "legacy",
	Short: "Start the legacy eksportisto",
	Long: `Start the legacy version of eksportisto.
Used during development of 2.0 to compare the data extraction.`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO Validate parameters
		log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))

		ctx := service.WithExitSignals(context.Background())
		group, ctx := errgroup.WithContext(ctx)

		if cpuprofile != "" {
			f, err := os.Create(cpuprofile)
			if err != nil {
				log.Error("Error while profiling", "err", err)
			}
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		}

		group.Go(func() error { return monitor.Start(ctx, &monitorConfig) })
		group.Go(func() error { return metrics.StartWithConfig(ctx, &httpConfig) })

		err := group.Wait()
		if err != nil && err != context.Canceled {
			log.Error("Error while running", "err", err)
			os.Exit(1)
		}
	},
}

func init() {
	legacyCmd.Flags().UintVar(&httpConfig.Port, "port", 8080, "Listening port for http server")
	legacyCmd.Flags().StringVar(&httpConfig.Interface, "address", "", "Listening address for http server")
	legacyCmd.Flags().DurationVar(&httpConfig.RequestTimeout, "reqTimeout", 25*time.Second, "Timeout when serving a request")

	legacyCmd.Flags().StringVar(&monitorConfig.NodeUri, "nodeUri", "ws://localhost:8546", "Connection string for celo-blockchain node")
	legacyCmd.Flags().StringVar(&monitorConfig.DataDir, "datadir", filepath.Join(homeDir(), ".eksportisto"), "Sqlite data directory, will be created if it doesn't exist")
	legacyCmd.Flags().StringVar(&monitorConfig.SensitiveAccountsFilePath, "sensitiveAccounts", "", "Sensitive accounts JSON file")
	legacyCmd.Flags().StringVar(&monitorConfig.FromBlock, "from-block", "", "Begin indexing the chain from this block. Can pass a number or \"latest\"")
	legacyCmd.Flags().StringVar(&cpuprofile, "cpuprofile", "", "write cpu profile to file")

}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}
