//go:build linux

package platform

import (
	"os"
	"testing"
)

func TestConfigureRuntimeEnvironmentSetsWebKitSandboxDefault(t *testing.T) {
	t.Setenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", "")

	changed := ConfigureRuntimeEnvironment()

	if got := os.Getenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS"); got != "1" {
		t.Fatalf("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS = %q, want 1", got)
	}
	if len(changed) != 1 {
		t.Fatalf("changed count = %d, want 1", len(changed))
	}
	if changed[0].Key != "WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS" {
		t.Fatalf("changed key = %q, want WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", changed[0].Key)
	}
}

func TestConfigureRuntimeEnvironmentDoesNotOverrideExplicitWebKitSandboxEnv(t *testing.T) {
	t.Setenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", "custom")

	changed := ConfigureRuntimeEnvironment()

	if got := os.Getenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS"); got != "custom" {
		t.Fatalf("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS = %q, want custom", got)
	}
	if len(changed) != 0 {
		t.Fatalf("changed count = %d, want 0", len(changed))
	}
}
