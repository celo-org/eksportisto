package indexer

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/bigquery"
	"github.com/spf13/viper"
)

type Output interface {
	Write([]*Row) error
}

type stdoutOutput struct{}

func newStdoutOutput() Output {
	return &stdoutOutput{}
}

func (*stdoutOutput) Write(rows []*Row) error {
	for _, row := range rows {
		data, err := json.Marshal(row.Values)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	}
	return nil
}

type bigqueryOutput struct {
	client   *bigquery.Client
	inserter *bigquery.Inserter
}

func newBigQueryOutput(ctx context.Context) (Output, error) {
	projectID := viper.GetString("indexer.bigquery.projectID")
	dataset := viper.GetString("indexer.bigquery.dataset")
	table := viper.GetString("indexer.bigquery.table")

	if projectID == "" {
		return nil, fmt.Errorf("projectID is not defined")
	}

	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	inserter := client.Dataset(dataset).Table(table).Inserter()

	return &bigqueryOutput{client, inserter}, nil
}

func (bqo *bigqueryOutput) Write(rows []*Row) error {
	return bqo.inserter.Put(context.Background(), rows)
}
