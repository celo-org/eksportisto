package monitor

import (
	"context"

	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type lockedGoldProcessor struct {
	ctx               context.Context
	logger            log.Logger
	lockedGoldAddress common.Address
	lockedGold        *contracts.LockedGold
}

func NewLockedGoldProcessor(ctx context.Context, logger log.Logger, lockedGoldAddress common.Address, lockedGold *contracts.LockedGold) *lockedGoldProcessor {
	return &lockedGoldProcessor{
		ctx:               ctx,
		logger:            logger,
		lockedGoldAddress: lockedGoldAddress,
		lockedGold:        lockedGold,
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

func (p lockedGoldProcessor) HandleLog(eventLog *types.Log) {
}
