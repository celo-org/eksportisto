package indexer

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/celo-org/celo-blockchain/core/types"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/kliento/celotokens"
	"github.com/celo-org/kliento/registry"
	"github.com/go-errors/errors"
	"github.com/neilotoole/errgroup"
)

// blockHandler is the struct responsible for processing one block
// it gets instantiate for each block that the worker handles, and
// it extends the baseBlockHandler by pointer reference.
type blockHandler struct {
	*worker
	registry      registry.Registry
	celoTokens    *celotokens.CeloTokens
	blockNumber   *big.Int
	blockRow      *Row
	logger        log.Logger
	block         *types.Block
	transactions  types.Transactions
	eventHandlers map[registry.ContractID]EventHandler
}

// newBlockHandler is called to instantiate a handler for a current
// block height. The struct lives as long as a block is being processed.
func (w *worker) newBlockHandler(block uint64) (*blockHandler, error) {
	r, err := registry.New(w.celoClient)
	if err != nil {
		return nil, err
	}

	handler := &blockHandler{
		worker:        w,
		registry:      r,
		celoTokens:    celotokens.New(r),
		blockNumber:   big.NewInt(int64(block)),
		logger:        w.logger.New("block", block),
		blockRow:      NewRow("blockNumber", block),
		eventHandlers: make(map[registry.ContractID]EventHandler),
	}

	return handler, nil
}

// run starts the processing of a block by firing all processors
// and a routine to collect rows from a channel
func (handler *blockHandler) run(ctx context.Context) (err error) {
	err = handler.registry.EnableCaching(ctx, handler.blockNumber)
	if err != nil {
		return err
	}
	group, ctx := errgroup.WithContext(ctx)
	rowsChan := make(chan *Row, 1000)

	err = metrics.RecordStepDuration(
		func() error { return handler.loadBlock(ctx) },
		"loadBlock",
	)
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

		return metrics.RecordStepDuration(func() error {
			return handler.output.Write(rows)
		}, "bigquery.write")
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
		return errors.Wrap(err, 0)
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
	group, ctx := errgroup.WithContextN(ctx, handler.concurrency, 2*handler.concurrency)
	processors, err := handler.initializeProcessors(ctx)
	if err != nil {
		return err
	}

	for _, processor := range processors {
		func(processor Processor) {
			if handler.isTip() && handler.collectMetrics {
				group.Go(func() error {
					return metrics.RecordProcessorDuration(
						func() error { return handler.checkError(processor.ObserveMetrics(ctx)) },
						processor,
						"ObserveMetrics",
					)
				})
			}
			if processor.ShouldCollect() {
				group.Go(func() error {
					return metrics.RecordProcessorDuration(
						func() error { return handler.checkError(processor.CollectData(ctx, rowsChan)) },
						processor,
						"CollectData",
					)
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
	return metrics.RecordProcessorDuration(
		func() error {
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
		},
		factory,
		"InitProcessors",
	)
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
