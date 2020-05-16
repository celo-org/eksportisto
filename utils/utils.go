package utils

import (
	"math/big"
)

func ShouldSample(blockNumber uint64, checkPoint uint64) bool {
	number := blockNumber % checkPoint
	if number == 0 {
		return true
	} else {
		return false
	}
}

func FromFixed(number *big.Int) float32 {
	var fixed1, _ = new(big.Float).SetString("1000000000000000000000000")
	ret := new(big.Float)
	ret.Quo(new(big.Float).SetInt(number), fixed1)
	retF, _ := ret.Float32()
	return retF
}
