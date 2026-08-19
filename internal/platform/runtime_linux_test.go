//go:build linux

package platform

import (
	"os"
	"testing"
)

func TestConfigureRuntimeEnvironmentSetsWebKitDefaults(t *testing.T) {
	defaults := []RuntimeEnvironment{
		{Key: "WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", Value: "1"},
		{Key: "WEBKIT_DISABLE_COMPOSITING_MODE", Value: "1"},
		{Key: "WEBKIT_DISABLE_DMABUF_RENDERER", Value: "1"},
		{Key: "GDK_DISABLE", Value: "dmabuf"},
		{Key: "GSK_RENDERER", Value: "cairo"},
	}
	for _, runtimeEnv := range defaults {
		t.Setenv(runtimeEnv.Key, "")
	}

	changed := ConfigureRuntimeEnvironment()

	if len(changed) != len(defaults) {
		t.Fatalf("changed count = %d, want %d", len(changed), len(defaults))
	}
	for i, runtimeEnv := range defaults {
		if got := os.Getenv(runtimeEnv.Key); got != runtimeEnv.Value {
			t.Fatalf("%s = %q, want %q", runtimeEnv.Key, got, runtimeEnv.Value)
		}
		if changed[i].Key != runtimeEnv.Key {
			t.Fatalf("changed[%d].Key = %q, want %s", i, changed[i].Key, runtimeEnv.Key)
		}
	}
}

func TestConfigureRuntimeEnvironmentDoesNotOverrideExplicitWebKitEnv(t *testing.T) {
	keys := []string{
		"WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS",
		"WEBKIT_DISABLE_COMPOSITING_MODE",
		"WEBKIT_DISABLE_DMABUF_RENDERER",
		"GDK_DISABLE",
		"GSK_RENDERER",
	}
	for _, key := range keys {
		t.Setenv(key, "custom")
	}

	changed := ConfigureRuntimeEnvironment()

	for _, key := range keys {
		if got := os.Getenv(key); got != "custom" {
			t.Fatalf("%s = %q, want custom", key, got)
		}
	}
	if len(changed) != 0 {
		t.Fatalf("changed count = %d, want 0", len(changed))
	}
}
