package cloudsync

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/service/cloudprovider/batchupload"

	_ "github.com/duckdb/duckdb-go/v2"
)

type recordingBatchProvider struct {
	batchCalls      int
	singleUploads   int
	batchItems      []batchupload.Item
	materializedRaw map[string][]byte
	deletedObjects  []string
}

func (p *recordingBatchProvider) UploadFiles(_ context.Context, items []batchupload.Item) error {
	p.batchCalls++
	p.batchItems = append([]batchupload.Item(nil), items...)
	p.materializedRaw = make(map[string][]byte, len(items))
	for _, item := range items {
		raw, err := os.ReadFile(item.LocalPath)
		if err != nil {
			return err
		}
		p.materializedRaw[item.CloudPath] = raw
	}
	return nil
}

func (p *recordingBatchProvider) UploadFile(context.Context, string, string) error {
	p.singleUploads++
	return nil
}
func (*recordingBatchProvider) DownloadFile(context.Context, string, string) error { return nil }
func (*recordingBatchProvider) ListObjects(context.Context, string) ([]string, error) {
	return nil, nil
}
func (p *recordingBatchProvider) DeleteObject(_ context.Context, key string) error {
	p.deletedObjects = append(p.deletedObjects, key)
	return nil
}
func (*recordingBatchProvider) TestConnection(context.Context) error    { return nil }
func (*recordingBatchProvider) EnsureDir(context.Context, string) error { return nil }
func (*recordingBatchProvider) GetCloudPath(userID, subPath string) string {
	return filepath.ToSlash(filepath.Join("v1", userID, subPath))
}

func TestSaveRemoteLibraryFilesCombinesBucketsAndSingletons(t *testing.T) {
	previousMode := applog.GetMode()
	applog.SetMode(applog.ModeCLI)
	defer applog.SetMode(previousMode)

	helper := NewHelper(context.Background(), nil, &appconf.AppConfig{BackupUserID: "user"})
	provider := &recordingBatchProvider{}
	buckets := map[string]map[string]*BucketContent{
		EntityKeyGames: {"0": {}},
	}

	err := helper.SaveRemoteLibraryFiles(
		provider,
		buckets,
		[]string{BucketKey(EntityKeyGames, "0")},
		nil,
		nil,
		[]string{SingletonCategories, SingletonTombstones},
	)
	if err != nil {
		t.Fatalf("SaveRemoteLibraryFiles() error = %v", err)
	}
	if provider.batchCalls != 1 || provider.singleUploads != 0 {
		t.Fatalf("batch calls = %d, single uploads = %d", provider.batchCalls, provider.singleUploads)
	}
	if len(provider.batchItems) != 3 || len(provider.materializedRaw) != 3 {
		t.Fatalf("batch item count = %d, materialized count = %d", len(provider.batchItems), len(provider.materializedRaw))
	}
	for _, item := range provider.batchItems {
		if len(provider.materializedRaw[item.CloudPath]) == 0 {
			t.Fatalf("empty materialized payload for %s", item.CloudPath)
		}
		if _, err := os.Stat(item.LocalPath); !os.IsNotExist(err) {
			t.Fatalf("temporary file %s still exists: %v", item.LocalPath, err)
		}
	}
	if _, ok := provider.materializedRaw[fmt.Sprintf("v1/user/%s", CategoriesFileKey)]; !ok {
		t.Fatalf("categories singleton was not included in batch: %v", provider.materializedRaw)
	}
}

func TestReconcileCoverAssetsDoesNotDeleteAllRemoteCoversFromEmptyMerge(t *testing.T) {
	previousMode := applog.GetMode()
	applog.SetMode(applog.ModeCLI)
	defer applog.SetMode(previousMode)

	helper := NewHelper(context.Background(), nil, &appconf.AppConfig{BackupUserID: "user"})
	provider := &recordingBatchProvider{}
	remote := Snapshot{Covers: []CoverAsset{
		{GameID: "game-1", Ext: ".webp"},
		{GameID: "game-2", Ext: ".jpg"},
	}}

	if _, err := helper.ReconcileCoverAssets(provider, LocalState{}, remote, true, Snapshot{}); err != nil {
		t.Fatalf("ReconcileCoverAssets() error = %v", err)
	}
	if len(provider.deletedObjects) != 0 {
		t.Fatalf("expected remote covers to be preserved, deleted %v", provider.deletedObjects)
	}
}

func TestListPlaySessionsSkipsRunningSessions(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE play_sessions (
		id TEXT PRIMARY KEY,
		game_id TEXT,
		start_time TIMESTAMPTZ,
		end_time TIMESTAMPTZ,
		duration INTEGER,
		updated_at TIMESTAMPTZ
	)`); err != nil {
		t.Fatalf("create play_sessions: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	if _, err := db.Exec(
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		 VALUES
			('running', 'game-1', ?, NULL, 75, ?),
			('completed', 'game-1', ?, ?, 120, ?)`,
		now.Add(-75*time.Second), now,
		now.Add(-120*time.Second), now, now,
	); err != nil {
		t.Fatalf("insert play sessions: %v", err)
	}

	helper := NewHelper(context.Background(), db, &appconf.AppConfig{})
	sessions, err := helper.listPlaySessions()
	if err != nil {
		t.Fatalf("list play sessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "completed" {
		t.Fatalf("expected only completed session in cloud snapshot, got %+v", sessions)
	}
}
