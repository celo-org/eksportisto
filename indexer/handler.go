package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/celo-org/celo-blockchain/common"
	"github.com/celo-org/celo-blockchain/core/types"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/indexer/data"
	"github.com/celo-org/kliento/celotokens"
	"github.com/celo-org/kliento/contracts/helpers"
	"github.com/celo-org/kliento/registry"
	"golang.org/x/sync/errgroup"
)

// baseBlockHandler is a struct that holds references to
// dependencies that need to be setup only once for a worker
// and are shared between all block handlers
type baseBlockHandler struct {
	*worker
	registry          registry.Registry
	celoTokens        *celotokens.CeloTokens
	debugEnabled      bool
	sensitiveAccounts map[common.Address]string
}

// newBaseHandler creates a new baseBlockHandler which
// runs the one-time setup needed for some dependencies of the block handler.
// This gets instantiated once per worker lifetime.
func (w *worker) newBaseBlockHandler() (*baseBlockHandler, error) {
	r, err := registry.New(w.celoClient)
	if err != nil {
		return nil, err
	}

	supported, err := w.celoClient.Rpc.SupportedModules()
	if err != nil {
		return nil, err
	}
	_, debugEnabled := supported["debug"]

	return &baseBlockHandler{
		worker:            w,
		registry:          r,
		celoTokens:        celotokens.New(r),
		debugEnabled:      debugEnabled,
		sensitiveAccounts: loadSensitiveAccounts(),
	}, nil
}

// blockHandler is the struct responsible for processing one block
// it gets instantiate for each block that the worker handles, and
// it extends the baseBlockHandler by pointer reference.
type blockHandler struct {
	*baseBlockHandler
	blockNumber   *big.Int
	blockRow      *data.Row
	logger        log.Logger
	isTip         bool
	block         *types.Block
	transactions  types.Transactions
	eventHandlers map[registry.ContractID]EventHandler
}

// newBlockHandler is called to instantiate a handler for a current
// block height. The struct lives as long as a block is being processed.
func (w *worker) newBlockHandler(blockNumber uint64, isTip bool) (*blockHandler, error) {
	handler := &blockHandler{
		baseBlockHandler: w.baseBlockHandler,
		blockNumber:      big.NewInt(int64(blockNumber)),
		logger:           w.logger.New("block", blockNumber),
		blockRow:         data.NewRow("blockNumber", blockNumber),
		isTip:            isTip,
		eventHandlers:    make(map[registry.ContractID]EventHandler),
	}

	return handler, nil
}

// run starts the processing of a block
func (handler *blockHandler) run(ctx context.Context) (err error) {
	handler.registry.EnableCaching(ctx, handler.blockNumber)
	group, ctx := errgroup.WithContext(ctx)
	rowsChan := make(chan interface{}, 1000)

	group.Go(func() error { return handler.spawnProcessors(ctx, rowsChan) })
	group.Go(func() error {
		// Collect all rows from the channel
		// rows := make([]interface{}, 100)
	Loop:
		for {
			select {
			case row, hasMore := <-rowsChan:
				if !hasMore {
					break Loop
				}
				marshal, _ := json.Marshal(row)
				handler.logger.Info(string(marshal))
				// rows = append(rows, row)
			case <-ctx.Done():
				break Loop
			}
		}
		return nil
	})

	return group.Wait()
}

// spawnProcessors initializes all processors by using the Factories, and execute
// extract data methods collecting all rows in the provided channel.
func (handler *blockHandler) spawnProcessors(ctx context.Context, rowsChan chan interface{}) error {
	group, ctx := errgroup.WithContext(ctx)
	processors, err := handler.initializeProcessors(ctx)
	if err != nil {
		return err
	}

	for _, processor := range processors {
		func(processor Processor) {
			if handler.isTip {
				group.Go(func() error {
					return processor.ObserveMetrics(ctx)
				})
			}
			if processor.ShouldCollect() {
				group.Go(func() error {
					return processor.CollectData(ctx, rowsChan)
				})
			}
		}(processor)
	}

	err = group.Wait()
	close(rowsChan)
	return err
}

// initializeProcessors creates instances of all the processors that can run at
// the current blockHeight. Some are skipped if contracts are not deployed yet
func (handler *blockHandler) initializeProcessors(ctx context.Context) ([]Processor, error) {
	processorChan := make(chan Processor, 100)
	group, ctx := errgroup.WithContext(ctx)

	for _, processorFactory := range Factories {
		func(processorFactory ProcessorFactory) {
			group.Go(func() error {
				return handler.initializeFactory(ctx, processorFactory, processorChan)
			})
		}(processorFactory)
	}

	err := group.Wait()
	if err != nil {
		return nil, err
	}
	close(processorChan)

	processors := make([]Processor, 0, 100)
	for processor := range processorChan {
		processors = append(processors, processor)

	}

	return processors, nil
}

func (handler *blockHandler) initializeFactory(
	ctx context.Context,
	factory ProcessorFactory,
	processorChan chan Processor,
) error {
	processors, err := factory.New(ctx, handler)
	if err != nil && registry.IsExpectedBeforeContractsDeployed(err) {
		handler.logger.Info("Skipping processor creation", "reason", err.Error())
		return nil
	} else if err != nil {
		return err
	}
	for _, processor := range processors {
		err := processor.Init(ctx)
		if err != nil {
			if skipError, ok := err.(*SkipProcessorError); ok {
				handler.logger.Info("Skipping processor", "reason", skipError.Reason.Error())
				return nil
			} else if registry.IsExpectedBeforeContractsDeployed(err) {
				handler.logger.Info("Skipping processor", "reason", err)
				return nil
			} else {
				return err
			}
		}
		contractId, eventHandler := processor.EventHandler()
		if eventHandler != nil {
			handler.eventHandlers[contractId] = eventHandler
		}

		processorChan <- processor
	}
	return nil
}

func (handler *blockHandler) extractEvent(ctx context.Context, txHash common.Hash, eventIdx int, eventLog *types.Log, txRow *data.Row, rows chan interface{}) error {
	logger := handler.logger.New("txHash", txHash.String(), "logTxIndex", eventIdx, "logBlockIndex", eventLog.Index)
	eventRow := txRow.Extend("logTxIndex", eventIdx, "logBlockIndex", eventLog.Index).WithId(
		fmt.Sprintf("%s.event.%d", txHash.String(), eventIdx),
	)

	parsed, err := handler.registry.TryParseLog(ctx, *eventLog, handler.blockNumber)
	if err != nil {
		logger.Error("log parsing failed", "err", err)
	} else if parsed != nil {
		// If the contract with the event has an event handler, call it with the parsed event
		if handler.isTip {
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
			).Extend(logSlice...)
		}
	} else {
		logger.Warn("log source unknown, logging raw event")

		getTopic := func(index int) interface{} {
			if len(eventLog.Topics) > index {
				return eventLog.Topics[index]
			} else {
				return nil
			}
		}
		rows <- eventRow.Extend(
			"topic0", getTopic(0),
			"topic1", getTopic(1),
			"topic2", getTopic(2),
			"topic3", getTopic(3),
			"data", eventLog.Data,
		)
	}
	return nil
}
