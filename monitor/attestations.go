package monitor

import (
	"context"

	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type attestationsProcessor struct {
	ctx                 context.Context
	logger              log.Logger
	attestationsAddress common.Address
	attestations        *contracts.Attestations
}

func NewAttestationsProcessor(ctx context.Context, logger log.Logger, attestationsAddress common.Address, attestations *contracts.Attestations) *attestationsProcessor {
	return &attestationsProcessor{
		ctx:                 ctx,
		logger:              logger,
		attestationsAddress: attestationsAddress,
		attestations:        attestations,
	}
}

func (p attestationsProcessor) ObserveState() {
	return
}

func (p attestationsProcessor) HandleLog(eventLog *types.Log) {
	logger := p.logger.New("contract", "Attestations", "logIndex", eventLog.Index)
	if eventLog.Address == p.attestationsAddress {
		eventName, eventRaw, ok, err := p.attestations.TryParseLog(*eventLog)
		if err != nil {
			logger.Warn("Ignoring event: Error parsing attestations event", "err", err, "eventId", eventLog.Topics[0].Hex())
			return
		}
		if !ok {
			return
		}

		switch eventName {
		case "AttestationsRequested":
			event := eventRaw.(*contracts.AttestationsAttestationsRequested)
			logEventLog(logger, "eventName", eventName, "identifier", hexutil.Encode(event.Identifier[:]), "account", event.Account, "attestationsRequested", event.AttestationsRequested.Uint64(), "attestationRequestFeeToken", event.AttestationRequestFeeToken)
		case "AttestationIssuerSelected":
			event := eventRaw.(*contracts.AttestationsAttestationIssuerSelected)
			logEventLog(logger, "eventName", eventName, "identifier", hexutil.Encode(event.Identifier[:]), "account", event.Account, "issuer", event.Issuer, "attestationRequestFeeToken", event.AttestationRequestFeeToken)
		case "AttestationCompleted":
			event := eventRaw.(*contracts.AttestationsAttestationCompleted)
			logEventLog(logger, "eventName", eventName, "identifier", hexutil.Encode(event.Identifier[:]), "account", event.Account, "issuer", event.Issuer)
		}

	}
}
