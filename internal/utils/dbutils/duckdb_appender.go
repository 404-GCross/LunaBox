package dbutils

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"

	duckdb "github.com/duckdb/duckdb-go/v2"
)

// AppendRows writes rows through DuckDB's vectorized Appender API. The caller
// controls the connection transaction and must create the destination table.
func AppendRows(
	ctx context.Context,
	conn *sql.Conn,
	schema string,
	table string,
	appendRows func(*duckdb.Appender) error,
) error {
	if conn == nil {
		return errors.New("DuckDB connection is nil")
	}
	if table == "" {
		return errors.New("DuckDB appender table is empty")
	}
	if appendRows == nil {
		return nil
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return conn.Raw(func(driverConn any) error {
		duckConn, ok := driverConn.(driver.Conn)
		if !ok {
			return fmt.Errorf("DuckDB raw connection has unexpected type %T", driverConn)
		}

		appender, err := duckdb.NewAppenderFromConn(duckConn, schema, table)
		if err != nil {
			return fmt.Errorf("create DuckDB appender for %s: %w", table, err)
		}
		if err := appendRows(appender); err != nil {
			if closeErr := appender.Close(); closeErr != nil {
				return errors.Join(err, fmt.Errorf("close DuckDB appender for %s: %w", table, closeErr))
			}
			return err
		}
		if err := appender.Close(); err != nil {
			return fmt.Errorf("close DuckDB appender for %s: %w", table, err)
		}
		return nil
	})
}
