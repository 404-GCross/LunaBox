package service

import (
	"context"
	"testing"
)

func TestBangumiServiceLoadsDevOAuthClientCredentialsFromEnv(t *testing.T) {
	t.Setenv(bangumiOAuthClientIDEnv, "dev-client-id")
	t.Setenv(bangumiOAuthClientSecretEnv, "dev-client-secret")

	svc := NewBangumiService()
	svc.SetOAuthClientCredentials("", "")
	svc.loadDevOAuthClientCredentials(context.WithValue(context.Background(), "buildtype", "dev"))

	if svc.clientID != "dev-client-id" {
		t.Fatalf("client id = %q, want %q", svc.clientID, "dev-client-id")
	}
	if svc.clientSecret != "dev-client-secret" {
		t.Fatalf("client secret = %q, want %q", svc.clientSecret, "dev-client-secret")
	}
}

func TestBangumiServiceSkipsEnvOutsideDevBuild(t *testing.T) {
	t.Setenv(bangumiOAuthClientIDEnv, "dev-client-id")
	t.Setenv(bangumiOAuthClientSecretEnv, "dev-client-secret")

	svc := NewBangumiService()
	svc.SetOAuthClientCredentials("", "")
	svc.loadDevOAuthClientCredentials(context.WithValue(context.Background(), "buildtype", "production"))

	if svc.clientID != "" || svc.clientSecret != "" {
		t.Fatalf("非 dev build 不应加载 env 文件，got id=%q secret=%q", svc.clientID, svc.clientSecret)
	}
}
