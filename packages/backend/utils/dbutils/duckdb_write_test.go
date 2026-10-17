package dbutils

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryDuckDBWriteConflictRetriesCompleteWrite(t *testing.T) {
	var attempts atomic.Int32
	err := RetryDuckDBWriteConflict(context.Background(), func() error {
		if attempts.Add(1) < 3 {
			return errors.New("TransactionContext Error: Failed to commit: write-write conflict on key")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RetryDuckDBWriteConflict() error = %v", err)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

func TestRetryDuckDBWriteConflictStopsOnOtherError(t *testing.T) {
	var attempts atomic.Int32
	wantErr := errors.New("constraint violation")
	err := RetryDuckDBWriteConflict(context.Background(), func() error {
		attempts.Add(1)
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("RetryDuckDBWriteConflict() error = %v, want %v", err, wantErr)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}

func TestWithDuckDBWriteLockSerializesSameDatabase(t *testing.T) {
	db := &sql.DB{}
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondEntered := make(chan struct{})
	firstDone := make(chan struct{})
	secondDone := make(chan struct{})

	go func() {
		defer close(firstDone)
		_ = WithDuckDBWriteLock(db, func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	<-firstEntered

	go func() {
		defer close(secondDone)
		_ = WithDuckDBWriteLock(db, func() error {
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("second writer entered while first writer held the lock")
	case <-time.After(25 * time.Millisecond):
	}

	close(releaseFirst)
	select {
	case <-secondEntered:
	case <-time.After(time.Second):
		t.Fatal("second writer did not enter after the first writer released the lock")
	}
	<-firstDone
	<-secondDone
}
