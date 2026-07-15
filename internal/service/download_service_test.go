package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
)

func openDownloadServiceTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if _, err := db.Exec(`CREATE TABLE download_tasks (
		id TEXT PRIMARY KEY,
		request_json TEXT,
		status TEXT,
		progress DOUBLE,
		downloaded BIGINT,
		total BIGINT,
		error TEXT,
		file_path TEXT,
		created_at TIMESTAMPTZ,
		updated_at TIMESTAMPTZ
	)`); err != nil {
		t.Fatalf("create download_tasks table: %v", err)
	}

	return db
}

func TestDownloadServiceLoadTasksPreservesFinishedTaskTimestamps(t *testing.T) {
	db := openDownloadServiceTestDB(t)
	historicalUpdatedAt := time.Date(2025, time.December, 8, 9, 30, 0, 0, time.UTC)

	if _, err := db.Exec(`
		INSERT INTO download_tasks (
			id, request_json, status, progress, downloaded, total, error, file_path, created_at, updated_at
		) VALUES (?, '{}', 'done', 100, 10, 10, '', '', NULL, ?)
	`, "legacy-task", historicalUpdatedAt); err != nil {
		t.Fatalf("insert legacy task: %v", err)
	}

	downloadService := NewDownloadService()
	downloadService.Init(context.Background(), db, &appconf.AppConfig{})

	var createdAt sql.NullTime
	var updatedAt time.Time
	if err := db.QueryRow(`
		SELECT created_at, updated_at
		FROM download_tasks
		WHERE id = 'legacy-task'
	`).Scan(&createdAt, &updatedAt); err != nil {
		t.Fatalf("query legacy task timestamps: %v", err)
	}

	if createdAt.Valid {
		t.Fatalf("legacy task creation time should remain unknown, got %s", createdAt.Time)
	}
	if !updatedAt.Equal(historicalUpdatedAt) {
		t.Fatalf("loading changed updated_at: got %s, want %s", updatedAt, historicalUpdatedAt)
	}

	tasks := downloadService.GetDownloadTasks()
	if len(tasks) != 1 || tasks[0].CreatedAt != nil {
		t.Fatalf("legacy task should not receive a fabricated creation time: %#v", tasks)
	}
}

func TestDownloadServiceUpsertTaskStoresInitialTimestamps(t *testing.T) {
	db := openDownloadServiceTestDB(t)
	createdAt := time.Date(2026, time.July, 14, 16, 20, 0, 0, time.UTC)
	downloadService := NewDownloadService()
	downloadService.db = db

	task := &DownloadTask{
		ID:        "new-task",
		Status:    DownloadStatusPending,
		CreatedAt: &createdAt,
	}
	if err := downloadService.upsertTask(task); err != nil {
		t.Fatalf("persist new task: %v", err)
	}

	var storedCreatedAt time.Time
	var storedUpdatedAt time.Time
	if err := db.QueryRow(`
		SELECT created_at, updated_at
		FROM download_tasks
		WHERE id = 'new-task'
	`).Scan(&storedCreatedAt, &storedUpdatedAt); err != nil {
		t.Fatalf("query new task timestamps: %v", err)
	}

	if !storedCreatedAt.Equal(createdAt) {
		t.Fatalf("created_at mismatch: got %s, want %s", storedCreatedAt, createdAt)
	}
	if !storedUpdatedAt.Equal(createdAt) {
		t.Fatalf("updated_at mismatch: got %s, want %s", storedUpdatedAt, createdAt)
	}
}
