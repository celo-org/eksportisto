package indexer

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/registry"
)

type lockedGoldProcessorFactory struct{}

func (lockedGoldProcessorFactory) New(_ context.Context, handler *blockHandler) ([]Processor, error) {
	return []Processor{&lockedGoldProcessor{blockHandler: handler, logger: handler.logger.New("processor", "lockedGold", "contract", "LockedGold")}}, nil
}

type lockedGoldProcessor struct {
	*blockHandler
	logger     log.Logger
	lockedGold *contracts.LockedGold
}

func (proc *lockedGoldProcessor) Logger() log.Logger { return proc.logger }
func (proc *lockedGoldProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}
func (proc *lockedGoldProcessor) Init(ctx context.Context) error {
	var err error
	proc.lockedGold, err = proc.registry.GetLockedGoldContract(ctx, proc.blockNumber)
	if err != nil {
		return err
	}
	return nil
}

func (proc *lockedGoldProcessor) ShouldCollect() bool {
	// This processor will only collect data at the end of the epoch
	return utils.ShouldSample(proc.blockNumber.Uint64(), EpochSize)
}

func (proc lockedGoldProcessor) CollectData(ctx context.Context, rows chan interface{}) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}
	contractRow := proc.blockRow.Extend("contract", "LockedGold")

	// LockedGold.getTotalLockedGold
	totalNonvoting, err := proc.lockedGold.GetNonvotingLockedGold(opts)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall("getNonvotingLockedGold", "totalNonvoting", totalNonvoting.String())

	// LockedGold.getTotalLockedGold
	totalLockedGold, err := proc.lockedGold.GetTotalLockedGold(opts)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall("getTotalLockedGold", "totalLockedGold", totalLockedGold)

	return nil
}

func (proc lockedGoldProcessor) ObserveMetrics(ctx context.Context) error {
	return nil
}
