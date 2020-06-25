package monitor

import (
	"context"

	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type reserveProcessor struct {
	ctx            context.Context
	logger         log.Logger
	reserveAddress common.Address
	reserve        *contracts.Reserve
}

func NewReserveProcessor(ctx context.Context, logger log.Logger, reserveAddress common.Address, reserve *contracts.Reserve) *reserveProcessor {
	return &reserveProcessor{
		ctx:            ctx,
		logger:         logger,
		reserveAddress: reserveAddress,
		reserve:        reserve,
	}
}

func (p reserveProcessor) ObserveState(opts *bind.CallOpts) error {
	logger := p.logger.New("contract", "Reserve")
	reserveRatio, err := p.reserve.GetReserveRatio(opts)

	// TODO: Properly handle when things are frozen
	if err != nil {
		return nil
	}

	logStateViewCall(logger, "method", "getReserveRatio", "reserveRatio", utils.FromFixed(reserveRatio))

	return nil
}

func (p reserveProcessor) HandleLog(eventLog *types.Log) {
	return
}
