//go:build linux

package apputils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lunabox/internal/version"
)

func TestGetLaunchExecutablePathUsesAppImageRuntimePath(t *testing.T) {
	restoreBuildMode := setBuildModeForTest("appimage")
	defer restoreBuildMode()

	appImagePath := createExecutableFileForTest(t, filepath.Join(t.TempDir(), "LunaBox Test.AppImage"))
	t.Setenv("LUNABOX_APPIMAGE_PATH", appImagePath)
	t.Setenv("APPIMAGE", "")

	got, err := GetLaunchExecutablePath()
	if err != nil {
		t.Fatalf("GetLaunchExecutablePath() error = %v", err)
	}
	if got != appImagePath {
		t.Fatalf("GetLaunchExecutablePath() = %q, want %q", got, appImagePath)
	}
}

func TestInstallCLIWritesAppImageWrapper(t *testing.T) {
	restoreBuildMode := setBuildModeForTest("appimage")
	defer restoreBuildMode()

	home := t.TempDir()
	appImagePath := createExecutableFileForTest(t, filepath.Join(t.TempDir(), "LunaBox's Test.AppImage"))
	t.Setenv("HOME", home)
	t.Setenv("LUNABOX_APPIMAGE_PATH", appImagePath)
	t.Setenv("APPIMAGE", "")

	changed, err := InstallCLI()
	if err != nil {
		t.Fatalf("InstallCLI() error = %v", err)
	}
	if !changed {
		t.Fatal("InstallCLI() changed = false, want true")
	}

	installed, err := IsCLIInstalled()
	if err != nil {
		t.Fatalf("IsCLIInstalled() error = %v", err)
	}
	if !installed {
		t.Fatal("IsCLIInstalled() = false, want true")
	}

	wrapperPath, err := GetCLIInstallPath()
	if err != nil {
		t.Fatalf("GetCLIInstallPath() error = %v", err)
	}
	content, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatalf("read wrapper: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, appImageCLIWrapperMarker) {
		t.Fatalf("wrapper is missing marker: %q", text)
	}
	if !strings.Contains(text, `cli "$@"`) {
		t.Fatalf("wrapper does not dispatch to AppImage CLI: %q", text)
	}

	changed, err = UninstallCLI()
	if err != nil {
		t.Fatalf("UninstallCLI() error = %v", err)
	}
	if !changed {
		t.Fatal("UninstallCLI() changed = false, want true")
	}
	if _, err := os.Stat(wrapperPath); !os.IsNotExist(err) {
		t.Fatalf("wrapper still exists after uninstall, stat error = %v", err)
	}
}

func setBuildModeForTest(mode string) func() {
	previous := version.BuildMode
	version.BuildMode = mode
	return func() {
		version.BuildMode = previous
	}
}

func createExecutableFileForTest(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/usr/bin/env sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
