package apputils

import (
	"fmt"
	"os"
	"path/filepath"
)

// CLIExecutableName is the file name of the standalone CLI binary shipped
// alongside LunaBox.exe in portable distributions.
const CLIExecutableName = "lunacli.exe"

// GetCLIDir returns the directory of the currently running executable, which
// is also where lunacli.exe is expected to live in portable builds.
func GetCLIDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("resolve absolute executable path: %w", err)
	}
	return filepath.Dir(abs), nil
}

// GetCLIPath returns the expected absolute path of lunacli.exe in the current
// portable layout.
func GetCLIPath() (string, error) {
	dir, err := GetCLIDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, CLIExecutableName), nil
}

// CLIExists reports whether lunacli.exe is actually present at the expected
// portable location.
func CLIExists() (bool, string, error) {
	p, err := GetCLIPath()
	if err != nil {
		return false, "", err
	}
	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, p, nil
		}
		return false, p, err
	}
	return !info.IsDir(), p, nil
}
