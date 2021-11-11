package indexer

import (
	"fmt"

	"github.com/celo-org/eksportisto/rdb"
)

const (
	Tip      = "tip"
	Backfill = "backfill"
)

func ParseInput(input string) (rdb.Queue, error) {
	if input == Tip {
		return rdb.TipQueue, nil
	} else if input == Backfill {
		return rdb.BackfillQueue, nil
	}
	return "", fmt.Errorf("invalid input: %s", input)
}
