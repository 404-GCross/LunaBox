package test

import (
	"context"
	"testing"
	"time"

	"lunabox/internal/service/cloudsync"
)

func TestCloudSyncStateRoundTrip(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// 空表
	got, err := cloudsync.LoadSyncState(ctx, db)
	if err != nil {
		t.Fatalf("load empty state: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty state, got %d rows", len(got))
	}

	now := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	rows := []cloudsync.SyncStateRow{
		{
			BucketKey:        "games/3",
			LocalHash:        "aaa",
			RemoteHash:       "bbb",
			RemoteRevisionID: "rev-1",
			UpdatedAt:        now,
		},
		{
			BucketKey:        cloudsync.SingletonStateKey(cloudsync.SingletonCategories),
			LocalHash:        "ccc",
			RemoteHash:       "ddd",
			RemoteRevisionID: "rev-1",
			UpdatedAt:        now,
		},
	}
	if err := cloudsync.SaveSyncState(ctx, db, rows); err != nil {
		t.Fatalf("save state: %v", err)
	}

	got, err = cloudsync.LoadSyncState(ctx, db)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	if got["games/3"].LocalHash != "aaa" || got["games/3"].RemoteHash != "bbb" {
		t.Errorf("games/3 row mismatch: %+v", got["games/3"])
	}

	// 再次 save 同 key 不同 hash → upsert 覆盖
	rows[0].LocalHash = "aaa-v2"
	if err := cloudsync.SaveSyncState(ctx, db, rows); err != nil {
		t.Fatalf("second save: %v", err)
	}
	got, _ = cloudsync.LoadSyncState(ctx, db)
	if got["games/3"].LocalHash != "aaa-v2" {
		t.Errorf("upsert did not overwrite: %+v", got["games/3"])
	}
}
