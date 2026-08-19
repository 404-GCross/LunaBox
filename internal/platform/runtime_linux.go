//go:build linux

package platform

import "os"

type RuntimeEnvironment struct {
	Key    string
	Value  string
	Reason string
}

func ConfigureRuntimeEnvironment() []RuntimeEnvironment {
	return setDefaultRuntimeEnvironment([]RuntimeEnvironment{
		{
			Key:    "WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS",
			Value:  "1",
			Reason: "avoid WebKitGTK bubblewrap/dbus-proxy aborts on restricted Linux sessions",
		},
		{
			Key:    "WEBKIT_DISABLE_COMPOSITING_MODE",
			Value:  "1",
			Reason: "avoid WebKitGTK accelerated compositing EGL initialization aborts",
		},
		{
			Key:    "WEBKIT_DISABLE_DMABUF_RENDERER",
			Value:  "1",
			Reason: "avoid WebKitGTK DMABUF renderer EGL initialization aborts",
		},
		{
			Key:    "GDK_DISABLE",
			Value:  "dmabuf",
			Reason: "avoid GTK DMA-BUF renderer paths on Linux sessions without a working EGL display",
		},
		{
			Key:    "GSK_RENDERER",
			Value:  "cairo",
			Reason: "force GTK software rendering when GL/EGL initialization is unavailable",
		},
	})
}

func setDefaultRuntimeEnvironment(defaults []RuntimeEnvironment) []RuntimeEnvironment {
	changed := make([]RuntimeEnvironment, 0, len(defaults))
	for _, runtimeEnv := range defaults {
		if os.Getenv(runtimeEnv.Key) != "" {
			continue
		}
		_ = os.Setenv(runtimeEnv.Key, runtimeEnv.Value)
		changed = append(changed, runtimeEnv)
	}
	return changed
}
