package indexer

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/celotokens"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/contracts/helpers"
	"github.com/celo-org/kliento/registry"
	"github.com/go-errors/errors"
	"github.com/prometheus/client_golang/prometheus"
)

type stableTokenProcessorFactory struct{}

func (stableTokenProcessorFactory) InitProcessors(
	ctx context.Context,
	handler *blockHandler,
) ([]Processor, error) {
	processors := make([]Processor, 0, 20)

	celoTokenContracts, err := handler.celoTokens.GetContracts(ctx, handler.blockNumber, true)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}

	for stableToken, celoTokenContract := range celoTokenContracts {
		// If a token's contract has not been registered with the Registry
		// yet, the contract will be nil. Ignore this.
		if celoTokenContract == nil {
			continue
		}
		stableTokenRegistryID, err := celotokens.GetRegistryID(stableToken)
		if err != nil {
			return nil, errors.Wrap(err, 1)
		}

		stableTokenContract, err := handler.registry.GetContractByID(ctx, string(stableTokenRegistryID), handler.blockNumber)
		if err != nil {
			return nil, errors.Wrap(err, 1)
		}

		var stableContractVariable = stableTokenContract.(*contracts.StableToken)

		totalSupplyGauge, err := metrics.CeloTokenSupply.GetMetricWithLabelValues(string(stableToken))
		if err != nil {
			return nil, errors.Wrap(err, 1)
		}

		processors = append(processors, &stableTokenProcessor{
			blockHandler:          handler,
			logger:                handler.logger.New("processor", "stableToken", "contract", string(stableTokenRegistryID)),
			stableTokenContract:   stableContractVariable,
			stableTokenRegistryID: stableTokenRegistryID,
			totalSupplyGauge:      totalSupplyGauge,
		})
	}
	return processors, nil
}

type stableTokenProcessor struct {
	*blockHandler
	logger                log.Logger
	stableTokenContract   *contracts.StableToken
	stableTokenRegistryID registry.ContractID
	totalSupplyGauge      prometheus.Gauge
}

func (proc *stableTokenProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

func (proc *stableTokenProcessor) ShouldCollect() bool {
	// This processor will run every block
	return true
}

func (proc stableTokenProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	proc.logger.Info("StableToken.CollectData")
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	contractRow := proc.blockRow.Extend("contract", proc.stableTokenRegistryID)

	// stableToken.totalSupply()
	totalSupply, err := proc.stableTokenContract.TotalSupply(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case rows <- contractRow.ViewCall("totalSupply", "totalSupply", totalSupply.String()):
	}

	// StableToken.GetInflationParameters()
	inflationFactor, inflationRate, updatePeriod, factorLastUpdated, err := proc.stableTokenContract.GetInflationParameters(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case rows <- contractRow.ViewCall(
		"getInflationParameters",
		"inflationFactor", helpers.FromFixed(inflationFactor),
		"inflationRate", helpers.FromFixed(inflationRate),
		"updatePeriod", updatePeriod,
		"factorLastUpdated", factorLastUpdated,
	):
	}
	return nil
}

func (proc stableTokenProcessor) ObserveMetrics(ctx context.Context) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	totalSupply, err := proc.stableTokenContract.TotalSupply(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	proc.totalSupplyGauge.Set(utils.ScaleFixed(totalSupply))
	return nil
}
