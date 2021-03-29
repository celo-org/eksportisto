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
	"github.com/celo-org/kliento/registry"
	"github.com/prometheus/client_golang/prometheus"
)

type exchangeProcessor struct {
	ctx                              context.Context
	logger                           log.Logger
	stableToken                      celotokens.CeloToken
	exchange                         *contracts.Exchange
	reserve                          *contracts.Reserve
	exchangeBucketRatioGauge         prometheus.Gauge
	exchangeCeloBucketRatioHistogram prometheus.Observer
	exchangeCeloBucketSizeGauge      prometheus.Gauge
	exchangeCeloExchangedRateGauge   prometheus.Gauge
	exchangeStableBucketSizeGauge    prometheus.Gauge
}

func NewExchangeProcessor(ctx context.Context, logger log.Logger, stableToken celotokens.CeloToken, exchangeRegistryID registry.ContractID, exchange *contracts.Exchange, reserve *contracts.Reserve) (*exchangeProcessor, error) {
	stableTokenStr := string(stableToken)
	exchangeBucketRatioGauge, err := metrics.ExchangeBucketRatio.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return nil, err
	}
	exchangeCeloBucketRatioHistogram, err := metrics.ExchangeCeloBucketRatio.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return nil, err
	}
	exchangeCeloBucketSizeGauge, err := metrics.ExchangeCeloBucketSize.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return nil, err
	}
	exchangeCeloExchangedRateGauge, err := metrics.ExchangeCeloExchangedRate.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return nil, err
	}
	exchangeStableBucketSizeGauge, err := metrics.ExchangeStableBucketSize.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return nil, err
	}
	return &exchangeProcessor{
		ctx:                              ctx,
		logger:                           logger.New("stableToken", stableToken, "contract", string(exchangeRegistryID)),
		stableToken:                      stableToken,
		exchange:                         exchange,
		reserve:                          reserve,
		exchangeBucketRatioGauge:         exchangeBucketRatioGauge,
		exchangeCeloBucketRatioHistogram: exchangeCeloBucketRatioHistogram,
		exchangeCeloBucketSizeGauge:      exchangeCeloBucketSizeGauge,
		exchangeCeloExchangedRateGauge:   exchangeCeloExchangedRateGauge,
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
	logStateViewCall(p.logger, "method", "reserveFraction", "fraction", reserveFraction.Uint64())

	// Exchange.goldBucket
	goldBucketSize, err := p.exchange.GoldBucket(opts)
	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "method", "goldBucket", "bucket", goldBucketSize)

	cUsdBucketSize, err := p.exchange.StableBucket(opts)

	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "method", "stableBucket", "bucket", cUsdBucketSize)

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

func (p exchangeProcessor) HandleEvent(parsedLog *registry.RegistryParsedLog) {
	// eventName, eventRaw, ok, err := p.exchange.TryParseLog(*eventLog)
	// if err != nil {
	// 	p.logger.Warn("Ignoring event: Error parsing exchange event", "err", err, "eventId", eventLog.Topics[0].Hex())
	// 	return
	// }
	// if !ok {
	// 	return
	// }

	switch parsedLog.Event {
	case "Exchanged":
		event := parsedLog.Log.(contracts.ExchangeExchanged)

		// Prevent updating the ExchangedRate metric for small trades that do not provide enough precision when calculating the effective price
		minSellAmountInWei := big.NewInt(1e6)
		if event.SellAmount.Cmp(minSellAmountInWei) < 0 {
			return
		}

		num := event.SellAmount
		dem := event.BuyAmount

		if event.SoldGold {
			num = event.BuyAmount
			dem = event.SellAmount
		}

		celoPrice := utils.DivideBigInts(num, dem)
		celoPriceF, _ := celoPrice.Float64()
		p.exchangeCeloExchangedRateGauge.Set(celoPriceF)
	case "BucketsUpdated":
		event := parsedLog.Log.(contracts.ExchangeBucketsUpdated)

		bucketRatio := utils.DivideBigInts(event.StableBucket, event.GoldBucket)
		bucketRatioF, _ := bucketRatio.Float64()
		p.exchangeBucketRatioGauge.Set(bucketRatioF)
	}
}
