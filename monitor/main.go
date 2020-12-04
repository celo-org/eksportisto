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

	kliento_mon "github.com/celo-org/kliento/monitor"
	"github.com/celo-org/kliento/registry"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	blockchainErrors "github.com/ethereum/go-ethereum/contract_comm/errors"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	NodeUri                   string
	DataDir                   string
	SensitiveAccountsFilePath string
	FromBlock                 string
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

	var startBlock *big.Int
	if cfg.FromBlock == "" {
		startBlock, err = store.LastPersistedBlock(ctx)
		if err != nil {
			return err
		}
	} else if cfg.FromBlock == "latest" {
		latestHeader, headerErr := cc.Eth.HeaderByNumber(ctx, nil)
		if headerErr != nil {
			return err
		}
		startBlock = latestHeader.Number
	} else {
		bigInt, ok := new(big.Int).SetString(cfg.FromBlock, 10)
		if !ok {
			return fmt.Errorf("Unable to parse FromBlock %s", cfg.FromBlock)
		}
		startBlock = bigInt
	}

	headers := make(chan *types.Header, 10)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return kliento_mon.HeaderListener(ctx, headers, cc, logger, startBlock) })
	g.Go(func() error { return blockProcessor(ctx, startBlock, headers, cc, logger, store, cfg) })
	return g.Wait()
}

func blockProcessor(ctx context.Context, startBlock *big.Int, headers <-chan *types.Header, cc *client.CeloClient, logger log.Logger, dbWriter db.RosettaDBWriter, cfg *Config) error {
	r, err := registry.New(cc)
	if err != nil {
		return err
	}

	supported, err := cc.Rpc.SupportedModules()
	if err != nil {
		return err
	}
	_, debugEnabled := supported["debug"]

	sensitiveAccounts := getSensitiveAccounts(cfg.SensitiveAccountsFilePath)

	var h *types.Header
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case h = <-headers:
		}
		logHeader(logger, h)

		start := time.Now()
		logger = logger.New("blockTimestamp", time.Unix(int64(h.Time), 0).Format(time.RFC3339), "blockNumber", h.Number.Uint64(), "blockGasUsed", h.GasUsed)

		finishHeader := func(ctx context.Context) error {
			if err := dbWriter.ApplyChanges(ctx, h.Number); err != nil {
				return err
			}

			metrics.LastBlockProcessed.Set(float64(h.Number.Int64()))
			elapsed := time.Since(start)
			logger.Debug("STATS", "elapsed", elapsed)
			return nil
		}

		metrics.BlockGasUsed.Set(float64(h.GasUsed))
		_, latestHeader, err := getHeaderInformation(ctx, cc, h)
		if err != nil {
			return err
		}
		tipMode := isTipMode(latestHeader, h.Number)

		transactionCtx := context.Background()

		block, err := cc.Eth.BlockByNumber(ctx, h.Number)
		if err != nil {
			fmt.Printf("ERROR %v", err)
			return err
		}
		txs := block.Transactions()
		for txIndex, tx := range txs {
			txHash := tx.Hash()

			txLogger := logger.New("txHash", txHash, "txIndex", txIndex)

			receipt, err := cc.Eth.TransactionReceipt(transactionCtx, txHash)
			if err != nil {
				return err
			}

			logTransaction(txLogger, "gasPrice", tx.GasPrice(), "gasUsed", receipt.GasUsed)

			metrics.GasPrice.Set(utils.ScaleFixed(tx.GasPrice()))

			for eventIdx, eventLog := range receipt.Logs {
				eventLogger := txLogger.New("logTxIndex", eventIdx, "logBlockIndex", eventLog.Index)
				parsed, err := r.TryParseLog(transactionCtx, eventLog, h.Number)
				if err != nil {
					eventLogger.Error("log parsing failed", "log", eventLog, "err", err)
				} else if parsed != nil {
					logEventLog(eventLogger, parsed...)
				}
			}

			if !debugEnabled {
				continue
			}
			internalTransfers, err := cc.Debug.TransactionTransfers(transactionCtx, txHash)
			if err != nil {
				return err
			}
			for _, internalTransfer := range internalTransfers {
				logTransfer(txLogger, "currencySymbol", "CELO", "from", internalTransfer.From, "to", internalTransfer.To, "value", internalTransfer.Value)
				if tipMode && sensitiveAccounts[internalTransfer.From] != "" {
					err = notifyFundsMoved(internalTransfer, sensitiveAccounts[internalTransfer.From])
					if err != nil {
						logger.Error(err.Error())
					}
				}
			}
		}

		g, processorCtx := errgroup.WithContext(context.Background())
		opts := &bind.CallOpts{
			BlockNumber: h.Number,
			Context:     processorCtx,
		}

		skipContractMetrics := func(err error) bool {
			if err != nil {
				if err == blockchainErrors.ErrRegistryContractNotDeployed {
					logger.Warn("skipping contract metrics before contracts are available", "err", err)
				} else {
					logger.Error("unexpected error while fetching contracts", "err", err)
				}
				finishHeader(ctx)
				return true
			}
			return false
		}

		election, err := r.GetElectionContract(ctx, h.Number)
		if skipContractMetrics(err) {
			continue
		}
		goldToken, err := r.GetGoldTokenContract(ctx, h.Number)
		if skipContractMetrics(err) {
			continue
		}
		lockedGold, err := r.GetLockedGoldContract(ctx, h.Number)
		if skipContractMetrics(err) {
			continue
		}
		reserve, err := r.GetReserveContract(ctx, h.Number)
		if skipContractMetrics(err) {
			continue
		}
		exchange, err := r.GetExchangeContract(ctx, h.Number)
		if skipContractMetrics(err) {
			continue
		}
		sortedOracles, err := r.GetSortedOraclesContract(ctx, h.Number)
		if skipContractMetrics(err) {
			continue
		}
		stableToken, err := r.GetStableTokenContract(ctx, h.Number)
		if skipContractMetrics(err) {
			continue
		}
		epochRewards, err := r.GetEpochRewardsContract(ctx, h.Number)
		if skipContractMetrics(err) {
			continue
		}

		stableTokenAddress, err := r.GetAddressFor(ctx, h.Number, registry.StableTokenContractID)
		if err != nil {
			return err
		}

		electionProcessor := NewElectionProcessor(processorCtx, logger, election)
		epochRewardsProcessor := NewEpochRewardsProcessor(processorCtx, logger, epochRewards)
		goldTokenProcessor := NewGoldTokenProcessor(processorCtx, logger, goldToken)
		lockedGoldProcessor := NewLockedGoldProcessor(processorCtx, logger, lockedGold)
		reserveProcessor := NewReserveProcessor(processorCtx, logger, reserve)
		sortedOraclesProcessor := NewSortedOraclesProcessor(processorCtx, logger, sortedOracles, exchange)
		stabilityProcessor := NewStabilityProcessor(processorCtx, logger, exchange, reserve)
		stableTokenProcessor := NewStableTokenProcessor(processorCtx, logger, stableToken)

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
		}

		err = g.Wait()
		if err != nil {
			return err
		}

		err = finishHeader(transactionCtx)
		if err != nil {
			return err
		}
	}
}

func isTipMode(latestHeader *types.Header, currentBlockNumber *big.Int) bool {
	return new(big.Int).Sub(latestHeader.Number, currentBlockNumber).Cmp(TipGap) < 0
}
