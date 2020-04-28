package monitor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/kliento/client"
	"github.com/celo-org/kliento/contracts"
	kliento_mon "github.com/celo-org/kliento/monitor"
	"github.com/celo-org/kliento/utils/bn"
	"github.com/celo-org/kliento/wrappers"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	NodeUri string
}

func Start(ctx context.Context, cfg *Config) error {
	logger := log.New("module", "monitor")
	cc, err := client.Dial(cfg.NodeUri)
	if err != nil {
		return err
	}

	headers := make(chan *types.Header, 10)

	logger.Info("Starting monitor")
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return kliento_mon.HeaderListener(ctx, headers, cc, logger, bn.Big1) })
	g.Go(func() error { return blockProcessor(ctx, headers, cc, logger) })
	return g.Wait()
}

func blockProcessor(ctx context.Context, headers <-chan *types.Header, cc *client.CeloClient, logger log.Logger) error {
	logger = logger.New("pipe", "processor")

	registry, err := wrappers.NewRegistry(cc)
	if err != nil {
		return err
	}

	var h *types.Header

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case h = <-headers:
		}

		logger.Info("Got new header")

		marshalledHeader, err := json.Marshal(h)
		if err != nil {
			return err
		}
		fmt.Println(string(marshalledHeader))
		opts := &bind.CallOpts{
			BlockNumber: h.Number,
			Context:     ctx,
		}

		stAddress, err := registry.GetAddressForString(opts, "StableToken")
		if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
			continue
		} else if err != nil {
			return err
		}

		stableToken, err := contracts.NewStableToken(stAddress, cc.Eth)
		if err != nil {
			return err
		}

		totalSupply, err := stableToken.TotalSupply(opts)
		metrics.TotalCUSDSupply.Observe(float64(totalSupply.Uint64()))

	}
}
