package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/bigquery"
	"github.com/celo-org/eksportisto/metrics"
	"github.com/spf13/viper"
)

type Output interface {
	Write(context.Context, []*Row) error
}

type stdoutOutput struct{}

func newStdoutOutput() Output {
	return &stdoutOutput{}
}

func (*stdoutOutput) Write(ctx context.Context, rows []*Row) error {
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

func (bqo *bigqueryOutput) Write(ctx context.Context, rows []*Row) error {
	if err := bqo.inserter.Put(ctx, rows); err != nil {
		if err2 := bqo.handleBigQueryError(ctx, err, rows); err2 != nil {
			return err2
		}
	}
	metrics.RowsInserted.Add(float64(len(rows)))
	return nil
}

func (bqo *bigqueryOutput) handleBigQueryError(ctx context.Context, writeError error, rows []*Row) error {
	if putMultiError, ok := writeError.(bigquery.PutMultiError); ok {
		if bigQueryError, ok := putMultiError[0].Errors[0].(*bigquery.Error); ok {
			if strings.Contains(bigQueryError.Message, "no such field") {
				errAddColumn := bqo.updateTableAddColumn(ctx,bigQueryError.Location)
				if errAddColumn != nil {
					return errAddColumn
				}
				if errOutputWrite := bqo.Write(ctx, rows); errOutputWrite != nil {
					return errOutputWrite
				}
			} else {
				return putMultiError
			}
		} else {
			return putMultiError
		}
	} else {
		return writeError
	}
	return nil
}

func  (bqo *bigqueryOutput) updateTableAddColumn(ctx context.Context, fieldName string) error {
	projectID := viper.GetString("indexer.bigquery.projectID")
	datasetID := viper.GetString("indexer.bigquery.dataset")
	tableID := viper.GetString("indexer.bigquery.table")

	if projectID == "" {
		return errors.New("projectID is not defined")
	}

	tableRef := bqo.client.Dataset(datasetID).Table(tableID)
	meta, err := tableRef.Metadata(ctx)
	if err != nil {
		return err
	}
	newSchema := append(meta.Schema,
		&bigquery.FieldSchema{Name: fieldName, Type: bigquery.StringFieldType},
	)
	update := bigquery.TableMetadataToUpdate{
		Schema: newSchema,
	}
	if _, err := tableRef.Update(ctx, update, meta.ETag); err != nil {
		return err
	}
	fmt.Println("Updated table schema, added field: ", fieldName)
	return nil
}
