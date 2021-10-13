package indexer

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/registry"
	"github.com/go-errors/errors"
)

type lockedGoldProcessorFactory struct{}

func (lockedGoldProcessorFactory) InitProcessors(
	ctx context.Context,
	handler *blockHandler,
) ([]Processor, error) {
	lockedGold, err := handler.registry.GetLockedGoldContract(ctx, handler.blockNumber)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	return []Processor{
		&lockedGoldProcessor{
			blockHandler: handler,
			logger:       handler.logger.New("processor", "lockedGold", "contract", "LockedGold"),
			lockedGold:   lockedGold,
		},
	}, nil
}

type lockedGoldProcessor struct {
	*blockHandler
	logger     log.Logger
	lockedGold *contracts.LockedGold
}

func (proc *lockedGoldProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

func (proc *lockedGoldProcessor) ShouldCollect() bool {
	// This processor will only collect data at the end of the epoch
	return utils.ShouldSample(proc.blockNumber.Uint64(), EpochSize)
}

func (proc lockedGoldProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}
	contractRow := proc.blockRow.Extend("contract", "LockedGold")

	// LockedGold.getTotalLockedGold
	totalNonvoting, err := proc.lockedGold.GetNonvotingLockedGold(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall("getNonvotingLockedGold", "totalNonvoting", totalNonvoting.String())

	// LockedGold.getTotalLockedGold
	totalLockedGold, err := proc.lockedGold.GetTotalLockedGold(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall("getTotalLockedGold", "totalLockedGold", totalLockedGold)

	return nil
}

func (proc lockedGoldProcessor) ObserveMetrics(ctx context.Context) error {
	return nil
}
