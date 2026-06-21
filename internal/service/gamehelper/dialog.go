package gamehelper

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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
func ExecutableOpenDialogOptions(title, defaultDirectory, defaultFilename string) runtime.OpenDialogOptions {
	options := runtime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: defaultDirectory,
		DefaultFilename:  defaultFilename,
	}
	if goruntime.GOOS == "darwin" {
		options.ResolvesAliases = true
		options.TreatPackagesAsDirectories = false
		return options
	}

	options.Filters = []runtime.FileFilter{
		executableFileFilter(),
		allFilesFileFilter(),
	}
	return options
}

// WineRunnerOpenDialogOptions mirrors the executable selector but lets the user browse
// into macOS .app packages so they can target a binary inside the bundle.
func WineRunnerOpenDialogOptions(title, defaultDirectory, defaultFilename string) runtime.OpenDialogOptions {
	options := ExecutableOpenDialogOptions(title, defaultDirectory, defaultFilename)
	if goruntime.GOOS == "darwin" {
		options.TreatPackagesAsDirectories = true
	}
	return options
}

func executableFileFilter() runtime.FileFilter {
	switch goruntime.GOOS {
	case "darwin":
		return runtime.FileFilter{
			DisplayName: "Applications and Executables",
			Pattern:     "*.app;*.exe;*.bat;*.cmd",
		}
	default:
		return runtime.FileFilter{
			DisplayName: "Executables",
			Pattern:     "*.exe;*.bat;*.cmd;*.lnk",
		}
	}
}

func allFilesFileFilter() runtime.FileFilter {
	return runtime.FileFilter{
		DisplayName: "All Files",
		Pattern:     "*.*",
	}
}
