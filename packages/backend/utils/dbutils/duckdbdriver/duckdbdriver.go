// Package duckdbdriver registers the DuckDB database/sql driver and the
// Appender used by dbutils.AppendRows. Import it for side effects in every
// binary that opens a DuckDB database:
//
//	import _ "lunabox/backend/utils/dbutils/duckdbdriver"
//
// It is the only package that links the DuckDB library, so packages that merely
// use services (for example UI code and its tests) do not link DuckDB.
package duckdbdriver

import (
	"database/sql/driver"

	duckdb "github.com/duckdb/duckdb-go/v2"

	"lunabox/backend/utils/dbutils"
)

func init() {
	dbutils.RegisterAppender(func(conn driver.Conn, schema, table string) (dbutils.Appender, error) {
		return duckdb.NewAppenderFromConn(conn, schema, table)
	})
}
