package dbutils

import (
	"context"
	"database/sql"
	"testing"

	duckdb "github.com/duckdb/duckdb-go/v2"
)

func TestAppendRowsWritesBatchInsideConnectionTransaction(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open DuckDB: %v", err)
	}
	defer db.Close()

	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("get DuckDB connection: %v", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `CREATE TEMP TABLE appender_batch_test (id INTEGER, value TEXT)`); err != nil {
		t.Fatalf("create staging table: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `BEGIN TRANSACTION`); err != nil {
		t.Fatalf("begin transaction: %v", err)
	}

	err = AppendRows(ctx, conn, "", "appender_batch_test", func(appender *duckdb.Appender) error {
		for index := 0; index < 1000; index++ {
			if err := appender.AppendRow(index, "value"); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("append rows: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		t.Fatalf("commit transaction: %v", err)
	}

	var count int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM appender_batch_test`).Scan(&count); err != nil {
		t.Fatalf("count appended rows: %v", err)
	}
	if count != 1000 {
		t.Fatalf("appended row count = %d, want 1000", count)
	}
}
