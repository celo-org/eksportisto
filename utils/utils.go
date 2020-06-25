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
		val := float64(FromFixed(v))
		total += val
	}
	return total / float64(len(xs))
}

// Returns 0 if dem is 0
func DivideBigInts(num *big.Int, dem *big.Int) *big.Float {
	res := big.NewFloat(0)

	if dem.Cmp(big.NewInt(0)) != 0 {
		retN := new(big.Float).SetInt(num)
		retD := new(big.Float).SetInt(dem)
		res = new(big.Float).Quo(retN, retD)
	}

	return res
}
