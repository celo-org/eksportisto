package utils

import (
	"math/big"

	"github.com/celo-org/kliento/contracts/helpers"
)

func ShouldSample(blockNumber uint64, checkPoint uint64) bool {
	number := blockNumber % checkPoint
	if number == 0 {
		return true
	} else {
		return false
	}
}

func ScaleFixed(number *big.Int) float64 {
	retF, _ := new(big.Float).Quo(new(big.Float).SetInt(number), big.NewFloat(1e18)).Float64()
	return retF
}

func BoolToFloat64(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func MeanFromFixed(xs []*big.Int) float64 {
	total := 0.0
	if len(xs) == 0 {
		return 0.0
	}
	for _, v := range xs {
		val := helpers.FromFixed(v)
		total += val
	}
	return total / float64(len(xs))
}

// Returns 0 if denominator is 0
func DivideBigInts(numerator *big.Int, denominator *big.Int) *big.Float {
	res := big.NewFloat(0)

	if denominator.Cmp(big.NewInt(0)) != 0 {
		retN := new(big.Float).SetInt(numerator)
		retD := new(big.Float).SetInt(denominator)
		res = new(big.Float).Quo(retN, retD)
	}

	return res
}
