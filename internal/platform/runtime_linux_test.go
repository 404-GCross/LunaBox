//go:build linux

package platform

import (
	"os"
	"testing"
)

func TestConfigureRuntimeEnvironmentSetsLinuxArm64WebKitDefaults(t *testing.T) {
	keys := linuxArm64SafeRuntimeEnvironmentKeys()
	for _, key := range keys {
		t.Setenv(key, "")
	}

	changed := configureRuntimeEnvironment("arm64", "")

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

func TestConfigureRuntimeEnvironmentLeavesLinuxAmd64Native(t *testing.T) {
	keys := linuxArm64SafeRuntimeEnvironmentKeys()
	for _, key := range keys {
		t.Setenv(key, "")
	}

	changed := configureRuntimeEnvironment("amd64", "")

	if len(changed) != 0 {
		t.Fatalf("changed count = %d, want 0", len(changed))
	}
	for _, key := range keys {
		if got := os.Getenv(key); got != "" {
			t.Fatalf("%s = %q, want empty", key, got)
		}
	}
}

func TestConfigureRuntimeEnvironmentAllowsLinuxArm64NativeMode(t *testing.T) {
	keys := linuxArm64SafeRuntimeEnvironmentKeys()
	for _, key := range keys {
		t.Setenv(key, "")
	}

	changed := configureRuntimeEnvironment("arm64", "native")

	if len(changed) != 0 {
		t.Fatalf("changed count = %d, want 0", len(changed))
	}
	for _, key := range keys {
		if got := os.Getenv(key); got != "" {
			t.Fatalf("%s = %q, want empty", key, got)
		}
	}
}

func TestConfigureRuntimeEnvironmentDoesNotOverrideExplicitLinuxArm64WebKitEnv(t *testing.T) {
	keys := linuxArm64SafeRuntimeEnvironmentKeys()
	for _, key := range keys {
		t.Setenv(key, "custom")
	}

	changed := configureRuntimeEnvironment("arm64", "")

	for _, key := range keys {
		if got := os.Getenv(key); got != "custom" {
			t.Fatalf("%s = %q, want custom", key, got)
		}
	}
	if len(changed) != 0 {
		t.Fatalf("changed count = %d, want 0", len(changed))
	}
}

func TestConfigureRuntimeEnvironmentClearsLinuxArm64MesaDriverOverrides(t *testing.T) {
	for _, key := range linuxArm64SafeRuntimeEnvironmentKeys() {
		t.Setenv(key, "custom")
	}
	for _, key := range linuxArm64ConflictingMesaEnvironmentKeys() {
		t.Setenv(key, "kgsl")
	}

	changed := configureRuntimeEnvironment("arm64", "")

	if len(changed) != len(linuxArm64ConflictingMesaEnvironmentKeys()) {
		t.Fatalf("changed count = %d, want %d", len(changed), len(linuxArm64ConflictingMesaEnvironmentKeys()))
	}
	for i, key := range linuxArm64ConflictingMesaEnvironmentKeys() {
		if _, ok := os.LookupEnv(key); ok {
			t.Fatalf("%s should be unset", key)
		}
		if changed[i].Key != key {
			t.Fatalf("changed[%d].Key = %q, want %s", i, changed[i].Key, key)
		}
	}
}

func TestConfigureRuntimeEnvironmentKeepsMesaDriverOverridesInNativeMode(t *testing.T) {
	for _, key := range linuxArm64SafeRuntimeEnvironmentKeys() {
		t.Setenv(key, "")
	}
	for _, key := range linuxArm64ConflictingMesaEnvironmentKeys() {
		t.Setenv(key, "kgsl")
	}

	changed := configureRuntimeEnvironment("arm64", "native")

	if len(changed) != 0 {
		t.Fatalf("changed count = %d, want 0", len(changed))
	}
	for _, key := range linuxArm64ConflictingMesaEnvironmentKeys() {
		if got := os.Getenv(key); got != "kgsl" {
			t.Fatalf("%s = %q, want kgsl", key, got)
		}
	}
}

func linuxArm64SafeRuntimeEnvironmentKeys() []string {
	runtimeEnv := linuxArm64SafeRuntimeEnvironment()
	keys := make([]string, 0, len(runtimeEnv))
	for _, item := range runtimeEnv {
		keys = append(keys, item.Key)
	}
	return keys
}

func linuxArm64ConflictingMesaEnvironmentKeys() []string {
	runtimeEnv := linuxArm64ConflictingMesaEnvironment()
	keys := make([]string, 0, len(runtimeEnv))
	for _, item := range runtimeEnv {
		keys = append(keys, item.Key)
	}
	return keys
}
