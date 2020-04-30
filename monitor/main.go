package monitor

import (
	"context"
	"os"
	"os/user"
	"path/filepath"

	"github.com/celo-org/eksportisto/db"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/kliento/client"
	"github.com/celo-org/kliento/contracts"
	kliento_mon "github.com/celo-org/kliento/monitor"
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
	handler := log.StreamHandler(os.Stdout, log.JSONFormat())
	logger := log.New()
	logger.SetHandler(handler)
	cc, err := client.Dial(cfg.NodeUri)
	if err != nil {
		return err
	}

	// Todo: Make this configurable
	datadir := filepath.Join(homeDir(), ".eksportisto")
	sqlitePath := filepath.Join(datadir, "state.db")
	store, err := db.NewSqliteDb(sqlitePath)
	startBlock, err := store.LastPersistedBlock(ctx)

	headers := make(chan *types.Header, 10)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return kliento_mon.HeaderListener(ctx, headers, cc, logger, startBlock) })
	g.Go(func() error { return blockProcessor(ctx, headers, cc, logger, store) })
	return g.Wait()
}

func blockProcessor(ctx context.Context, headers <-chan *types.Header, cc *client.CeloClient, logger log.Logger, dbWriter db.RosettaDBWriter) error {
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

		logger.Info("BlockHeader", "number", h.Number)

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
		logger.Info("t", "t", totalSupply)
		metrics.TotalCUSDSupply.Observe(float64(totalSupply.Uint64()))

		if err := dbWriter.ApplyChanges(ctx, h.Number); err != nil {
			return err
		}
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
