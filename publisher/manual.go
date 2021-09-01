package publisher

import (
	"context"
	"fmt"

	"github.com/celo-org/eksportisto/indexer"
	"github.com/celo-org/eksportisto/rdb"
	"github.com/celo-org/kliento/client"
	"github.com/spf13/viper"
)

type ManualPublisherMode = string

const (
	EpochBlocks ManualPublisherMode = "epoch-blocks"
	BlocksList  ManualPublisherMode = "blocks-list"
)

type manualPublisher struct {
	mode       ManualPublisherMode
	blocks     []int
	celoClient *client.CeloClient
	db         *rdb.RedisDB
}

func newManualPublisherMode(mode string) (ManualPublisherMode, error) {
	if mode == EpochBlocks {
		return EpochBlocks, nil
	} else if mode == BlocksList {
		return BlocksList, nil
	} else {
		return "", fmt.Errorf("%s is not a valid manual publisher mode", mode)
	}
}

func newManualPublisher(_ context.Context) (publisher, error) {
	if !viper.GetBool("publisher.manual.enabled") {
		return nil, nil
	}

	db := rdb.NewRedisDatabase()

	celoClient, err := client.Dial(viper.GetString("celoNodeURI"))
	if err != nil {
		return nil, err
	}

	mode, err := newManualPublisherMode(viper.GetString("publisher.manual.mode"))
	if err != nil {
		return nil, err
	}

	var blocks []int
	if mode == BlocksList {
		blocks = viper.GetIntSlice("publisher.manual.blocks")
	}

	return &manualPublisher{mode, blocks, celoClient, db}, nil
}

func (svc *manualPublisher) start(ctx context.Context) error {
	blocks, err := svc.getBlocks(ctx)
	if err != nil {
		return err
	}

	for _, block := range blocks {
		err := svc.db.RPush(ctx, rdb.BackfillQueue, block).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

func (svc *manualPublisher) getBlocks(ctx context.Context) ([]int, error) {
	if svc.mode == BlocksList {
		return svc.blocks, nil
	} else {
		latestBlock, err := svc.celoClient.Eth.BlockByNumber(ctx, nil)
		if err != nil {
			return nil, err
		}
		blockHeight := latestBlock.Number().Uint64()
		epochs := int(blockHeight / indexer.EpochSize)
		blocks := make([]int, 0, epochs)
		for i := 1; i <= epochs; i++ {
			blocks = append(blocks, i*int(indexer.EpochSize))
		}
		return blocks, nil
	}
}
