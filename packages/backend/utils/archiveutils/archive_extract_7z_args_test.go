package archiveutils

import (
	"fmt"
	"runtime"
	"slices"
	"testing"
)

func TestBuild7zExtractArgs(t *testing.T) {
	got := build7zExtractArgs("source.7z", "target")
	want := []string{
		"x",
		"-y",
		"-aoa",
		fmt.Sprintf("-mmt=%d", runtime.NumCPU()),
		"-bd",
		"-otarget",
		"source.7z",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("build7zExtractArgs() = %q, want %q", got, want)
	}
}
