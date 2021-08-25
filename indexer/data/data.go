package data

import "fmt"

type Row struct {
	Values map[string]interface{}
	ID     *string
}

func NewRow(args ...interface{}) *Row {
	values := make(map[string]interface{})
	for i := 0; i < len(args); i += 2 {
		key := args[i].(string)
		value := args[i+1]
		values[key] = value
	}

	return &Row{Values: values}
}

func (row *Row) Extend(args ...interface{}) *Row {
	values := make(map[string]interface{})
	for k, v := range row.Values {
		values[k] = v
	}
	for i := 0; i < len(args); i += 2 {
		key := args[i].(string)
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
		return "<n/a>"
	}
}

// Helper method to create a row for a view call
func (row *Row) ViewCall(method string, args ...interface{}) *Row {
	viewCallRow := row.Extend("method", method).Extend(args...)
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
