package importer

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
)

func TestCleanupStagingTablesDropsEveryImportTemporaryTable(t *testing.T) {
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

	for _, tableName := range stagingTableNames() {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`CREATE TEMP TABLE %s (id INTEGER)`, tableName)); err != nil {
			t.Fatalf("create temporary table %s: %v", tableName, err)
		}
	}

	CleanupStagingTables(ctx, conn)
	for _, tableName := range stagingTableNames() {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`SELECT 1 FROM %s LIMIT 1`, tableName)); err == nil {
			t.Fatalf("temporary table %s still exists after cleanup", tableName)
		}
	}
}
