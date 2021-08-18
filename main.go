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
	"github.com/celo-org/eksportisto/queue"
	"github.com/celo-org/eksportisto/server"
	"github.com/celo-org/kliento/utils/service"

	"github.com/celo-org/celo-blockchain/log"
	"golang.org/x/sync/errgroup"
)

func main() {
	var httpConfig server.HttpServerConfig
	flag.UintVar(&httpConfig.Port, "port", 8080, "Listening port for http server")
	flag.StringVar(&httpConfig.Interface, "address", "", "Listening address for http server")
	flag.DurationVar(&httpConfig.RequestTimeout, "reqTimeout", 25*time.Second, "Timeout when serving a request")

	var monitorConfig monitor.Config
	flag.StringVar(&monitorConfig.NodeUri, "nodeUri", "ws://localhost:8546", "Connection string for celo-blockchain node")
	flag.StringVar(&monitorConfig.DataDir, "datadir", filepath.Join(homeDir(), ".eksportisto"), "Sqlite data directory, will be created if it doesn't exist")
	flag.StringVar(&monitorConfig.SensitiveAccountsFilePath, "sensitiveAccounts", "", "Sensitive accounts JSON file")
	flag.StringVar(&monitorConfig.FromBlock, "from-block", "", "Begin indexing the chain from this block. Can pass a number or \"latest\"")

	var outputType = flag.String("output", "stdout", "Where to output to: stdout / pubsub")
	var pubSubProjectID = flag.String("pubsub-project-id", "", "Project ID for the pubsub output")
	var pubSubTopicID = flag.String("pubsub-topic-id", "", "Topic ID for the pubsub output")
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

	flag.Parse()

	// TODO Validate parameters
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))

	if *outputType == "stdout" {
		monitorConfig.Output = os.Stdout
		log.Info("Using stdout output")
	} else if *outputType == "pubsub" {
		if *pubSubProjectID == "" || *pubSubTopicID == "" {
			log.Error("--pubsub-topic-id and --pubsub-project-id must be provided")
		}
		queueWriter, err := queue.NewQueueWriter(*pubSubProjectID, *pubSubTopicID)
		if err != nil {
			log.Error("queue.NewQueueWriter", "err", err)
		}
		monitorConfig.Output = queueWriter
		log.Info("Using pubsub output")
	} else {
		log.Error("output must be one of: stdout, pubsub")
	}

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
