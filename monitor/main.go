package monitor

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/celo-org/eksportisto/db"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/kliento/client"
	"github.com/celo-org/kliento/contracts"
	kliento_mon "github.com/celo-org/kliento/monitor"
	"github.com/celo-org/kliento/wrappers"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
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

	var election *contracts.Election
	var electionAddress common.Address

	var governance *contracts.Governance
	var governanceAddress common.Address

	var stableToken *contracts.StableToken
	var stableTokenAddress common.Address

	var reserve *contracts.Reserve
	var reserveAddress common.Address

	var exchange *contracts.Exchange
	var exchangeAddress common.Address

	var h *types.Header

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case h = <-headers:
		}

		logHeader(logger, h)

		logger = logger.New("blockTimestamp", time.Unix(int64(h.Time), 0).Format(time.RFC3339), "blockNumber", h.Number)

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
		// Todo: Right now we  are assuming that core contract addresses do not change which allows us to avoid having to check the registry
		if (electionAddress == common.Address{}) {
			electionAddress, err = registry.GetAddressForString(opts, "Election")
			if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}

			election, err = contracts.NewElection(electionAddress, cc.Eth)
			if err != nil {
				return err
			}
		}

		if (exchangeAddress == common.Address{}) {
			exchangeAddress, err = registry.GetAddressForString(opts, "Exchange")
			if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}

			exchange, err = contracts.NewExchange(exchangeAddress, cc.Eth)
			if err != nil {
				return err
			}
		}

		if (reserveAddress == common.Address{}) {
			reserveAddress, err = registry.GetAddressForString(opts, "Reserve")
			if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}
			reserve, err = contracts.NewReserve(reserveAddress, cc.Eth)

			if err != nil {
				return err
			}
		}

		if (stableTokenAddress == common.Address{}) {
			stableTokenAddress, err = registry.GetAddressForString(opts, "StableToken")
			if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}

			stableToken, err = contracts.NewStableToken(stableTokenAddress, cc.Eth)
			if err != nil {
				return err
			}
		}

		if (governanceAddress == common.Address{}) {
			governanceAddress, err = registry.GetAddressForString(opts, "Governance")
			if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}

			governance, err = contracts.NewGovernance(governanceAddress, cc.Eth)
			if err != nil {
				return err
			}
		}

		_ = NewStableTokenProcessor(ctx, logger, stableTokenAddress, stableToken)
		electionProcessor := NewElectionProcessor(ctx, logger, electionAddress, election)
		governanceProcessor := NewGovernanceProcessor(ctx, logger, governanceAddress, governance)
		_ = NewStabilityProcessor(ctx, logger, exchange, reserve)

		// if err := stableTokenProcessor.ObserveState(opts); err != nil {
		// 	return err
		// }
		// if err := stabilityProcessor.ObserveState(opts); err != nil {
		// 	return err
		// }

		for _, txHash := range header.Transactions {
			receipt, err := cc.Eth.TransactionReceipt(ctx, txHash)
			if err != nil {
				return err
			}

			txLogger := getTxLogger(logger, receipt, header)
			logTransaction(txLogger)
			for _, eventLog := range receipt.Logs {
				electionProcessor.HandleLog(eventLog)
				governanceProcessor.HandleLog(eventLog)
			}
		}

		if err := dbWriter.ApplyChanges(ctx, h.Number); err != nil {
			return err
		}

		metrics.LastBlockProcessed.Set(float64(h.Number.Int64()))
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
