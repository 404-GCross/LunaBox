package dbutils

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
)

// Appender writes rows through DuckDB's vectorized Appender API.
// *duckdb.Appender satisfies it.
type Appender interface {
	AppendRow(args ...driver.Value) error
	Close() error
}

// AppenderFactory creates an Appender on a raw DuckDB driver connection.
type AppenderFactory func(conn driver.Conn, schema, table string) (Appender, error)

var (
	appenderMu      sync.RWMutex
	appenderFactory AppenderFactory
)

// RegisterAppender installs the DuckDB Appender implementation. It is called by
// lunabox/backend/utils/dbutils/duckdbdriver, which keeps this package (and
// every service that appends rows) free of the cgo DuckDB dependency.
func RegisterAppender(factory AppenderFactory) {
	appenderMu.Lock()
	appenderFactory = factory
	appenderMu.Unlock()
}

// AppendRows writes rows through DuckDB's vectorized Appender API. The caller
// controls the connection transaction and must create the destination table.
func AppendRows(
	ctx context.Context,
	conn *sql.Conn,
	schema string,
	table string,
	appendRows func(Appender) error,
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
	appenderMu.RLock()
	newAppender := appenderFactory
	appenderMu.RUnlock()
	if newAppender == nil {
		return errors.New("DuckDB appender is not registered: import lunabox/backend/utils/dbutils/duckdbdriver")
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

		appender, err := newAppender(duckConn, schema, table)
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
