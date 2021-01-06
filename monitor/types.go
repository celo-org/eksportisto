package monitor

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

type ContractProcessor interface {
	ObserveState(opts *bind.CallOpts) error
	ObserveMetric(opts *bind.CallOpts) error
}
