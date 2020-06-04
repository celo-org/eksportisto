package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/celo-org/eksportisto/db"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/client"
	"github.com/celo-org/kliento/client/debug"
	"github.com/celo-org/kliento/contracts"
	kliento_mon "github.com/celo-org/kliento/monitor"
	"github.com/celo-org/kliento/registry"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	NodeUri                   string
	DataDir                   string
	SensitiveAccountsFilePath string
}

var EpochSize = uint64(17280)   // 17280 = 12 * 60 * 24
var BlocksPerHour = uint64(720) // 720 = 12 * 60
var TipGap = big.NewInt(50)

func getSensitiveAccounts(filePath string) map[common.Address]string {
	if filePath == "" {
		return make(map[common.Address]string)
	}

	bz, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	var addresses map[common.Address]string
	err = json.Unmarshal(bz, &addresses)
	if err != nil {
		panic(err)
	}

	return addresses
}

func notifyFundsMoved(transfer debug.Transfer, url string) error {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(fmt.Sprintf(`{"from":"%s","to":"%s","amount":"%s"}`,
		transfer.From.Hex(), transfer.To.Hex(), transfer.Value.String()),
	)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("unable to notify, received status code %d", resp.StatusCode)
	}
	return nil
}

func getHeaderInformation(ctx context.Context, cc *client.CeloClient, h *types.Header) (*ethclient.HeaderAndTxnHashes, *types.Header, error) {
	var header *ethclient.HeaderAndTxnHashes
	var latestHeader *types.Header
	var byHashErr error
	var latestHeaderErr error

	var wg sync.WaitGroup
	wg.Add(2)
	go func(hash common.Hash) {
		header, byHashErr = cc.Eth.HeaderAndTxnHashesByHash(ctx, hash)
		wg.Done()
	}(h.Hash())
	go func() {
		latestHeader, latestHeaderErr = cc.Eth.HeaderByNumber(ctx, nil)
		wg.Done()
	}()
	wg.Wait()

	if byHashErr != nil {
		return nil, nil, byHashErr
	}
	if latestHeaderErr != nil {
		return nil, nil, latestHeaderErr
	}
	return header, latestHeader, nil
}

func Start(ctx context.Context, cfg *Config) error {
	handler := log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.JSONFormat()))
	logger := log.New()
	logger.SetHandler(handler)
	cc, err := client.Dial(cfg.NodeUri)
	if err != nil {
		return err
	}

	os.MkdirAll(cfg.DataDir, os.ModePerm)
	sqlitePath := filepath.Join(cfg.DataDir, "state.db")
	store, err := db.NewSqliteDb(sqlitePath)
	if err != nil {
		return err
	}
	startBlock, err := store.LastPersistedBlock(ctx)
	if err != nil {
		return err
	}

	headers := make(chan *types.Header, 10)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return kliento_mon.HeaderListener(ctx, headers, cc, logger, startBlock) })
	g.Go(func() error { return blockProcessor(ctx, headers, cc, logger, store, cfg) })
	return g.Wait()
}

func blockProcessor(ctx context.Context, headers <-chan *types.Header, cc *client.CeloClient, logger log.Logger, dbWriter db.RosettaDBWriter, cfg *Config) error {
	r, err := registry.New(cc)
	if err != nil {
		return err
	}

	var accounts *contracts.Accounts
	var accountsAddress common.Address

	var attestations *contracts.Attestations
	var attestationsAddress common.Address

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

	var sortedOracles *contracts.SortedOracles
	var sortedOraclesAddress common.Address

	var stableToken *contracts.StableToken
	var stableTokenAddress common.Address

	var validators *contracts.Validators
	var validatorsAddress common.Address

	var h *types.Header

	sensitiveAccounts := getSensitiveAccounts(cfg.SensitiveAccountsFilePath)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case h = <-headers:
		}
		logHeader(logger, h)

		start := time.Now()
		logger = logger.New("blockTimestamp", time.Unix(int64(h.Time), 0).Format(time.RFC3339), "blockNumber", h.Number.Uint64())

		header, latestHeader, err := getHeaderInformation(ctx, cc, h)
		if err != nil {
			return err
		}
		tipMode := isTipMode(latestHeader, h.Number)

		// Todo: Use Rosetta's db to detect election contract address changes mid block
		// Todo: Right now this assumes that the only interesting events happen after all the core contracts are available
		// Todo: Right now we  are assuming that core contract addresses do not change which allows us to avoid having to check the registry
		if (accountsAddress == common.Address{}) {
			accountsAddress, err = r.GetAddressFor(ctx, h.Number, registry.AccountsContractID)
			if err == client.ErrContractNotDeployed || err == registry.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}

			accounts, err = contracts.NewAccounts(accountsAddress, cc.Eth)
			if err != nil {
				return err
			}
		}

		if (attestationsAddress == common.Address{}) {
			attestationsAddress, err = r.GetAddressFor(ctx, h.Number, registry.AttestationsContractID)
			if err == client.ErrContractNotDeployed || err == registry.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}

			attestations, err = contracts.NewAttestations(attestationsAddress, cc.Eth)
			if err != nil {
				return err
			}
		}

		if (electionAddress == common.Address{}) {
			electionAddress, err = r.GetAddressFor(ctx, h.Number, registry.ElectionContractID)
			if err == client.ErrContractNotDeployed || err == registry.ErrRegistryNotDeployed {
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
			epochRewardsAddress, err = r.GetAddressFor(ctx, h.Number, registry.EpochRewardsContractID)
			if err == client.ErrContractNotDeployed || err == registry.ErrRegistryNotDeployed {
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
			exchangeAddress, err = r.GetAddressFor(ctx, h.Number, registry.ExchangeContractID)
			if err == client.ErrContractNotDeployed || err == registry.ErrRegistryNotDeployed {
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
			goldTokenAddress, err = r.GetAddressFor(ctx, h.Number, registry.GoldTokenContractID)
			if err == client.ErrContractNotDeployed || err == registry.ErrRegistryNotDeployed {
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
			governanceAddress, err = r.GetAddressFor(ctx, h.Number, registry.GovernanceContractID)
			if err == client.ErrContractNotDeployed || err == registry.ErrRegistryNotDeployed {
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
			lockedGoldAddress, err = r.GetAddressFor(ctx, h.Number, registry.LockedGoldContractID)
			if err == client.ErrContractNotDeployed || err == registry.ErrRegistryNotDeployed {
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
			reserveAddress, err = r.GetAddressFor(ctx, h.Number, registry.ReserveContractID)
			if err == client.ErrContractNotDeployed || err == registry.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}
			reserve, err = contracts.NewReserve(reserveAddress, cc.Eth)

			if err != nil {
				return err
			}
		}

		if (sortedOraclesAddress == common.Address{}) {
			sortedOraclesAddress, err = r.GetAddressFor(ctx, h.Number, registry.SortedOraclesContractID)
			if err == client.ErrContractNotDeployed || err == registry.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}

			sortedOracles, err = contracts.NewSortedOracles(sortedOraclesAddress, cc.Eth)
			if err != nil {
				return err
			}
		}

		if (stableTokenAddress == common.Address{}) {
			stableTokenAddress, err = r.GetAddressFor(ctx, h.Number, registry.StableTokenContractID)
			if err == client.ErrContractNotDeployed || err == registry.ErrRegistryNotDeployed {
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
			validatorsAddress, err = r.GetAddressFor(ctx, h.Number, registry.ValidatorsContractID)
			if err == client.ErrContractNotDeployed || err == registry.ErrRegistryNotDeployed {
				continue
			} else if err != nil {
				return err
			}

			validators, err = contracts.NewValidators(validatorsAddress, cc.Eth)
			if err != nil {
				return err
			}
		}

		accountsProcessor := NewAccountsProcessor(ctx, logger, accountsAddress, accounts)
		attestationsProcessor := NewAttestationsProcessor(ctx, logger, attestationsAddress, attestations)
		electionProcessor := NewElectionProcessor(ctx, logger, electionAddress, election)
		epochRewardsProcessor := NewEpochRewardsProcessor(ctx, logger, epochRewardsAddress, epochRewards)
		goldTokenProcessor := NewGoldTokenProcessor(ctx, logger, goldTokenAddress, goldToken)
		governanceProcessor := NewGovernanceProcessor(ctx, logger, governanceAddress, governance)
		lockedGoldProcessor := NewLockedGoldProcessor(ctx, logger, lockedGoldAddress, lockedGold)
		reserveProcessor := NewReserveProcessor(ctx, logger, reserveAddress, reserve)
		sortedOraclesProcessor := NewSortedOraclesProcessor(ctx, logger, sortedOraclesAddress, sortedOracles)
		stabilityProcessor := NewStabilityProcessor(ctx, logger, exchange, reserve)
		stableTokenProcessor := NewStableTokenProcessor(ctx, logger, stableTokenAddress, stableToken)
		validatorsProcessor := NewValidatorsProcessor(ctx, logger, validatorsAddress, validators)

		g, ctxProcessor := errgroup.WithContext(ctx)

		opts := &bind.CallOpts{
			BlockNumber: h.Number,
			Context:     ctxProcessor,
		}

		if tipMode {
			g.Go(func() error { return goldTokenProcessor.ObserveMetric(opts) })
			g.Go(func() error { return stableTokenProcessor.ObserveMetric(opts) })
			g.Go(func() error { return epochRewardsProcessor.ObserveMetric(opts) })
			g.Go(func() error { return sortedOraclesProcessor.ObserveMetric(opts, stableTokenAddress, h.Time) })
			g.Go(func() error { return stabilityProcessor.ObserveMetric(opts) })
		}

		if utils.ShouldSample(h.Number.Uint64(), BlocksPerHour) {
			g.Go(func() error { return goldTokenProcessor.ObserveState(opts) })
			g.Go(func() error { return reserveProcessor.ObserveState(opts) })
			g.Go(func() error { return stableTokenProcessor.ObserveState(opts) })
			g.Go(func() error { return sortedOraclesProcessor.ObserveState(opts, stableTokenAddress) })
		}

		if utils.ShouldSample(h.Number.Uint64(), EpochSize) {
			g.Go(func() error { return electionProcessor.ObserveState(opts) })
			g.Go(func() error { return epochRewardsProcessor.ObserveState(opts) })
			g.Go(func() error { return lockedGoldProcessor.ObserveState(opts) })
			g.Go(func() error { return stabilityProcessor.ObserveState(opts) })
			g.Go(func() error { return validatorsProcessor.ObserveState(opts) })

			filterLogs, err := cc.Eth.FilterLogs(ctx, ethereum.FilterQuery{
				FromBlock: h.Number,
				ToBlock:   h.Number,
			})
			if err != nil {
				return err
			}

			for _, epochLog := range filterLogs {
				if epochLog.BlockHash != epochLog.TxHash {
					// Already handled by TransactionReceipt
					continue
				}
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
				accountsProcessor.HandleLog(eventLog)
				attestationsProcessor.HandleLog(eventLog)
				electionProcessor.HandleLog(eventLog)
				epochRewardsProcessor.HandleLog(eventLog)
				governanceProcessor.HandleLog(eventLog)
				validatorsProcessor.HandleLog(eventLog)
				sortedOraclesProcessor.HandleLog(eventLog)
			}

			internalTransfers, err := cc.Debug.TransactionTransfers(ctx, txHash)
			if err != nil {
				return err
			}
			for _, internalTransfer := range internalTransfers {
				logTransfer(txLogger, "currencySymbol", "cGLD", "from", internalTransfer.From, "to", internalTransfer.To, "value", internalTransfer.Value)
				if tipMode && sensitiveAccounts[internalTransfer.From] != "" {
					err = notifyFundsMoved(internalTransfer, sensitiveAccounts[internalTransfer.From])
					if err != nil {
						logger.Error(err.Error())
					}
				}
			}
		}

		if err := dbWriter.ApplyChanges(ctx, h.Number); err != nil {
			return err
		}

		metrics.LastBlockProcessed.Set(float64(h.Number.Int64()))
		elapsed := time.Since(start)
		logger.Debug("STATS", "elapsed", elapsed)
	}
}

func isTipMode(latestHeader *types.Header, currentBlockNumber *big.Int) bool {
	return new(big.Int).Sub(latestHeader.Number, currentBlockNumber).Cmp(TipGap) < 0
}
