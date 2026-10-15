//go:build linux

package platform

import (
	"os"
	"runtime"
	"strings"
)

const linuxWebKitModeEnv = "LUNABOX_WEBKIT_MODE"

type RuntimeEnvironment struct {
	Key    string
	Value  string
	Reason string
}

func ConfigureRuntimeEnvironment() []RuntimeEnvironment {
	return configureRuntimeEnvironment(runtime.GOARCH, os.Getenv(linuxWebKitModeEnv))
}

func configureRuntimeEnvironment(goarch string, webKitMode string) []RuntimeEnvironment {
	if defaultLinuxWebKitMode(goarch, webKitMode) != "safe" {
		return nil
	}
	changed := setDefaultRuntimeEnvironment(linuxArm64SafeRuntimeEnvironment())
	changed = append(changed, setRuntimeEnvironment(linuxArm64ForcedMesaEnvironment())...)
	return changed
}

func defaultLinuxWebKitMode(goarch string, webKitMode string) string {
	mode := strings.ToLower(strings.TrimSpace(webKitMode))
	if mode == "native" {
		return "native"
	}
	if goarch == "arm64" {
		return "safe"
	}
	return "native"
}

func linuxArm64SafeRuntimeEnvironment() []RuntimeEnvironment {
	return []RuntimeEnvironment{
		{
			Key:    "WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS",
			Value:  "1",
			Reason: "avoid WebKitGTK bubblewrap/dbus-proxy aborts on Linux arm64 sessions",
		},
		{
			Key:    "WEBKIT_DISABLE_COMPOSITING_MODE",
			Value:  "1",
			Reason: "avoid WebKitGTK accelerated compositing EGL initialization aborts on Linux arm64",
		},
		{
			Key:    "WEBKIT_DISABLE_DMABUF_RENDERER",
			Value:  "1",
			Reason: "avoid WebKitGTK DMABUF renderer EGL initialization aborts on Linux arm64",
		},
		{
			Key:    "LIBGL_ALWAYS_SOFTWARE",
			Value:  "1",
			Reason: "force software OpenGL for Linux arm64 WebKitGTK stability",
		},
	}
}

func linuxArm64ForcedMesaEnvironment() []RuntimeEnvironment {
	return []RuntimeEnvironment{
		{
			Key:    "MESA_LOADER_DRIVER_OVERRIDE",
			Value:  "llvmpipe",
			Reason: "avoid WebKitGTK Zink/EGL initialization aborts on Linux arm64",
		},
		{
			Key:    "GALLIUM_DRIVER",
			Value:  "llvmpipe",
			Reason: "avoid WebKitGTK Zink/EGL initialization aborts on Linux arm64",
		},
	}
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

func setRuntimeEnvironment(vars []RuntimeEnvironment) []RuntimeEnvironment {
	changed := make([]RuntimeEnvironment, 0, len(vars))
	for _, runtimeEnv := range vars {
		if os.Getenv(runtimeEnv.Key) == runtimeEnv.Value {
			continue
		}
		_ = os.Setenv(runtimeEnv.Key, runtimeEnv.Value)
		changed = append(changed, runtimeEnv)
	}
	return changed
}
