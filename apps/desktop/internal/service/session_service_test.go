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

func TestCreatePendingSessionUsesNullEndTime(t *testing.T) {
	db := setupSessionServiceTestDB(t)
	sessionService := NewSessionService()
	sessionService.Init(context.Background(), db, &appconf.AppConfig{})

	startTime := time.Now().Add(-time.Minute).Truncate(time.Second)
	sessionID, err := sessionService.CreatePendingSession("game-1", startTime)
	if err != nil {
		t.Fatalf("create pending session: %v", err)
	}

	var endTimeIsNull bool
	var duration int
	if err := db.QueryRow(
		`SELECT end_time IS NULL, duration FROM play_sessions WHERE id = ?`,
		sessionID,
	).Scan(&endTimeIsNull, &duration); err != nil {
		t.Fatalf("query pending session: %v", err)
	}

	if !endTimeIsNull {
		t.Fatal("expected pending session end_time to be NULL")
	}
	if duration != 0 {
		t.Fatalf("expected pending duration 0, got %d", duration)
	}
}

func TestSaveSessionHeartbeatUpdatesOnlyRunningSession(t *testing.T) {
	db := setupSessionServiceTestDB(t)
	sessionService := NewSessionService()
	sessionService.Init(context.Background(), db, &appconf.AppConfig{})

	startTime := time.Now().Add(-2 * time.Minute).Truncate(time.Second)
	sessionID, err := sessionService.CreatePendingSession("game-1", startTime)
	if err != nil {
		t.Fatalf("create pending session: %v", err)
	}

	heartbeatAt := startTime.Add(75 * time.Second)
	if err := sessionService.saveSessionHeartbeat(sessionID, 75, heartbeatAt); err != nil {
		t.Fatalf("save session heartbeat: %v", err)
	}

	var endTimeIsNull bool
	var duration int
	var updatedAt time.Time
	if err := db.QueryRow(
		`SELECT end_time IS NULL, duration, updated_at FROM play_sessions WHERE id = ?`,
		sessionID,
	).Scan(&endTimeIsNull, &duration, &updatedAt); err != nil {
		t.Fatalf("query heartbeat session: %v", err)
	}
	if !endTimeIsNull {
		t.Fatal("expected heartbeat to keep end_time NULL")
	}
	if duration != 75 {
		t.Fatalf("expected heartbeat duration 75, got %d", duration)
	}
	if updatedAt.Sub(heartbeatAt).Abs() > time.Second {
		t.Fatalf("expected heartbeat time %s, got %s", heartbeatAt, updatedAt)
	}

	endTime := startTime.Add(90 * time.Second)
	if err := sessionService.completeUnfinishedSessionWithDuration(sessionID, endTime, 90); err != nil {
		t.Fatalf("complete running session: %v", err)
	}
	if err := sessionService.saveSessionHeartbeat(sessionID, 120, startTime.Add(120*time.Second)); err != nil {
		t.Fatalf("save late heartbeat: %v", err)
	}

	if err := db.QueryRow(
		`SELECT end_time IS NULL, duration, updated_at FROM play_sessions WHERE id = ?`,
		sessionID,
	).Scan(&endTimeIsNull, &duration, &updatedAt); err != nil {
		t.Fatalf("query completed session after late heartbeat: %v", err)
	}
	if endTimeIsNull {
		t.Fatal("expected completed session end_time to remain set")
	}
	if duration != 90 {
		t.Fatalf("expected completed duration 90, got %d", duration)
	}
	if updatedAt.Sub(endTime).Abs() > time.Second {
		t.Fatalf("expected completed updated_at %s, got %s", endTime, updatedAt)
	}
}

func TestCleanupUnfinishedSessionsUsesLastHeartbeat(t *testing.T) {
	db := setupSessionServiceTestDB(t)
	sessionService := NewSessionService()
	sessionService.Init(context.Background(), db, &appconf.AppConfig{})

	startTime := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	heartbeatAt := startTime.Add(125 * time.Second)
	if _, err := db.Exec(
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		 VALUES (?, ?, ?, NULL, ?, ?)`,
		"session-heartbeat-recovery",
		"game-1",
		startTime,
		125,
		heartbeatAt,
	); err != nil {
		t.Fatalf("insert running session: %v", err)
	}

	if err := sessionService.CleanupUnfinishedSessions(); err != nil {
		t.Fatalf("cleanup unfinished sessions: %v", err)
	}

	var endTime time.Time
	var duration int
	if err := db.QueryRow(
		`SELECT end_time, duration FROM play_sessions WHERE id = ?`,
		"session-heartbeat-recovery",
	).Scan(&endTime, &duration); err != nil {
		t.Fatalf("query recovered session: %v", err)
	}
	if duration != 125 {
		t.Fatalf("expected recovered duration 125, got %d", duration)
	}
	if endTime.Sub(heartbeatAt).Abs() > time.Second {
		t.Fatalf("expected recovered end_time %s, got %s", heartbeatAt, endTime)
	}
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
