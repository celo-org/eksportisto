package monitor

import (
	"context"

	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
)

type goldTokenProcessor struct {
	ctx       context.Context
	logger    log.Logger
	goldToken *contracts.GoldToken
}

func NewGoldTokenProcessor(ctx context.Context, logger log.Logger, goldToken *contracts.GoldToken) *goldTokenProcessor {
	return &goldTokenProcessor{
		ctx:       ctx,
		logger:    logger,
		goldToken: goldToken,
	}
}

func (p goldTokenProcessor) ObserveState(opts *bind.CallOpts) error {
	logger := p.logger.New("contract", "GoldToken")
	totalSupply, err := p.goldToken.TotalSupply(opts)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "totalSupply", "totalSupply", totalSupply)

	return nil
}

func (p goldTokenProcessor) ObserveMetric(opts *bind.CallOpts) error {
	totalSupply, err := p.goldToken.TotalSupply(opts)
	if err != nil {
		return err
	}
	metrics.TotalCGLDSupply.Set(utils.ScaleFixed(totalSupply))
	return nil
}
