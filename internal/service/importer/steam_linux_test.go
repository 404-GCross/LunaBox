//go:build linux

package importer

import (
	"os"
	"path/filepath"
	"testing"

	"lunabox/internal/common/enums"
)

func TestFindSteamInstallPathLinuxUsesEnvCandidate(t *testing.T) {
	root := t.TempDir()
	steamPath := filepath.Join(root, "custom-steam")
	if err := os.MkdirAll(filepath.Join(steamPath, "steamapps"), 0755); err != nil {
		t.Fatalf("create steamapps: %v", err)
	}

	t.Setenv("STEAM_DIR", steamPath)
	t.Setenv("STEAM_HOME", "")
	t.Setenv("STEAM_ROOT", "")
	t.Setenv("STEAM_COMPAT_CLIENT_INSTALL_PATH", "")
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "missing-xdg"))
	t.Setenv("HOME", filepath.Join(root, "missing-home"))

	got, err := findSteamInstallPath()
	if err != nil {
		t.Fatalf("findSteamInstallPath returned error: %v", err)
	}
	if got != steamPath {
		t.Fatalf("expected %q, got %q", steamPath, got)
	}
}

func TestFindSteamInstallPathLinuxUsesXDGDataHome(t *testing.T) {
	root := t.TempDir()
	xdgDataHome := filepath.Join(root, "xdg-data")
	steamPath := filepath.Join(xdgDataHome, "Steam")
	if err := os.MkdirAll(filepath.Join(steamPath, "steamapps"), 0755); err != nil {
		t.Fatalf("create steamapps: %v", err)
	}

	t.Setenv("STEAM_DIR", "")
	t.Setenv("STEAM_HOME", "")
	t.Setenv("STEAM_ROOT", "")
	t.Setenv("STEAM_COMPAT_CLIENT_INSTALL_PATH", "")
	t.Setenv("XDG_DATA_HOME", xdgDataHome)
	t.Setenv("HOME", filepath.Join(root, "missing-home"))

	got, err := findSteamInstallPath()
	if err != nil {
		t.Fatalf("findSteamInstallPath returned error: %v", err)
	}
	if got != steamPath {
		t.Fatalf("expected %q, got %q", steamPath, got)
	}
}

func TestNormalizeSteamLinuxInstallPathAcceptsSteamappsDirectory(t *testing.T) {
	root := t.TempDir()
	steamapps := filepath.Join(root, "Steam", "steamapps")
	if err := os.MkdirAll(steamapps, 0755); err != nil {
		t.Fatalf("create steamapps: %v", err)
	}

	got, ok := normalizeSteamLinuxInstallPath(steamapps)
	if !ok {
		t.Fatal("expected steamapps path to be accepted")
	}
	if got != filepath.Dir(steamapps) {
		t.Fatalf("expected %q, got %q", filepath.Dir(steamapps), got)
	}
}

func TestDefaultSteamImportedLaunchModeLinuxUsesSteamLaunch(t *testing.T) {
	if got := defaultSteamImportedLaunchMode(); got != enums.LaunchModeSteam {
		t.Fatalf("expected Steam launch mode on Linux, got %q", got)
	}
}
