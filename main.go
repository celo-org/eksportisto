package main

import (
	"context"
	"flag"
	"os"
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

	flag.Parse()

	// TODO Validate parameters

	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))

	ctx := service.WithExitSignals(context.Background())
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error { return monitor.Start(ctx, &monitorConfig) })
	group.Go(func() error { return server.Start(ctx, &httpConfig) })

	err := group.Wait()
	if err != nil && err != context.Canceled {
		log.Error("Error while running", "err", err)
		os.Exit(1)
	}
}
