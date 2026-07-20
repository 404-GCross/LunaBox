package gamehelper

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"lunabox/internal/wailsruntime"
)

// ExecutableDialogDefaults derives the default directory/filename for an executable picker
// from an existing path, normalizing relative inputs and unwrapping macOS .app bundles.
func ExecutableDialogDefaults(currentPath string) (string, string) {
	currentPath = strings.TrimSpace(currentPath)
	if currentPath == "" {
		return "", ""
	}

	cleanPath := filepath.Clean(currentPath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		absPath = cleanPath
	}

	info, err := os.Stat(absPath)
	if err == nil {
		if info.IsDir() {
			if IsMacAppBundlePath(absPath) {
				return filepath.Dir(absPath), filepath.Base(absPath)
			}
			return absPath, ""
		}
		return filepath.Dir(absPath), filepath.Base(absPath)
	}

	if filepath.Ext(absPath) == "" {
		return "", ""
	}

	parentDir := filepath.Dir(absPath)
	if parentInfo, statErr := os.Stat(parentDir); statErr == nil && parentInfo.IsDir() {
		return parentDir, filepath.Base(absPath)
	}

	return "", ""
}

// IsMacAppBundlePath reports whether path points at a macOS .app bundle.
func IsMacAppBundlePath(path string) bool {
	return goruntime.GOOS == "darwin" && strings.EqualFold(filepath.Ext(strings.TrimSpace(path)), ".app")
}

// ExecutableOpenDialogOptions builds open-dialog options for selecting a game executable.
// On macOS the filters are omitted so Unix executables with no extension stay selectable
// and .app bundles can be picked as package files.
func ExecutableOpenDialogOptions(title, defaultDirectory, defaultFilename string) wailsruntime.OpenDialogOptions {
	options := wailsruntime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: defaultDirectory,
		DefaultFilename:  defaultFilename,
	}
	if goruntime.GOOS == "darwin" {
		options.ResolvesAliases = true
		options.TreatPackagesAsDirectories = false
		return options
	}

	options.Filters = []wailsruntime.FileFilter{
		executableFileFilter(),
		allFilesFileFilter(),
	}
	return options
}

// WineRunnerOpenDialogOptions mirrors the executable selector but lets the user browse
// into macOS .app packages so they can target a binary inside the bundle.
func WineRunnerOpenDialogOptions(title, defaultDirectory, defaultFilename string) wailsruntime.OpenDialogOptions {
	options := ExecutableOpenDialogOptions(title, defaultDirectory, defaultFilename)
	if goruntime.GOOS == "darwin" {
		options.TreatPackagesAsDirectories = true
	}
	return options
}

func executableFileFilter() wailsruntime.FileFilter {
	switch goruntime.GOOS {
	case "darwin":
		return wailsruntime.FileFilter{
			DisplayName: "Applications and Executables",
			Pattern:     "*.app;*.exe;*.bat;*.cmd",
		}
	default:
		return wailsruntime.FileFilter{
			DisplayName: "Executables",
			Pattern:     "*.exe;*.bat;*.cmd;*.lnk",
		}
	}
}

func allFilesFileFilter() wailsruntime.FileFilter {
	return wailsruntime.FileFilter{
		DisplayName: "All Files",
		Pattern:     "*.*",
	}
}
