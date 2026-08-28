package main

import (
	"log/slog"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"lunabox/internal/applog"
	"lunabox/internal/version"
)

func TestRepairStaleAppImageProtocolRegistration(t *testing.T) {
	if goruntime.GOOS != "linux" {
		t.Skip("Linux protocol registration is only available on Linux")
	}

	tmpDir := t.TempDir()
	appImagePath := filepath.Join(tmpDir, "LunaBox.AppImage")
	writeExecutable(t, appImagePath, "#!/bin/sh\n")
	setupAppImageProtocolRepairTest(t, tmpDir, appImagePath)

	applicationsDir := filepath.Join(tmpDir, ".local", "share", "applications")
	if err := os.MkdirAll(applicationsDir, 0o755); err != nil {
		t.Fatalf("create applications dir: %v", err)
	}
	desktopPath := filepath.Join(applicationsDir, "io.github.saramanda9988.lunabox.desktop")
	if err := os.WriteFile(desktopPath, []byte(strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=LunaBox",
		"Exec=/missing/LunaBox %u",
		"MimeType=x-scheme-handler/lunabox;",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("write stale desktop entry: %v", err)
	}

	repairStaleAppImageProtocolRegistration(newTestLogger(t, tmpDir))

	content, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("read repaired desktop entry: %v", err)
	}
	expectedExec := "Exec=\"" + appImagePath + "\" %u"
	if !strings.Contains(string(content), expectedExec) {
		t.Fatalf("desktop Exec was not repaired; got:\n%s\nwant line containing %q", content, expectedExec)
	}
}

func TestRepairStaleAppImageProtocolRegistrationKeepsValidInstalledHandler(t *testing.T) {
	if goruntime.GOOS != "linux" {
		t.Skip("Linux protocol registration is only available on Linux")
	}

	tmpDir := t.TempDir()
	appImagePath := filepath.Join(tmpDir, "LunaBox.AppImage")
	installedPath := filepath.Join(tmpDir, "usr", "bin", "LunaBox")
	writeExecutable(t, appImagePath, "#!/bin/sh\n")
	writeExecutable(t, installedPath, "#!/bin/sh\n")
	setupAppImageProtocolRepairTest(t, tmpDir, appImagePath)

	applicationsDir := filepath.Join(tmpDir, ".local", "share", "applications")
	if err := os.MkdirAll(applicationsDir, 0o755); err != nil {
		t.Fatalf("create applications dir: %v", err)
	}
	desktopPath := filepath.Join(applicationsDir, "io.github.saramanda9988.lunabox.desktop")
	originalContent := strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=LunaBox",
		"Exec=" + installedPath + " %u",
		"MimeType=x-scheme-handler/lunabox;",
		"",
	}, "\n")
	if err := os.WriteFile(desktopPath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("write installed desktop entry: %v", err)
	}

	repairStaleAppImageProtocolRegistration(newTestLogger(t, tmpDir))

	content, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("read desktop entry: %v", err)
	}
	if string(content) != originalContent {
		t.Fatalf("valid installed handler should not be replaced; got:\n%s", content)
	}
}

func setupAppImageProtocolRepairTest(t *testing.T, tmpDir string, appImagePath string) {
	t.Helper()

	previousBuildMode := version.BuildMode
	version.BuildMode = "appimage"
	t.Cleanup(func() {
		version.BuildMode = previousBuildMode
	})
	t.Setenv("HOME", tmpDir)
	t.Setenv("LUNABOX_APPIMAGE_PATH", appImagePath)
	t.Setenv("APPIMAGE", appImagePath)

	fakeBinDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(fakeBinDir, 0o755); err != nil {
		t.Fatalf("create fake bin dir: %v", err)
	}
	writeExecutable(t, filepath.Join(fakeBinDir, "xdg-mime"), strings.Join([]string{
		"#!/bin/sh",
		"case \"$1\" in",
		"  query) printf '%s\\n' io.github.saramanda9988.lunabox.desktop ;;",
		"  default) exit 0 ;;",
		"  *) exit 1 ;;",
		"esac",
		"",
	}, "\n"))
	writeExecutable(t, filepath.Join(fakeBinDir, "update-desktop-database"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newTestLogger(t *testing.T, tmpDir string) *applog.FileLogger {
	t.Helper()
	return applog.NewFileLogger(filepath.Join(tmpDir, "app.log"), slog.LevelDebug)
}
