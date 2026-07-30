//go:build !windows && !linux

package protocol

import "fmt"

func RegisterURLScheme(_ string) error {
	return fmt.Errorf("register-protocol is only supported on Windows and Linux")
}

func UnregisterURLScheme() error {
	return fmt.Errorf("unregister-protocol is only supported on Windows and Linux")
}

func GetRegisteredURLSchemeExe() (string, error) {
	return "", nil
}
