package gamehelper

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultGameDirectory derives a usable game directory when an importer only
// knows the launch path. Existing directory paths are preserved.
func DefaultGameDirectory(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	cleanPath := filepath.Clean(path)
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		return cleanPath
	}
	if IsMacAppBundlePath(cleanPath) {
		return cleanPath
	}

	dir := filepath.Dir(cleanPath)
	if dir == "." {
		return ""
	}
	return dir
}
