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
	// Not super important right now

	reserveFraction, err := p.exchange.ReserveFraction(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "contract", "Exchange", "method", "reserveFraction", "fraction", reserveFraction)

	goldBucketSize, err := p.exchange.GoldBucket(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "contract", "Exchange", "method", "goldBucket", "bucket", goldBucketSize)

	unfrozenReserveGoldBalance, err := p.reserve.GetUnfrozenReserveGoldBalance(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "contract", "Reserve", "method", "getUnfrozenReserveGoldBalance", "vaule", goldBucketSize)

	res := big.Float{}
	res.Quo(new(big.Float).SetInt(goldBucketSize), new(big.Float).SetInt(unfrozenReserveGoldBalance))

	ret, _ := res.Float64()
	metrics.ExchangeGoldBucketRatio.Observe(ret)

	return nil
}

func (p stabilityProcessor) HandleLog(eventLog *types.Log) {

}
