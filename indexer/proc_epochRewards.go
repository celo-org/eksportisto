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
	"github.com/go-errors/errors"
)

type epochRewardsProcessorFactory struct{}

func (epochRewardsProcessorFactory) InitProcessors(
	ctx context.Context,
	handler *blockHandler,
) ([]Processor, error) {
	epochRewards, err := handler.registry.GetEpochRewardsContract(ctx, handler.blockNumber)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}

	return []Processor{
		&epochRewardsProcessor{
			blockHandler: handler,
			logger:       handler.logger.New("processor", "epochRewards", "contract", "EpochRewards"),
			epochRewards: epochRewards,
		},
	}, nil
}

type epochRewardsProcessor struct {
	*blockHandler
	logger       log.Logger
	epochRewards *contracts.EpochRewards
}

func (proc *epochRewardsProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

func (proc *epochRewardsProcessor) ShouldCollect() bool {
	// This processor will only collect data at the end of the epoch
	return utils.ShouldSample(proc.blockNumber.Uint64(), EpochSize)
}

func (proc epochRewardsProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	proc.logger.Info("EpochRewards.CollectData")
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	contractRow := proc.blockRow.Extend("contract", "EpochRewards")

	// EpochRewards.getTargetGoldTotalSupply
	targetGoldTotalSupply, err := proc.epochRewards.GetTargetGoldTotalSupply(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case rows <- contractRow.ViewCall(
		"getTargetTotalSupply",
		"targetTotalSupply", targetGoldTotalSupply.String(),
	):
	}

	// EpochRewards.getTargetVoterRewards
	targetVoterRewards, err := proc.epochRewards.GetTargetVoterRewards(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case rows <- contractRow.ViewCall(
		"getTargetVoterRewards",
		"targetVoterRewards", targetVoterRewards.String(),
	):
	}

	// EpochRewards.getRewardsMultiplier
	rewardsMultiplier, err := proc.epochRewards.GetRewardsMultiplier(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case rows <- contractRow.ViewCall(
		"getRewardsMultiplier",
		"rewardsMultiplier", helpers.FromFixed(rewardsMultiplier),
	):
	}

	rmMax, rmOverspendAdjustmentFactor, rmUnderspendAdjustmentFactor, err := proc.epochRewards.GetRewardsMultiplierParameters(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case rows <- contractRow.ViewCall(
		"getRewardsMultiplierParameteres",
		"max", helpers.FromFixed(rmMax),
		"overspendAdjustmentFactor", helpers.FromFixed(rmOverspendAdjustmentFactor),
		"underspendAdjustmentFactor", helpers.FromFixed(rmUnderspendAdjustmentFactor),
	):
	}

	tvyTarget, tvyMax, tvyAdjustmentFactor, err := proc.epochRewards.GetTargetVotingYieldParameters(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case rows <- contractRow.ViewCall(
		"getTargetVotingYieldParameteres",
		"target", helpers.FromFixed(tvyTarget),
		"max", helpers.FromFixed(tvyMax),
		"adjustmentFactor", helpers.FromFixed(tvyAdjustmentFactor),
	):
	}

	targetVotingGoldFraction, err := proc.epochRewards.GetTargetVotingGoldFraction(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case rows <- contractRow.ViewCall(
		"getTargetVotingGoldFraction",
		"targetVotingGoldFraction", helpers.FromFixed(targetVotingGoldFraction),
	):
	}

	votingGoldFraction, err := proc.epochRewards.GetVotingGoldFraction(opts)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case rows <- contractRow.ViewCall(
		"getVotingGoldFraction",
		"votingGoldFraction", helpers.FromFixed(votingGoldFraction),
	):
	}

	// EpochRewards.calculateTargetEpochRewards
	validatorTargetEpochRewards, voterTargetEpochRewards, communityTargetEpochRewards, carbonOffsettingTargetEpochRewards, err := proc.epochRewards.CalculateTargetEpochRewards(opts)
	if err != nil {
		// TODO: This will error when contract still frozen
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case rows <- contractRow.ViewCall(
		"calculateTargetEpochRewards",
		"validatorTargetEpochRewards", validatorTargetEpochRewards,
		"voterTargetEpochRewards", voterTargetEpochRewards,
		"communityTargetEpochRewards", communityTargetEpochRewards,
		"carbonOffsettingTargetEpochRewards", carbonOffsettingTargetEpochRewards,
	):
	}

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
		return errors.Wrap(err, 0)
	}
	metrics.VotingGoldFraction.Set(helpers.FromFixed(votingGoldFraction))
	return nil
}
