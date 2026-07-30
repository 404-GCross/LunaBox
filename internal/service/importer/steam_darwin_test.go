//go:build darwin

package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindSteamInstallPathInHome(t *testing.T) {
	homeDir := t.TempDir()
	expected := filepath.Join(homeDir, "Library", "Application Support", "Steam")
	if err := os.MkdirAll(filepath.Join(expected, "steamapps"), 0o755); err != nil {
		t.Fatalf("create Steam library: %v", err)
	}

	actual, err := findSteamInstallPathInHome(homeDir)
	if err != nil {
		t.Fatalf("find Steam install path: %v", err)
	}
	if actual != expected {
		t.Fatalf("expected Steam path %q, got %q", expected, actual)
	}
}

func TestFindSteamInstallPathInHomeRequiresSteamApps(t *testing.T) {
	if _, err := findSteamInstallPathInHome(t.TempDir()); err == nil {
		t.Fatal("expected missing steamapps directory to be rejected")
	}
}

func TestIsImportableSteamGameRequiresInstalledNumericAppID(t *testing.T) {
	installDir := t.TempDir()
	base := SteamLocalGame{
		AppID:      "123456",
		Name:       "Native Steam Game",
		InstallDir: installDir,
		StateFlags: steamFullyInstalledFlag,
	}
	if !isImportableSteamGame(base) {
		t.Fatal("expected fully installed Steam game with numeric AppID to be importable")
	}

	invalidAppID := base
	invalidAppID.AppID = "shortcut"
	if isImportableSteamGame(invalidAppID) {
		t.Fatal("expected non-numeric AppID to be rejected")
	}

	incomplete := base
	incomplete.StateFlags = 2
	if isImportableSteamGame(incomplete) {
		t.Fatal("expected incomplete Steam game to be rejected")
	}
}
