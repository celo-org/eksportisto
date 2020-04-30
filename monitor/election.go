package monitor

import (
	"context"

	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

func electionLogProcessor(ctx context.Context, logger log.Logger, electionAddress common.Address, election *contracts.Election, eventLog *types.Log) {
	logger = logger.New("contract", "Election")
	if eventLog.Address == electionAddress {
		eventName, eventRaw, ok, err := election.TryParseLog(*eventLog)
		if err != nil {
			logger.Warn("Ignoring event: Error parsing election event", "err", err, "eventId", eventLog.Topics[0].Hex())
			return
		}
		if !ok {
			return
		}

		switch eventName {
		case "ValidatorGroupVoteCast":
			event := eventRaw.(*contracts.ElectionValidatorGroupVoteCast)
			logEventLog(logger, "eventName", eventName, "account", event.Account, "group", event.Group, "value", event.Value)
		case "ValidatorGroupVoteActivated":
			event := eventRaw.(*contracts.ElectionValidatorGroupVoteActivated)
			logEventLog(logger, "eventName", eventName, "account", event.Account, "group", event.Group, "value", event.Value)
		}
	}
}
