//go:build linux

package platform

import (
	"os"
	"testing"
)

func TestConfigureRuntimeEnvironmentSetsWebKitDefaults(t *testing.T) {
	keys := []string{
		"WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS",
		"WEBKIT_DISABLE_COMPOSITING_MODE",
		"WEBKIT_DISABLE_DMABUF_RENDERER",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}

	changed := ConfigureRuntimeEnvironment()

	if len(changed) != len(keys) {
		t.Fatalf("changed count = %d, want %d", len(changed), len(keys))
	}
	for i, key := range keys {
		if got := os.Getenv(key); got != "1" {
			t.Fatalf("%s = %q, want 1", key, got)
		}
		if changed[i].Key != key {
			t.Fatalf("changed[%d].Key = %q, want %s", i, changed[i].Key, key)
		}
	}
}

func TestConfigureRuntimeEnvironmentDoesNotOverrideExplicitWebKitEnv(t *testing.T) {
	keys := []string{
		"WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS",
		"WEBKIT_DISABLE_COMPOSITING_MODE",
		"WEBKIT_DISABLE_DMABUF_RENDERER",
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
