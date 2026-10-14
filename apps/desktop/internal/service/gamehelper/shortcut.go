package gamehelper

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveLaunchShortcutIconPath picks the icon source for a launch shortcut: the game
// executable when it can supply an icon, otherwise the current LunaBox executable.
func ResolveLaunchShortcutIconPath(gamePath string) string {
	trimmedPath := strings.TrimSpace(gamePath)
	if trimmedPath != "" {
		absPath, err := filepath.Abs(filepath.Clean(trimmedPath))
		if err == nil {
			if info, statErr := os.Stat(absPath); statErr == nil && !info.IsDir() && CanUseShortcutIconSource(absPath) {
				return absPath
			}
		}
	}

	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	absExePath, err := filepath.Abs(exePath)
	if err != nil {
		return exePath
	}
	return absExePath
}

// CanUseShortcutIconSource reports whether the file extension can supply an icon for a
// Windows Internet Shortcut.
func CanUseShortcutIconSource(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe", ".ico", ".dll":
		return true
	default:
		return false
	}
}
