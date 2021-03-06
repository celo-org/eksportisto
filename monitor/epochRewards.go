package monitor

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/contracts/helpers"
)

type epochRewardsProcessor struct {
	ctx          context.Context
	logger       log.Logger
	epochRewards *contracts.EpochRewards
}

func NewEpochRewardsProcessor(ctx context.Context, logger log.Logger, epochRewards *contracts.EpochRewards) *epochRewardsProcessor {
	return &epochRewardsProcessor{
		ctx:          ctx,
		logger:       logger,
		epochRewards: epochRewards,
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

	logStateViewCall(logger, "method", "getRewardsMultiplier", "rewardsMultiplier", helpers.FromFixed(rewardsMultiplier))

	// Todo: This is a fraction and therefore not actually a uint
	// logStateViewCall(logger, "method", "getVotingGoldFraction", "votingGoldFraction", votingGoldFraction.Uint64())

	// TODO: this is actually all fractions and thus not very useful to log
	// EpochRewards.calculateTargetEpochRewards
	validatorTargetEpochRewards, voterTargetEpochRewards, communityTargetEpochRewards, carbonOffsettingTargetEpochRewards, err := p.epochRewards.CalculateTargetEpochRewards(opts)
	if err != nil {
		// TODO: This will error when contract still frozen
		return nil
	}

	logStateViewCall(logger, "method", "calculateTargetEpochRewards", "validatorTargetEpochRewards", validatorTargetEpochRewards.Uint64(), "voterTargetEpochRewards", voterTargetEpochRewards.Uint64(), "communityTargetEpochRewards", communityTargetEpochRewards.Uint64(), "carbonOffsettingTargetEpochRewards", carbonOffsettingTargetEpochRewards.Uint64())

	return nil
}

func (p epochRewardsProcessor) ObserveMetric(opts *bind.CallOpts) error {
	// EpochRewards.getVotingGoldFraction
	votingGoldFraction, err := p.epochRewards.GetVotingGoldFraction(opts)
	if err != nil {
		return err
	}
	metrics.VotingGoldFraction.Set(helpers.FromFixed(votingGoldFraction))
	return nil
}
