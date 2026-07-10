package cloudprovider

import (
	"context"

	"lunabox/internal/service/cloudprovider/batchupload"
)

type CloudStorageProvider interface {
	UploadFile(ctx context.Context, cloudPath, localPath string) error
	DownloadFile(ctx context.Context, cloudPath, localPath string) error
	ListObjects(ctx context.Context, prefix string) ([]string, error)
	DeleteObject(ctx context.Context, key string) error
	TestConnection(ctx context.Context) error
	EnsureDir(ctx context.Context, path string) error
	GetCloudPath(userID, subPath string) string
}

// BatchUploadProvider is an optional capability implemented by providers that
// can reduce control-plane requests when several files are uploaded together.
type BatchUploadProvider interface {
	UploadFiles(ctx context.Context, items []batchupload.Item) error
}
