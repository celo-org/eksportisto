package indexer

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/celotokens"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/registry"
	"github.com/go-errors/errors"
	"github.com/prometheus/client_golang/prometheus"
)

type celoTokenProcessorFactory struct{}

func (celoTokenProcessorFactory) InitProcessors(
	ctx context.Context,
	handler *blockHandler,
) ([]Processor, error) {
	processors := make([]Processor, 0, 20)
	celoTokenContracts, err := handler.celoTokens.GetContracts(ctx, handler.blockNumber, false)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}

	for token, contract := range celoTokenContracts {
		// If a token's contract has not been registered with the Registry
		// yet, the contract will be nil. Ignore this.
		if contract == nil {
			continue
		}

		tokenRegistryID, err := celotokens.GetRegistryID(token)
		if err != nil {
			return nil, errors.Wrap(err, 1)
		}

		totalSupplyGauge, err := metrics.CeloTokenSupply.GetMetricWithLabelValues(string(token))
		if err != nil {
			return nil, errors.Wrap(err, 1)
		}

		processors = append(processors, &celoTokenProcessor{
			blockHandler:     handler,
			logger:           handler.logger.New("processor", "celoToken", "contract", string(tokenRegistryID)),
			token:            token,
			tokenContract:    contract,
			tokenRegistryID:  tokenRegistryID,
			totalSupplyGauge: totalSupplyGauge,
		})
	}

	return processors, nil
}

type celoTokenProcessor struct {
	*blockHandler
	logger           log.Logger
	token            celotokens.CeloToken
	tokenContract    contracts.CeloTokenContract
	tokenRegistryID  registry.ContractID
	totalSupplyGauge prometheus.Gauge
}

func (proc *celoTokenProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

func (proc *celoTokenProcessor) ShouldCollect() bool {
	// This processor will run once per hour
	return utils.ShouldSample(proc.blockNumber.Uint64(), BlocksPerHour)
}

func (proc celoTokenProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	contractRow := proc.blockRow.Extend("contract", proc.tokenRegistryID)

	totalSupply, err := proc.tokenContract.TotalSupply(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall("totalSupply", "totalSupply", totalSupply.String())
	return nil
}

func (proc celoTokenProcessor) ObserveMetrics(ctx context.Context) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	totalSupply, err := proc.tokenContract.TotalSupply(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	proc.totalSupplyGauge.Set(utils.ScaleFixed(totalSupply))
	return nil
}
