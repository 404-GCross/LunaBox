package umbra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
	"lunabox/internal/service/cloudprovider/batchupload"
)

func TestUploadFilesUsesUmbraBatchEndpoints(t *testing.T) {
	var mu sync.Mutex
	presignBatchSizes := make([]int, 0)
	confirmBatchSizes := make([]int, 0)
	putsAtConfirm := make([]int, 0)
	putCount := 0
	nextBackupID := uint64(1)

	objectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("object method = %s", r.Method)
		}
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		putCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer objectServer.Close()

	type presignRequest struct {
		Items []json.RawMessage `json:"items"`
	}
	type confirmTarget struct {
		BackupID uint64 `json:"backup_id"`
	}
	type confirmRequest struct {
		Items []confirmTarget `json:"items"`
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/client/backup/presign-batch", func(w http.ResponseWriter, r *http.Request) {
		var request presignRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		presignBatchSizes = append(presignBatchSizes, len(request.Items))
		results := make([]umbrsdk.BatchPresignResultItem, len(request.Items))
		for i := range request.Items {
			backupID := nextBackupID
			nextBackupID++
			results[i] = umbrsdk.BatchPresignResultItem{
				BackupID:     backupID,
				PresignedURL: fmt.Sprintf("%s/object/%d", objectServer.URL, backupID),
				ExpiresIn:    3600,
			}
		}
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]any{"items": results, "total": len(results)},
		})
	})
	mux.HandleFunc("/api/v1/client/backup/confirm-batch", func(w http.ResponseWriter, r *http.Request) {
		var request confirmRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		confirmBatchSizes = append(confirmBatchSizes, len(request.Items))
		putsAtConfirm = append(putsAtConfirm, putCount)
		mu.Unlock()
		results := make([]umbrsdk.BatchConfirmResultItem, len(request.Items))
		for i, item := range request.Items {
			results[i] = umbrsdk.BatchConfirmResultItem{BackupID: item.BackupID, SizeBytes: 10}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": umbrsdk.BatchConfirmResult{Items: results, Total: len(results)},
		})
	})
	apiServer := httptest.NewServer(mux)
	defer apiServer.Close()

	tokenStore := umbrsdk.NewMemoryTokenStore()
	if err := tokenStore.Save(context.Background(), &umbrsdk.TokenSet{
		AccessToken: "token",
		TokenType:   "bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	deviceStore := umbrsdk.NewMemoryDeviceStore()
	if err := deviceStore.Save(context.Background(), &umbrsdk.DeviceCredentials{DeviceID: "dev_test", DeviceSecret: "secret"}); err != nil {
		t.Fatal(err)
	}
	client, err := umbrsdk.New(umbrsdk.Config{
		BaseURL:     apiServer.URL,
		ClientID:    "client",
		RedirectURI: "http://127.0.0.1:1420/auth/callback",
		TokenStore:  tokenStore,
		DeviceStore: deviceStore,
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := &Provider{client: client, userID: "user"}

	items := make([]batchupload.Item, 51)
	for i := range items {
		localPath := filepath.Join(t.TempDir(), fmt.Sprintf("item-%02d.json", i))
		if err := os.WriteFile(localPath, []byte(fmt.Sprintf("payload-%02d", i)), 0o600); err != nil {
			t.Fatal(err)
		}
		items[i] = batchupload.Item{
			CloudPath: fmt.Sprintf("v1/user/sync/library/games/item-%02d.json", i),
			LocalPath: localPath,
		}
	}

	if err := provider.UploadFiles(context.Background(), items); err != nil {
		t.Fatalf("UploadFiles() error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(presignBatchSizes) != "[50 1]" {
		t.Fatalf("presign batch sizes = %v, want [50 1]", presignBatchSizes)
	}
	if fmt.Sprint(confirmBatchSizes) != "[50 1]" {
		t.Fatalf("confirm batch sizes = %v, want [50 1]", confirmBatchSizes)
	}
	if fmt.Sprint(putsAtConfirm) != "[50 51]" {
		t.Fatalf("PUT counts at confirm = %v, want [50 51]", putsAtConfirm)
	}
	if putCount != len(items) {
		t.Fatalf("PUT count = %d, want %d", putCount, len(items))
	}
}
