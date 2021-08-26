package indexer

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"unicode"

	"cloud.google.com/go/bigquery"
)

type Row struct {
	Values map[string]bigquery.Value
	ID     *string
}

func NewRow(args ...interface{}) *Row {
	values := make(map[string]bigquery.Value)
	for i := 0; i < len(args); i += 2 {
		key := args[i].(string)
		value := args[i+1]
		values[key] = value
	}

	return &Row{Values: values}
}

func (row *Row) Extend(args ...interface{}) *Row {
	values := make(map[string]bigquery.Value)
	for k, v := range row.Values {
		values[k] = v
	}
	for i := 0; i < len(args); i += 2 {
		key := args[i].(string)
		keyRunes := []rune(key)
		keyRunes[0] = unicode.ToLower(keyRunes[0])
		key = string(keyRunes)

		value := args[i+1]
		values[key] = value
	}

	return &Row{
		Values: values,
	}
}

func (row *Row) WithId(id string) *Row {
	row.ID = &id
	return row
}

func (row *Row) getID() string {
	if row.ID != nil {
		return *row.ID
	} else {
		data, err := json.Marshal(row.Values)
		if err != nil {
			return "<n/a>"
		} else {
			return fmt.Sprintf("%x", md5.Sum(data))
		}
	}
}

// Helper method to create a row for a view call
func (row *Row) ViewCall(method string, args ...interface{}) *Row {
	viewCallRow := row.Extend("method", method, "type", "ViewCall").Extend(args...)
	return viewCallRow.WithId(fmt.Sprintf("%s.%s", row.getID(), method))
}

// Helper method to create a row for a contract
func (row *Row) Contract(contract string, args ...interface{}) *Row {
	viewCallRow := row.Extend("contract", contract).Extend(args...)
	return viewCallRow.WithId(fmt.Sprintf("%s.%s", row.getID(), contract))
}

func (row *Row) AppendID(idSuffix string) *Row {
	return row.WithId(row.getID() + "." + idSuffix)
}

func (row *Row) Save() (map[string]bigquery.Value, string, error) {
	return row.Values, row.getID(), nil
}
