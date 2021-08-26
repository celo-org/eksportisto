package indexer

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/contracts/helpers"
	"github.com/celo-org/kliento/registry"
)

type epochRewardsProcessorFactory struct{}

func (epochRewardsProcessorFactory) New(_ context.Context, handler *blockHandler) ([]Processor, error) {
	return []Processor{&epochRewardsProcessor{blockHandler: handler, logger: handler.logger.New("processor", "epochRewards", "contract", "EpochRewards")}}, nil
}

func (proc *epochRewardsProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

type epochRewardsProcessor struct {
	*blockHandler
	logger       log.Logger
	epochRewards *contracts.EpochRewards
}

func (proc *epochRewardsProcessor) Init(ctx context.Context) error {
	var err error
	proc.epochRewards, err = proc.registry.GetEpochRewardsContract(ctx, proc.blockNumber)
	if err != nil {
		return err
	}
	return nil
}

func (proc *epochRewardsProcessor) ShouldCollect() bool {
	// This processor will only collect data at the end of the epoch
	return utils.ShouldSample(proc.blockNumber.Uint64(), EpochSize)
}

func (proc epochRewardsProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	contractRow := proc.blockRow.Extend("contract", "EpochRewards")

	// EpochRewards.getTargetGoldTotalSupply
	targetGoldTotalSupply, err := proc.epochRewards.GetTargetGoldTotalSupply(opts)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall(
		"getTargetTotalSupply",
		"targetTotalSupply", targetGoldTotalSupply.String(),
	)

	// EpochRewards.getTargetVoterRewards
	targetVoterRewards, err := proc.epochRewards.GetTargetVoterRewards(opts)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall(
		"getTargetVoterRewards",
		"targetVoterRewards", targetVoterRewards.String(),
	)

	// EpochRewards.getRewardsMultiplier
	rewardsMultiplier, err := proc.epochRewards.GetRewardsMultiplier(opts)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall(
		"getRewardsMultiplier",
		"rewardsMultiplier", helpers.FromFixed(rewardsMultiplier),
	)

	rmMax, rmOverspendAdjustmentFactor, rmUnderspendAdjustmentFactor, err := proc.epochRewards.GetRewardsMultiplierParameters(opts)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall(
		"getRewardsMultiplierParameteres",
		"max", helpers.FromFixed(rmMax),
		"overspendAdjustmentFactor", helpers.FromFixed(rmOverspendAdjustmentFactor),
		"underspendAdjustmentFactor", helpers.FromFixed(rmUnderspendAdjustmentFactor),
	)

	tvyTarget, tvyMax, tvyAdjustmentFactor, err := proc.epochRewards.GetTargetVotingYieldParameters(opts)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall(
		"getTargetVotingYieldParameteres",
		"target", helpers.FromFixed(tvyTarget),
		"max", helpers.FromFixed(tvyMax),
		"adjustmentFactor", helpers.FromFixed(tvyAdjustmentFactor),
	)

	targetVotingGoldFraction, err := proc.epochRewards.GetTargetVotingGoldFraction(opts)
	if err != nil {
		return err
	}

	rows <- contractRow.ViewCall(
		"getTargetVotingGoldFraction",
		"targetVotingGoldFraction", helpers.FromFixed(targetVotingGoldFraction),
	)

	// EpochRewards.calculateTargetEpochRewards
	validatorTargetEpochRewards, voterTargetEpochRewards, communityTargetEpochRewards, carbonOffsettingTargetEpochRewards, err := proc.epochRewards.CalculateTargetEpochRewards(opts)
	if err != nil {
		// TODO: This will error when contract still frozen
		return nil
	}

	rows <- contractRow.ViewCall(
		"calculateTargetEpochRewards",
		"validatorTargetEpochRewards", validatorTargetEpochRewards,
		"voterTargetEpochRewards", voterTargetEpochRewards,
		"communityTargetEpochRewards", communityTargetEpochRewards,
		"carbonOffsettingTargetEpochRewards", carbonOffsettingTargetEpochRewards,
	)

	return nil
}

func (proc epochRewardsProcessor) ObserveMetrics(ctx context.Context) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}
	// EpochRewards.getVotingGoldFraction
	votingGoldFraction, err := proc.epochRewards.GetVotingGoldFraction(opts)
	if err != nil {
		return err
	}
	metrics.VotingGoldFraction.Set(helpers.FromFixed(votingGoldFraction))
	return nil
}
