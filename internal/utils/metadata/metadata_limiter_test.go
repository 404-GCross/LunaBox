package metadata

import (
	"bytes"
	"context"
	"errors"
	"io"
	"lunabox/internal/common/enums"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMetadataRateLimiterAppliesUpstreamWindow(t *testing.T) {
	const source MetadataSource = "test"
	start := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	now := start
	waits := []time.Duration{}

	limiter := newMetadataRateLimiter(map[MetadataSource]MetadataRateLimitPolicy{
		source: {
			Source:         source,
			UpstreamLimit:  2,
			UpstreamWindow: 10 * time.Second,
		},
	})
	limiter.now = func() time.Time {
		return now
	}
	limiter.wait = func(ctx context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		now = now.Add(delay)
		return nil
	}

	if err := limiter.Acquire(context.Background(), source); err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	if err := limiter.Acquire(context.Background(), source); err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}
	if len(waits) != 0 {
		t.Fatalf("expected no wait before the window is full, got %v", waits)
	}

	if err := limiter.Acquire(context.Background(), source); err != nil {
		t.Fatalf("third acquire failed: %v", err)
	}
	if len(waits) != 1 || waits[0] != 10*time.Second {
		t.Fatalf("expected one 10s wait after filling the window, got %v", waits)
	}
}

func TestMetadataRateLimitRetryDelayUsesBoundedExponentialBackoff(t *testing.T) {
	originalLimiter := sharedMetadataRateLimiter
	defer func() {
		sharedMetadataRateLimiter = originalLimiter
	}()

	limiter := newMetadataRateLimiter(map[MetadataSource]MetadataRateLimitPolicy{
		enums.VNDB: {
			Source:              enums.VNDB,
			Interval:            4 * time.Second,
			RateLimitRetryDelay: time.Minute,
			MaxRetryDelay:       5 * time.Minute,
		},
	})
	sharedMetadataRateLimiter = limiter

	want := []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute}
	got := make([]time.Duration, 0, len(want))
	for retryIndex := range want {
		got = append(got, metadataRateLimitRetryDelay(enums.VNDB, "", retryIndex))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected VNDB retry backoff: got %v, want %v", got, want)
	}
	if got := metadataRateLimitRetryDelay(enums.VNDB, "600", 0); got != 10*time.Minute {
		t.Fatalf("expected Retry-After to override the local cap when longer, got %s", got)
	}
}

func TestDoLimitedMetadataRequestRetriesVNDBFourTimes(t *testing.T) {
	originalLimiter := sharedMetadataRateLimiter
	defer func() {
		sharedMetadataRateLimiter = originalLimiter
	}()

	start := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	now := start
	waits := []time.Duration{}
	limiter := newMetadataRateLimiter(map[MetadataSource]MetadataRateLimitPolicy{
		enums.VNDB: {
			Source:              enums.VNDB,
			RateLimitRetryDelay: time.Minute,
			MaxRetryDelay:       5 * time.Minute,
			MaxRateLimitRetries: 4,
		},
	})
	limiter.now = func() time.Time {
		return now
	}
	limiter.wait = func(ctx context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		now = now.Add(delay)
		return nil
	}
	sharedMetadataRateLimiter = limiter

	requestCount := 0
	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		status := http.StatusTooManyRequests
		body := "throttled"
		if requestCount == 5 {
			status = http.StatusOK
			body = `{}`
		}
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}
	req, err := http.NewRequest(http.MethodPost, "https://api.vndb.org/kana/vn", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := doLimitedMetadataRequest(client, req, enums.VNDB)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer closeResponseBody(resp.Body)
	if requestCount != 5 {
		t.Fatalf("expected one initial request and four retries, got %d requests", requestCount)
	}
	wantWaits := []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute}
	if !reflect.DeepEqual(waits, wantWaits) {
		t.Fatalf("unexpected retry waits: got %v, want %v", waits, wantWaits)
	}
}

func TestDoLimitedMetadataRequestCancelsDuringBackoff(t *testing.T) {
	originalLimiter := sharedMetadataRateLimiter
	defer func() {
		sharedMetadataRateLimiter = originalLimiter
	}()

	limiter := newMetadataRateLimiter(map[MetadataSource]MetadataRateLimitPolicy{
		enums.VNDB: {
			Source:              enums.VNDB,
			RateLimitRetryDelay: time.Minute,
			MaxRateLimitRetries: 4,
		},
	})
	sharedMetadataRateLimiter = limiter

	ctx, cancel := context.WithCancel(context.Background())
	requestCount := 0
	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		cancel()
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("throttled")),
			Request:    req,
		}, nil
	})}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.vndb.org/kana/vn", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = doLimitedMetadataRequest(client, req, enums.VNDB)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cooldown wait to be canceled, got %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected cancellation before retry, got %d requests", requestCount)
	}
}

func TestVNDBDefaultPolicyIsConservativeForLongBatches(t *testing.T) {
	policy, ok := DefaultMetadataRateLimitPolicies()[enums.VNDB]
	if !ok {
		t.Fatal("expected VNDB policy")
	}
	if policy.Interval != 4*time.Second {
		t.Fatalf("expected VNDB interval to be 4s, got %s", policy.Interval)
	}
	if policy.UpstreamLimit != 200 || policy.UpstreamWindow != 5*time.Minute {
		t.Fatalf("unexpected VNDB upstream window: limit=%d window=%s", policy.UpstreamLimit, policy.UpstreamWindow)
	}
	if policy.RateLimitRetryDelay != time.Minute {
		t.Fatalf("expected VNDB retry delay to be 1m, got %s", policy.RateLimitRetryDelay)
	}
	if policy.MaxRetryDelay != 5*time.Minute {
		t.Fatalf("expected VNDB retry delay cap to be 5m, got %s", policy.MaxRetryDelay)
	}
	if policy.MaxRateLimitRetries != 4 {
		t.Fatalf("expected VNDB to retry four times, got %d", policy.MaxRateLimitRetries)
	}
}

func TestIsRateLimitError(t *testing.T) {
	cases := []error{
		errors.New("vndb metadata request remained rate limited after retry: status 429"),
		errors.New("VNDB API returned status: 429"),
		errors.New("too many requests"),
		errors.New("limit exceeded by upstream"),
		errors.New("request was throttled"),
	}

	for _, err := range cases {
		if !IsRateLimitError(err) {
			t.Fatalf("expected rate limit error for %q", err.Error())
		}
	}

	if IsRateLimitError(errors.New("no results found")) {
		t.Fatal("did not expect no-result errors to be treated as rate limits")
	}
}

type metadataRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f metadataRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
