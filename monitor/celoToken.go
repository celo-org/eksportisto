package monitor

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/celotokens"
	"github.com/celo-org/kliento/contracts"
	"github.com/prometheus/client_golang/prometheus"
)

type celoTokenProcessorInfo struct {
	contractName string
	// totalSupplyMetric prometheus.Gauge
}

var celoTokenContractNames = map[celotokens.CeloToken]string{
	celotokens.CELO: "GoldToken",
	celotokens.CUSD: "StableToken",
}

type celoTokenProcessor struct {
	ctx              context.Context
	logger           log.Logger
	token            celotokens.CeloToken
	tokenContract    contracts.CeloTokenContract
	totalSupplyGauge prometheus.Gauge
}

func NewCeloTokenProcessor(ctx context.Context, logger log.Logger, token celotokens.CeloToken, tokenContract contracts.CeloTokenContract) (*celoTokenProcessor, error) {
	totalSupplyGauge, err := metrics.CeloTokenSupply.GetMetricWithLabelValues(string(token))

	if err != nil {
		return nil, err
	}
	return &celoTokenProcessor{
		ctx:              ctx,
		logger:           logger.New("contract", celoTokenContractNames[token]),
		token:            token,
		tokenContract:    tokenContract,
		totalSupplyGauge: totalSupplyGauge,
	}, nil
}

func (p celoTokenProcessor) ObserveState(opts *bind.CallOpts) error {
	totalSupply, err := p.tokenContract.TotalSupply(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "method", "totalSupply", "totalSupply", totalSupply)

	return nil
}

func (p celoTokenProcessor) ObserveMetric(opts *bind.CallOpts) error {
	totalSupply, err := p.tokenContract.TotalSupply(opts)
	if err != nil {
		return err
	}
	p.totalSupplyGauge.Set(utils.ScaleFixed(totalSupply))
	return nil
}
