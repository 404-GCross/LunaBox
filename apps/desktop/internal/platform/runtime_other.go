//go:build !linux

package platform

type RuntimeEnvironment struct {
	Key    string
	Value  string
	Reason string
}

func ConfigureRuntimeEnvironment() []RuntimeEnvironment {
	return nil
}
