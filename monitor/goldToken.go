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

type goldTokenProcessor struct {
	ctx              context.Context
	logger           log.Logger
	goldTokenAddress common.Address
	goldToken        *contracts.GoldToken
}

func NewGoldTokenProcessor(ctx context.Context, logger log.Logger, goldTokenAddress common.Address, goldToken *contracts.GoldToken) *goldTokenProcessor {
	return &goldTokenProcessor{
		ctx:              ctx,
		logger:           logger,
		goldTokenAddress: goldTokenAddress,
		goldToken:        goldToken,
	}
}

func (p goldTokenProcessor) ObserveState(opts *bind.CallOpts) error {
	logger := p.logger.New("contract", "GoldToken")
	totalSupply, err := p.goldToken.TotalSupply(opts)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "totalSupply", "totalSupply", totalSupply)

	return nil
}

func (p goldTokenProcessor) ObserveMetric(opts *bind.CallOpts) error {
	totalSupply, err := p.goldToken.TotalSupply(opts)
	if err != nil {
		return err
	}
	metrics.TotalCGLDSupply.Set(utils.ScaleFixed(totalSupply))
	return nil
}

func (p goldTokenProcessor) HandleLog(eventLog *types.Log) {
	logger := p.logger.New("contract", "GoldToken")
	if eventLog.Address == p.goldTokenAddress {
		eventName, eventRaw, ok, err := p.goldToken.TryParseLog(*eventLog)
		if err != nil {
			logger.Warn("Ignoring event: Error parsing goldToken event", "err", err, "eventId", eventLog.Topics[0].Hex())
			return
		}
		if !ok {
			return
		}

		switch eventName {
		case "Transfer":
			event := eventRaw.(*contracts.GoldTokenTransfer)
			logEventLog(logger, "eventName", eventName, "from", event.From, "to", event.To, "value", event.Value)
		case "TransferComment":
			event := eventRaw.(*contracts.GoldTokenTransferComment)
			logEventLog(logger, "eventName", eventName, "comment", event.Comment)
		}
	}
}
