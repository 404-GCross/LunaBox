package service

import (
	"testing"

	"lunabox/internal/version"
)

func TestBangumiServiceLoadsCentralizedBuildCredentials(t *testing.T) {
	previousClientID := version.BangumiOAuthClientID
	previousClientSecret := version.BangumiOAuthClientSecret
	t.Cleanup(func() {
		version.BangumiOAuthClientID = previousClientID
		version.BangumiOAuthClientSecret = previousClientSecret
	})

	version.BangumiOAuthClientID = "dev-client-id"
	version.BangumiOAuthClientSecret = "dev-client-secret"
	svc := NewBangumiService()

	if svc.clientID != "dev-client-id" {
		t.Fatalf("client id = %q, want %q", svc.clientID, "dev-client-id")
	}
	if svc.clientSecret != "dev-client-secret" {
		t.Fatalf("client secret = %q, want %q", svc.clientSecret, "dev-client-secret")
	}
}
