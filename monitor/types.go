package monitor

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
)

type ContractProcessor interface {
	HandleLog(eventLog *types.Log)
	ObserveState(opts *bind.CallOpts) error
}
