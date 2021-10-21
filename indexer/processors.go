package indexer

import (
	"context"
	"fmt"

	"github.com/celo-org/kliento/registry"
)

// Generic interface for a processor that runs for a block
type Processor interface {
	ShouldCollect() bool
	CollectData(context.Context, chan *Row) error
	ObserveMetrics(context.Context) error
	EventHandler() (registry.ContractID, EventHandler)
}

type ProcessorFactory interface {
	InitProcessors(context.Context, *blockHandler) ([]Processor, error)
}

type EventHandler interface {
	HandleEvent(parsedLog *registry.RegistryParsedLog)
}

var Factories = []ProcessorFactory{
	chaindataProcessorFactory{},
	exchangeProcessorFactory{},
	electionProcessorFactory{},
	epochRewardsProcessorFactory{},
	lockedGoldProcessorFactory{},
	reserveProcessorFactory{},
	sortedOraclesProcessorFactory{},
	epochLogsProcessorFactory{},
	goldTokenProcessorFactory{},
	stableTokenProcessorFactory{},
}

// Error that doesn't bubble up, but only results in the processor being skipped
type SkipProcessorError struct {
	Reason error
}

func (e *SkipProcessorError) Error() string {
	return fmt.Sprintf("processor error: %v", e.Reason)
}
