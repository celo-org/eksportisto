package monitor

import (
	"context"

	"github.com/celo-org/eksportisto/metrics"
	"github.com/celo-org/kliento/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/log"
)

func stableTokenStateViewCalls(ctx context.Context, logger log.Logger, opts *bind.CallOpts, stableToken *contracts.StableToken) error {
	logger = logger.New("contract", "StableToken")

	if totalSupply, err := stableToken.TotalSupply(opts); err != nil {
		metrics.TotalCUSDSupply.Observe(float64(totalSupply.Uint64()))
		logStateViewCall(logger, "method", "totalSupply", "totalSupply", totalSupply)
	}

	return nil
}
