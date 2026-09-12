package protocol

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// HandlerMatchesTarget reports whether the lunabox:// handler registered at
// registeredPath already launches targetPath, so the registration can be kept
// as-is. Platform specifics such as LunaBox's own launcher wrappers are
// resolved by platformHandlerMatchesTarget.
func HandlerMatchesTarget(registeredPath string, targetPath string) bool {
	if sameExecutablePath(registeredPath, targetPath) {
		return true
	}
	return platformHandlerMatchesTarget(registeredPath, targetPath)
}

// IsManagedHandler reports whether registeredPath was written by LunaBox's own
// protocol registration, which means it can safely be replaced on repair.
func IsManagedHandler(registeredPath string) bool {
	return platformManagedHandler(registeredPath)
}

func sameExecutablePath(left string, right string) bool {
	leftPath, leftOK := comparableExecutablePath(left)
	rightPath, rightOK := comparableExecutablePath(right)
	if !leftOK || !rightOK {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(leftPath, rightPath)
	}
	return leftPath == rightPath
}

func comparableExecutablePath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	if filepath.IsAbs(path) || strings.ContainsRune(path, os.PathSeparator) {
		abs, err := filepath.Abs(filepath.Clean(path))
		if err != nil {
			return "", false
		}
		return abs, true
	}
	resolved, err := exec.LookPath(path)
	if err != nil {
		return "", false
	}
	abs, err := filepath.Abs(filepath.Clean(resolved))
	if err != nil {
		return "", false
	}
	return abs, true
}
