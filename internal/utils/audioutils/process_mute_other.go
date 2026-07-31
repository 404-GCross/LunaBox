//go:build !windows

package audioutils

// SetProcessMuted is a platform-compatible placeholder. Background game mute
// is currently exposed only on Windows.
func SetProcessMuted(processID uint32, muted bool) (bool, error) {
	return false, nil
}
