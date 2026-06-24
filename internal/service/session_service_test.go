package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/applog"

	_ "github.com/duckdb/duckdb-go/v2"
)

func setupSessionServiceTestDB(t *testing.T) *sql.DB {
	t.Helper()

	applog.SetMode(applog.ModeCLI)

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	queries := []string{
		`CREATE TABLE IF NOT EXISTS play_sessions (
			id TEXT PRIMARY KEY,
			game_id TEXT,
			start_time TIMESTAMPTZ,
			end_time TIMESTAMPTZ,
			duration INTEGER,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sync_tombstones (
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			parent_id TEXT DEFAULT '',
			secondary_id TEXT DEFAULT '',
			deleted_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (entity_type, entity_id, parent_id, secondary_id)
		)`,
	}
	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Fatalf("init test schema: %v", err)
		}
	}

	return db
}

func TestCompleteUnfinishedSessionWithDurationUsesExplicitEndTime(t *testing.T) {
	db := setupSessionServiceTestDB(t)
	sessionService := NewSessionService()
	sessionService.Init(context.Background(), db, &appconf.AppConfig{})

	startTime := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	endTime := time.Now().Truncate(time.Second)
	if _, err := db.Exec(
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		"session-active",
		"game-1",
		startTime,
		startTime,
		0,
		startTime,
	); err != nil {
		t.Fatalf("insert pending session: %v", err)
	}

	if err := sessionService.completeUnfinishedSessionWithDuration("session-active", endTime, 125); err != nil {
		t.Fatalf("complete unfinished session: %v", err)
	}

	var storedEnd time.Time
	var duration int
	if err := db.QueryRow(
		`SELECT end_time, duration FROM play_sessions WHERE id = ?`,
		"session-active",
	).Scan(&storedEnd, &duration); err != nil {
		t.Fatalf("query completed session: %v", err)
	}

	if duration != 125 {
		t.Fatalf("expected active duration 125, got %d", duration)
	}
	if storedEnd.Sub(endTime).Abs() > time.Second {
		t.Fatalf("expected end_time %s, got %s", endTime, storedEnd)
	}
}

func TestCompleteUnfinishedSessionWithDurationDeletesShortActiveSession(t *testing.T) {
	db := setupSessionServiceTestDB(t)
	sessionService := NewSessionService()
	sessionService.Init(context.Background(), db, &appconf.AppConfig{})

	startTime := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	endTime := time.Now().Truncate(time.Second)
	if _, err := db.Exec(
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		"session-short-active",
		"game-1",
		startTime,
		startTime,
		0,
		startTime,
	); err != nil {
		t.Fatalf("insert pending session: %v", err)
	}

	if err := sessionService.completeUnfinishedSessionWithDuration("session-short-active", endTime, 12); err != nil {
		t.Fatalf("complete short unfinished session: %v", err)
	}

	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM play_sessions WHERE id = ?`,
		"session-short-active",
	).Scan(&count); err != nil {
		t.Fatalf("query session count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected short active session to be deleted, found %d rows", count)
	}
}
