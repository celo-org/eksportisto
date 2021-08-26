package indexer

import (
	"context"
	"fmt"

	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/kliento/registry"
)

// Generic interface for a processor that runs for a block
type Processor interface {
	ShouldCollect() bool
	CollectData(context.Context, chan *Row) error
	ObserveMetrics(context.Context) error
	Init(context.Context) error
	Logger() log.Logger
	EventHandler() (registry.ContractID, EventHandler)
}

type ProcessorFactory interface {
	New(context.Context, *blockHandler) ([]Processor, error)
}

type EventHandler interface {
	HandleEvent(parsedLog *registry.RegistryParsedLog)
}

var Factories = []ProcessorFactory{
	// Ordering can be important here, as some processors
	// rely on previous ones to load data.
	chaindataProcessorFactory{},
	exchangeProcessorFactory{},
	electionProcessorFactory{},
	epochRewardsProcessorFactory{},
	celoTokenProcessorFactory{},
	lockedGoldProcessorFactory{},
	reserveProcessorFactory{},
	sortedOraclesProcessorFactory{},
	epochLogsProcessorFactory{},
}

// Error that doesn't bubble up, but only results in the processor being skipped
type SkipProcessorError struct {
	Reason error
}

func (e *SkipProcessorError) Error() string {
	return fmt.Sprintf("processor error: %v", e.Reason)
}
