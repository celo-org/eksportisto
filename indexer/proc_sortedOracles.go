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
)

type sortedOraclesProcessorFactory struct{}

func (sortedOraclesProcessorFactory) New(ctx context.Context, handler *blockHandler) ([]Processor, error) {
	return []Processor{&sortedOraclesProcessor{blockHandler: handler, logger: handler.logger.New("processor", "sortedOracles", "contract", "SortedOracles")}}, nil
}

type stableTokenInfo struct {
	address            common.Address
	exchange           *contracts.Exchange
	exchangeRegistryID registry.ContractID
	stableToken        celotokens.CeloToken
}

type sortedOraclesProcessor struct {
	*blockHandler
	logger log.Logger

	sortedOracles    *contracts.SortedOracles
	stableTokenInfos map[celotokens.CeloToken]stableTokenInfo
}

func (proc *sortedOraclesProcessor) Logger() log.Logger { return proc.logger }
func (proc *sortedOraclesProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

func (proc *sortedOraclesProcessor) Init(ctx context.Context) error {
	sortedOracles, err := proc.registry.GetSortedOraclesContract(ctx, proc.blockNumber)
	if err != nil {
		return err
	}
	stableTokenAddresses, err := proc.celoTokens.GetAddresses(ctx, proc.blockNumber, true)
	if err != nil {
		return err
	}
	exchanges, err := proc.celoTokens.GetExchangeContracts(ctx, proc.blockNumber)
	if err != nil {
		return err
	}

	stableTokenInfos := make(map[celotokens.CeloToken]stableTokenInfo)
	for stableToken, stableTokenAddress := range stableTokenAddresses {
		exchange, ok := exchanges[stableToken]
		// This should never happen, but let's be safe
		if !ok || exchange == nil {
			return fmt.Errorf("no exchange provided for stable token %s", stableToken)
		}
		registryID, err := celotokens.GetRegistryID(stableToken)
		if err != nil {
			return err
		}

		stableTokenInfos[stableToken] = stableTokenInfo{
			address:            stableTokenAddress,
			exchange:           exchange,
			exchangeRegistryID: registryID,
			stableToken:        stableToken,
		}
	}

	proc.stableTokenInfos = stableTokenInfos
	proc.sortedOracles = sortedOracles
	return nil
}

func (proc *sortedOraclesProcessor) ShouldCollect() bool {
	// This processor will run once per hour
	return utils.ShouldSample(proc.blockNumber.Uint64(), BlocksPerHour)
}

func (proc sortedOraclesProcessor) CollectData(ctx context.Context, rows chan interface{}) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	for _, stableTokenInfo := range proc.stableTokenInfos {
		err := proc.collectDataForStableToken(opts, stableTokenInfo, rows)
		if err != nil {
			return err
		}
	}
	return nil
}

func (proc sortedOraclesProcessor) collectDataForStableToken(opts *bind.CallOpts, stableTokenInfo stableTokenInfo, rows chan interface{}) error {
	contractRow := proc.blockRow.Contract("SortedOracles", "stableToken", stableTokenInfo.stableToken, "address", stableTokenInfo.address).AppendID(stableTokenInfo.address.String())
	// SortedOracles.IsOldestReportExpired
	isOldestReportExpired, lastReportAddress, err := proc.sortedOracles.IsOldestReportExpired(opts, stableTokenInfo.address)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall(
		"IsOldestReportExpired",
		"isOldestReportExpired", isOldestReportExpired,
		"lastReportAddress", lastReportAddress)

	// SortedOracles.NumRates
	numRates, err := proc.sortedOracles.NumRates(opts, stableTokenInfo.address)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall("NumRates", "numRates", numRates)

	// SortedOracles.NumRates
	medianRateNumerator, medianRateDenominator, err := proc.sortedOracles.MedianRate(opts, stableTokenInfo.address)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall(
		"MedianRate",
		"medianRateNumerator", medianRateNumerator,
		"medianRateDenominator", medianRateDenominator,
	)

	// SortedOracles.MedianTimestamp
	medianTimestamp, err := proc.sortedOracles.MedianTimestamp(opts, stableTokenInfo.address)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall("MedianTimestamp", "medianTimestamp", medianTimestamp)

	// SortedOracles.GetRates
	rateAddresses, rateValues, medianRelations, err := proc.sortedOracles.GetRates(opts, stableTokenInfo.address)
	if err != nil {
		return err
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
	timestampAddresses, timestamp, medianRelations, err := proc.sortedOracles.GetTimestamps(opts, stableTokenInfo.address)
	if err != nil {
		return err
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

	celoBucketSize, stableBucketSize, err := stableTokenInfo.exchange.GetBuyAndSellBuckets(opts, true)

	if err != nil {
		return err
	}

	rows <- proc.blockRow.Contract(string(stableTokenInfo.exchangeRegistryID)).ViewCall(
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
	blockTime := proc.block.Time()

	for _, stableTokenInfo := range proc.stableTokenInfos {
		err := proc.observeMetricForStableToken(opts, stableTokenInfo, blockTime)
		if err != nil {
			return err
		}
	}
	return nil
}

func (proc sortedOraclesProcessor) observeMetricForStableToken(opts *bind.CallOpts, stableTokenInfo stableTokenInfo, blockTime uint64) error {
	stableTokenStr := string(stableTokenInfo.stableToken)
	isOldestReportExpired, _, err := proc.sortedOracles.IsOldestReportExpired(opts, stableTokenInfo.address)
	if err != nil {
		return err
	}
	sortedOraclesIsOldestReportExpiredGauge, err := metrics.SortedOraclesIsOldestReportExpired.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return err
	}
	sortedOraclesIsOldestReportExpiredGauge.Set(utils.BoolToFloat64(isOldestReportExpired))

	numRates, err := proc.sortedOracles.NumRates(opts, stableTokenInfo.address)
	if err != nil {
		return err
	}
	sortedOraclesNumRatesGauge, err := metrics.SortedOraclesNumRates.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return err
	}
	sortedOraclesNumRatesGauge.Set(float64(numRates.Uint64()))

	medianRateNumerator, medianRateDenominator, err := proc.sortedOracles.MedianRate(opts, stableTokenInfo.address)
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

	sortedOraclesMedianRateGauge, err := metrics.SortedOraclesMedianRate.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return err
	}
	sortedOraclesMedianRateGauge.Set(medianRateMetric)

	_, rateValues, _, err := proc.sortedOracles.GetRates(opts, stableTokenInfo.address)
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
	sortedOraclesRateMaxDeviationGauge, err := metrics.SortedOraclesRateMaxDeviation.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return err
	}
	sortedOraclesRateMaxDeviationGauge.Set(maxDiff)

	medianTimestamp, err := proc.sortedOracles.MedianTimestamp(opts, stableTokenInfo.address)
	if err != nil {
		return err
	}
	sortedOraclesMedianTimestampGauge, err := metrics.SortedOraclesMedianTimestamp.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return err
	}
	sortedOraclesMedianTimestampGauge.Set(float64(blockTime - medianTimestamp.Uint64()))

	celoBucketSize, stableBucketSize, err := stableTokenInfo.exchange.GetBuyAndSellBuckets(opts, true)

	if err != nil {
		return err
	}

	celoStablePrice := utils.DivideBigInts(celoBucketSize, stableBucketSize)

	stablePrice := big.NewFloat(0)

	if celoStablePrice.Cmp(big.NewFloat(0)) != 0 {
		stablePrice = new(big.Float).Quo(medianRate, celoStablePrice)
	}

	stablePriceF, _ := stablePrice.Float64()
	exchangeImpliedStableRateGauge, err := metrics.ExchangeImpliedStableRate.GetMetricWithLabelValues(stableTokenStr)
	if err != nil {
		return err
	}
	exchangeImpliedStableRateGauge.Set(stablePriceF)
	return nil
}
