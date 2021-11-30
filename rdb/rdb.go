package rdb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

type Queue = string

const (
	PriorityQueue               Queue  = "blocks:queue"
	BackfillQueue               Queue  = "blocks:queue:backfill"
	TipQueue                    Queue  = "blocks:queue:tip"
	BackfillCursor              string = "blocks:cursor"
	BlocksMap                   string = "blocks:indexed"
	GetIndexedBlocksBatchScript string = `
local startKey = tonumber(ARGV[1])
local endKey = startKey + tonumber(ARGV[2])
local res = {}

for i = startKey,endKey do
    res[i-startKey+1] = redis.call('HGET', KEYS[1], i)
end

return res`
)

type RedisDB struct {
	*redis.Client
}

func (db *RedisDB) GetBlocksBatch(ctx context.Context, cursor, size uint64) (map[uint64]bool, error) {
	blocks := make(map[uint64]bool)
	result, err := db.Eval(ctx, GetIndexedBlocksBatchScript, []string{BlocksMap}, cursor, size).Result()
	if err != nil {
		return nil, err
	}

	resultList, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("return value is not a list")
	}

	for index, result := range resultList {
		blockHeight := cursor + uint64(index)
		blocks[blockHeight] = !(result == nil)
	}

	return blocks, nil
}

func (db *RedisDB) EnqueueBlock(ctx context.Context, blockNumber uint64) error {
	return db.ZAdd(ctx, PriorityQueue, &redis.Z{Score: float64(blockNumber), Member: blockNumber}).Err()
}

func (db *RedisDB) PopBlock(ctx context.Context, dequeueTimeout time.Duration) (uint64, error) {
	result, err := db.BZPopMax(ctx, dequeueTimeout, PriorityQueue).Result()
	if err != nil && err != redis.Nil {
		return 0, err
	}
	return strconv.ParseUint(result.Z.Member.(string), 10, 64)
}

func (db *RedisDB) QueueLength(ctx context.Context) (uint64, error) {
	return db.ZCard(ctx, PriorityQueue).Uint64()
}

func NewRedisDatabase() *RedisDB {
	client := redis.NewClient(&redis.Options{
		Addr:     viper.GetString("redis.address"),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
	})
	return &RedisDB{client}
}
