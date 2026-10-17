package dbutils

// The lifecycle tests open real DuckDB databases. They cannot import
// dbutils/duckdbdriver (it imports this package), so register the
// database/sql driver directly.
import _ "github.com/duckdb/duckdb-go/v2"
