//go:build !windows && !linux

package protocol

import "fmt"

func RegisterPortableURLScheme(string) error {
	return fmt.Errorf("portable protocol registration is not supported on this platform")
}

func GetRegisteredURLSchemeExe() (string, error) {
	return "", nil
}

func UnregisterPortableURLScheme() error {
	return fmt.Errorf("portable protocol registration is not supported on this platform")
}

// platformHandlerMatchesTarget reports whether a LunaBox-specific handler
// wrapper points at targetPath. Only Linux creates wrapper scripts, so the
// plain path comparison in HandlerMatchesTarget is enough elsewhere.
func platformHandlerMatchesTarget(string, string) bool {
	return false
}

// platformManagedHandler reports whether registeredPath is a LunaBox-created
// handler wrapper. Only Linux creates wrapper scripts.
func platformManagedHandler(string) bool {
	return false
}
