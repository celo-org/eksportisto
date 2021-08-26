package indexer

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/celo-org/celo-blockchain/common"
	"github.com/celo-org/celo-blockchain/core/types"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/kliento/celotokens"
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
	blockRow      *Row
	logger        log.Logger
	isTip         bool
	block         *types.Block
	transactions  types.Transactions
	eventHandlers map[registry.ContractID]EventHandler
}

// newBlockHandler is called to instantiate a handler for a current
// block height. The struct lives as long as a block is being processed.
func (w *worker) newBlockHandler(b block) (*blockHandler, error) {
	handler := &blockHandler{
		baseBlockHandler: w.baseBlockHandler,
		blockNumber:      big.NewInt(int64(b.number)),
		logger:           w.logger.New("block", b.number),
		blockRow:         NewRow("blockNumber", b.number),
		isTip:            b.fromTip(),
		eventHandlers:    make(map[registry.ContractID]EventHandler),
	}

	return handler, nil
}

// run starts the processing of a block by firing all processors
// and a routine to collect rows from a channel
func (handler *blockHandler) run(ctx context.Context) (err error) {
	handler.registry.EnableCaching(ctx, handler.blockNumber)
	group, ctx := errgroup.WithContext(ctx)
	rowsChan := make(chan *Row, 1000)

	err = handler.loadBlock(ctx)
	if err != nil {
		return err
	}

	group.Go(func() error { return handler.spawnProcessors(ctx, rowsChan) })
	group.Go(func() error {
		rows := make([]*Row, 0, 1000)
	Loop:
		for {
			select {
			case row, hasMore := <-rowsChan:
				if !hasMore {
					break Loop
				}
				rows = append(rows, row)
			case <-ctx.Done():
				break Loop
			}
		}
		return handler.output.Write(rows)
	})

	return group.Wait()
}

func (handler *blockHandler) loadBlock(ctx context.Context) error {
	var err error
	handler.block, err = handler.celoClient.Eth.BlockByNumber(ctx, handler.blockNumber)
	if err != nil {
		if strings.Contains(err.Error(), "cannot unmarshal") {
			return nil
		}
		return err
	}

	handler.transactions = handler.block.Transactions()

	handler.blockRow = NewRow(
		"blockTimestamp", time.Unix(int64(handler.block.Time()), 0).Format(time.RFC3339),
		"blockNumber", handler.block.NumberU64(),
		"blockGasUsed", handler.block.GasUsed(),
	).WithId(handler.block.Number().String())

	return nil
}

// spawnProcessors initializes all processors by using the Factories, and execute
// extract data methods collecting all rows in the provided channel.
func (handler *blockHandler) spawnProcessors(ctx context.Context, rowsChan chan *Row) error {
	group, ctx := errgroup.WithContext(ctx)
	processors, err := handler.initializeProcessors(ctx)
	if err != nil {
		return err
	}

	for _, processor := range processors {
		func(processor Processor) {
			if handler.isTip {
				group.Go(func() error {
					return handler.checkError(processor.ObserveMetrics(ctx))
				})
			}
			if processor.ShouldCollect() {
				group.Go(func() error {
					return handler.checkError(processor.CollectData(ctx, rowsChan))
				})
			}
		}(processor)
	}

	err = group.Wait()
	close(rowsChan)
	return err
}

// initializeProcessors creates instances of all the processors that can run at
// the current blockHeight. Some are skipped if contracts are not deployed yet.
// This is parallelized because factory initialization sometimes talks to nodes
// as well and that blocks execution if we do it sequentially.
func (handler *blockHandler) initializeProcessors(ctx context.Context) ([]Processor, error) {
	processorChan := make(chan Processor, 100)
	group, ctx := errgroup.WithContext(ctx)

	for _, processorFactory := range Factories {
		func(processorFactory ProcessorFactory) {
			group.Go(func() error {
				return handler.initializeProcessorsForFactory(ctx, processorFactory, processorChan)
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

// initializeProcessorsForFactory sets up all processors
// for a factory if they can run for the current block height
// and sends them on a channel
func (handler *blockHandler) initializeProcessorsForFactory(
	ctx context.Context,
	factory ProcessorFactory,
	processorChan chan Processor,
) error {
	processors, err := factory.InitProcessors(ctx, handler)
	if err != nil {
		return handler.checkError(err)
	}
	for _, processor := range processors {
		contractId, eventHandler := processor.EventHandler()
		if eventHandler != nil {
			handler.eventHandlers[contractId] = eventHandler
		}

		processorChan <- processor
	}
	return nil
}

func (handler *blockHandler) checkError(err error) error {
	if err == nil {
		return nil
	}

	if registry.IsExpectedBeforeContractsDeployed(err) {
		handler.logger.Info("Skipping processor creation", "reason", err.Error())
		return nil
	} else if skipError, ok := err.(*SkipProcessorError); ok {
		handler.logger.Info("Skipping processor", "reason", skipError.Reason.Error())
		return nil
	} else {
		return err
	}
}
