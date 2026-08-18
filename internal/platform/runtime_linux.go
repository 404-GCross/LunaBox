//go:build linux

package platform

import "os"

type RuntimeEnvironment struct {
	Key    string
	Value  string
	Reason string
}

func ConfigureRuntimeEnvironment() []RuntimeEnvironment {
	return setDefaultRuntimeEnvironment(map[string]RuntimeEnvironment{
		"WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS": {
			Key:    "WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS",
			Value:  "1",
			Reason: "avoid WebKitGTK bubblewrap/dbus-proxy aborts on restricted Linux sessions",
		},
	})
}

func setDefaultRuntimeEnvironment(defaults map[string]RuntimeEnvironment) []RuntimeEnvironment {
	changed := make([]RuntimeEnvironment, 0, len(defaults))
	for key, runtimeEnv := range defaults {
		if os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, runtimeEnv.Value)
		changed = append(changed, runtimeEnv)
	}
	return changed
}
