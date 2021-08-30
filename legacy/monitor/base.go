package monitor

import (
	"github.com/celo-org/celo-blockchain/core/types"
	"github.com/celo-org/celo-blockchain/log"
)

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
