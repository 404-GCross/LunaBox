//go:build linux

package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallAppImageProtocolLauncherWritesStableWrapper(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	appImagePath := filepath.Join(t.TempDir(), "LunaBox's Test.AppImage")
	if err := os.WriteFile(appImagePath, []byte("#!/usr/bin/env sh\n"), 0755); err != nil {
		t.Fatalf("write AppImage: %v", err)
	}

	launcherPath, err := installAppImageProtocolLauncher(appImagePath)
	if err != nil {
		t.Fatalf("installAppImageProtocolLauncher() error = %v", err)
	}

	expectedPath := filepath.Join(home, ".local", "share", "LunaBox", linuxAppImageLauncherName)
	if launcherPath != expectedPath {
		t.Fatalf("launcher path = %q, want %q", launcherPath, expectedPath)
	}
	if !IsAppImageProtocolLauncher(launcherPath) {
		t.Fatal("launcher is not recognized as generated AppImage launcher")
	}
	if !IsAppImageProtocolLauncherFor(launcherPath, appImagePath) {
		t.Fatal("launcher is not recognized as targeting the AppImage")
	}

	content, err := os.ReadFile(launcherPath)
	if err != nil {
		t.Fatalf("read launcher: %v", err)
	}
	text := string(content)
	for _, want := range []string{
		linuxAppImageLauncherMarker,
		"exec \"$appimage\" \"$@\"",
		"LunaBox-*-linux-*.AppImage",
		"$home_dir/Downloads",
		"$home_dir/下载",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("launcher content missing %q: %s", want, text)
		}
	}
}
