package apputils

import (
	"os"
	"path/filepath"
)

// GetCLIDir returns the directory where the standalone CLI binary is expected
// to be bundled for the current platform.
func GetCLIDir() (string, error) {
	return getCLIDir()
}

// GetCLIPath returns the expected absolute path of the standalone CLI binary
// in the current application layout.
func GetCLIPath() (string, error) {
	dir, err := GetCLIDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, CLIExecutableName), nil
}

// CLIExists reports whether the standalone CLI binary is actually present at
// the expected bundled location.
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
