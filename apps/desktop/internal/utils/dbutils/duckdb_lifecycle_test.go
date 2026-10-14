package dbutils

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"testing"
)

func TestIsWALReplayError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "duckdb replaying wal",
			err:  errors.New(`Failure while replaying WAL file "lunabox.db.wal"`),
			want: true,
		},
		{
			name: "wal replay wording",
			err:  errors.New("database could not complete WAL replay"),
			want: true,
		},
		{
			name: "non wal startup failure",
			err:  errors.New("database file is locked"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsWALReplayError(tt.err); got != tt.want {
				t.Fatalf("IsWALReplayError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenDuckDBWithWALRecoveryDeletesWALAndRetries(t *testing.T) {
	originalOpen := openDuckDB
	originalStat := statFile
	originalRemove := removeFile
	defer func() {
		openDuckDB = originalOpen
		statFile = originalStat
		removeFile = originalRemove
	}()

	var attempts int
	openDuckDB = func(ctx context.Context, dbPath string) (*sql.DB, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("Failure while replaying WAL file")
		}
		return nil, nil
	}

	var removedPath string
	statFile = func(name string) (os.FileInfo, error) {
		return nil, nil
	}
	removeFile = func(name string) error {
		removedPath = name
		return nil
	}

	_, err := OpenDuckDBWithWALRecovery(context.Background(), "C:/data/lunabox.db", nil)
	if err != nil {
		t.Fatalf("OpenDuckDBWithWALRecovery() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("open attempts = %d, want 2", attempts)
	}
	if removedPath != "C:/data/lunabox.db.wal" {
		t.Fatalf("removed path = %q", removedPath)
	}
}

func TestOpenDuckDBWithWALRecoveryDoesNotDeleteForNonWALError(t *testing.T) {
	originalOpen := openDuckDB
	originalRemove := removeFile
	defer func() {
		openDuckDB = originalOpen
		removeFile = originalRemove
	}()

	openDuckDB = func(ctx context.Context, dbPath string) (*sql.DB, error) {
		return nil, errors.New("database file is locked")
	}
	removeFile = func(name string) error {
		t.Fatalf("removeFile should not be called for non-WAL error")
		return nil
	}

	_, err := OpenDuckDBWithWALRecovery(context.Background(), "lunabox.db", nil)
	if err == nil {
		t.Fatal("OpenDuckDBWithWALRecovery() error = nil, want error")
	}
}

func TestOpenDuckDBWithWALRecoveryFailsWhenWALMissing(t *testing.T) {
	originalOpen := openDuckDB
	originalStat := statFile
	defer func() {
		openDuckDB = originalOpen
		statFile = originalStat
	}()

	openDuckDB = func(ctx context.Context, dbPath string) (*sql.DB, error) {
		return nil, errors.New("Failure while replaying WAL file")
	}
	statFile = func(name string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	_, err := OpenDuckDBWithWALRecovery(context.Background(), "lunabox.db", nil)
	if err == nil {
		t.Fatal("OpenDuckDBWithWALRecovery() error = nil, want error")
	}
}

func TestSafeCloseDuckDBClosesWhenCheckpointFails(t *testing.T) {
	driverName := "duckdbutils_safe_close_test"
	closeCount.Store(0)
	sql.Register(driverName, failingCheckpointDriver{})

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}

	err = SafeCloseDuckDB(context.Background(), db, nil)
	if err == nil {
		t.Fatal("SafeCloseDuckDB() error = nil, want checkpoint error")
	}
	if got := closeCount.Load(); got != 1 {
		t.Fatalf("close count = %d, want 1", got)
	}
}

var closeCount atomic.Int32

type failingCheckpointDriver struct{}

func (failingCheckpointDriver) Open(name string) (driver.Conn, error) {
	return failingCheckpointConn{}, nil
}

type failingCheckpointConn struct{}

func (failingCheckpointConn) Prepare(query string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepare unsupported")
}

func (failingCheckpointConn) Close() error {
	closeCount.Add(1)
	return nil
}

func (failingCheckpointConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("begin unsupported")
}

func (failingCheckpointConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return nil, errors.New("checkpoint failed")
}

func (failingCheckpointConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return emptyRows{}, nil
}

func (failingCheckpointConn) Ping(ctx context.Context) error {
	return nil
}

type emptyRows struct{}

func (emptyRows) Columns() []string {
	return []string{}
}

func (emptyRows) Close() error {
	return nil
}

func (emptyRows) Next(dest []driver.Value) error {
	return io.EOF
}
