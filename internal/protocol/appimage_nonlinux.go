//go:build !linux

package protocol

// IsAppImageProtocolLauncher reports whether path is LunaBox's generated
// AppImage protocol launcher. AppImage launchers only exist on Linux, so this
// is always false elsewhere.
func IsAppImageProtocolLauncher(path string) bool {
	return false
}

// IsAppImageProtocolLauncherFor reports whether path is the generated AppImage
// protocol launcher for the given AppImage file. AppImage launchers only exist
// on Linux, so this is always false elsewhere.
func IsAppImageProtocolLauncherFor(path string, appImagePath string) bool {
	return false
}
