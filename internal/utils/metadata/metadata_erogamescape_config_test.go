package metadata

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErogameScapeGetterUsesConfiguredBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/game.php" {
			t.Errorf("expected configured base path, got %q", r.URL.Path)
		}
		if r.URL.Query().Get("game") != "123" {
			t.Errorf("expected game query 123, got %q", r.URL.Query().Get("game"))
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<div id="soft-title"><span class="bold">Configured Game</span></div><div id="main_image"><img src="%s/images/cover.jpg"></div>`, DefaultErogameScapeBaseURL)
	}))
	defer server.Close()

	getter := NewErogameScapeInfoGetter(
		WithHTTPClient(server.Client()),
		WithErogameScapeBaseURL(server.URL+"/custom/"),
	)
	result, err := getter.FetchMetadata("123", "")
	if err != nil {
		t.Fatalf("fetch metadata with configured base URL: %v", err)
	}
	if result.Game.Name != "Configured Game" {
		t.Fatalf("expected configured server response, got %q", result.Game.Name)
	}
	wantCoverURL := server.URL + "/custom/images/cover.jpg"
	if result.Game.CoverURL != wantCoverURL {
		t.Fatalf("expected configured cover URL %q, got %q", wantCoverURL, result.Game.CoverURL)
	}
}
