package monitor

import (
	"context"

	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
    "github.com/celo-org/kliento/celotokens"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
    "github.com/prometheus/client_golang/prometheus"
)

type celoTokenProcessorInfo struct {
    contractName string
    totalSupplyMetric prometheus.Gauge
}

var celoTokenProcessorInfos = map[celotokens.CeloToken]celoTokenProcessorInfo{
    celotokens.CELO: celoTokenProcessorInfo{
        contractName: "GoldToken",
        totalSupplyMetric: metrics.TotalCGLDSupply,
    },
    celotokens.CUSD: celoTokenProcessorInfo{
        contractName: "StableToken",
        totalSupplyMetric: metrics.TotalCUSDSupply,
    },
}

type celoTokenProcessor struct {
	ctx       context.Context
	logger    log.Logger
    token     celotokens.CeloToken
	tokenContract contracts.CeloTokenContract
}

func NewCeloTokenProcessor(ctx context.Context, logger log.Logger, token celotokens.CeloToken, tokenContract contracts.CeloTokenContract) *celoTokenProcessor {
	return &celoTokenProcessor{
		ctx:       ctx,
		logger:    logger,
        token:     token,
		tokenContract: tokenContract,
	}
}

func (p celoTokenProcessor) ObserveState(opts *bind.CallOpts) error {
	logger := p.logger.New("contract", celoTokenProcessorInfos[p.token].contractName)
	totalSupply, err := p.tokenContract.TotalSupply(opts)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "totalSupply", "totalSupply", totalSupply)

	return nil
}

func (p celoTokenProcessor) ObserveMetric(opts *bind.CallOpts) error {
	totalSupply, err := p.tokenContract.TotalSupply(opts)
	if err != nil {
		return err
	}
	celoTokenProcessorInfos[p.token].totalSupplyMetric.Set(utils.ScaleFixed(totalSupply))
	return nil
}
