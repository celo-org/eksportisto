package monitor

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/contracts/helpers"
)

type reserveProcessor struct {
	ctx     context.Context
	logger  log.Logger
	reserve *contracts.Reserve
}

func NewReserveProcessor(ctx context.Context, logger log.Logger, reserve *contracts.Reserve) *reserveProcessor {
	return &reserveProcessor{
		ctx:     ctx,
		logger:  logger.New("contract", "Reserve"),
		reserve: reserve,
	}
}

func (p reserveProcessor) ObserveState(opts *bind.CallOpts) error {
	reserveRatio, err := p.reserve.GetReserveRatio(opts)

	// TODO: Properly handle when things are frozen
	if err != nil {
		return nil
	}

	logStateViewCall(p.logger, "method", "getReserveRatio", "reserveRatio", helpers.FromFixed(reserveRatio))

	// Reserve.getReserveGoldBalance
	reserveGoldBalance, err := p.reserve.GetReserveGoldBalance(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "method", "getReserveGoldBalance", "reserveGoldBalance", reserveGoldBalance)

	// Reserve.getOtherReserveAddressesGoldBalance
	otherReserveAddressesGoldBalance, err := p.reserve.GetOtherReserveAddressesGoldBalance(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "method", "getOtherReserveAddressesGoldBalance", "otherReserveAddressesGoldBalance", otherReserveAddressesGoldBalance)

	// Reserve.getUnfrozenBalance
	unfrozenBalance, err := p.reserve.GetUnfrozenBalance(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "method", "getUnfrozenBalance", "value", unfrozenBalance)

	// Reserve.getFrozenReserveGoldBalance
	frozenReserveGoldBalance, err := p.reserve.GetFrozenReserveGoldBalance(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "method", "getFrozenReserveGoldBalance", "value", frozenReserveGoldBalance)

	// Reserve.getUnfrozenReserveGoldBalance
	unfrozenReserveGoldBalance, err := p.reserve.GetUnfrozenReserveGoldBalance(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "method", "getUnfrozenReserveGoldBalance", "value", unfrozenReserveGoldBalance)

	return nil
}
