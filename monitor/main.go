package monitor

import (
	"context"
	"os"
	"os/user"
	"path/filepath"

	"github.com/celo-org/eksportisto/db"
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
	handler := log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.JSONFormat()))
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

// func getElectionAddrAndContract(cc *client.CeloClient, opts *bind.CallOpts, registry *wrappers.RegistryWrapper) (common.Address, contracts.Election, error) {
// 	electionAddress, err := registry.GetAddressForString(opts, "Election")
// 	if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
// 		return nil, nil, nil
// 	} else if err != nil {
// 		return nil, nil, err
// 	}

// 	election, err := contracts.NewElection(electionAddress, cc.Eth)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	return electionAddress, election, nil
// }

func blockProcessor(ctx context.Context, headers <-chan *types.Header, cc *client.CeloClient, logger log.Logger, dbWriter db.RosettaDBWriter) error {
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

		logHeader(logger, h)

		opts := &bind.CallOpts{
			BlockNumber: h.Number,
			Context:     ctx,
		}

		header, err := cc.Eth.HeaderAndTxnHashesByHash(ctx, h.Hash())
		if err != nil {
			return err
		}

		// Todo: Use Rosetta's db to detect election contract address changes mid block
		// Todo: Right now this assumes that the only interesting events happen after all the core contracts are available
		electionAddress, err := registry.GetAddressForString(opts, "Election")
		if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
			return nil
		} else if err != nil {
			return err
		}

		election, err := contracts.NewElection(electionAddress, cc.Eth)
		if err != nil {
			return err
		}

		stAddress, err := registry.GetAddressForString(opts, "StableToken")
		if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
			return nil
		} else if err != nil {
			return err
		}

		stableToken, err := contracts.NewStableToken(stAddress, cc.Eth)
		if err != nil {
			return err
		}

		if err := stableTokenStateViewCalls(ctx, logger, opts, stableToken); err != nil {
			return err
		}

		for _, txHash := range header.Transactions {
			receipt, err := cc.Eth.TransactionReceipt(ctx, txHash)
			if err != nil {
				return err
			}

			txLogger := getTxLogger(logger, receipt, header)
			logTransaction(txLogger)
			for _, eventLog := range receipt.Logs {
				electionLogProcessor(ctx, txLogger, electionAddress, election, eventLog)
			}
		}

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
