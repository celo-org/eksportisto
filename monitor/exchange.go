package monitor

import (
	"context"
	"math/big"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/celotokens"
	"github.com/celo-org/kliento/contracts"
	"github.com/prometheus/client_golang/prometheus"
)

type exchangeProcessor struct {
	ctx                              context.Context
	logger                           log.Logger
	stableToken                      celotokens.CeloToken
	exchange                         *contracts.Exchange
	reserve                          *contracts.Reserve
	exchangeCeloBucketRatioHistogram prometheus.Observer
	exchangeCeloBucketSizeGauge      prometheus.Gauge
	exchangeStableBucketSizeGauge    prometheus.Gauge
}

func NewExchangeProcessor(ctx context.Context, logger log.Logger, stableToken celotokens.CeloToken, exchange *contracts.Exchange, reserve *contracts.Reserve) (*exchangeProcessor, error) {
	exchangeCeloBucketRatioHistogram, err := metrics.ExchangeCeloBucketRatio.GetMetricWithLabelValues(string(stableToken))
	if err != nil {
		return nil, err
	}
	exchangeCeloBucketSizeGauge, err := metrics.ExchangeCeloBucketSize.GetMetricWithLabelValues(string(stableToken))
	if err != nil {
		return nil, err
	}
	exchangeStableBucketSizeGauge, err := metrics.ExchangeStableBucketSize.GetMetricWithLabelValues(string(stableToken))
	if err != nil {
		return nil, err
	}
	return &exchangeProcessor{
		ctx:                              ctx,
		logger:                           logger.New("stableToken", stableToken),
		stableToken:                      stableToken,
		exchange:                         exchange,
		reserve:                          reserve,
		exchangeCeloBucketRatioHistogram: exchangeCeloBucketRatioHistogram,
		exchangeCeloBucketSizeGauge:      exchangeCeloBucketSizeGauge,
		exchangeStableBucketSizeGauge:    exchangeStableBucketSizeGauge,
	}, nil
}

func (p exchangeProcessor) ObserveState(opts *bind.CallOpts) error {
	// Exchange.ReserveFraction
	reserveFraction, err := p.exchange.ReserveFraction(opts)
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

	cUsdBucketSize, err := p.exchange.StableBucket(opts)

	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "contract", "Exchange", "method", "stableBucket", "bucket", cUsdBucketSize)

	return nil
}

func (p exchangeProcessor) ObserveMetric(opts *bind.CallOpts) error {
	goldBucketSize, err := p.exchange.GoldBucket(opts)
	if err != nil {
		return err
	}

	p.exchangeCeloBucketSizeGauge.Set(utils.ScaleFixed(goldBucketSize))

	cUsdBucketSize, err := p.exchange.StableBucket(opts)
	if err != nil {
		return err
	}

	p.exchangeStableBucketSizeGauge.Set(utils.ScaleFixed(cUsdBucketSize))

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
	p.exchangeCeloBucketRatioHistogram.Observe(ret)
	return nil
}
