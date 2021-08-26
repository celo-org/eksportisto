package indexer

import (
	"context"

	"github.com/celo-org/celo-blockchain"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/registry"
)

type epochLogsProcessorFactory struct{}

func (epochLogsProcessorFactory) New(_ context.Context, handler *blockHandler) ([]Processor, error) {
	return []Processor{&epochLogsProcessor{blockHandler: handler, logger: handler.logger.New("processor", "epochLogs")}}, nil
}

func (proc *epochLogsProcessor) Logger() log.Logger { return proc.logger }
func (proc *epochLogsProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

type epochLogsProcessor struct {
	*blockHandler
	logger log.Logger
}

func (proc *epochLogsProcessor) Init(ctx context.Context) error {
	return nil
}

func (proc *epochLogsProcessor) ShouldCollect() bool {
	// This processor will only collect data at the end of the epoch
	return utils.ShouldSample(proc.blockNumber.Uint64(), EpochSize)
}

func (proc *epochLogsProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	filterLogs, err := proc.celoClient.Eth.FilterLogs(ctx, celo.FilterQuery{
		FromBlock: proc.blockNumber,
		ToBlock:   proc.blockNumber,
	})
	if err != nil {
		return err
	}

	for eventIdx, epochLog := range filterLogs {
		// explicitly log epoch events which don't appear in normal transaction receipts
		if epochLog.BlockHash == epochLog.TxHash {
			err := proc.extractEvent(ctx, epochLog.BlockHash, eventIdx, &epochLog, proc.blockRow, rows)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (proc *epochLogsProcessor) ObserveMetrics(ctx context.Context) error {
	return nil
}
