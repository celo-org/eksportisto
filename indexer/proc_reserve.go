package indexer

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/contracts/helpers"
	"github.com/celo-org/kliento/registry"
	"github.com/go-errors/errors"
)

type reserveProcessorFactory struct{}

func (reserveProcessorFactory) InitProcessors(
	ctx context.Context,
	handler *blockHandler,
) ([]Processor, error) {
	reserve, err := handler.registry.GetReserveContract(ctx, handler.blockNumber)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	return []Processor{
		&reserveProcessor{
			blockHandler: handler,
			logger:       handler.logger.New("processor", "reserve", "contract", "Reserve"),
			reserve:      reserve,
		},
	}, nil
}

type reserveProcessor struct {
	*blockHandler
	logger  log.Logger
	reserve *contracts.Reserve
}

func (proc *reserveProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

func (proc *reserveProcessor) ShouldCollect() bool {
	// This processor will run once per hour or at epoch change
	return (utils.ShouldSample(proc.blockNumber.Uint64(), BlocksPerHour) ||
		utils.ShouldSample(proc.blockNumber.Uint64(), EpochSize))
}

func (proc reserveProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}
	contractRow := proc.blockRow.Extend("contract", "Reserve")

	reserveRatio, err := proc.reserve.GetReserveRatio(opts)

	// TODO: Properly handle when things are frozen
	if err != nil {
		return nil
	}

	rows <- contractRow.ViewCall("getReserveRatio", "reserveRatio", helpers.FromFixed(reserveRatio))

	// Reserve.getReserveGoldBalance
	reserveGoldBalance, err := proc.reserve.GetReserveGoldBalance(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall("getReserveGoldBalance", "reserveGoldBalance", reserveGoldBalance.String())

	// Reserve.getOtherReserveAddressesGoldBalance
	otherReserveAddressesGoldBalance, err := proc.reserve.GetOtherReserveAddressesGoldBalance(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall(
		"getOtherReserveAddressesGoldBalance",
		"otherReserveAddressesGoldBalance", otherReserveAddressesGoldBalance.String(),
	)

	// Reserve.getUnfrozenBalance
	unfrozenBalance, err := proc.reserve.GetUnfrozenBalance(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall("getUnfrozenBalance", "value", unfrozenBalance.String())

	// Reserve.getFrozenReserveGoldBalance
	frozenReserveGoldBalance, err := proc.reserve.GetFrozenReserveGoldBalance(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall("getFrozenReserveGoldBalance", "value", frozenReserveGoldBalance.String())

	// Reserve.getUnfrozenReserveGoldBalance
	unfrozenReserveGoldBalance, err := proc.reserve.GetUnfrozenReserveGoldBalance(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall("getUnfrozenReserveGoldBalance", "value", unfrozenReserveGoldBalance)

	return nil
}

func (proc reserveProcessor) ObserveMetrics(ctx context.Context) error {
	return nil
}
