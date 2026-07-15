package dbutils

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Logger interface {
	Info(message string)
	Warning(message string)
	Error(message string)
}

var (
	openDuckDB = func(ctx context.Context, dbPath string) (*sql.DB, error) {
		db, err := sql.Open("duckdb", dbPath)
		if err != nil {
			return nil, err
		}
		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			return nil, err
		}
		return db, nil
	}
	statFile   = os.Stat
	removeFile = os.Remove
)

func OpenDuckDBWithWALRecovery(ctx context.Context, dbPath string, logger Logger) (*sql.DB, error) {
	db, err := openDuckDB(ctx, dbPath)
	if err == nil {
		return db, nil
	}

	if !IsWALReplayError(err) {
		if logger != nil {
			logger.Error("DuckDB startup failed before recovery: " + err.Error())
		}
		return nil, err
	}

	walPath := dbPath + ".wal"
	if logger != nil {
		logger.Warning("DuckDB startup failed while replaying WAL: " + err.Error())
		logger.Warning("discarding unreplayable DuckDB WAL and reverting to the last checkpoint: " + walPath)
	}

	if _, statErr := statFile(walPath); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, fmt.Errorf("DuckDB WAL replay failed but WAL file does not exist at %s: %w", walPath, err)
		}
		return nil, fmt.Errorf("stat DuckDB WAL file %s after WAL replay failure: %w (original error: %v)", walPath, statErr, err)
	}

	if removeErr := removeFile(walPath); removeErr != nil {
		return nil, fmt.Errorf("delete DuckDB WAL file %s after WAL replay failure: %w (original error: %v)", walPath, removeErr, err)
	}

	if logger != nil {
		logger.Warning("DuckDB WAL file deleted; uncheckpointed changes may be lost, retrying database open")
	}

	retryDB, retryErr := openDuckDB(ctx, dbPath)
	if retryErr != nil {
		return nil, fmt.Errorf("open DuckDB after deleting WAL file %s: %w (original WAL replay error: %v)", walPath, retryErr, err)
	}

	if logger != nil {
		logger.Info("DuckDB opened successfully from the last checkpoint after deleting the unreplayable WAL")
	}
	return retryDB, nil
}

// CheckpointDuckDB writes committed WAL changes into the main database file.
// It intentionally uses a regular checkpoint so runtime callers do not abort
// other active transactions.
func CheckpointDuckDB(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	if _, err := db.ExecContext(ctx, "CHECKPOINT"); err != nil {
		return fmt.Errorf("checkpoint DuckDB: %w", err)
	}
	return nil
}

func IsWALReplayError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "wal") &&
		(strings.Contains(message, "replay") || strings.Contains(message, "replaying"))
}

func SafeCloseDuckDB(ctx context.Context, db *sql.DB, logger Logger) error {
	if db == nil {
		return nil
	}

	if logger != nil {
		logger.Info("DuckDB checkpoint started before close")
	}

	_, checkpointErr := db.ExecContext(ctx, "FORCE CHECKPOINT")
	if checkpointErr != nil {
		if logger != nil {
			logger.Error("DuckDB checkpoint before close failed: " + checkpointErr.Error())
		}
	} else if logger != nil {
		logger.Info("DuckDB checkpoint before close succeeded")
	}

	if logger != nil {
		logger.Info("DuckDB close started")
	}
	closeErr := db.Close()
	if closeErr != nil {
		if logger != nil {
			logger.Error("DuckDB close failed: " + closeErr.Error())
		}
	} else if logger != nil {
		logger.Info("DuckDB close succeeded")
	}

	return errors.Join(checkpointErr, closeErr)
}
