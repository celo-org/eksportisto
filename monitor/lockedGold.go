package monitor

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/kliento/contracts"
)

type lockedGoldProcessor struct {
	ctx        context.Context
	logger     log.Logger
	lockedGold *contracts.LockedGold
}

func NewLockedGoldProcessor(ctx context.Context, logger log.Logger, lockedGold *contracts.LockedGold) *lockedGoldProcessor {
	return &lockedGoldProcessor{
		ctx:        ctx,
		logger:     logger,
		lockedGold: lockedGold,
	}
}

func (p lockedGoldProcessor) ObserveState(opts *bind.CallOpts) error {
	logger := p.logger.New("contract", "LockedGold")

	// LockedGold.getTotalLockedGold
	totalNonvoting, err := p.lockedGold.GetNonvotingLockedGold(opts)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "getNonvotingLockedGold", "totalNonvoting", totalNonvoting)

	// LockedGold.getTotalLockedGold
	totalLockedGold, err := p.lockedGold.GetTotalLockedGold(opts)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "getTotalLockedGold", "totalLockedGold", totalLockedGold)

	return nil
}
