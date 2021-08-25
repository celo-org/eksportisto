package indexer

import (
	"context"
	"fmt"

	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/kliento/registry"
)

type Processor interface {
	ShouldCollect() bool
	CollectData(context.Context, chan interface{}) error
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
	// Ordering is important here, as some processors
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

type SkipProcessorError struct {
	Reason error
}

func (e *SkipProcessorError) Error() string {
	return fmt.Sprintf("processor error: %v", e.Reason)
}
