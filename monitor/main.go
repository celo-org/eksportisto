package monitor

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/celo-org/eksportisto/db"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/kliento/client"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/contracts/helpers"

	"github.com/celo-org/kliento/registry"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/log"
)

type Config struct {
	NodeUri   string
	DataDir   string
	FromBlock string
	ToBlock   string
}

// specifies number of blocks to query logs for at a time
var MaxBlockInterval = int64(100)

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

	from, ok1 := new(big.Int).SetString(cfg.FromBlock, 10)
	to, ok2 := new(big.Int).SetString(cfg.ToBlock, 10)
	if !ok1 || !ok2 {
		return fmt.Errorf("Parsing 'from' or 'to' error")
	}

	return eventProcessor(ctx, from, to, cc, logger, store)
}

func eventProcessor(ctx context.Context, startBlock *big.Int, endBlock *big.Int, cc *client.CeloClient, logger log.Logger, dbWriter db.RosettaDBWriter) error {
	r, err := registry.New(cc)
	if err != nil {
		return err
	}
	err = r.EnableCaching(ctx, startBlock)
	if err != nil {
		return err
	}

	h, err := cc.Eth.HeaderByNumber(ctx, startBlock)
	if err != nil {
		return err
	}

	registeredAddresses, err := r.AllAddresses(ctx, h.Number)
	if err != nil {
		return err
	}

	gasPriceMinimum := big.NewInt(0)

	// TODO: parallelize
	for startInterval := startBlock.Int64(); startInterval < endBlock.Int64(); startInterval += MaxBlockInterval {
		from := big.NewInt(startInterval)
		to := big.NewInt(startInterval + MaxBlockInterval - 1)
		filterLogs, err := cc.Eth.FilterLogs(ctx, ethereum.FilterQuery{
			FromBlock: from,
			ToBlock:   to,
			Addresses: registeredAddresses,
		})
		if err != nil {
			return err
		}

		// TODO: parallelize
		for _, eventLog := range filterLogs {
			eventLogger := logger.New(
				"blocknumber", eventLog.BlockNumber,
				"logindex", eventLog.Index,
				"txhash", eventLog.TxHash,
			)
			parsed, err := r.TryParseLog(ctx, eventLog, h.Number)
			if err != nil {
				eventLogger.Error("log parsing failed", "err", err)
			} else if parsed != nil {
				switch v := parsed.Log.(type) {
				case *contracts.GasPriceMinimumGasPriceMinimumUpdated:
					if gasPriceMinimum.Cmp(v.GasPriceMinimum) == 0 {
						// skip log when update is redundant
						continue
					}
					gasPriceMinimum = v.GasPriceMinimum
				}

				logSlice, err := helpers.EventToSlice(parsed.Log)
				if err != nil {
					eventLogger.Error("event slice encoding failed", "contract", parsed.Contract, "event", parsed.Event, "err", err)
				} else {
					logEventLog(eventLogger, append([]interface{}{"contract", parsed.Contract, "event", parsed.Event}, logSlice...)...)
				}
			}
		}

		if err := dbWriter.ApplyChanges(ctx, to); err != nil {
			return err
		}
		metrics.LastBlockProcessed.Set(float64(to.Int64()))
	}

	return nil

}
