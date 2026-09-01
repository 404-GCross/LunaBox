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
	type progressUpdate struct {
		stage   string
		current int
		total   int
	}
	progressUpdates := make([]progressUpdate, 0, 2)
	helper.SetProgressReporter(func(stage, _ string, current, total int) {
		progressUpdates = append(progressUpdates, progressUpdate{stage, current, total})
	})
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
	if len(progressUpdates) != 2 ||
		progressUpdates[0] != (progressUpdate{"uploading_files", 0, 3}) ||
		progressUpdates[1] != (progressUpdate{"uploading_files", 3, 3}) {
		t.Fatalf("progress updates = %+v", progressUpdates)
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

	merged := Snapshot{}
	if _, err := helper.ReconcileCoverAssets(provider, LocalState{}, remote, true, &merged); err != nil {
		t.Fatalf("ReconcileCoverAssets() error = %v", err)
	}
	if len(provider.deletedObjects) != 0 {
		t.Fatalf("expected remote covers to be preserved, deleted %v", provider.deletedObjects)
	}
}

func TestReconcileCoverAssetsSkipsUnchangedContentAcrossTimePrecision(t *testing.T) {
	now := time.Date(2026, 8, 31, 14, 0, 0, 0, time.UTC)
	localUpdatedAt := now.Add(750 * time.Millisecond)
	contentHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	game := Game{ID: "game-1", UpdatedAt: localUpdatedAt}
	localCover := LocalCover{
		Asset:     CoverAsset{GameID: game.ID, Ext: ".webp", UpdatedAt: localUpdatedAt, Hash: contentHash},
		LocalPath: "unused.webp",
		LocalURL:  "/local/covers/game-1.webp",
	}
	local := LocalState{Covers: map[string]LocalCover{game.ID: localCover}}
	remote := Snapshot{Covers: []CoverAsset{{GameID: game.ID, Ext: ".webp", UpdatedAt: now, Hash: contentHash}}}
	merged := Snapshot{Games: []Game{game}, Covers: []CoverAsset{localCover.Asset}}
	provider := &recordingBatchProvider{}
	helper := NewHelper(context.Background(), nil, &appconf.AppConfig{BackupUserID: "user"})

	coverURLs, err := helper.ReconcileCoverAssets(provider, local, remote, true, &merged)
	if err != nil {
		t.Fatalf("ReconcileCoverAssets() error = %v", err)
	}
	if provider.batchCalls != 0 || provider.singleUploads != 0 {
		t.Fatalf("unchanged cover was uploaded: batch=%d single=%d", provider.batchCalls, provider.singleUploads)
	}
	if coverURLs[game.ID] != localCover.LocalURL {
		t.Fatalf("cover URL = %q, want %q", coverURLs[game.ID], localCover.LocalURL)
	}
}

func TestReconcileCoverAssetsUploadsChangedContent(t *testing.T) {
	now := time.Date(2026, 8, 31, 14, 0, 0, 0, time.UTC)
	localPath := filepath.Join(t.TempDir(), "game-1.webp")
	if err := os.WriteFile(localPath, []byte("new cover"), 0o644); err != nil {
		t.Fatalf("write local cover: %v", err)
	}
	localHash, err := coverFileSHA256(localPath)
	if err != nil {
		t.Fatalf("hash local cover: %v", err)
	}
	game := Game{ID: "game-1", UpdatedAt: now.Add(time.Millisecond)}
	localCover := LocalCover{
		Asset:     CoverAsset{GameID: game.ID, Ext: ".webp", UpdatedAt: game.UpdatedAt, Hash: localHash},
		LocalPath: localPath,
		LocalURL:  "/local/covers/game-1.webp",
	}
	local := LocalState{Covers: map[string]LocalCover{game.ID: localCover}}
	remote := Snapshot{Covers: []CoverAsset{{
		GameID: game.ID, Ext: ".webp", UpdatedAt: now,
		Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}}}
	merged := Snapshot{Games: []Game{game}, Covers: []CoverAsset{localCover.Asset}}
	provider := &recordingBatchProvider{}
	helper := NewHelper(context.Background(), nil, &appconf.AppConfig{BackupUserID: "user"})

	if _, err := helper.ReconcileCoverAssets(provider, local, remote, true, &merged); err != nil {
		t.Fatalf("ReconcileCoverAssets() error = %v", err)
	}
	if provider.batchCalls != 1 || len(provider.batchItems) != 1 {
		t.Fatalf("changed cover upload calls = %d, items = %d", provider.batchCalls, len(provider.batchItems))
	}
}

func TestReconcileCoverAssetsUpgradesLegacyHashOnce(t *testing.T) {
	now := time.Date(2026, 8, 31, 14, 0, 0, 0, time.UTC)
	localPath := filepath.Join(t.TempDir(), "game-1.webp")
	if err := os.WriteFile(localPath, []byte("cover"), 0o644); err != nil {
		t.Fatalf("write local cover: %v", err)
	}
	localHash, err := coverFileSHA256(localPath)
	if err != nil {
		t.Fatalf("hash local cover: %v", err)
	}
	game := Game{ID: "game-1", UpdatedAt: now}
	localCover := LocalCover{
		Asset:     CoverAsset{GameID: game.ID, Ext: ".webp", UpdatedAt: now, Hash: localHash},
		LocalPath: localPath,
		LocalURL:  "/local/covers/game-1.webp",
	}
	local := LocalState{Covers: map[string]LocalCover{game.ID: localCover}}
	remote := Snapshot{Covers: []CoverAsset{{GameID: game.ID, Ext: ".webp", UpdatedAt: now, Hash: "legacy-metadata-hash-0000000000"}}}
	merged := Snapshot{Games: []Game{game}, Covers: []CoverAsset{localCover.Asset}}
	provider := &recordingBatchProvider{}
	helper := NewHelper(context.Background(), nil, &appconf.AppConfig{BackupUserID: "user"})

	if _, err := helper.ReconcileCoverAssets(provider, local, remote, true, &merged); err != nil {
		t.Fatalf("ReconcileCoverAssets() error = %v", err)
	}
	if provider.batchCalls != 1 || len(provider.batchItems) != 1 {
		t.Fatalf("legacy cover upgrade calls = %d, items = %d", provider.batchCalls, len(provider.batchItems))
	}
	if merged.Covers[0].Hash != localHash {
		t.Fatalf("merged cover hash = %q, want %q", merged.Covers[0].Hash, localHash)
	}
}

func TestVerifiedDownloadedCoverHashRejectsContentMismatch(t *testing.T) {
	coverPath := filepath.Join(t.TempDir(), "cover.webp")
	if err := os.WriteFile(coverPath, []byte("downloaded cover"), 0o644); err != nil {
		t.Fatalf("write downloaded cover: %v", err)
	}

	_, err := verifiedDownloadedCoverHash(
		coverPath,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err == nil {
		t.Fatal("expected SHA-256 mismatch error")
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
