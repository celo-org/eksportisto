package indexer

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	//"github.com/celo-org/eksportisto/utils"
	//"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/contracts/helpers"
	"github.com/celo-org/kliento/registry"
	"github.com/go-errors/errors"
)

type stableTokenEURProcessorFactory struct{}

func (stableTokenEURProcessorFactory) InitProcessors(
	ctx context.Context,
	handler *blockHandler,
) ([]Processor, error) {
	stableTokenEUR, err := handler.registry.GetStableTokenEURContract(ctx, handler.blockNumber)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}

	return []Processor{
		&stableTokenEURProcessor{
			blockHandler: handler,
			logger:       handler.logger.New("processor", "stableTokenEUR", "contract", "StableTokenEUR"),
			stableTokenEUR: stableTokenEUR,
		},
	}, nil
}

type stableTokenEURProcessor struct {
	*blockHandler
	logger       log.Logger
	stableTokenEUR *contracts.StableToken
}

func (proc *stableTokenEURProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

func (proc *stableTokenEURProcessor) ShouldCollect() bool {
	// TODO: How often should it collect the data? Per Block? Per Epoch?
	// This processor will collect data every block
	return true
	// This processor will only collect data at the end of the epoch
	//return utils.ShouldSample(proc.blockNumber.Uint64(), EpochSize)
}

func (proc stableTokenEURProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	contractRow := proc.blockRow.Extend("contract", "StableTokenEUR")

	// StableTokenEUR.GetInflationParameters
	inflationFactor, inflationRate, updatePeriod, factorLastUpdated, err := proc.stableTokenEUR.GetInflationParameters(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall(
		"getInflationParameters",
		"inflationFactor",helpers.FromFixed(inflationFactor),
		"inflationRate",helpers.FromFixed(inflationRate),
		"updatePeriod", updatePeriod,
		"factorLastUpdated", factorLastUpdated,
	)
	// StableTokenEUR.TotalSupply()
	totalSupply, err := proc.stableTokenEUR.TotalSupply(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall(
		"totalSupply",
		"stableTokenTotalSupply",totalSupply.String(),
	)
	return nil
}

func (proc stableTokenEURProcessor) ObserveMetrics(ctx context.Context) error {
	/*
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}
	*/
	// no metrics for now
	return nil
}