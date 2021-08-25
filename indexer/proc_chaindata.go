package indexer

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/celo-org/celo-blockchain/common"
	"github.com/celo-org/celo-blockchain/core/types"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/indexer/data"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/kliento/client/debug"
	"github.com/celo-org/kliento/registry"
	"golang.org/x/sync/errgroup"
)

type chaindataProcessorFactory struct{}

func (chaindataProcessorFactory) New(_ context.Context, handler *blockHandler) ([]Processor, error) {
	return []Processor{&chaindataProcessor{blockHandler: handler, logger: handler.logger.New("processor", "chaindata")}}, nil
}

func (proc *chaindataProcessor) Logger() log.Logger { return proc.logger }
func (proc *chaindataProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

type chaindataProcessor struct {
	*blockHandler
	logger log.Logger
}

func (proc *chaindataProcessor) Init(ctx context.Context) error {
	var err error
	proc.block, err = proc.celoClient.Eth.BlockByNumber(ctx, proc.blockNumber)
	if err != nil {
		if strings.Contains(err.Error(), "cannot unmarshal") {
			return &SkipProcessorError{Reason: err}
		}
		return err
	}

	proc.transactions = proc.block.Transactions()
	proc.blockRow = data.NewRow(
		"blockTimestamp", time.Unix(int64(proc.block.Time()), 0).Format(time.RFC3339),
		"blockNumber", proc.block.NumberU64(),
		"blockGasUsed", proc.block.GasUsed(),
	)

	return nil
}

func (proc *chaindataProcessor) ShouldCollect() bool {
	// This processor will always run
	return true
}

func (proc *chaindataProcessor) CollectData(ctx context.Context, rows chan interface{}) error {
	group, ctx := errgroup.WithContext(ctx)
	for txIndex, tx := range proc.transactions {
		func(txIndex int, tx *types.Transaction) {
			group.Go(func() error { return proc.collectTransaction(ctx, txIndex, tx, rows) })
		}(txIndex, tx)
	}
	return group.Wait()
}

func (proc *chaindataProcessor) collectTransaction(
	ctx context.Context,
	txIndex int,
	tx *types.Transaction,
	rows chan interface{},
) error {
	txHash := tx.Hash()

	txRow := proc.blockRow.Extend("txHash", txHash, "txIndex", txIndex)

	receipt, err := proc.celoClient.Eth.TransactionReceipt(ctx, txHash)
	if err != nil {
		return err
	}

	rows <- txRow.Extend("type", "Transaction", "gasPrice", tx.GasPrice(), "gasUsed", receipt.GasUsed).WithId(txHash.Hex())

	for eventIdx, eventLog := range receipt.Logs {
		err := proc.extractEvent(ctx, txHash, eventIdx, eventLog, txRow, rows)
		if err != nil {
			return err
		}
	}

	if proc.debugEnabled {
		err := proc.extractInternalTransactions(ctx, txHash, txRow, rows)
		if err != nil {
			return err
		}
	}

	return nil
}

func (proc *chaindataProcessor) extractInternalTransactions(ctx context.Context, txHash common.Hash, txRow *data.Row, rows chan interface{}) error {
	internalTransfers, err := proc.celoClient.Debug.TransactionTransfers(ctx, txHash)
	if err != nil {
		return err
	}
	// TODO: Figure out why this was needed.
	// if skipContractMetrics(err) {
	// 	continue
	// } else if err != nil {
	// 	return err
	// }
	for index, internalTransfer := range internalTransfers {
		rows <- txRow.Extend(
			"currencySymbol", "CELO",
			"from", internalTransfer.From,
			"to", internalTransfer.To,
			"value", internalTransfer.Value,
		).WithId(fmt.Sprintf("%s.internalTransfer.%d", txHash.String(), index))

		if proc.isTip && proc.sensitiveAccounts[internalTransfer.From] != "" {
			err = proc.notifyFundsMoved(internalTransfer, proc.sensitiveAccounts[internalTransfer.From])
			if err != nil {
				proc.logger.Error(err.Error())
			}
		}
	}
	return nil
}

func (proc *chaindataProcessor) ObserveMetrics(ctx context.Context) error {
	metrics.BlockGasUsed.Set(float64(proc.block.GasUsed()))
	return nil
}

func (proc *chaindataProcessor) notifyFundsMoved(transfer debug.Transfer, url string) error {
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
