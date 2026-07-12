package umbra

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestUploadFilesUsesUmbraSyncExchange(t *testing.T) {
	var mu sync.Mutex
	exchangeBatchSizes := make([]int, 0)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/client/sync/snapshot", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": umbrsdk.SyncSnapshotPage{
				Records:        []umbrsdk.SyncChange{},
				ExchangeCursor: "cursor-0",
			},
		})
	})
	mux.HandleFunc("/api/v1/client/sync/exchange", func(w http.ResponseWriter, r *http.Request) {
		var request umbrsdk.SyncExchangeInput
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		exchangeBatchSizes = append(exchangeBatchSizes, len(request.Mutations))
		mu.Unlock()
		accepted := make([]umbrsdk.SyncAcceptedMutation, len(request.Mutations))
		for i, mutation := range request.Mutations {
			accepted[i] = umbrsdk.SyncAcceptedMutation{MutationID: mutation.MutationID, RecordVersion: 1}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": umbrsdk.SyncExchangeResult{
				Accepted:  accepted,
				Conflicts: []umbrsdk.SyncConflict{},
				Rejected:  []umbrsdk.SyncRejectedMutation{},
				Changes:   []umbrsdk.SyncChange{},
			},
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

	tempDir := t.TempDir()
	items := make([]batchupload.Item, 501)
	for i := range items {
		localPath := filepath.Join(tempDir, fmt.Sprintf("item-%03d.json", i))
		if err := os.WriteFile(localPath, []byte(fmt.Sprintf(`{"item":%d}`, i)), 0o600); err != nil {
			t.Fatal(err)
		}
		items[i] = batchupload.Item{
			CloudPath: fmt.Sprintf("v1/user/sync/library/games/item-%03d.json", i),
			LocalPath: localPath,
		}
	}

	if err := provider.UploadFiles(context.Background(), items); err != nil {
		t.Fatalf("UploadFiles() error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(exchangeBatchSizes) != "[500 1]" {
		t.Fatalf("exchange batch sizes = %v, want [500 1]", exchangeBatchSizes)
	}
}
