package monitor

import (
	"github.com/celo-org/celo-blockchain/accounts/abi/bind"

	"github.com/celo-org/kliento/registry"
)

type ContractProcessor interface {
	ObserveState(opts *bind.CallOpts) error
	ObserveMetric(opts *bind.CallOpts) error
}

type EventHandler interface {
	HandleEvent(parsedLog *registry.RegistryParsedLog)
}
