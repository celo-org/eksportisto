package monitor

import (
	"context"

	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
)

type stableTokenProcessor struct {
	ctx         context.Context
	logger      log.Logger
	stableToken *contracts.StableToken
}

func NewStableTokenProcessor(ctx context.Context, logger log.Logger, stableToken *contracts.StableToken) *stableTokenProcessor {
	return &stableTokenProcessor{
		ctx:         ctx,
		logger:      logger,
		stableToken: stableToken,
	}
}

func (p stableTokenProcessor) ObserveState(opts *bind.CallOpts) error {
	logger := p.logger.New("contract", "StableToken")

	totalSupply, err := p.stableToken.TotalSupply(opts)
	if err != nil {
		return err
	}

	// metrics.TotalCUSDSupply.Observe(float64(totalSupply.Uint64()))
	logStateViewCall(logger, "method", "totalSupply", "totalSupply", totalSupply)

	return nil
}

func (p stableTokenProcessor) ObserveMetric(opts *bind.CallOpts) error {
	totalSupply, err := p.stableToken.TotalSupply(opts)
	if err != nil {
		return err
	}

	metrics.TotalCUSDSupply.Set(utils.ScaleFixed(totalSupply))
	return nil
}
