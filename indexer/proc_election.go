package indexer

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/registry"
	"github.com/go-errors/errors"
)

type electionProcessorFactory struct{}

func (electionProcessorFactory) InitProcessors(
	ctx context.Context,
	handler *blockHandler,
) ([]Processor, error) {
	election, err := handler.registry.GetElectionContract(ctx, handler.blockNumber)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}

	return []Processor{
		&electionProcessor{
			blockHandler: handler,
			logger:       handler.logger.New("processor", "election", "contract", "Election"),
			election:     election,
		},
	}, nil
}

func (proc *electionProcessor) EventHandler() (registry.ContractID, EventHandler) {
	return "", nil
}

type electionProcessor struct {
	*blockHandler
	logger   log.Logger
	election *contracts.Election
}

func (proc *electionProcessor) ShouldCollect() bool {
	// This processor will only collect data at the end of the epoch
	return utils.ShouldSample(proc.blockNumber.Uint64(), EpochSize)
}

func (proc electionProcessor) CollectData(ctx context.Context, rows chan *Row) error {
	opts := &bind.CallOpts{
		BlockNumber: proc.blockNumber,
		Context:     ctx,
	}

	contractRow := proc.blockRow.Extend("contract", "Election")

	// Election.getActiveVotes
	activeVotes, err := proc.election.GetActiveVotes(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall(
		"getActiveVotes",
		"activeVotes", activeVotes.String(),
	)

	// Election.getTotalVotes
	totalVotes, err := proc.election.GetTotalVotes(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall(
		"getTotalVotes",
		"totalVotes", totalVotes.String(),
	)

	// Election.getElectableValidators
	electableValidatorsMin, electableValidatorsMax, err := proc.election.GetElectableValidators(opts)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	rows <- contractRow.ViewCall(
		"getElectableValidators",
		"electableValidatorsMin", electableValidatorsMin.Uint64(),
		"electableValidatorsMax", electableValidatorsMax.Uint64(),
	)

	return nil
}

func (proc electionProcessor) ObserveMetrics(ctx context.Context) error {
	return nil
}
