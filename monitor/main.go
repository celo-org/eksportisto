package monitor

import (
	"context"
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/celo-org/eksportisto/db"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/client"
	"github.com/celo-org/kliento/contracts"
	kliento_mon "github.com/celo-org/kliento/monitor"
	"github.com/celo-org/kliento/wrappers"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	NodeUri string
}

var EpochSize = uint64(17280)
var BlocksPerHour = uint64(720)
var TipGap = big.NewInt(50)

// var EpochSize = uint64(1)
// var BlocksPerHour = uint64(1)

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

func blockProcessor(ctx context.Context, headers <-chan *types.Header, cc *client.CeloClient, logger log.Logger, dbWriter db.RosettaDBWriter) error {
	registry, err := wrappers.NewRegistry(cc)
	if err != nil {
		return err
	}

	var election *contracts.Election
	var electionAddress common.Address

	var epochRewards *contracts.EpochRewards
	var epochRewardsAddress common.Address

	var exchange *contracts.Exchange
	var exchangeAddress common.Address

	var goldToken *contracts.GoldToken
	var goldTokenAddress common.Address

	var governance *contracts.Governance
	var governanceAddress common.Address

	var lockedGold *contracts.LockedGold
	var lockedGoldAddress common.Address

	var reserve *contracts.Reserve
	var reserveAddress common.Address

	var stableToken *contracts.StableToken
	var stableTokenAddress common.Address

	var validators *contracts.Validators
	var validatorsAddress common.Address

	var h *types.Header

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case h = <-headers:
		}
		logHeader(logger, h)

		start := time.Now()
		logger = logger.New("blockTimestamp", time.Unix(int64(h.Time), 0).Format(time.RFC3339), "blockNumber", h.Number.Uint64())

		header, err := cc.Eth.HeaderAndTxnHashesByHash(ctx, h.Hash())

		if err != nil {
			return err
		}

		opts := &bind.CallOpts{
			BlockNumber: h.Number,
			Context:     ctx,
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

		if (epochRewardsAddress == common.Address{}) {
			epochRewardsAddress, err = registry.GetAddressForString(opts, "EpochRewards")
			if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}

			epochRewards, err = contracts.NewEpochRewards(epochRewardsAddress, cc.Eth)
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

		if (goldTokenAddress == common.Address{}) {
			goldTokenAddress, err = registry.GetAddressForString(opts, "GoldToken")
			if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}

			goldToken, err = contracts.NewGoldToken(goldTokenAddress, cc.Eth)
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

		if (lockedGoldAddress == common.Address{}) {
			lockedGoldAddress, err = registry.GetAddressForString(opts, "LockedGold")
			if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}
			lockedGold, err = contracts.NewLockedGold(lockedGoldAddress, cc.Eth)

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

		if (validatorsAddress == common.Address{}) {
			validatorsAddress, err = registry.GetAddressForString(opts, "Validators")
			if err == client.ErrContractNotDeployed || err == wrappers.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}

			validators, err = contracts.NewValidators(validatorsAddress, cc.Eth)
			if err != nil {
				return err
			}
		}

		electionProcessor := NewElectionProcessor(ctx, logger, electionAddress, election)
		epochRewardsProcessor := NewEpochRewardsProcessor(ctx, logger, epochRewardsAddress, epochRewards)
		goldTokenProcessor := NewGoldTokenProcessor(ctx, logger, goldTokenAddress, goldToken)
		governanceProcessor := NewGovernanceProcessor(ctx, logger, governanceAddress, governance)
		lockedGoldProcessor := NewLockedGoldProcessor(ctx, logger, lockedGoldAddress, lockedGold)
		reserveProcessor := NewReserveProcessor(ctx, logger, reserveAddress, reserve)
		stabilityProcessor := NewStabilityProcessor(ctx, logger, exchange, reserve)
		stableTokenProcessor := NewStableTokenProcessor(ctx, logger, stableTokenAddress, stableToken)
		validatorsProcessor := NewValidatorsProcessor(ctx, logger, validatorsAddress, validators)

		g, ctxProcessor := errgroup.WithContext(opts.Context)

		opts = &bind.CallOpts{
			BlockNumber: opts.BlockNumber,
			Context:     ctxProcessor,
		}

		if utils.ShouldSample(h.Number.Uint64(), BlocksPerHour) {
			g.Go(func() error { return goldTokenProcessor.ObserveState(opts) })
			g.Go(func() error { return reserveProcessor.ObserveState(opts) })
			g.Go(func() error { return stableTokenProcessor.ObserveState(opts) })
		}

		if utils.ShouldSample(h.Number.Uint64(), EpochSize) {
			g.Go(func() error { return electionProcessor.ObserveState(opts) })
			g.Go(func() error { return epochRewardsProcessor.ObserveState(opts) })
			g.Go(func() error { return lockedGoldProcessor.ObserveState(opts) })
			g.Go(func() error { return stabilityProcessor.ObserveState(opts) })

			filterLogs, err := cc.Eth.FilterLogs(ctx, ethereum.FilterQuery{
				FromBlock: h.Number,
				ToBlock:   h.Number,
			})
			if err != nil {
				return err
			}

			for _, epochLog := range filterLogs {
				electionProcessor.HandleLog(&epochLog)
				epochRewardsProcessor.HandleLog(&epochLog)
				governanceProcessor.HandleLog(&epochLog)
				validatorsProcessor.HandleLog(&epochLog)
				goldTokenProcessor.HandleLog(&epochLog)
			}
		}

		err = g.Wait()
		if err != nil {
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
				electionProcessor.HandleLog(eventLog)
				epochRewardsProcessor.HandleLog(eventLog)
				governanceProcessor.HandleLog(eventLog)
				validatorsProcessor.HandleLog(eventLog)
			}
		}

		if err := dbWriter.ApplyChanges(ctx, h.Number); err != nil {
			return err
		}

		if isTipMode(ctx, cc, h.Number) {
			stabilityProcessor.ObserveMetric(opts)
			goldTokenProcessor.ObserveMetric(opts)
			stableTokenProcessor.ObserveMetric(opts)
		}
		metrics.LastBlockProcessed.Set(float64(h.Number.Int64()))
		elapsed := time.Since(start)
		logger.Debug("STATS", "elapsed", elapsed)
	}
}

func isTipMode(ctx context.Context, cc *client.CeloClient, currentBlockNumber *big.Int) bool {
	latestHeader, err := cc.Eth.HeaderByNumber(ctx, nil)
	if err != nil {
		return false
	}
	return new(big.Int).Sub(latestHeader.Number, currentBlockNumber).Cmp(TipGap) < 0
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
