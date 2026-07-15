package metadata

import (
	"io"
	"lunabox/internal/common/enums"
	"lunabox/internal/version"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type hikarinagiRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn hikarinagiRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestHikarinagiGetterUsesClientCredentialsAndCachesToken(t *testing.T) {
	previousClientID := version.HikarinagiOAuthClientID
	previousClientSecret := version.HikarinagiOAuthClientSecret
	previousLimiter := sharedMetadataRateLimiter
	t.Cleanup(func() {
		version.HikarinagiOAuthClientID = previousClientID
		version.HikarinagiOAuthClientSecret = previousClientSecret
		sharedMetadataRateLimiter = previousLimiter
		resetHikarinagiTokenCacheForTest()
	})
	version.HikarinagiOAuthClientID = "client-id"
	version.HikarinagiOAuthClientSecret = "client-secret"
	sharedMetadataRateLimiter = newMetadataRateLimiter(map[MetadataSource]MetadataRateLimitPolicy{})
	resetHikarinagiTokenCacheForTest()

	var tokenRequests int32
	client := &http.Client{Transport: hikarinagiRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		response := func(body string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		}

		switch {
		case req.URL.String() == hikarinagiTokenURL:
			atomic.AddInt32(&tokenRequests, 1)
			if req.Method != http.MethodPost {
				t.Fatalf("token method = %s", req.Method)
			}
			clientID, clientSecret, ok := req.BasicAuth()
			if !ok || clientID != "client-id" || clientSecret != "client-secret" {
				t.Fatalf("unexpected token basic auth: %q %q %v", clientID, clientSecret, ok)
			}
			if err := req.ParseForm(); err != nil {
				t.Fatalf("parse token form: %v", err)
			}
			if req.Form.Get("grant_type") != "client_credentials" || req.Form.Get("scope") != hikarinagiScope {
				t.Fatalf("unexpected token form: %#v", req.Form)
			}
			return response(`{"access_token":"test-access-token","token_type":"Bearer","expires_in":3600,"scope":"catalog:read"}`)
		case strings.Contains(req.URL.Path, "/open/search"):
			assertHikarinagiBearerToken(t, req)
			if req.URL.Query().Get("q") != "CLANNAD" || req.URL.Query().Get("types") != "galgame" {
				t.Fatalf("unexpected search query: %s", req.URL.RawQuery)
			}
			return response(`{"success":true,"data":{"items":[{"type":"galgame","id":371,"title":"CLANNAD","subtitle":null,"developer":"Key","cover":null}],"meta":{"page":1,"page_size":1,"total_items":1,"item_count":1,"total_pages":1}},"request_id":"req-search"}`)
		case strings.HasSuffix(req.URL.Path, "/open/galgames/371"):
			assertHikarinagiBearerToken(t, req)
			return response(`{"success":true,"data":{"id":371,"origin_title":"CLANNAD","trans_title":"克兰娜德","covers":[{"url":"https://example.com/low.jpg","width":600,"height":800,"sexual":0,"violence":0,"votes":1},{"url":"https://example.com/best.jpg","width":600,"height":800,"sexual":0,"violence":0,"votes":9}],"release_date":"2004-04-28T00:00:00.000Z","origin_intro":"origin","trans_intro":"translated","nsfw":false,"tags":[{"name":"泣きゲー","likes":20},{"name":"学园","likes":10}]},"request_id":"req-detail"}`)
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	getter := NewHikarinagiInfoGetter(WithHTTPClient(client), WithTagLimit(2))
	result, err := getter.FetchMetadataByName("CLANNAD", "")
	if err != nil {
		t.Fatalf("FetchMetadataByName returned error: %v", err)
	}
	if result.Game.Name != "克兰娜德" || result.Game.Company != "Key" {
		t.Fatalf("unexpected game identity: %#v", result.Game)
	}
	if result.Game.CoverURL != "https://example.com/best.jpg" || result.Game.ReleaseDate != "2004-04-28" {
		t.Fatalf("unexpected cover/date: %#v", result.Game)
	}
	if result.Game.SourceType != enums.Hikarinagi || result.Game.SourceID != "371" {
		t.Fatalf("unexpected source: %#v", result.Game)
	}
	if len(result.Tags) != 2 || result.Tags[0].Name != "泣きゲー" || result.Tags[1].Weight != 0.5 {
		t.Fatalf("unexpected tags: %#v", result.Tags)
	}
	if atomic.LoadInt32(&tokenRequests) != 1 {
		t.Fatalf("token requests = %d, want 1", tokenRequests)
	}
}

func TestHikarinagiGetterRequiresInjectedCredentials(t *testing.T) {
	previousClientID := version.HikarinagiOAuthClientID
	previousClientSecret := version.HikarinagiOAuthClientSecret
	t.Cleanup(func() {
		version.HikarinagiOAuthClientID = previousClientID
		version.HikarinagiOAuthClientSecret = previousClientSecret
		resetHikarinagiTokenCacheForTest()
	})
	version.HikarinagiOAuthClientID = ""
	version.HikarinagiOAuthClientSecret = ""
	resetHikarinagiTokenCacheForTest()

	_, err := NewHikarinagiInfoGetter().FetchMetadata("1", "")
	if err == nil || !strings.Contains(err.Error(), "requires injected OAuth client credentials") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertHikarinagiBearerToken(t *testing.T, req *http.Request) {
	t.Helper()
	if got := req.Header.Get("Authorization"); got != "Bearer test-access-token" {
		t.Fatalf("Authorization = %q", got)
	}
}

func resetHikarinagiTokenCacheForTest() {
	hikarinagiTokenCache.mu.Lock()
	defer hikarinagiTokenCache.mu.Unlock()
	hikarinagiTokenCache.clientID = ""
	hikarinagiTokenCache.clientSecret = ""
	hikarinagiTokenCache.token = ""
	hikarinagiTokenCache.expiresAt = time.Time{}
}
