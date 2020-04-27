package main

import (
	"flag"
	"math/rand"
	"os"
	"time"

	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/server"
	"github.com/docker/distribution/context"
	"github.com/ethereum/go-ethereum/log"
)

var (
	addr              = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
	uniformDomain     = flag.Float64("uniform.domain", 0.0002, "The domain for the uniform distribution.")
	normDomain        = flag.Float64("normal.domain", 0.0002, "The domain for the normal distribution.")
	normMean          = flag.Float64("normal.mean", 0.00001, "The mean for the normal distribution.")
	oscillationPeriod = flag.Duration("oscillation-period", 10*time.Minute, "The duration of the rate oscillation period.")
)

func main() {

	flag.Parse()

	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))

	ctx := context.Background()

	go func() {
		for {
			v := rand.ExpFloat64() / 1e6
			metrics.TotalCUSDSupply.Observe(v)
			time.Sleep(5 * time.Second)
		}
	}()

	server.Start(ctx, &server.HttpServerConfig{
		Port:           8080,
		Interface:      "localhost",
		RequestTimeout: 25 * time.Second,
	})
}
