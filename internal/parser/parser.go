// Package parse reads CSV/XLSX submissions, sniffs their schema, and reports per-row errors
package parser

type ColumnType string

const (
	ColumnTypeString ColumnType = "string"
	ColumnTypeNumber ColumnType = "number"
	ColumnTypeDate ColumnType = "date"
)

type Column struct {
	Name string
	Type ColumnType
}

type RowError struct {
	Row int
	Column string
	Message string
}

type Result struct {
	Columns []Column
	RowCount int
	Errors []RowError
}

// Sniffs schema form the headr + a sample of rows
func parseCSV(path string) (Result, error) {
	panic("TODO")
}

// Reads the first sheet of the XLSX file, sniffs schema form the headr + a sample of rows
func parseXLSX(path string) (Result, error) {
	panic("TODO")
}

// Infers a ColumnType from a sample value
func sniffColumnType(sample []string) ColumnType {
	panic("TODO")
}
