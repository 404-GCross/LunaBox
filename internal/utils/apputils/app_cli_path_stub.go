//go:build !windows

package apputils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const CLIExecutableName = "lunacli"

func getCLIDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("resolve absolute executable path: %w", err)
	}
	if runtime.GOOS == "darwin" {
		const bundleMarker = ".app/Contents/MacOS/"
		if idx := strings.Index(abs, bundleMarker); idx >= 0 {
			return filepath.Join(abs[:idx+len(".app")], "Contents", "Resources", "bin"), nil
		}
	}
	return filepath.Dir(abs), nil
}

func GetCLIInstallPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".local", "bin", CLIExecutableName), nil
}

func IsCLIInstalled() (bool, error) {
	source, err := GetCLIPath()
	if err != nil {
		return false, err
	}
	target, err := GetCLIInstallPath()
	if err != nil {
		return false, err
	}

	resolved, err := os.Readlink(target)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read CLI symlink: %w", err)
	}
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(filepath.Dir(target), resolved)
	}
	resolved, _ = filepath.Abs(resolved)
	source, _ = filepath.Abs(source)
	return resolved == source, nil
}

func InstallCLI() (bool, error) {
	source, err := GetCLIPath()
	if err != nil {
		return false, err
	}
	target, err := GetCLIInstallPath()
	if err != nil {
		return false, err
	}

	if info, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("CLI binary does not exist: %s", source)
		}
		return false, fmt.Errorf("stat CLI binary: %w", err)
	} else if info.IsDir() {
		return false, fmt.Errorf("CLI binary path is a directory: %s", source)
	}

	installed, err := IsCLIInstalled()
	if err != nil {
		return false, err
	}
	if installed {
		return false, nil
	}

	if existing, err := os.Lstat(target); err == nil {
		if existing.Mode()&os.ModeSymlink == 0 {
			return false, fmt.Errorf("CLI install target already exists and is not a symlink: %s", target)
		}
		if err := os.Remove(target); err != nil {
			return false, fmt.Errorf("remove stale CLI symlink: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat CLI install target: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return false, fmt.Errorf("create CLI install directory: %w", err)
	}
	if err := os.Symlink(source, target); err != nil {
		return false, fmt.Errorf("create CLI symlink: %w", err)
	}
	return true, nil
}

func UninstallCLI() (bool, error) {
	target, err := GetCLIInstallPath()
	if err != nil {
		return false, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat CLI install target: %w", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, fmt.Errorf("CLI install target is not a symlink: %s", target)
	}
	if err := os.Remove(target); err != nil {
		return false, fmt.Errorf("remove CLI symlink: %w", err)
	}
	return true, nil
}

func IsDirInUserPath(_ string) (bool, error) {
	return false, nil
}

func AddDirToUserPath(_ string) (bool, error) {
	return false, fmt.Errorf("PATH editing is only supported on Windows")
}

func RemoveDirFromUserPath(_ string) (bool, error) {
	return false, fmt.Errorf("PATH editing is only supported on Windows")
}
