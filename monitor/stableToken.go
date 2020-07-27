package monitor

import (
	"context"

	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type stableTokenProcessor struct {
	ctx                context.Context
	logger             log.Logger
	stableTokenAddress common.Address
	stableToken        *contracts.StableToken
}

func NewStableTokenProcessor(ctx context.Context, logger log.Logger, stableTokenAddress common.Address, stableToken *contracts.StableToken) *stableTokenProcessor {
	return &stableTokenProcessor{
		ctx:                ctx,
		logger:             logger,
		stableTokenAddress: stableTokenAddress,
		stableToken:        stableToken,
	}
}

func (p stableTokenProcessor) ObserveState(opts *bind.CallOpts) error {
	logger := p.logger.New("contract", "StableToken")

	totalSupply, err := p.stableToken.TotalSupply(opts)
	if err != nil {
		return err
	}

	// metrics.TotalCUSDSupply.Observe(float64(totalSupply.Uint64()))
	logStateViewCall(logger, "method", "totalSupply", "totalSupply", totalSupply)

	return nil
}

func (p stableTokenProcessor) ObserveMetric(opts *bind.CallOpts) error {
	totalSupply, err := p.stableToken.TotalSupply(opts)
	if err != nil {
		return err
	}

	metrics.TotalCUSDSupply.Set(utils.ScaleFixed(totalSupply))
	return nil
}

func (p stableTokenProcessor) HandleLog(eventLog *types.Log) {
	logger := p.logger.New("contract", "StableToken")
	if eventLog.Address == p.stableTokenAddress {
		eventName, eventRaw, ok, err := p.stableToken.TryParseLog(*eventLog)
		if err != nil {
			logger.Warn("Ignoring event: Error parsing stableToken event", "err", err, "eventId", eventLog.Topics[0].Hex())
			return
		}
		if !ok {
			return
		}

		switch eventName {
		case "Transfer":
			event := eventRaw.(*contracts.StableTokenTransfer)
			logEventLog(logger, "eventName", eventName, "from", event.From, "to", event.To, "value", event.Value, "txHash", eventLog.TxHash.String())
		case "TransferComment":
			event := eventRaw.(*contracts.StableTokenTransferComment)
			logEventLog(logger, "eventName", eventName, "comment", event.Comment, "txHash", eventLog.TxHash.String())
		}
	}
}
