package monitor

import (
	"context"

	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type stableTokenProcessor struct {
	ctx                context.Context
	logger             log.Logger
	stableTokenAddress common.Address
	stableToken        *contracts.StableToken
}

func NewStableTokenProcessor(ctx context.Context, logger log.Logger, stableTokenAddress common.Address, stableToken *contracts.StableToken) *stableTokenProcessor {
	return &stableTokenProcessor{
		ctx:                ctx,
		logger:             logger,
		stableTokenAddress: stableTokenAddress,
		stableToken:        stableToken,
	}
}

func (p stableTokenProcessor) ObserveState(opts *bind.CallOpts) error {
	// Not super important right now
	// logger := p.logger.New("contract", "StableToken")

	// totalSupply, err := p.stableToken.TotalSupply(opts)
	// if err != nil {
	// 	return err
	// }

	// metrics.TotalCUSDSupply.Observe(float64(totalSupply.Uint64()))
	// logStateViewCall(logger, "method", "totalSupply", "totalSupply", totalSupply)

	return nil
}

func (p stableTokenProcessor) HandleLog(eventLog *types.Log) {

}
