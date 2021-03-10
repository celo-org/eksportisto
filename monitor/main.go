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
	"strings"
	"time"

	"github.com/celo-org/eksportisto/db"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/celotokens"
	"github.com/celo-org/kliento/client"
	"github.com/celo-org/kliento/client/debug"
	"github.com/celo-org/kliento/contracts/helpers"

	celo "github.com/celo-org/celo-blockchain"
	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/common"
	"github.com/celo-org/celo-blockchain/core/types"
	"github.com/celo-org/celo-blockchain/log"
	kliento_mon "github.com/celo-org/kliento/monitor"
	"github.com/celo-org/kliento/registry"
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
	err = r.EnableCaching(ctx, startBlock)
	if err != nil {
		return err
	}

	celoTokens := celotokens.New(r)

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

		logger = logger.New("blockTimestamp", time.Unix(int64(h.Time), 0).Format(time.RFC3339), "blockNumber", h.Number.Int64(), "blockGasUsed", h.GasUsed)

		finishHeader := func(ctx context.Context) error {
			if err := dbWriter.ApplyChanges(ctx, h.Number); err != nil {
				return err
			}

			metrics.LastBlockProcessed.Set(float64(h.Number.Int64()))
			return nil
		}

		metrics.BlockGasUsed.Set(float64(h.GasUsed))
		latestHeader, err := cc.Eth.HeaderByNumber(ctx, nil)
		if err != nil {
			return err
		}
		tipMode := isTipMode(latestHeader, h.Number)

		transactionCtx := context.Background()

		parseAndLogEvent := func(logger log.Logger, eventIdx int, eventLog *types.Log) {
			eventLogger := logger.New("logTxIndex", eventIdx, "logBlockIndex", eventLog.Index)
			parsed, err := r.TryParseLog(transactionCtx, *eventLog, h.Number)
			if err != nil {
				eventLogger.Error("log parsing failed", "err", err)
			} else if parsed != nil {
				logSlice, err := helpers.EventToSlice(parsed.Log)
				if err != nil {
					eventLogger.Error("event slice encoding failed", "contract", parsed.Contract, "event", parsed.Event, "err", err)
				} else {
					logEventLog(eventLogger, append([]interface{}{"contract", parsed.Contract, "event", parsed.Event}, logSlice...)...)
				}
			} else {
				eventLogger.Warn("log source unknown, logging raw event")
				logEventLog(eventLogger, "rawEvent", *eventLog)
			}
		}

		skipContractMetrics := func(err error) bool {
			if err != nil {
				if registry.IsExpectedBeforeContractsDeployed(err) {
					logger.Warn("skipping contract metrics before contracts are available")
				} else {
					logger.Error("unexpected error while fetching contracts", "err", err)
				}
				finishHeader(ctx)
				return true
			}
			return false
		}

		block, err := cc.Eth.BlockByNumber(ctx, h.Number)
		var txs types.Transactions = nil
		if err != nil {
			unmarshalError := strings.Contains(err.Error(), "cannot unmarshal")
			if !unmarshalError {
				return err
			}
			logger.Error("block by number rpc unmarshalling failed")
		} else {
			txs = block.Transactions()
		}

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
				parseAndLogEvent(txLogger, eventIdx, eventLog)
			}

			if !debugEnabled {
				continue
			}
			internalTransfers, err := cc.Debug.TransactionTransfers(transactionCtx, txHash)
			if skipContractMetrics(err) {
				continue
			} else if err != nil {
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

		election, err := r.GetElectionContract(ctx, h.Number)
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

		epochRewards, err := r.GetEpochRewardsContract(ctx, h.Number)
		if skipContractMetrics(err) {
			continue
		}

		celoTokenContracts, err := celoTokens.GetContracts(ctx, h.Number, false)
		if skipContractMetrics(err) {
			continue
		}

		exchangeContracts, err := celoTokens.GetExchangeContracts(ctx, h.Number)
		if skipContractMetrics(err) {
			continue
		}

		stableTokenAddresses, err := celoTokens.GetAddresses(ctx, h.Number, true)
		if err != nil {
			return err
		}

		electionProcessor := NewElectionProcessor(processorCtx, logger, election)
		epochRewardsProcessor := NewEpochRewardsProcessor(processorCtx, logger, epochRewards)
		lockedGoldProcessor := NewLockedGoldProcessor(processorCtx, logger, lockedGold)
		reserveProcessor := NewReserveProcessor(processorCtx, logger, reserve)
		sortedOraclesProcessor := NewSortedOraclesProcessor(processorCtx, logger, sortedOracles, exchange)

		// Create a celo token processor for each celo token
		celoTokenProcessors := make(map[celotokens.CeloToken]ContractProcessor)
		for token, contract := range celoTokenContracts {
			celoTokenProcessors[token], err = NewCeloTokenProcessor(processorCtx, logger, token, contract)
			if err != nil {
				return err
			}
		}
		// Create an exchange processor for each stable token's exchange
		exchangeProcessors := make(map[celotokens.CeloToken]ContractProcessor)
		for stableToken, exchangeContract := range exchangeContracts {
			exchangeProcessors[stableToken], err = NewExchangeProcessor(ctx, logger, stableToken, exchangeContract, reserve)
			if err != nil {
				return err
			}
		}

		if tipMode {
			g.Go(func() error { return epochRewardsProcessor.ObserveMetric(opts) })
			g.Go(func() error { return sortedOraclesProcessor.ObserveMetric(opts, stableTokenAddresses, h.Time) })
			// Celo token processors
			for _, processor := range celoTokenProcessors {
				g.Go(func() error { return processor.ObserveMetric(opts) })
			}
			// Exchange processors
			for _, processor := range exchangeProcessors {
				g.Go(func() error { return processor.ObserveMetric(opts) })
			}
		}

		if utils.ShouldSample(h.Number.Uint64(), BlocksPerHour) {
			g.Go(func() error { return reserveProcessor.ObserveState(opts) })
			g.Go(func() error { return sortedOraclesProcessor.ObserveState(opts, stableTokenAddresses) })
			// Celo token processors
			for _, processor := range celoTokenProcessors {
				g.Go(func() error { return processor.ObserveState(opts) })
			}
		}

		if utils.ShouldSample(h.Number.Uint64(), EpochSize) {
			g.Go(func() error { return electionProcessor.ObserveState(opts) })
			g.Go(func() error { return epochRewardsProcessor.ObserveState(opts) })
			g.Go(func() error { return lockedGoldProcessor.ObserveState(opts) })
			g.Go(func() error { return reserveProcessor.ObserveState(opts) })
			// Exchange processors
			for _, processor := range exchangeProcessors {
				g.Go(func() error { return processor.ObserveState(opts) })
			}

			filterLogs, err := cc.Eth.FilterLogs(ctx, celo.FilterQuery{
				FromBlock: h.Number,
				ToBlock:   h.Number,
			})
			if err != nil {
				return err
			}

			for eventIdx, epochLog := range filterLogs {
				// explicitly log epoch events which don't appear in normal transaction receipts
				if epochLog.BlockHash == epochLog.TxHash {
					parseAndLogEvent(logger, eventIdx, &epochLog)
				}
			}
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
