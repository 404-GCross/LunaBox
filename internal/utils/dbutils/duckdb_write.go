package dbutils

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"
)

const defaultDuckDBWriteAttempts = 3

var duckDBWriteLocks sync.Map

// WithDuckDBWriteLock serializes coordinated writers that share one database
// handle. It leaves reads and unrelated database handles independent.
func WithDuckDBWriteLock(db *sql.DB, write func() error) error {
	if write == nil {
		return nil
	}
	if db == nil {
		return errors.New("DuckDB handle is nil")
	}

	value, _ := duckDBWriteLocks.LoadOrStore(db, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	return write()
}

// RetryDuckDBWriteConflict reruns a complete autocommit statement or
// transaction after DuckDB aborts it because of a concurrent writer.
func RetryDuckDBWriteConflict(ctx context.Context, write func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var err error
	for attempt := 0; attempt < defaultDuckDBWriteAttempts; attempt++ {
		err = write()
		if err == nil || !IsDuckDBWriteConflict(err) {
			return err
		}
		if attempt == defaultDuckDBWriteAttempts-1 {
			break
		}

		delay := time.Duration(attempt+1) * 25 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return errors.Join(err, ctx.Err())
		}
	}
	return err
}

func IsDuckDBWriteConflict(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "transactioncontext error") &&
		(strings.Contains(message, "write-write conflict") ||
			strings.Contains(message, "conflict on tuple"))
}
