package indexer

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/common"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/celotokens"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/contracts/helpers"
	"github.com/celo-org/kliento/registry"
	"github.com/go-errors/errors"
)

type sortedOraclesProcessorFactory struct{}

func (sortedOraclesProcessorFactory) InitProcessors(
	ctx context.Context,
	handler *blockHandler,
) ([]Processor, error) {
	sortedOracles, err := handler.registry.GetSortedOraclesContract(ctx, handler.blockNumber)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	stableTokenAddresses, err := handler.celoTokens.GetAddresses(ctx, handler.blockNumber, true)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	exchanges, err := handler.celoTokens.GetExchangeContracts(ctx, handler.blockNumber)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}

	processors := make([]Processor, 0, 10)

	for stableToken, stableTokenAddress := range stableTokenAddresses {
		exchange, ok := exchanges[stableToken]
		// Stable tokens show up before they are fully activated
		if !ok || exchange == nil {
			continue
		}
		registryID, err := celotokens.GetRegistryID(stableToken)
		if err != nil {
			return nil, errors.Wrap(err, 1)
		}

		processors = append(processors, &sortedOraclesProcessor{
			blockHandler: handler,
			logger: handler.logger.New(
				"processor", "sortedOracles",
				"contract", "SortedOracles",
				"token", stableToken,
			),
			address:            stableTokenAddress,
			exchange:           exchange,
			exchangeRegistryID: registryID,
			stableToken:        stableToken,
			sortedOracles:      sortedOracles,
		})
	}

	return processors, nil
}

type sortedOraclesProcessor struct {
	*blockHandler
	logger log.Logger

	sortedOracles      *contracts.SortedOracles
	address            common.Address
	exchange           *contracts.Exchange
	exchangeRegistryID registry.ContractID
	stableToken        celotokens.CeloToken
}

func (proc *sortedOraclesProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

func (proc *sortedOraclesProcessor) ShouldCollect() bool {
	// This processor will run every block
	return true
}

func (proc sortedOraclesProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	contractRow := proc.blockRow.Contract(
		"SortedOracles",
		"stableToken", proc.stableToken,
		"address", proc.address,
	).AppendID(proc.address.String())

	// SortedOracles.IsOldestReportExpired
	isOldestReportExpired, lastReportAddress, err := proc.sortedOracles.IsOldestReportExpired(opts, proc.address)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall(
		"IsOldestReportExpired",
		"isOldestReportExpired", isOldestReportExpired,
		"lastReportAddress", lastReportAddress)

	// SortedOracles.NumRates
	numRates, err := proc.sortedOracles.NumRates(opts, proc.address)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall("NumRates", "numRates", numRates)

	// SortedOracles.NumRates
	medianRateNumerator, medianRateDenominator, err := proc.sortedOracles.MedianRate(opts, proc.address)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall(
		"MedianRate",
		"medianRateNumerator", medianRateNumerator,
		"medianRateDenominator", medianRateDenominator,
	)

	// SortedOracles.MedianTimestamp
	medianTimestamp, err := proc.sortedOracles.MedianTimestamp(opts, proc.address)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall("MedianTimestamp", "medianTimestamp", medianTimestamp)

	// SortedOracles.GetRates
	rateAddresses, rateValues, medianRelations, err := proc.sortedOracles.GetRates(opts, proc.address)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	for i, rateAddress := range rateAddresses {
		rows <- contractRow.ViewCall(
			"GetRates",
			"rateAddress", rateAddress,
			"rateValue", helpers.FromFixed(rateValues[i]),
			"medianRelation", medianRelations[i],
			"index", i,
		).AppendID(fmt.Sprintf("%d", i))
	}

	// SortedOracles.GetTimestamps
	timestampAddresses, timestamp, medianRelations, err := proc.sortedOracles.GetTimestamps(opts, proc.address)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	for i, timestampAddress := range timestampAddresses {
		rows <- contractRow.ViewCall(
			"GetTimestamps",
			"timestampAddress", timestampAddress,
			"reportedTimestamp", timestamp[i],
			"medianRelation", medianRelations[i],
			"index", i,
		).AppendID(fmt.Sprintf("%d", i))
	}

	celoBucketSize, stableBucketSize, err := proc.exchange.GetBuyAndSellBuckets(opts, true)

	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- proc.blockRow.Contract(string(proc.exchangeRegistryID)).ViewCall(
		"getBuyAndSellBuckets",
		"celoBucketSize", celoBucketSize,
		"stableBucketSize", stableBucketSize,
	)

	return nil
}

func (proc sortedOraclesProcessor) ObserveMetrics(ctx context.Context) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	stableTokenStr := string(proc.stableToken)
	isOldestReportExpired, _, err := proc.sortedOracles.IsOldestReportExpired(opts, proc.address)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	sortedOraclesIsOldestReportExpiredGauge, err := metrics.SortedOraclesIsOldestReportExpired.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	sortedOraclesIsOldestReportExpiredGauge.Set(utils.BoolToFloat64(isOldestReportExpired))

	numRates, err := proc.sortedOracles.NumRates(opts, proc.address)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	sortedOraclesNumRatesGauge, err := metrics.SortedOraclesNumRates.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	sortedOraclesNumRatesGauge.Set(float64(numRates.Uint64()))

	medianRateNumerator, medianRateDenominator, err := proc.sortedOracles.MedianRate(opts, proc.address)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	medianRate := big.NewFloat(0)

	if medianRateDenominator.Cmp(big.NewInt(0)) != 0 {
		retN := new(big.Float).SetInt(medianRateNumerator)
		retD := new(big.Float).SetInt(medianRateDenominator)
		medianRate = new(big.Float).Quo(retN, retD)
	}
	medianRateMetric, _ := medianRate.Float64()

	sortedOraclesMedianRateGauge, err := metrics.SortedOraclesMedianRate.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	sortedOraclesMedianRateGauge.Set(medianRateMetric)

	rateAddresses, rateValues, _, err := proc.sortedOracles.GetRates(opts, proc.address)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	mean := utils.MeanFromFixed(rateValues)
	maxDiff := 0.0

	for _, rateValue := range rateValues {
		diff := math.Abs(helpers.FromFixed(rateValue)/mean - 1)
		if diff > maxDiff {
			maxDiff = diff
		}
	}
	sortedOraclesRateMaxDeviationGauge, err := metrics.SortedOraclesRateMaxDeviation.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	sortedOraclesRateMaxDeviationGauge.Set(maxDiff)

	medianTimestamp, err := proc.sortedOracles.MedianTimestamp(opts, proc.address)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	sortedOraclesMedianTimestampGauge, err := metrics.SortedOraclesMedianTimestamp.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if proc.block != nil {
		blockTime := proc.block.Time()
		sortedOraclesMedianTimestampGauge.Set(float64(blockTime - medianTimestamp.Uint64()))
	}

	// Export each individual report value as a metric.
	for i := range rateValues {
		sortedOraclesReportValue, err := metrics.SortedOraclesReportValue.GetMetricWithLabelValues(stableTokenStr, rateAddresses[i].String())
		if err != nil {
			return errors.Wrap(err, 0)
		}
		sortedOraclesReportValue.Set(helpers.FromFixed(rateValues[i]))
	}

	celoBucketSize, stableBucketSize, err := proc.exchange.GetBuyAndSellBuckets(opts, true)

	if err != nil {
		return errors.Wrap(err, 0)
	}

	celoStablePrice := utils.DivideBigInts(celoBucketSize, stableBucketSize)

	stablePrice := big.NewFloat(0)

	if celoStablePrice.Cmp(big.NewFloat(0)) != 0 {
		stablePrice = new(big.Float).Quo(medianRate, celoStablePrice)
	}

	stablePriceF, _ := stablePrice.Float64()
	exchangeImpliedStableRateGauge, err := metrics.ExchangeImpliedStableRate.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	exchangeImpliedStableRateGauge.Set(stablePriceF)
	return nil
}
