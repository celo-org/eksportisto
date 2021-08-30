package monitor

import (
	"context"

	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/kliento/contracts"
)

type electionProcessor struct {
	ctx      context.Context
	logger   log.Logger
	election *contracts.Election
}

func NewElectionProcessor(ctx context.Context, logger log.Logger, election *contracts.Election) *electionProcessor {
	return &electionProcessor{
		ctx:      ctx,
		logger:   logger,
		election: election,
	}
}

func (p electionProcessor) ObserveState(opts *bind.CallOpts) error {
	logger := p.logger.New("contract", "Election")

	// Election.getActiveVotes
	activeVotes, err := p.election.GetActiveVotes(opts)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "getActiveVotes", "activeVotes", activeVotes)

	// Election.getTotalVotes
	totalVotes, err := p.election.GetTotalVotes(opts)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "getTotalVotes", "totalVotes", totalVotes)

	// Election.getElectableValidators
	electableValidatorsMin, electableValidatorsMax, err := p.election.GetElectableValidators(opts)
	if err != nil {
		return err
	}

	logStateViewCall(logger, "method", "getElectableValidators", "electableValidatorsMin", electableValidatorsMin.Uint64())
	logStateViewCall(logger, "method", "getElectableValidators", "electableValidatorsMax", electableValidatorsMax.Uint64())

	return nil
}
