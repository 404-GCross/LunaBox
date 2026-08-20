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
