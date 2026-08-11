//go:build linux

package apputils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindExecutablesAllowsWineExecutablesOnLinux(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"Game.exe", "patch.bat"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	executables := FindExecutables(dir, nil)
	if len(executables) != 2 {
		t.Fatalf("expected 2 executables, got %#v", executables)
	}

	best := SelectBestExecutable(executables, "Game")
	if filepath.Base(best) != "Game.exe" {
		t.Fatalf("expected Game.exe as best executable, got %q", best)
	}
}

func TestFindExecutablesAllowsNativeExecutablesOnLinux(t *testing.T) {
	dir := t.TempDir()
	launchPath := filepath.Join(dir, "launcher")
	readmePath := filepath.Join(dir, "readme.txt")
	if err := os.WriteFile(launchPath, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readmePath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	executables := FindExecutables(dir, nil)
	if len(executables) != 1 {
		t.Fatalf("expected 1 executable, got %#v", executables)
	}
	if filepath.Base(executables[0]) != "launcher" {
		t.Fatalf("expected launcher as executable, got %q", executables[0])
	}
}
