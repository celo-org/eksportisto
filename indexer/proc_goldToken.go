package indexer

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/registry"
	"github.com/go-errors/errors"
)

type goldTokenProcessorFactory struct{}

func (goldTokenProcessorFactory) InitProcessors(
	ctx context.Context,
	handler *blockHandler,
) ([]Processor, error) {
	goldToken, err := handler.registry.GetGoldTokenContract(ctx, handler.blockNumber)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}

	return []Processor{
		&goldTokenProcessor{
			blockHandler: handler,
			logger:       handler.logger.New("processor", "goldToken", "contract", "GoldToken"),
			goldToken: goldToken,
		},
	}, nil
}

type goldTokenProcessor struct {
	*blockHandler
	logger       log.Logger
	goldToken *contracts.GoldToken
}

func (proc *goldTokenProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

func (proc *goldTokenProcessor) ShouldCollect() bool {
	// This processor will collect data every block
	return true
}

func (proc goldTokenProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	contractRow := proc.blockRow.Extend("contract", "GoldToken")

	// GoldToken.TotalSupply()
	totalSupply, err := proc.goldToken.TotalSupply(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	rows <- contractRow.ViewCall(
		"totalSupply",
		"totalSupply",totalSupply.String(),
	)
	return nil
}

func (proc goldTokenProcessor) ObserveMetrics(ctx context.Context) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	totalSupplyGauge, err := metrics.CeloTokenSupply.GetMetricWithLabelValues("GoldToken")
	if err != nil {
		return errors.Wrap(err, 1)
	}
	totalSupply, err := proc.goldToken.TotalSupply(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	totalSupplyGauge.Set(utils.ScaleFixed(totalSupply))
	return nil
}