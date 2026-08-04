package webdav

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lunabox/internal/utils/proxyutils"
	"lunabox/internal/version"
)

type testProxyConfig struct {
	mode string
	url  string
}

func (c testProxyConfig) NetworkProxyConfig() (string, string) {
	return c.mode, c.url
}

func TestDownloadFileUsesRestyRetryAndApplicationHeaders(t *testing.T) {
	attempts := 0
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attempts++
		if req.URL.Host != "webdav-test.invalid" {
			t.Fatalf("proxied URL = %q", req.URL.String())
		}
		if got := req.Header.Get("User-Agent"); got != version.UserAgent() {
			t.Fatalf("User-Agent = %q", got)
		}
		username, password, ok := req.BasicAuth()
		if !ok || username != "luna" || password != "secret" {
			t.Fatalf("BasicAuth = %q, %q, %v", username, password, ok)
		}
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("backup-data"))
	}))
	defer proxyServer.Close()

	provider, err := NewProvider(Config{
		URL:      "http://webdav-test.invalid/root",
		Username: "luna",
		Password: "secret",
		ProxyConfig: testProxyConfig{
			mode: proxyutils.ProxyModeManual,
			url:  proxyServer.URL,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}
	destination := filepath.Join(t.TempDir(), "backup.zip")
	if err := provider.DownloadFile(context.Background(), "backup.zip", destination); err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if got := string(data); got != "backup-data" {
		t.Fatalf("downloaded data = %q", got)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestUploadFileRetriesParentEnsureAfterForbidden(t *testing.T) {
	putAttempts := 0
	mkcolRequests := make(map[string]bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPut:
			putAttempts++
			if req.URL.Path != "/root/a/b/file.txt" {
				t.Fatalf("PUT path = %q", req.URL.Path)
			}
			if putAttempts == 1 {
				http.Error(w, "<!DOCTYPE html><html>forbidden</html>", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusCreated)
		case "PROPFIND":
			w.WriteHeader(http.StatusNotFound)
		case "MKCOL":
			mkcolRequests[req.URL.Path] = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer server.Close()

	provider, err := NewProvider(Config{URL: server.URL + "/root"})
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}
	source := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(source, []byte("backup-data"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := provider.UploadFile(context.Background(), "a/b/file.txt", source); err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}
	if putAttempts != 2 {
		t.Fatalf("PUT attempts = %d, want 2", putAttempts)
	}
	for _, path := range []string{"/root/a/", "/root/a/b/"} {
		if !mkcolRequests[path] {
			t.Fatalf("missing MKCOL %s; got %#v", path, mkcolRequests)
		}
	}
}

func TestResponseErrorRedactsForbiddenHTML(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Status:     "403 Forbidden",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("<!DOCTYPE html><html>secret</html>")),
	}
	resp.Header.Set("Content-Type", "text/html")

	err := responseError("上传", resp)
	if err == nil {
		t.Fatal("responseError returned nil")
	}
	if strings.Contains(err.Error(), "<!DOCTYPE") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("forbidden HTML was not redacted: %v", err)
	}
	if !strings.Contains(err.Error(), "写入权限") {
		t.Fatalf("expected permission hint, got: %v", err)
	}
}
