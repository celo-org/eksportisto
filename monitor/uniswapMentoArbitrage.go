package monitor

import (
	"context"
	"fmt"
	"math/big"

	"github.com/celo-org/celo-blockchain/common"
	"github.com/celo-org/celo-blockchain/core/types"
	"github.com/celo-org/celo-blockchain/crypto"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/eksportisto/uniswap"
	"github.com/celo-org/kliento/celotokens"
	"github.com/celo-org/kliento/client"
	"github.com/celo-org/kliento/contracts"
)

type DecentralizedExchange int

const (
	Mento DecentralizedExchange = iota
	Uniswap
	Moola
)

type uniswapMentoArbitrageProcessor struct {
	ctx               context.Context
	logger            log.Logger
	cc                *client.CeloClient
	uniswapFactory    *uniswap.UniswapV2Factory
	exchangeContracts map[celotokens.CeloToken]*contracts.Exchange
}

type trade struct {
	sellToken  common.Address
	sellAmount *big.Int
	buyToken   common.Address
	buyAmount  *big.Int
}

var uniswapSwapEventTopic = common.BytesToHash(crypto.Keccak256([]byte("Swap(address,uint256,uint256,uint256,uint256,address)")))
var mentoSwapEventTopic = common.BytesToHash(crypto.Keccak256([]byte("Exchanged(address,uint256,uint256,bool)")))

func NewUniswapMentoArbitrageProcessor(ctx context.Context, logger log.Logger, cc *client.CeloClient, exchangeContracts map[celotokens.CeloToken]*contracts.Exchange, factoryAddress common.Address) (*uniswapMentoArbitrageProcessor, error) {
	factory, err := uniswap.NewUniswapV2Factory(factoryAddress, cc.Eth)
	if err != nil {
		return nil, err
	}

	return &uniswapMentoArbitrageProcessor{
		ctx:               ctx,
		logger:            logger,
		cc:                cc,
		uniswapFactory:    factory,
		exchangeContracts: exchangeContracts,
	}, nil
}

// Thinking this can return an array of "trades", eg
// [Ubeswap CELO -> mcUSD, Moola mcUSD -> cUSD, Mento cUSD -> CELO]
// and return nil trades if the transaction is not an arb transaction
func (p uniswapMentoArbitrageProcessor) TryParseArbitrageTransaction(txReceipt *types.Receipt) ([]trade, error) {
	// 1. (Mento || Ubeswap): assetA -> assetB
	//    eg: 10 -> 100
	// 2. Optional: Moola unwrap || wrap assetB
	//    eg: 100 -> 100
	// 3. (Ubeswap || Mento): assetB -> assetA
	//    eg: 100 -> 11, profit of 11 - 10 = 1 assetA

	var trades []trade

	for _, log := range txReceipt.Logs {
		topic0 := log.Topics[0]
		if topic0 == uniswapSwapEventTopic {
			// TODO maybe: Confirm the address is that of a uniswap pair by looking it up in the factory
			pair, err := uniswap.NewUniswapV2Pair(log.Address, p.cc.Eth)
			if err != nil {
				return nil, err
			}
			token0, err := pair.Token0(nil)
			if err != nil {
				return nil, err
			}
			token1, err := pair.Token1(nil)
			if err != nil {
				return nil, err
			}
			// Order of token0, token1 doesn't matter
			pairFromFactory, err := p.uniswapFactory.GetPair(nil, token0, token1)
			if err != nil {
				return nil, err
			}
			// Also catches if pairFromFactory is the zero address
			if pairFromFactory != log.Address {
				return nil, fmt.Errorf("Log pair address %s not equal to factory pair %s", log.Address.Hex(), pairFromFactory.Hex())
			}
			swap, err := pair.ParseSwap(log)
			if err != nil {
				return nil, err
			}
			netAmountOut0 := swap.Amount0Out.Sub(swap.Amount0In)
			netAmountOut1 := swap.Amount1Out.Sub(swap.Amount1In)
			zero := big.NewInt(0)
			var sellToken common.Address
			if netAmountOut0.Cmp(zero) > 0 {
				sellToken = 
			}
			trades = append(trades, trade{
				
			})
		}
	}
	return trades, nil
}
