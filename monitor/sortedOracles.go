package monitor

import (
	"context"

	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type sortedOraclesProcessor struct {
	ctx                  context.Context
	logger               log.Logger
	sortedOraclesAddress common.Address
	sortedOracles        *contracts.SortedOracles
}

func NewSortedOraclesProcessor(ctx context.Context, logger log.Logger, sortedOraclesAddress common.Address, sortedOracles *contracts.SortedOracles) *sortedOraclesProcessor {
	return &sortedOraclesProcessor{
		ctx:                  ctx,
		logger:               logger,
		sortedOraclesAddress: sortedOraclesAddress,
		sortedOracles:        sortedOracles,
	}
}

func (p sortedOraclesProcessor) ObserveState(opts *bind.CallOpts) error {

	return nil
}

func (p sortedOraclesProcessor) HandleLog(eventLog *types.Log) {

}
