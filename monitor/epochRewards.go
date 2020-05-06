package monitor

import (
	"context"

	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type epochRewardsProcessor struct {
	ctx                 context.Context
	logger              log.Logger
	epochRewardsAddress common.Address
	epochRewards        *contracts.EpochRewards
}

func NewEpochRewardsProcessor(ctx context.Context, logger log.Logger, epochRewardsAddress common.Address, epochRewards *contracts.EpochRewards) *epochRewardsProcessor {
	return &epochRewardsProcessor{
		ctx:                 ctx,
		logger:              logger,
		epochRewardsAddress: epochRewardsAddress,
		epochRewards:        epochRewards,
	}
}

func (p epochRewardsProcessor) ObserveState() {
}

func (p epochRewardsProcessor) HandleLog(eventLog *types.Log) {
	logger := p.logger.New("contract", "EpochRewards")
	if eventLog.Address == p.epochRewardsAddress {
		eventName, eventRaw, ok, err := p.epochRewards.TryParseLog(*eventLog)
		if err != nil {
			logger.Warn("Ignoring event: Error parsing epochRewards event", "err", err, "eventId", eventLog.Topics[0].Hex())
			return
		}
		if !ok {
			return
		}

		switch eventName {
		case "TargetVotingYieldUpdated":
			event := eventRaw.(*contracts.EpochRewardsTargetVotingYieldUpdated)
			logEventLog(logger, "eventName", eventName, "fraction", event.Fraction)
		}
	}
}
