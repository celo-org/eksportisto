package monitor

import (
	"context"

	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
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

func (p epochRewardsProcessor) ObserveState(opts *bind.CallOpts, lastBlockOfEpoch bool) error {
	logger := p.logger.New("contract", "EpochRewards")

	if lastBlockOfEpoch {
		// EpochRewards.getTargetGoldTotalSupply
		targetGoldTotalSupply, err := p.epochRewards.GetTargetGoldTotalSupply(opts)
		if err != nil {
			return err
		}

		logStateViewCall(logger, "method", "getTargetGoldTotalSupply", "targetGoldTotalSupply", targetGoldTotalSupply.Uint64())

		// EpochRewards.getTargetVoterRewards
		targetVoterRewards, err := p.epochRewards.GetTargetVoterRewards(opts)
		if err != nil {
			return err
		}

		logStateViewCall(logger, "method", "getTargetVoterRewards", "targetVoterRewards", targetVoterRewards.Uint64())

		// EpochRewards.getRewardsMultiplier
		rewardsMultiplier, err := p.epochRewards.GetRewardsMultiplier(opts)
		if err != nil {
			return err
		}

		logStateViewCall(logger, "method", "getRewardsMultiplier", "rewardsMultiplier", rewardsMultiplier.Uint64())

		// EpochRewards.getVotingGoldFraction
		votingGoldFraction, err := p.epochRewards.GetVotingGoldFraction(opts)
		if err != nil {
			return err
		}

		logStateViewCall(logger, "method", "getVotingGoldFraction", "votingGoldFraction", votingGoldFraction.Uint64())

		// EpochRewards.calculateTargetEpochRewards
		validatorTargetEpochRewards, voterTargetEpochRewards, communityTargetEpochRewards, carbonOffsettingTargetEpochRewards, err := p.epochRewards.CalculateTargetEpochRewards(opts)
		if err != nil {
			return err
		}

		logStateViewCall(logger, "method", "calculateTargetEpochRewards", "validatorTargetEpochRewards", validatorTargetEpochRewards.Uint64())
		logStateViewCall(logger, "method", "calculateTargetEpochRewards", "voterTargetEpochRewards", voterTargetEpochRewards.Uint64())
		logStateViewCall(logger, "method", "calculateTargetEpochRewards", "communityTargetEpochRewards", communityTargetEpochRewards.Uint64())
		logStateViewCall(logger, "method", "calculateTargetEpochRewards", "carbonOffsettingTargetEpochRewards", carbonOffsettingTargetEpochRewards.Uint64())
	}
	return nil
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
