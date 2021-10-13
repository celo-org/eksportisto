package indexer

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/celo-org/celo-blockchain/common"
	"github.com/celo-org/celo-blockchain/core/types"
	"github.com/celo-org/kliento/contracts/helpers"
	"github.com/celo-org/kliento/registry"
	"github.com/spf13/viper"
)

var EpochSize = uint64(17280)   // 17280 = 12 * 60 * 24
var BlocksPerHour = uint64(720) // 720 = 12 * 60

type Mode string

const (
	DataMode    Mode = "data"
	MetricsMode Mode = "metrics"
	BothMode    Mode = "both"
)

func newMode(mode string) (Mode, error) {
	if mode == string(DataMode) {
		return DataMode, nil
	} else if mode == string(MetricsMode) {
		return MetricsMode, nil
	} else if mode == string(BothMode) {
		return BothMode, nil
	} else {
		return "", fmt.Errorf("invalid mode %s", mode)
	}
}

func (mode Mode) shouldCollectData() bool {
	return mode == DataMode || mode == BothMode
}

func (mode Mode) shouldCollectMetrics() bool {
	return mode == MetricsMode || mode == BothMode
}

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

// extractEvent parses an event log and decodes when possible,
// it outputs rows. It is defined on the blockHandler because
// it's shared between multiple processors.
func (handler *blockHandler) extractEvent(
	ctx context.Context,
	txHash common.Hash,
	eventIdx int,
	eventLog *types.Log,
	txRow *Row,
	rows chan *Row,
) error {
	logger := handler.logger.New("txHash", txHash.String(), "logTxIndex", eventIdx, "logBlockIndex", eventLog.Index)
	eventRowId := fmt.Sprintf("%s.event.%d", txHash.String(), eventIdx)
	eventRow := txRow.Extend("logTxIndex", eventIdx, "logBlockIndex", eventLog.Index).WithId(eventRowId)

	rawEvent, err := json.Marshal(eventLog)
	if err != nil {
		logger.Error("event log marshalling failed", "err", err)
	} else if rawEvent != nil {
		getTopic := func(index int) interface{} {
			if len(eventLog.Topics) > index {
				return eventLog.Topics[index]
			} else {
				return nil
			}
		}
		addHexPrefix := func(eventLogData string) interface{} {
			var sb strings.Builder
			sb.WriteString("0x")
			sb.WriteString(eventLogData)
			return sb.String()
		}
		eventRow = eventRow.Extend("type", "Event",
			"loggedBy", eventLog.Address.Hex(),
			"rawEvent", string(rawEvent),
			"topic0", getTopic(0),
			"topic1", getTopic(1),
			"topic2", getTopic(2),
			"topic3", getTopic(3),
			"data", addHexPrefix(hex.EncodeToString(eventLog.Data))).WithId(eventRowId)
	}

	parsed, err := handler.registry.TryParseLog(ctx, *eventLog, handler.blockNumber)
	if err != nil {
		logger.Error("log parsing failed", "err", err)
	} else if parsed != nil {
		// If the contract with the event has an event handler, call it with the parsed event
		if handler.isTip() {
			if handler, ok := handler.eventHandlers[registry.ContractID(parsed.Contract)]; ok {
				handler.HandleEvent(parsed)
			}
		}
		logSlice, err := helpers.EventToSlice(parsed.Log)
		if err != nil {
			logger.Error("event slice encoding failed", "contract", parsed.Contract, "event", parsed.Event, "err", err)
		} else {
			rows <- eventRow.Extend(
				"contract", parsed.Contract,
				"event", parsed.Event,
			).Extend(logSlice...).WithId(eventRowId)
		}
	}
	return nil
}
