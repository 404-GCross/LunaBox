package tricksutils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectWinetricksPrefersConfiguredPath(t *testing.T) {
	dir := t.TempDir()
	configured := filepath.Join(dir, "custom-winetricks")
	if err := os.WriteFile(configured, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write configured executable: %v", err)
	}

	t.Setenv("PATH", "")
	tool := DetectWinetricks(configured)
	if !tool.Available {
		t.Fatalf("expected configured winetricks to be available: %+v", tool)
	}
	if tool.Path != configured || tool.Source != "config" {
		t.Fatalf("unexpected configured tool: %+v", tool)
	}
}

func TestDetectProtontricksReportsConfiguredPathError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce the Unix executable bit")
	}

	dir := t.TempDir()
	configured := filepath.Join(dir, "protontricks")
	if err := os.WriteFile(configured, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("write configured executable: %v", err)
	}

	tool := DetectProtontricks(configured)
	if tool.Available {
		t.Fatalf("expected configured protontricks without executable bit to be unavailable")
	}
	if tool.Path != configured || tool.Source != "config" || tool.Error == "" {
		t.Fatalf("unexpected configured tool error: %+v", tool)
	}
}
