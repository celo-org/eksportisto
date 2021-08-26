package indexer

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

type exchangeProcessorFactory struct{}

func (exchangeProcessorFactory) InitProcessors(
	ctx context.Context,
	handler *blockHandler,
) ([]Processor, error) {
	processors := make([]Processor, 0, 20)
	exchangeContracts, err := handler.celoTokens.GetExchangeContracts(ctx, handler.blockNumber)
	if err != nil {
		return nil, err
	}
	reserve, err := handler.registry.GetReserveContract(ctx, handler.blockNumber)
	if err != nil {
		return nil, err
	}

	for stableToken, exchangeContract := range exchangeContracts {
		exchangeRegistryID, err := celotokens.GetExchangeRegistryID(stableToken)
		if err != nil {
			return nil, err
		}

		if exchangeContract == nil {
			continue
		}

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

		processors = append(processors, &exchangeProcessor{
			blockHandler: handler,
			logger: handler.logger.New(
				"stableToken", stableToken,
				"contract", string(exchangeRegistryID),
			),
			stableToken:                      stableToken,
			exchange:                         exchangeContract,
			exchangeRegistryID:               exchangeRegistryID,
			reserve:                          reserve,
			exchangeBucketRatioGauge:         exchangeBucketRatioGauge,
			exchangeCeloBucketRatioHistogram: exchangeCeloBucketRatioHistogram,
			exchangeCeloBucketSizeGauge:      exchangeCeloBucketSizeGauge,
			exchangeCeloExchangedRateGauge:   exchangeCeloExchangedRateGauge,
			exchangeStableBucketSizeGauge:    exchangeStableBucketSizeGauge,
		})
	}

	return processors, nil
}

type exchangeProcessor struct {
	*blockHandler
	logger                           log.Logger
	stableToken                      celotokens.CeloToken
	exchange                         *contracts.Exchange
	reserve                          *contracts.Reserve
	exchangeBucketRatioGauge         prometheus.Gauge
	exchangeCeloBucketRatioHistogram prometheus.Observer
	exchangeCeloBucketSizeGauge      prometheus.Gauge
	exchangeCeloExchangedRateGauge   prometheus.Gauge
	exchangeStableBucketSizeGauge    prometheus.Gauge
	exchangeRegistryID               registry.ContractID
}

func (proc *exchangeProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return proc.exchangeRegistryID, proc
}

func (proc *exchangeProcessor) ShouldCollect() bool {
	// This processor will only collect data at the end of the epoch
	return utils.ShouldSample(proc.blockNumber.Uint64(), EpochSize)
}

func (proc exchangeProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	contractRow := proc.blockRow.Extend("contract", proc.exchangeRegistryID)

	// Exchange.ReserveFraction
	reserveFraction, err := proc.exchange.ReserveFraction(opts)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall(
		"reserveFraction",
		"fraction", reserveFraction.Uint64(),
	)

	// Exchange.goldBucket
	goldBucketSize, err := proc.exchange.GoldBucket(opts)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall(
		"goldBucket",
		"fraction", goldBucketSize.String(),
	)

	stableBucketSize, err := proc.exchange.StableBucket(opts)

	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall(
		"stableBucket",
		"fraction", stableBucketSize.String(),
	)

	return nil
}

func (proc exchangeProcessor) ObserveMetrics(ctx context.Context) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	goldBucketSize, err := proc.exchange.GoldBucket(opts)
	if err != nil {
		return err
	}

	proc.exchangeCeloBucketSizeGauge.Set(utils.ScaleFixed(goldBucketSize))

	cUsdBucketSize, err := proc.exchange.StableBucket(opts)
	if err != nil {
		return err
	}

	proc.exchangeStableBucketSizeGauge.Set(utils.ScaleFixed(cUsdBucketSize))

	unfrozenReserveGoldBalance, err := proc.reserve.GetUnfrozenReserveGoldBalance(opts)

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
	proc.exchangeCeloBucketRatioHistogram.Observe(ret)
	return nil
}

func (proc exchangeProcessor) HandleEvent(parsedLog *registry.RegistryParsedLog) {
	switch parsedLog.Event {
	case "Exchanged":
		event := parsedLog.Log.(*contracts.ExchangeExchanged)

		// Prevent updating the ExchangedRate metric for small trades that do not provide enough precision when calculating the effective price
		minSellAmountInWei := big.NewInt(1e6)
		if event.SellAmount.Cmp(minSellAmountInWei) < 0 {
			return
		}

		var num, dem *big.Int

		if event.SoldGold {
			num = event.BuyAmount
			dem = event.SellAmount
		} else {
			num = event.SellAmount
			dem = event.BuyAmount
		}

		celoPrice := utils.DivideBigInts(num, dem)
		celoPriceF, _ := celoPrice.Float64()
		proc.exchangeCeloExchangedRateGauge.Set(celoPriceF)
	case "BucketsUpdated":
		event := parsedLog.Log.(*contracts.ExchangeBucketsUpdated)

		bucketRatio := utils.DivideBigInts(event.StableBucket, event.GoldBucket)
		bucketRatioF, _ := bucketRatio.Float64()
		proc.exchangeBucketRatioGauge.Set(bucketRatioF)
	}
}
