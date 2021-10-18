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

type stableTokenProcessorFactory struct{}

func (stableTokenProcessorFactory) InitProcessors(
	ctx context.Context,
	handler *blockHandler,
) ([]Processor, error) {
	stableToken, err := handler.registry.GetStableTokenContract(ctx, handler.blockNumber)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}

	return []Processor{
		&stableTokenProcessor{
			blockHandler: handler,
			logger:       handler.logger.New("processor", "stableToken", "contract", "StableToken"),
			stableToken: stableToken,
		},
	}, nil
}

type stableTokenProcessor struct {
	*blockHandler
	logger       log.Logger
	stableToken *contracts.StableToken
}

func (proc *stableTokenProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

func (proc *stableTokenProcessor) ShouldCollect() bool {
	// TODO: How often should it collect the data? Per Block? Per Epoch?
	// This processor will collect data every block
	return true
	// This processor will only collect data at the end of the epoch
	//return utils.ShouldSample(proc.blockNumber.Uint64(), EpochSize)
}

func (proc stableTokenProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	contractRow := proc.blockRow.Extend("contract", "StableToken")

	// StableToken.GetInflationParameters
	inflationFactor, inflationRate, updatePeriod, factorLastUpdated, err := proc.stableToken.GetInflationParameters(opts)
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
	totalSupply, err := proc.stableToken.TotalSupply(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall(
		"totalSupply",
		"stableTokenTotalSupply",totalSupply.String(),
	)
	return nil
}

func (proc stableTokenProcessor) ObserveMetrics(ctx context.Context) error {
	/*
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}
	*/
	// no metrics for now
	return nil
}