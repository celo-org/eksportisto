package monitor

import (
	"context"
	"math"
	"math/big"

	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/celotokens"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/contracts/helpers"
	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/common"
	"github.com/celo-org/celo-blockchain/log"
)

type sortedOraclesProcessor struct {
	ctx           context.Context
	logger        log.Logger
	sortedOracles *contracts.SortedOracles
	exchange      *contracts.Exchange
}

func NewSortedOraclesProcessor(ctx context.Context, logger log.Logger, sortedOracles *contracts.SortedOracles, exchange *contracts.Exchange) *sortedOraclesProcessor {
	return &sortedOraclesProcessor{
		ctx:           ctx,
		logger:        logger.New("contract", "SortedOracles"),
		sortedOracles: sortedOracles,
		exchange:      exchange,
	}
}

func (p sortedOraclesProcessor) ObserveState(opts *bind.CallOpts, stableTokenAddresses map[celotokens.CeloToken]common.Address) error {
	for stableToken, stableTokenAddress := range stableTokenAddresses {
		err := p.observeStateForStableToken(opts, stableToken, stableTokenAddress)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p sortedOraclesProcessor) observeStateForStableToken(opts *bind.CallOpts, stableToken celotokens.CeloToken, stableTokenAddress common.Address) error {
	logger := p.logger.New("stableToken", stableToken, "address", stableTokenAddress)
	// SortedOracles.IsOldestReportExpired
	isOldestReportExpired, lastReportAddress, err := p.sortedOracles.IsOldestReportExpired(opts, stableTokenAddress)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "IsOldestReportExpired", "isOldestReportExpired", isOldestReportExpired)
	logStateViewCall(logger, "method", "IsOldestReportExpired", "lastReportAddress", lastReportAddress)

	// SortedOracles.NumRates
	numRates, err := p.sortedOracles.NumRates(opts, stableTokenAddress)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "NumRates", "numRates", numRates)

	// SortedOracles.NumRates
	medianRateNumerator, medianRateDenominator, err := p.sortedOracles.MedianRate(opts, stableTokenAddress)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "MedianRate", "medianRateNumerator", medianRateNumerator)
	logStateViewCall(logger, "method", "MedianRate", "medianRateDenominator", medianRateDenominator)

	// SortedOracles.MedianTimestamp
	medianTimestamp, err := p.sortedOracles.MedianTimestamp(opts, stableTokenAddress)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "MedianTimestamp", "medianTimestamp", medianTimestamp)

	// SortedOracles.GetRates
	rateAddresses, rateValues, medianRelations, err := p.sortedOracles.GetRates(opts, stableTokenAddress)
	if err != nil {
		return err
	}

	for i, rateAddress := range rateAddresses {
		logStateViewCall(logger, "method", "GetRates", "rateAddress", rateAddress, "rateValue", helpers.FromFixed(rateValues[i]), "medianRelation", medianRelations[i], "index", i)
	}

	// SortedOracles.GetTimestamps
	timestampAddresses, timestamp, medianRelations, err := p.sortedOracles.GetTimestamps(opts, stableTokenAddress)
	if err != nil {
		return err
	}

	for i, timestampAddress := range timestampAddresses {
		logStateViewCall(logger, "method", "GetTimestamps", "timestampAddress", timestampAddress, "reportedTimestamp", timestamp[i], "medianRelation", medianRelations[i], "index", i)
	}

	celoBucketSize, stableBucketSize, err := p.exchange.GetBuyAndSellBuckets(opts, true)

	if err != nil {
		return err
	}

	logStateViewCall(p.logger, "contract", "Exchange", "method", "getBuyAndSellBuckets", "celoBucketSize", celoBucketSize, "stableBucketSize", stableBucketSize)

	return nil
}

func (p sortedOraclesProcessor) ObserveMetric(opts *bind.CallOpts, stableTokenAddresses map[celotokens.CeloToken]common.Address, blockTime uint64) error {
	for _, stableTokenAddress := range stableTokenAddresses {
		err := p.observeMetricForStableToken(opts, stableTokenAddress, blockTime)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p sortedOraclesProcessor) observeMetricForStableToken(opts *bind.CallOpts, stableTokenAddress common.Address, blockTime uint64) error {
	isOldestReportExpired, _, err := p.sortedOracles.IsOldestReportExpired(opts, stableTokenAddress)
	if err != nil {
		return err
	}
	metrics.SortedOraclesIsOldestReportExpired.Set(utils.BoolToFloat64(isOldestReportExpired))

	numRates, err := p.sortedOracles.NumRates(opts, stableTokenAddress)
	if err != nil {
		return err
	}
	metrics.SortedOraclesNumRates.Set(float64(numRates.Uint64()))

	medianRateNumerator, medianRateDenominator, err := p.sortedOracles.MedianRate(opts, stableTokenAddress)
	if err != nil {
		return err
	}
	medianRate := big.NewFloat(0)

	if medianRateDenominator.Cmp(big.NewInt(0)) != 0 {
		retN := new(big.Float).SetInt(medianRateNumerator)
		retD := new(big.Float).SetInt(medianRateDenominator)
		medianRate = new(big.Float).Quo(retN, retD)
	}
	medianRateMetric, _ := medianRate.Float64()

	metrics.SortedOraclesMedianRate.Set(medianRateMetric)

	_, rateValues, _, err := p.sortedOracles.GetRates(opts, stableTokenAddress)
	if err != nil {
		return err
	}

	mean := utils.MeanFromFixed(rateValues)
	maxDiff := 0.0

	for _, rateValue := range rateValues {
		diff := math.Abs(helpers.FromFixed(rateValue)/mean - 1)
		if diff > maxDiff {
			maxDiff = diff
		}
	}
	metrics.SortedOraclesRateMaxDeviation.Set(maxDiff)

	medianTimestamp, err := p.sortedOracles.MedianTimestamp(opts, stableTokenAddress)
	if err != nil {
		return err
	}

	metrics.SortedOraclesMedianTimestamp.Set(float64(blockTime - medianTimestamp.Uint64()))

	celoBucketSize, stableBucketSize, err := p.exchange.GetBuyAndSellBuckets(opts, true)

	if err != nil {
		return err
	}

	celoStablePrice := utils.DivideBigInts(celoBucketSize, stableBucketSize)

	stablePrice := big.NewFloat(0)

	if celoStablePrice.Cmp(big.NewFloat(0)) != 0 {
		stablePrice = new(big.Float).Quo(medianRate, celoStablePrice)
	}

	stablePriceF, _ := stablePrice.Float64()
	metrics.ExchangeImpliedStableRate.Set(stablePriceF)
	return nil
}
