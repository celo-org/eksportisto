package monitor

import (
	"context"
	"math/big"

	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type stabilityProcessor struct {
	ctx      context.Context
	logger   log.Logger
	exchange *contracts.Exchange
	reserve  *contracts.Reserve
}

func NewStabilityProcessor(ctx context.Context, logger log.Logger, exchange *contracts.Exchange, reserve *contracts.Reserve) *stabilityProcessor {
	return &stabilityProcessor{
		ctx:      ctx,
		logger:   logger,
		exchange: exchange,
		reserve:  reserve,
	}
}

func (p stabilityProcessor) ObserveState(opts *bind.CallOpts) error {
	// Exchange.ReserveFraction
	_, err := p.exchange.ReserveFraction(opts)
	if err != nil {
		return err
	}

	// TODO: This is a fraction and not actually an uint
	logStateViewCall(p.logger, "contract", "Exchange", "method", "reserveFraction", "fraction", reserveFraction.Uint64())

	// Exchange.goldBucket
	goldBucketSize, err := p.exchange.GoldBucket(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "contract", "Exchange", "method", "goldBucket", "bucket", goldBucketSize)

	// Reserve.getReserveGoldBalance
	reserveGoldBalance, err := p.reserve.GetReserveGoldBalance(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "contract", "Reserve", "method", "getReserveGoldBalance", "reserveGoldBalance", reserveGoldBalance)

	// Reserve.getOtherReserveAddressesGoldBalance
	otherReserveAddressesGoldBalance, err := p.reserve.GetOtherReserveAddressesGoldBalance(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "contract", "Reserve", "method", "getOtherReserveAddressesGoldBalance", "otherReserveAddressesGoldBalance", otherReserveAddressesGoldBalance)

	// Reserve.getUnfrozenBalance
	unfrozenBalance, err := p.reserve.GetUnfrozenBalance(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "contract", "Reserve", "method", "getUnfrozenBalance", "value", unfrozenBalance)

	// Reserve.getFrozenReserveGoldBalance
	frozenReserveGoldBalance, err := p.reserve.GetFrozenReserveGoldBalance(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "contract", "Reserve", "method", "getFrozenReserveGoldBalance", "value", frozenReserveGoldBalance)

	// Reserve.getUnfrozenReserveGoldBalance
	unfrozenReserveGoldBalance, err := p.reserve.GetUnfrozenReserveGoldBalance(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "contract", "Reserve", "method", "getUnfrozenReserveGoldBalance", "value", unfrozenReserveGoldBalance)

	return nil
}

func (p stabilityProcessor) ObserveMetric(opts *bind.CallOpts) error {
	goldBucketSize, err := p.exchange.GoldBucket(opts)
	if err != nil {
		return err
	}

	unfrozenReserveGoldBalance, err := p.reserve.GetUnfrozenReserveGoldBalance(opts)
	if err != nil {
		return err
	}

	// If the unfrozen balance is 0, ignore for now
	if unfrozenReserveGoldBalance.Cmp(big.NewInt(0)) == 0 {
		return nil
	}

	res := big.Float{}
	res.Quo(new(big.Float).SetInt(goldBucketSize), new(big.Float).SetInt(unfrozenReserveGoldBalance))

	ret, _ := res.Float64()
	metrics.ExchangeGoldBucketRatio.Observe(ret)
	return nil
}

func (p stabilityProcessor) HandleLog(eventLog *types.Log) {

}
