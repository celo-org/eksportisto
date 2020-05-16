package monitor

import (
	"context"

	"github.com/celo-org/eksportisto/utils"
	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type validatorsProcessor struct {
	ctx               context.Context
	logger            log.Logger
	validatorsAddress common.Address
	validators        *contracts.Validators
}

func NewValidatorsProcessor(ctx context.Context, logger log.Logger, validatorsAddress common.Address, validators *contracts.Validators) *validatorsProcessor {
	return &validatorsProcessor{
		ctx:               ctx,
		logger:            logger,
		validatorsAddress: validatorsAddress,
		validators:        validators,
	}
}

func (p validatorsProcessor) ObserveState(opts *bind.CallOpts) error {
	return nil
}

func (p validatorsProcessor) HandleLog(eventLog *types.Log) {
	logger := p.logger.New("contract", "Validators")
	if eventLog.Address == p.validatorsAddress {
		eventName, eventRaw, ok, err := p.validators.TryParseLog(*eventLog)
		if err != nil {
			logger.Warn("Ignoring event: Error parsing validators event", "err", err, "eventId", eventLog.Topics[0].Hex())
			return
		}
		if !ok {
			return
		}

		switch eventName {
		case "ValidatorScoreUpdated":
			event := eventRaw.(*contracts.ValidatorsValidatorScoreUpdated)
			logEventLog(logger, "eventName", eventName, "validator", event.Validator, "score", utils.FromFixed(event.Score), "epochScore", utils.FromFixed(event.EpochScore))
		case "ValidatorEpochPaymentDistributed":
			event := eventRaw.(*contracts.ValidatorsValidatorEpochPaymentDistributed)
			logEventLog(logger, "eventName", eventName, "validator", event.Validator, "validatorPayment", event.ValidatorPayment, "group", event.Group, "groupPayment", event.GroupPayment)
		case "ValidatorRegistered":
			event := eventRaw.(*contracts.ValidatorsValidatorRegistered)
			logEventLog(logger, "eventName", eventName, "validator", event.Validator)
		case "ValidatorGroupRegistered":
			event := eventRaw.(*contracts.ValidatorsValidatorGroupRegistered)
			logEventLog(logger, "eventName", eventName, "group", event.Group)
		case "ValidatorGroupMemberAdded":
			event := eventRaw.(*contracts.ValidatorsValidatorGroupMemberAdded)
			logEventLog(logger, "eventName", eventName, "validator", event.Validator, "group", event.Group)
		case "ValidatorAffiliated":
			event := eventRaw.(*contracts.ValidatorsValidatorAffiliated)
			logEventLog(logger, "eventName", eventName, "validator", event.Validator, "group", event.Group)
		}
	}
}
