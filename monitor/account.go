package monitor

import (
	"context"

	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type accountProcessor struct {
	ctx      context.Context
	logger   log.Logger
	address  common.Address
	accounts *contracts.Accounts
}

func NewAccountsProcessor(ctx context.Context, logger log.Logger, address common.Address, accounts *contracts.Accounts) *accountProcessor {
	return &accountProcessor{
		ctx:      ctx,
		logger:   logger,
		address:  address,
		accounts: accounts,
	}
}

func (p accountProcessor) ObserveState() {
	return
}

var accountParam = "account"
var signerParam = "signer"

func (p accountProcessor) HandleLog(eventLog *types.Log) {
	logger := p.logger.New("contract", "Accounts", "logIndex", eventLog.Index)
	if eventLog.Address == p.address {
		eventName, eventRaw, ok, err := p.accounts.TryParseLog(*eventLog)
		if err != nil {
			logger.Warn("Ignoring event: Error parsing accounts event", "err", err, "eventId", eventLog.Topics[0].Hex())
			return
		}
		if !ok {
			return
		}

		var params = make([]interface{}, 0)
		params = append(params, "eventName", eventName)

		switch e := eventRaw.(type) {
		case *contracts.AccountsAttestationSignerAuthorized:
			params = append(params, accountParam, e.Account, signerParam, e.Signer)
		case *contracts.AccountsVoteSignerAuthorized:
			params = append(params, accountParam, e.Account, signerParam, e.Signer)
		case *contracts.AccountsValidatorSignerAuthorized:
			params = append(params, accountParam, e.Account, signerParam, e.Signer)
		case *contracts.AccountsAttestationSignerRemoved:
			params = append(params, accountParam, e.Account, signerParam, e.OldSigner)
		case *contracts.AccountsVoteSignerRemoved:
			params = append(params, accountParam, e.Account, signerParam, e.OldSigner)
		case *contracts.AccountsValidatorSignerRemoved:
			params = append(params, accountParam, e.Account, signerParam, e.OldSigner)
		case *contracts.AccountsAccountDataEncryptionKeySet:
			params = append(params, accountParam, e.Account, "dataEncryptionKey", e.DataEncryptionKey)
		case *contracts.AccountsAccountNameSet:
			params = append(params, accountParam, e.Account, "name", e.Name)
		case *contracts.AccountsAccountMetadataURLSet:
			params = append(params, accountParam, e.Account, "metadataUrl", e.MetadataURL)
		case *contracts.AccountsAccountWalletAddressSet:
			params = append(params, accountParam, e.Account, "walletAddress", e.WalletAddress)
		case *contracts.AccountsAccountCreated:
			params = append(params, accountParam, e.Account)
		}

		logEventLog(logger, params...)
	}
}
