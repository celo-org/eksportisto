package main

import (
	"context"
	"flag"
	"os"
	"os/user"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/celo-org/eksportisto/monitor"
	"github.com/celo-org/eksportisto/server"
	"github.com/celo-org/kliento/utils/service"

	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/sync/errgroup"
)

func main() {
	var httpConfig server.HttpServerConfig
	flag.UintVar(&httpConfig.Port, "port", 8080, "Listening port for http server")
	flag.StringVar(&httpConfig.Interface, "address", "", "Listening address for http server")
	flag.DurationVar(&httpConfig.RequestTimeout, "reqTimeout", 25*time.Second, "Timeout when serving a request")

	var monitorConfig monitor.Config
	flag.StringVar(&monitorConfig.NodeUri, "nodeUri", "http://localhost:8545", "Connection string for celo-blockchain node")
	flag.StringVar(&monitorConfig.DataDir, "datadir", filepath.Join(homeDir(), ".eksportisto"), "Sqlite data directory, will be created if it doesn't exist")
	flag.StringVar(&monitorConfig.SensitiveAccountsFilePath, "sensitiveAccounts", "", "Sensitive accounts JSON file")

	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

	flag.Parse()

	// TODO Validate parameters

	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))

	ctx := service.WithExitSignals(context.Background())
	group, ctx := errgroup.WithContext(ctx)

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Error("Error while profiling", "err", err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	group.Go(func() error { return monitor.Start(ctx, &monitorConfig) })
	group.Go(func() error { return server.Start(ctx, &httpConfig) })

	err := group.Wait()
	if err != nil && err != context.Canceled {
		log.Error("Error while running", "err", err)
		os.Exit(1)
	}
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
