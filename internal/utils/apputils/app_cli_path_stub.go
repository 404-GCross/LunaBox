//go:build !windows

package apputils

import "fmt"

func IsDirInUserPath(_ string) (bool, error) {
	return false, nil
}

func AddDirToUserPath(_ string) (bool, error) {
	return false, fmt.Errorf("PATH management is only supported on Windows")
}

func RemoveDirFromUserPath(_ string) (bool, error) {
	return false, fmt.Errorf("PATH management is only supported on Windows")
}
