package database

import (
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// ExportOptions contains configuration for database export
type ExportOptions struct {
	Table      string
	ID         interface{}
	OutputFile string
	DBUrl      string
}

// ForeignKey represents a foreign key relationship
type ForeignKey struct {
	ConstraintName    string
	TableName         string
	ColumnName        string
	ForeignTableName  string
	ForeignColumnName string
}

// Record represents a database record with table and column data
type Record struct {
	Table   string
	Columns []string
	Values  []interface{}
}

// Exporter handles database export operations
type Exporter struct {
	db      *sql.DB
	visited map[string]bool // Track visited records to avoid cycles
	records []Record        // Collected records in dependency order
}

// NewExporter creates a new database exporter
func NewExporter(dbUrl string) (*Exporter, error) {
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Exporter{
		db:      db,
		visited: make(map[string]bool),
		records: []Record{},
	}, nil
}

// Close closes the database connection
func (e *Exporter) Close() error {
	return e.db.Close()
}

// Export recursively exports a record and all its dependencies
func (e *Exporter) Export(table string, id interface{}) ([]Record, error) {
	// Reset state for new export
	e.visited = make(map[string]bool)
	e.records = []Record{}

	// Start recursive export
	if err := e.exportRecord(table, id); err != nil {
		return nil, err
	}

	return e.records, nil
}

// exportRecord recursively exports a single record and its dependencies
func (e *Exporter) exportRecord(table string, id interface{}) error {
	// Create unique key for this record
	recordKey := fmt.Sprintf("%s:%v", table, id)

	// Skip if already visited (handles circular dependencies)
	if e.visited[recordKey] {
		return nil
	}
	e.visited[recordKey] = true

	// Get the primary key column name
	pkColumn, err := e.getPrimaryKeyColumn(table)
	if err != nil {
		return fmt.Errorf("failed to get primary key for table %s: %w", table, err)
	}

	// Get foreign keys that this record references
	foreignKeys, err := e.getForeignKeys(table)
	if err != nil {
		return fmt.Errorf("failed to get foreign keys for table %s: %w", table, err)
	}

	// Fetch the record
	record, err := e.fetchRecord(table, pkColumn, id)
	if err != nil {
		return fmt.Errorf("failed to fetch record from %s: %w", table, err)
	}

	if record == nil {
		return fmt.Errorf("record not found: %s.%s = %v", table, pkColumn, id)
	}

	// Recursively export foreign key dependencies first
	for _, fk := range foreignKeys {
		// Find the column index
		colIndex := -1
		for i, col := range record.Columns {
			if col == fk.ColumnName {
				colIndex = i
				break
			}
		}

		if colIndex == -1 {
			continue
		}

		// Get the foreign key value
		fkValue := record.Values[colIndex]

		// Skip NULL foreign keys
		if fkValue == nil {
			continue
		}

		// Recursively export the referenced record
		if err := e.exportRecord(fk.ForeignTableName, fkValue); err != nil {
			// Log warning but continue - some FKs might be optional
			fmt.Printf("Warning: failed to export FK %s.%s -> %s: %v\n",
				table, fk.ColumnName, fk.ForeignTableName, err)
		}
	}

	// Add this record after its dependencies
	e.records = append(e.records, *record)

	return nil
}

// fetchRecord retrieves a single record from the database
func (e *Exporter) fetchRecord(table string, pkColumn string, id interface{}) (*Record, error) {
	// Get column names
	columns, err := e.getTableColumns(table)
	if err != nil {
		return nil, err
	}

	// Build query
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1",
		strings.Join(columns, ", "),
		table,
		pkColumn,
	)

	// Execute query
	row := e.db.QueryRow(query, id)

	// Prepare scan destinations
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// Scan row
	if err := row.Scan(valuePtrs...); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &Record{
		Table:   table,
		Columns: columns,
		Values:  values,
	}, nil
}

// getTableColumns returns all column names for a table
func (e *Exporter) getTableColumns(table string) ([]string, error) {
	query := `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_name = $1
		ORDER BY ordinal_position
	`

	rows, err := e.db.Query(query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}

	return columns, nil
}

// getPrimaryKeyColumn returns the primary key column name for a table
func (e *Exporter) getPrimaryKeyColumn(table string) (string, error) {
	query := `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indrelid = $1::regclass
		AND i.indisprimary
	`

	var pkColumn string
	err := e.db.QueryRow(query, table).Scan(&pkColumn)
	if err != nil {
		return "", err
	}

	return pkColumn, nil
}

// getForeignKeys returns all foreign key relationships for a table
func (e *Exporter) getForeignKeys(table string) ([]ForeignKey, error) {
	query := `
		SELECT
			tc.constraint_name,
			tc.table_name,
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_name = $1
	`

	rows, err := e.db.Query(query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		if err := rows.Scan(&fk.ConstraintName, &fk.TableName, &fk.ColumnName,
			&fk.ForeignTableName, &fk.ForeignColumnName); err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}

	return fks, nil
}

// GenerateSQL converts records to SQL INSERT statements
func GenerateSQL(records []Record, writer io.Writer) error {
	// Write header
	fmt.Fprintf(writer, "-- Database export generated by agentenv\n")
	fmt.Fprintf(writer, "-- Generated at: %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(writer, "BEGIN;\n\n")

	// Generate INSERT statements for each record
	for _, record := range records {
		// Build column list
		columns := strings.Join(record.Columns, ", ")

		// Build value list with proper escaping
		values := make([]string, len(record.Values))
		for i, val := range record.Values {
			values[i] = formatValue(val)
		}
		valueList := strings.Join(values, ", ")

		// Generate INSERT statement with ON CONFLICT DO NOTHING to handle duplicates
		fmt.Fprintf(writer, "INSERT INTO %s (%s)\n", record.Table, columns)
		fmt.Fprintf(writer, "VALUES (%s)\n", valueList)
		fmt.Fprintf(writer, "ON CONFLICT DO NOTHING;\n\n")
	}

	fmt.Fprintf(writer, "COMMIT;\n")

	return nil
}

// formatValue formats a value for SQL INSERT statement
func formatValue(val interface{}) string {
	if val == nil {
		return "NULL"
	}

	switch v := val.(type) {
	case []byte:
		// Handle bytea columns
		return fmt.Sprintf("'%s'", escapeSQLString(string(v)))
	case string:
		return fmt.Sprintf("'%s'", escapeSQLString(v))
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%v", v)
	case float32, float64:
		return fmt.Sprintf("%v", v)
	case time.Time:
		return fmt.Sprintf("'%s'", v.Format(time.RFC3339))
	default:
		// For complex types (JSONB, arrays, etc.), convert to string and escape
		str := fmt.Sprintf("%v", v)
		return fmt.Sprintf("'%s'", escapeSQLString(str))
	}
}

// escapeSQLString escapes single quotes and backslashes for SQL strings
func escapeSQLString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "''")
	return s
}
