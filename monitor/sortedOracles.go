package monitor

import (
	"context"

	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type sortedOraclesProcessor struct {
	ctx                  context.Context
	logger               log.Logger
	sortedOraclesAddress common.Address
	sortedOracles        *contracts.SortedOracles
}

func NewSortedOraclesProcessor(ctx context.Context, logger log.Logger, sortedOraclesAddress common.Address, sortedOracles *contracts.SortedOracles) *sortedOraclesProcessor {
	return &sortedOraclesProcessor{
		ctx:                  ctx,
		logger:               logger,
		sortedOraclesAddress: sortedOraclesAddress,
		sortedOracles:        sortedOracles,
	}
}

func (p sortedOraclesProcessor) ObserveState(opts *bind.CallOpts, stableTokenAddress common.Address) error {
	logger := p.logger.New("contract", "SortedOracles")

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
		logStateViewCall(logger, "method", "GetRates", "rateAddress", rateAddress, "rateValue", utils.FromFixed(rateValues[i]), "medianRelation", medianRelations[i], "index", i)
	}

	// SortedOracles.GetTimestamps
	timestampAddresses, timestamp, medianRelations, err := p.sortedOracles.GetTimestamps(opts, stableTokenAddress)
	if err != nil {
		return err
	}

	for i, timestampAddress := range timestampAddresses {
		logStateViewCall(logger, "method", "GetTimestamps", "timestampAddress", timestampAddress, "reportedTimestamp", timestamp[i], "medianRelation", medianRelations[i], "index", i)
	}

	return nil
}

func (p sortedOraclesProcessor) HandleLog(eventLog *types.Log) {
	logger := p.logger.New("contract", "SortedOracles")
	if eventLog.Address == p.sortedOraclesAddress {
		eventName, eventRaw, ok, err := p.sortedOracles.TryParseLog(*eventLog)
		if err != nil {
			logger.Warn("Ignoring event: Error parsing sortedOracles event", "err", err, "eventId", eventLog.Topics[0].Hex())
			return
		}
		if !ok {
			return
		}

		switch eventName {
		case "OracleAdded":
			event := eventRaw.(*contracts.SortedOraclesOracleAdded)
			logEventLog(logger, "eventName", eventName, "token", event.Token, "oracleAddress", event.OracleAddress)
		case "OracleRemoved":
			event := eventRaw.(*contracts.SortedOraclesOracleRemoved)
			logEventLog(logger, "eventName", eventName, "token", event.Token, "oracleAddress", event.OracleAddress)
		case "OracleReported":
			event := eventRaw.(*contracts.SortedOraclesOracleReported)
			logEventLog(logger, "eventName", eventName, "token", event.Token, "oracle", event.Oracle, "timestamp", event.Timestamp, "value", event.Value)
		case "OracleReportRemoved":
			event := eventRaw.(*contracts.SortedOraclesOracleReportRemoved)
			logEventLog(logger, "eventName", eventName, "token", event.Token, "oracle", event.Oracle)
		case "ReportExpirySet":
			event := eventRaw.(*contracts.SortedOraclesReportExpirySet)
			logEventLog(logger, "eventName", eventName, "reportExpiry", event.ReportExpiry)
		}
	}
}
