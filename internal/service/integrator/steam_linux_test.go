//go:build linux

package integrator

import (
	"context"
	"lunabox/internal/models"
	"os"
	"path/filepath"
	"testing"
)

func TestFindSteamRootLinuxUsesEnvCandidate(t *testing.T) {
	steamRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(steamRoot, "steamapps"), 0o755); err != nil {
		t.Fatalf("create steamapps: %v", err)
	}
	t.Setenv("STEAM_DIR", steamRoot)

	got, err := findSteamRoot()
	if err != nil {
		t.Fatalf("findSteamRoot() returned error: %v", err)
	}
	if got != steamRoot {
		t.Fatalf("findSteamRoot() = %q, want %q", got, steamRoot)
	}
}

func TestLinuxResolveSteamTargetFindsNativeGame(t *testing.T) {
	steamRoot := t.TempDir()
	installDir := filepath.Join(steamRoot, "steamapps", "common", "Native Game")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("create native game dir: %v", err)
	}
	manifestPath := filepath.Join(steamRoot, "steamapps", "appmanifest_123456.acf")
	if err := os.WriteFile(manifestPath, []byte(`"AppState"
{
	"appid"		"123456"
	"name"		"Native Game"
	"installdir"		"Native Game"
	"StateFlags"		"4"
}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	t.Setenv("STEAM_DIR", steamRoot)

	result, err := resolveSteamPlatformTarget(context.Background(), models.Game{
		Path:          installDir,
		GameDirectory: installDir,
	})
	if err != nil {
		t.Fatalf("resolveSteamPlatformTarget() returned error: %v", err)
	}
	if !result.Status.Ready || result.Status.State != SteamLaunchStateReady {
		t.Fatalf("expected ready native status, got %+v", result.Status)
	}
	if result.Status.LaunchID != "123456" || result.Status.LaunchKind != "native" {
		t.Fatalf("unexpected launch identity: %+v", result.Status)
	}
}

func TestLinuxSelectSteamLoginUser(t *testing.T) {
	data := []byte(`"users"
{
	"76561198000000001"
	{
		"AccountName"		"older"
		"MostRecent"		"0"
		"Timestamp"		"100"
	}
	"76561198000000002"
	{
		"AccountName"		"current"
		"MostRecent"		"1"
		"Timestamp"		"200"
	}
}`)

	got, found := selectSteamLoginUser(parseSteamLoginUsers(data), "")
	if !found || got != "39734274" {
		t.Fatalf("selectSteamLoginUser() = %q, %v; want %q, true", got, found, "39734274")
	}
}
