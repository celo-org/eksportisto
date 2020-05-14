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

func (p epochRewardsProcessor) ObserveState(opts *bind.CallOpts) error {
	logger := p.logger.New("contract", "EpochRewards")

	// EpochRewards.getTargetGoldTotalSupply
	targetGoldTotalSupply, err := p.epochRewards.GetTargetGoldTotalSupply(opts)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "getTargetGoldTotalSupply", "targetGoldTotalSupply", targetGoldTotalSupply)

	// EpochRewards.getTargetVoterRewards
	targetVoterRewards, err := p.epochRewards.GetTargetVoterRewards(opts)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "getTargetVoterRewards", "targetVoterRewards", targetVoterRewards)

	// EpochRewards.getRewardsMultiplier
	rewardsMultiplier, err := p.epochRewards.GetRewardsMultiplier(opts)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "getRewardsMultiplier", "rewardsMultiplier", utils.FromFixed(rewardsMultiplier))

	// Todo: This is a fraction and therefore not actually a uint
	// logStateViewCall(logger, "method", "getVotingGoldFraction", "votingGoldFraction", votingGoldFraction.Uint64())

	// TODO: this is actually all fractions and thus not very useful to log
	// EpochRewards.calculateTargetEpochRewards
	// _, _, _, _, err = p.epochRewards.CalculateTargetEpochRewards(opts)
	// if err != nil {
	// 	return err
	// }

	// logStateViewCall(logger, "method", "calculateTargetEpochRewards", "validatorTargetEpochRewards", validatorTargetEpochRewards.Uint64())
	// logStateViewCall(logger, "method", "calculateTargetEpochRewards", "voterTargetEpochRewards", voterTargetEpochRewards.Uint64())
	// logStateViewCall(logger, "method", "calculateTargetEpochRewards", "communityTargetEpochRewards", communityTargetEpochRewards.Uint64())
	// logStateViewCall(logger, "method", "calculateTargetEpochRewards", "carbonOffsettingTargetEpochRewards", carbonOffsettingTargetEpochRewards.Uint64())
	return nil
}

func (p epochRewardsProcessor) ObserveMetric(opts *bind.CallOpts) error {
	// EpochRewards.getVotingGoldFraction
	votingGoldFraction, err := p.epochRewards.GetVotingGoldFraction(opts)
	if err != nil {
		return err
	}
	metrics.VotingGoldFraction.Set(float64(votingGoldFraction.Uint64()))
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
			logEventLog(logger, "eventName", eventName, "fraction", utils.FromFixed(event.Fraction))
		}
	}
}
