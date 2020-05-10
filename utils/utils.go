package utils

import (
	"math/big"

	"github.com/ethereum/go-ethereum/consensus/istanbul"
)

type ChainParameters struct {
	ChainId   *big.Int
	EpochSize uint64
}

func (cp *ChainParameters) IsLastBlockOfEpoch(blockNumber uint64) bool {
	return istanbul.IsLastBlockOfEpoch(blockNumber, cp.EpochSize)
}

func FromFixed(number *big.Int) float32 {
	var fixed1, _ = new(big.Float).SetString("1000000000000000000000000")
	ret := new(big.Float)
	ret.Quo(new(big.Float).SetInt(number), fixed1)
	retF, _ := ret.Float32()
	return retF
}
