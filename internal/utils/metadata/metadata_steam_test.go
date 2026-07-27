package metadata

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestBuildSteamPortraitCoverURLsPrioritizesLanguage(t *testing.T) {
	got := buildSteamPortraitCoverURLs(12345, "schinese")
	want := []string{
		"https://cdn.akamai.steamstatic.com/steam/apps/12345/library_600x900_schinese.jpg",
		"https://cdn.akamai.steamstatic.com/steam/apps/12345/library_600x900.jpg",
		"https://cdn.akamai.steamstatic.com/steam/apps/12345/library_600x900_english.jpg",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected Steam portrait cover candidates: got %v, want %v", got, want)
	}
}

func TestResolveSteamCoverURLUsesFirstAvailablePortrait(t *testing.T) {
	requested := make([]string, 0, 2)
	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.String())
		status := http.StatusNotFound
		if strings.HasSuffix(req.URL.Path, "/library_600x900.jpg") {
			status = http.StatusOK
		}
		return steamCoverTestResponse(req, status, "image/jpeg"), nil
	})}

	getter := NewSteamInfoGetterWithLanguage("zh-CN", WithHTTPClient(client))
	got := getter.resolveSteamCoverURL(12345, "schinese", "https://example.com/header.jpg")
	want := "https://cdn.akamai.steamstatic.com/steam/apps/12345/library_600x900.jpg"
	if got != want {
		t.Fatalf("unexpected resolved Steam cover: got %q, want %q", got, want)
	}
	if len(requested) != 2 {
		t.Fatalf("expected localized and generic portrait probes, got %v", requested)
	}
}

func TestResolveSteamCoverURLFallsBackToHeaderImage(t *testing.T) {
	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return steamCoverTestResponse(req, http.StatusNotFound, "text/html"), nil
	})}

	getter := NewSteamInfoGetterWithLanguage("zh-CN", WithHTTPClient(client))
	const headerImage = "https://example.com/header.jpg"
	if got := getter.resolveSteamCoverURL(12345, "schinese", headerImage); got != headerImage {
		t.Fatalf("expected Steam header fallback %q, got %q", headerImage, got)
	}
}

func TestFetchSteamMetadataStoresPortraitAsCoverSource(t *testing.T) {
	originalLimiter := sharedMetadataRateLimiter
	defer func() {
		sharedMetadataRateLimiter = originalLimiter
	}()
	sharedMetadataRateLimiter = newMetadataRateLimiter(nil)

	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodHead {
			status := http.StatusNotFound
			if strings.HasSuffix(req.URL.Path, "/library_600x900_schinese.jpg") {
				status = http.StatusOK
			}
			return steamCoverTestResponse(req, status, "image/jpeg"), nil
		}

		body := `{
			"12345": {
				"success": true,
				"data": {
					"steam_appid": 12345,
					"name": "Sample Game",
					"header_image": "https://example.com/header.jpg",
					"short_description": "Sample",
					"metacritic": {"score": 80}
				}
			}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}

	getter := NewSteamInfoGetterWithLanguage("zh-CN", WithHTTPClient(client))
	result, err := getter.FetchMetadata("12345", "")
	if err != nil {
		t.Fatalf("failed to fetch Steam metadata: %v", err)
	}

	want := "https://cdn.akamai.steamstatic.com/steam/apps/12345/library_600x900_schinese.jpg"
	if result.Game.CoverURL != want || result.Game.CoverSourceURL != want {
		t.Fatalf("expected localized portrait cover %q, got cover=%q source=%q", want, result.Game.CoverURL, result.Game.CoverSourceURL)
	}
}

func steamCoverTestResponse(req *http.Request, status int, contentType string) *http.Response {
	header := make(http.Header)
	header.Set("Content-Type", contentType)
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}
}
