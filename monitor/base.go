package monitor

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

func logEventLog(logger log.Logger, params ...interface{}) {
	logger.Info("RECEIVED_EVENT_LOG", params...)
}

func getTxLogger(logger log.Logger, receipt *types.Receipt, header *ethclient.HeaderAndTxnHashes) log.Logger {
	return logger.New("txHash", receipt.TxHash, "blockNumber", header.Number, "blockTimestamp", header.Time)
}

func logHeader(logger log.Logger, header *types.Header) {
	logger.Debug("RECEIVED_HEADER", "blockNumber", header.Number)
}

func logTransaction(logger log.Logger) {
	logger.Debug("RECEIVED_TRANSACTION")
}

func logStateViewCall(logger log.Logger, params ...interface{}) {
	logger.Info("STATE_VIEW_CALL", params...)
}
