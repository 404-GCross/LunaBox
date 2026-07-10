package cloudsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/service/cloudprovider/batchupload"
)

type recordingBatchProvider struct {
	batchCalls      int
	singleUploads   int
	batchItems      []batchupload.Item
	materializedRaw map[string][]byte
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
func (*recordingBatchProvider) DeleteObject(context.Context, string) error { return nil }
func (*recordingBatchProvider) TestConnection(context.Context) error       { return nil }
func (*recordingBatchProvider) EnsureDir(context.Context, string) error    { return nil }
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
