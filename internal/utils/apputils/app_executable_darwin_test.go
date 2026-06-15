//go:build darwin

package apputils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindExecutablesAllowsWindowsExecutablesOnDarwin(t *testing.T) {
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
