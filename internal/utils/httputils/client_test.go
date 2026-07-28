package httputils

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lunabox/internal/utils/proxyutils"
	"lunabox/internal/version"
)

func TestNewClientSetsDefaultUserAgent(t *testing.T) {
	userAgents := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		userAgents <- req.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, proxyDescription, err := NewClient(ClientOptions{
		Timeout:   time.Second,
		ProxyMode: proxyutils.ProxyModeDirect,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if proxyDescription != proxyutils.ProxyModeDirect {
		t.Fatalf("unexpected proxy description: %q", proxyDescription)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if got := <-userAgents; got != version.UserAgent() {
		t.Fatalf("unexpected default User-Agent: %q", got)
	}
	if got := req.Header.Get("User-Agent"); got != "" {
		t.Fatalf("original request was mutated: User-Agent=%q", got)
	}
}

func TestNewClientPreservesRequestUserAgent(t *testing.T) {
	userAgents := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		userAgents <- req.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, _, err := NewClient(ClientOptions{
		Timeout:   time.Second,
		ProxyMode: proxyutils.ProxyModeDirect,
		UserAgent: "configured-agent",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("User-Agent", "request-agent")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	_ = resp.Body.Close()

	if got := <-userAgents; got != "request-agent" {
		t.Fatalf("request User-Agent was replaced: %q", got)
	}
}

func TestNewClientUsesConfiguredUserAgent(t *testing.T) {
	userAgents := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		userAgents <- req.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, _, err := NewClient(ClientOptions{
		Timeout:   time.Second,
		ProxyMode: proxyutils.ProxyModeDirect,
		UserAgent: "configured-agent",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	_ = resp.Body.Close()

	if got := <-userAgents; got != "configured-agent" {
		t.Fatalf("unexpected configured User-Agent: %q", got)
	}
}
