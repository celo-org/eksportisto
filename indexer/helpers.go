package indexer

import (
	"encoding/json"
	"io/ioutil"

	"github.com/celo-org/celo-blockchain/common"
	"github.com/celo-org/celo-blockchain/core/types"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/spf13/viper"
)

func loadSensitiveAccounts() map[common.Address]string {
	filePath := viper.GetString("indexer.sensitiveAccountsPath")
	if filePath == "" {
		return make(map[common.Address]string)
	}

	bz, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	var addresses map[common.Address]string
	err = json.Unmarshal(bz, &addresses)
	if err != nil {
		panic(err)
	}

	return addresses
}

func logEventLog(logger log.Logger, params ...interface{}) {
	logger.Info("RECEIVED_EVENT_LOG", params...)
}

func logHeader(logger log.Logger, header *types.Header) {
	logger.Debug("RECEIVED_HEADER", "blockNumber", header.Number)
}

func logTransaction(logger log.Logger, params ...interface{}) {
	logger.Info("RECEIVED_TRANSACTION", params...)
}

func logTransfer(logger log.Logger, params ...interface{}) {
	logger.Info("RECEIVED_TRANSFER", params...)
}

func logStateViewCall(logger log.Logger, params ...interface{}) {
	logger.Info("STATE_VIEW_CALL", params...)
}
