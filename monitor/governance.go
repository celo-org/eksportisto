package monitor

import (
	"context"

	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type governanceProcessor struct {
	ctx               context.Context
	logger            log.Logger
	governanceAddress common.Address
	governance        *contracts.Governance
}

func NewGovernanceProcessor(ctx context.Context, logger log.Logger, governanceAddress common.Address, governance *contracts.Governance) *governanceProcessor {
	return &governanceProcessor{
		ctx:               ctx,
		logger:            logger,
		governanceAddress: governanceAddress,
		governance:        governance,
	}
}

func (p governanceProcessor) ObserveState() {
	return
}

func (p governanceProcessor) HandleLog(eventLog *types.Log) {
	logger := p.logger.New("contract", "Governance", "logIndex", eventLog.Index)
	if eventLog.Address == p.governanceAddress {
		eventName, eventRaw, ok, err := p.governance.TryParseLog(*eventLog)
		if err != nil {
			logger.Warn("Ignoring event: Error parsing governance event", "err", err, "eventId", eventLog.Topics[0].Hex())
			return
		}
		if !ok {
			return
		}

		switch eventName {
		case "ProposalVoted":
			event := eventRaw.(*contracts.GovernanceProposalVoted)
			logEventLog(logger, "eventName", eventName, "account", event.Account, "voteValue", event.Value, "weight", event.Weight, "proposalId", event.ProposalId.Uint64())
		case "ProposalUpvoted":
			event := eventRaw.(*contracts.GovernanceProposalUpvoted)
			logEventLog(logger, "eventName", eventName, "account", event.Account, "upvotes", event.Upvotes, "proposalId", event.ProposalId.Uint64())
		case "ProposalApproved":
			event := eventRaw.(*contracts.GovernanceProposalApproved)
			logEventLog(logger, "eventName", eventName, "proposalId", event.ProposalId.Uint64())
		case "ProposalExecuted":
			event := eventRaw.(*contracts.GovernanceProposalExecuted)
			logEventLog(logger, "eventName", eventName, "proposalId", event.ProposalId.Uint64())
		case "ProposalDequeued":
			event := eventRaw.(*contracts.GovernanceProposalDequeued)
			logEventLog(logger, "eventName", eventName, "proposalId", event.ProposalId.Uint64())
		case "ProposalQueued":
			event := eventRaw.(*contracts.GovernanceProposalQueued)
			logEventLog(logger, "eventName", eventName, "proposalId", event.ProposalId.Uint64())
		case "ProposalExpired":
			event := eventRaw.(*contracts.GovernanceProposalExpired)
			logEventLog(logger, "eventName", eventName, "proposalId", event.ProposalId.Uint64())
		}

	}
}
