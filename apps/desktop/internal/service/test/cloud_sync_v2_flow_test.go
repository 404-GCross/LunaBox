package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/service/cloudprovider"
	"lunabox/internal/service/cloudsync"
)

// mockProvider 是一个内存伪 provider，用于断言 SyncToCloud 的远端 IO 顺序与文件集合。
type mockProvider struct {
	mu        sync.Mutex
	store     map[string][]byte // cloudKey -> payload
	uploadLog []string          // 按顺序记录每次 Upload 的 cloudKey
}

func newMockProvider() *mockProvider {
	return &mockProvider{store: map[string][]byte{}}
}

func (p *mockProvider) UploadFile(ctx context.Context, cloudPath, localPath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.store[cloudPath] = data
	p.uploadLog = append(p.uploadLog, cloudPath)
	p.mu.Unlock()
	return nil
}

func (p *mockProvider) DownloadFile(ctx context.Context, cloudPath, localPath string) error {
	p.mu.Lock()
	data, ok := p.store[cloudPath]
	p.mu.Unlock()
	if !ok {
		return fmt.Errorf("not found: %s", cloudPath)
	}
	return os.WriteFile(localPath, data, 0644)
}

func (p *mockProvider) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := []string{}
	for key := range p.store {
		if strings.HasPrefix(key, prefix) {
			out = append(out, key)
		}
	}
	return out, nil
}

func (p *mockProvider) DeleteObject(ctx context.Context, key string) error {
	p.mu.Lock()
	delete(p.store, key)
	p.mu.Unlock()
	return nil
}

func (p *mockProvider) TestConnection(ctx context.Context) error         { return nil }
func (p *mockProvider) EnsureDir(ctx context.Context, path string) error { return nil }
func (p *mockProvider) GetCloudPath(userID, subPath string) string {
	return filepath.ToSlash(filepath.Join("v1", userID, subPath))
}

// 确认实现了接口
var _ cloudprovider.CloudStorageProvider = (*mockProvider)(nil)

func newSyncTestConfig() *appconf.AppConfig {
	return &appconf.AppConfig{
		CloudSyncEnabled:   true,
		CloudBackupEnabled: true,
		BackupUserID:       "test-user",
	}
}

func TestSyncToCloud_FirstBootstrapUploadsAllNonEmptyBuckets(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	cfg := newSyncTestConfig()
	now := time.Now().UTC()

	// 一条 game 落到桶 3、一条落到桶 9
	if _, err := db.Exec(`INSERT INTO games (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, "3aaa", "G1", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO games (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, "9bbb", "G2", now, now); err != nil {
		t.Fatal(err)
	}

	helper := cloudsync.NewHelper(ctx, db, cfg)
	provider := newMockProvider()

	if err := helper.SyncToCloud(provider); err != nil {
		t.Fatalf("SyncToCloud failed: %v", err)
	}

	// 上传顺序断言：manifest 必须最后写
	manifestKey := provider.GetCloudPath(cfg.BackupUserID, cloudsync.ManifestKey)
	if len(provider.uploadLog) == 0 {
		t.Fatal("expected at least one upload")
	}
	last := provider.uploadLog[len(provider.uploadLog)-1]
	if last != manifestKey {
		t.Errorf("manifest must be last upload, got %s as last (full log: %v)", last, provider.uploadLog)
	}

	// 应当包含 games/3 与 games/9 两个桶
	gamesBucket3 := provider.GetCloudPath(cfg.BackupUserID, "sync/library/games/3.json")
	gamesBucket9 := provider.GetCloudPath(cfg.BackupUserID, "sync/library/games/9.json")
	if _, ok := provider.store[gamesBucket3]; !ok {
		t.Errorf("expected games/3.json uploaded")
	}
	if _, ok := provider.store[gamesBucket9]; !ok {
		t.Errorf("expected games/9.json uploaded")
	}
	// 空桶不应当上传：games/0
	gamesBucket0 := provider.GetCloudPath(cfg.BackupUserID, "sync/library/games/0.json")
	if _, ok := provider.store[gamesBucket0]; ok {
		t.Errorf("did not expect empty bucket games/0.json to be uploaded")
	}
}

func TestSyncToCloud_NoChangeIsNoop(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	cfg := newSyncTestConfig()
	now := time.Now().UTC()

	if _, err := db.Exec(`INSERT INTO games (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, "3aaa", "G1", now, now); err != nil {
		t.Fatal(err)
	}

	helper := cloudsync.NewHelper(ctx, db, cfg)
	provider := newMockProvider()

	// 第一次：bootstrap
	if err := helper.SyncToCloud(provider); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	provider.mu.Lock()
	firstLog := append([]string{}, provider.uploadLog...)
	provider.uploadLog = nil
	provider.mu.Unlock()

	if len(firstLog) == 0 {
		t.Fatal("expected uploads on first sync")
	}

	// 第二次：本地远端都没动 → 应当不再上传任何桶或 singleton；manifest 可能也不被写
	if err := helper.SyncToCloud(provider); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	provider.mu.Lock()
	secondLog := provider.uploadLog
	provider.mu.Unlock()

	for _, key := range secondLog {
		if strings.Contains(key, "/games/") || strings.Contains(key, "/play_sessions/") ||
			strings.Contains(key, "categories.json") || strings.Contains(key, "tombstones.json") {
			t.Errorf("noop sync should not re-upload %s", key)
		}
	}
}

func TestSyncToCloud_LocalChangeOnlyUploadsAffectedBucket(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	cfg := newSyncTestConfig()
	now := time.Now().UTC()

	// 初始两条 games 在不同桶
	if _, err := db.Exec(`INSERT INTO games (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, "3aaa", "G1", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO games (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, "9bbb", "G2", now, now); err != nil {
		t.Fatal(err)
	}

	helper := cloudsync.NewHelper(ctx, db, cfg)
	provider := newMockProvider()

	if err := helper.SyncToCloud(provider); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	provider.mu.Lock()
	provider.uploadLog = nil
	provider.mu.Unlock()

	// 只改桶 3 的 game：name 变 + updated_at 推后
	later := now.Add(time.Hour)
	if _, err := db.Exec(`UPDATE games SET name = ?, updated_at = ? WHERE id = ?`, "G1 edited", later, "3aaa"); err != nil {
		t.Fatal(err)
	}

	if err := helper.SyncToCloud(provider); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	provider.mu.Lock()
	uploadLog := append([]string{}, provider.uploadLog...)
	provider.mu.Unlock()

	uploaded := map[string]bool{}
	for _, k := range uploadLog {
		uploaded[k] = true
	}

	bucket3 := provider.GetCloudPath(cfg.BackupUserID, "sync/library/games/3.json")
	bucket9 := provider.GetCloudPath(cfg.BackupUserID, "sync/library/games/9.json")
	if !uploaded[bucket3] {
		t.Errorf("expected games/3.json re-uploaded, log: %v", uploadLog)
	}
	if uploaded[bucket9] {
		t.Errorf("did not expect games/9.json to be re-uploaded, log: %v", uploadLog)
	}
}

func TestSyncToCloud_MigrationFromV1(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	cfg := newSyncTestConfig()
	now := time.Now().UTC()

	if _, err := db.Exec(`INSERT INTO games (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, "3aaa", "G1", now, now); err != nil {
		t.Fatal(err)
	}

	provider := newMockProvider()
	// 预置一个旧的 v1 latest.json（仅含远端独占的一条 game，触发 LWW 合并）
	v1Path := provider.GetCloudPath(cfg.BackupUserID, cloudsync.SnapshotKey)
	v1Payload := `{
		"schema_version": 1,
		"revision_id": "v1-rev",
		"games": [
			{"id": "5ccc", "name": "G-Remote", "updated_at": "2026-06-15T11:00:00Z", "created_at": "2026-06-15T11:00:00Z"}
		]
	}`
	provider.mu.Lock()
	provider.store[v1Path] = []byte(v1Payload)
	provider.mu.Unlock()

	helper := cloudsync.NewHelper(ctx, db, cfg)
	if err := helper.SyncToCloud(provider); err != nil {
		t.Fatalf("migration sync failed: %v", err)
	}

	// 迁移成功后 latest.json 应当被删除
	provider.mu.Lock()
	_, stillExists := provider.store[v1Path]
	provider.mu.Unlock()
	if stillExists {
		t.Errorf("v1 latest.json should be deleted after migration")
	}

	// 新 manifest 应当存在
	manifestPath := provider.GetCloudPath(cfg.BackupUserID, cloudsync.ManifestKey)
	provider.mu.Lock()
	_, hasManifest := provider.store[manifestPath]
	provider.mu.Unlock()
	if !hasManifest {
		t.Errorf("expected v2 manifest.json after migration")
	}

	// 远端独占的 game 5ccc 应当合并落库
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM games WHERE id = ?`, "5ccc").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected v1-remote game 5ccc to be merged into local DB, got count=%d", count)
	}
}
