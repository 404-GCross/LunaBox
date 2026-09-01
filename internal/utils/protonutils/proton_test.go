//go:build linux

package protonutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverToolsFindsDWProtonFromHeroic(t *testing.T) {
	home := t.TempDir()
	toolDir := filepath.Join(home, ".config", "heroic", "tools", "proton", "DW-Proton")
	writeExecutable(t, filepath.Join(toolDir, "proton"))
	if err := os.WriteFile(filepath.Join(toolDir, "compatibilitytool.vdf"), []byte(`"compatibilitytools"
{
  "compat_tools"
  {
    "dw-proton" // Internal name of this tool
    {
      "display_name" "DW-Proton"
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write compatibilitytool.vdf: %v", err)
	}

	tools := discoverTools(discoverOptions{home: home})

	if len(tools) != 1 {
		t.Fatalf("expected one Proton tool, got %#v", tools)
	}
	if tools[0].Name != "dw-proton" || tools[0].DisplayName != "DW-Proton" {
		t.Fatalf("unexpected Proton tool metadata: %#v", tools[0])
	}
	if tools[0].ProtonPath != filepath.Join(toolDir, "proton") {
		t.Fatalf("unexpected Proton path: %#v", tools[0])
	}
}

func TestDiscoverToolsFindsSteamLibraryProton(t *testing.T) {
	home := t.TempDir()
	steamRoot := filepath.Join(home, ".local", "share", "Steam")
	extraLibrary := filepath.Join(home, "Games", "SteamLibrary")
	writeExecutable(t, filepath.Join(extraLibrary, "steamapps", "common", "Proton 9.0", "proton"))
	if err := os.MkdirAll(filepath.Join(steamRoot, "steamapps"), 0o755); err != nil {
		t.Fatalf("create steamapps: %v", err)
	}
	if err := os.WriteFile(filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"), []byte(`"libraryfolders"
{
  "0" { "path" "`+filepath.ToSlash(extraLibrary)+`" }
}`), 0o644); err != nil {
		t.Fatalf("write libraryfolders.vdf: %v", err)
	}

	tools := discoverTools(discoverOptions{home: home})

	if len(tools) != 1 {
		t.Fatalf("expected one Proton tool, got %#v", tools)
	}
	if tools[0].Source != "steam" || !tools[0].BuiltIn {
		t.Fatalf("unexpected Proton tool source: %#v", tools[0])
	}
}

func TestDiscoverToolsFindsProtonPlusLutrisWineRunner(t *testing.T) {
	home := t.TempDir()
	toolDir := filepath.Join(home, ".local", "share", "lutris", "runners", "wine", "GE-Proton11-6")
	writeExecutable(t, filepath.Join(toolDir, "proton"))

	tools := discoverTools(discoverOptions{home: home})

	if len(tools) != 1 {
		t.Fatalf("expected one Proton tool, got %#v", tools)
	}
	if tools[0].Source != "lutris" {
		t.Fatalf("unexpected Proton tool source: %#v", tools[0])
	}
	if tools[0].ProtonPath != filepath.Join(toolDir, "proton") {
		t.Fatalf("unexpected Proton path: %#v", tools[0])
	}
}

func TestNormalizeCompatDataPathAcceptsPfxDirectory(t *testing.T) {
	got := NormalizeCompatDataPath("/home/u/.local/share/LunaBox/proton-compatdata/game/pfx")
	want := "/home/u/.local/share/LunaBox/proton-compatdata/game"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create executable dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}
