package monitor

import (
	"context"

	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/contracts/helpers"
	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
)

type reserveProcessor struct {
	ctx     context.Context
	logger  log.Logger
	reserve *contracts.Reserve
}

func NewReserveProcessor(ctx context.Context, logger log.Logger, reserve *contracts.Reserve) *reserveProcessor {
	return &reserveProcessor{
		ctx:     ctx,
		logger:  logger,
		reserve: reserve,
	}
}

func (p reserveProcessor) ObserveState(opts *bind.CallOpts) error {
	logger := p.logger.New("contract", "Reserve")
	reserveRatio, err := p.reserve.GetReserveRatio(opts)

	// TODO: Properly handle when things are frozen
	if err != nil {
		return nil
	}

	logStateViewCall(logger, "method", "getReserveRatio", "reserveRatio", helpers.FromFixed(reserveRatio))

	return nil
}
