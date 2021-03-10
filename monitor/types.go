package monitor

import (
	"github.com/celo-org/celo-blockchain/accounts/abi/bind"
)

type ContractProcessor interface {
	ObserveState(opts *bind.CallOpts) error
	ObserveMetric(opts *bind.CallOpts) error
}
