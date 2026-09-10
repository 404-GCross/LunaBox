package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/common/vo"
	"lunabox/internal/service/cloudprovider"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
)

type retentionTestProvider struct {
	keys    []string
	deleted []string
}

func (p *retentionTestProvider) UploadFile(context.Context, string, string) error { return nil }
func (p *retentionTestProvider) DownloadFile(context.Context, string, string) error {
	return nil
}
func (p *retentionTestProvider) ListObjects(_ context.Context, prefix string) ([]string, error) {
	items := make([]string, 0, len(p.keys))
	for _, key := range p.keys {
		if strings.HasPrefix(key, prefix) {
			items = append(items, key)
		}
	}
	return items, nil
}
func (p *retentionTestProvider) DeleteObject(_ context.Context, key string) error {
	p.deleted = append(p.deleted, key)
	return nil
}
func (p *retentionTestProvider) TestConnection(context.Context) error { return nil }
func (p *retentionTestProvider) EnsureDir(context.Context, string) error {
	return nil
}
func (p *retentionTestProvider) GetCloudPath(userID, subPath string) string {
	return filepath.ToSlash(filepath.Join("v1", userID, subPath))
}

var _ cloudprovider.CloudStorageProvider = (*retentionTestProvider)(nil)

func TestNextScheduledDBBackup(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 9, 8, 10, 30, 0, 0, location)

	t.Run("interval waits from last backup", func(t *testing.T) {
		config := &appconf.AppConfig{
			ScheduledDBBackupMode:            appconf.ScheduledDBBackupModeInterval,
			ScheduledDBBackupIntervalMinutes: 60,
		}
		got := nextScheduledDBBackup(now, config, now.Add(-20*time.Minute))
		want := now.Add(40 * time.Minute)
		if !got.Equal(want) {
			t.Fatalf("next backup = %v, want %v", got, want)
		}
	})

	t.Run("missed daily time runs now", func(t *testing.T) {
		config := &appconf.AppConfig{
			ScheduledDBBackupMode: appconf.ScheduledDBBackupModeDaily,
			ScheduledDBBackupTime: "09:00",
		}
		got := nextScheduledDBBackup(now, config, now.AddDate(0, 0, -1))
		if !got.Equal(now) {
			t.Fatalf("next backup = %v, want %v", got, now)
		}
	})

	t.Run("completed daily backup waits until tomorrow", func(t *testing.T) {
		config := &appconf.AppConfig{
			ScheduledDBBackupMode: appconf.ScheduledDBBackupModeDaily,
			ScheduledDBBackupTime: "09:00",
		}
		got := nextScheduledDBBackup(now, config, now.Add(-time.Hour))
		want := time.Date(2026, 9, 9, 9, 0, 0, 0, location)
		if !got.Equal(want) {
			t.Fatalf("next backup = %v, want %v", got, want)
		}
	})
}

func TestShouldRestoreCloudGameBackup(t *testing.T) {
	localTime := time.Date(2026, 9, 8, 10, 0, 0, 0, time.Local)
	latest := vo.CloudBackupItem{
		Key:       "v1/user/saves/game/2026-09-08T11-00-00.zip",
		CreatedAt: localTime.Add(time.Hour),
	}

	if !shouldRestoreCloudGameBackup(latest, localTime, "") {
		t.Fatal("expected a newer cloud backup to be restored")
	}
	if shouldRestoreCloudGameBackup(latest, latest.CreatedAt.Add(time.Minute), "") {
		t.Fatal("expected a newer local save to be kept")
	}
	if shouldRestoreCloudGameBackup(latest, time.Time{}, latest.Key) {
		t.Fatal("expected the last synchronized cloud backup to be skipped")
	}
}

func TestCleanupOldCloudDBBackupsUsesDatabaseRetention(t *testing.T) {
	provider := &retentionTestProvider{}
	for day := 1; day <= 4; day++ {
		provider.keys = append(provider.keys, fmt.Sprintf("v1/user/database/lunabox_2026-09-0%dT03-00-00.zip", day))
	}
	provider.keys = append(provider.keys, "v1/user/database/latest.zip")

	backupService := NewBackupService()
	backupService.Init(context.Background(), nil, &appconf.AppConfig{
		BackupUserID:           "user",
		CloudDBBackupRetention: 2,
		CloudBackupRetention:   4,
	})
	backupService.SetCloudProviderFactoryForTest(func() (cloudprovider.CloudStorageProvider, error) {
		return provider, nil
	})

	backupService.cleanupOldCloudDBBackups()
	if len(provider.deleted) != 2 {
		t.Fatalf("deleted %d backups, want 2", len(provider.deleted))
	}
	if !strings.Contains(provider.deleted[0], "2026-09-02") || !strings.Contains(provider.deleted[1], "2026-09-01") {
		t.Fatalf("unexpected deleted backups: %v", provider.deleted)
	}
}

func TestCreateDBBackupForShutdownUsesIndependentContext(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`CREATE TABLE backup_test (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create test table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO backup_test VALUES (1, 'LunaBox')`); err != nil {
		t.Fatalf("insert test row: %v", err)
	}

	appCtx, cancel := context.WithCancel(context.Background())
	cancel()

	backupService := NewBackupService()
	backupService.Init(appCtx, db, &appconf.AppConfig{LocalDBBackupRetention: 5})

	if _, err := backupService.CreateDBBackup(); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateDBBackup() error = %v, want context.Canceled", err)
	}

	backup, err := backupService.CreateDBBackupForShutdown()
	if err != nil {
		t.Fatalf("CreateDBBackupForShutdown() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(backup.Path) })

	info, err := os.Stat(backup.Path)
	if err != nil {
		t.Fatalf("stat shutdown backup: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("shutdown backup is empty")
	}
}
