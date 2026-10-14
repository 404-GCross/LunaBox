package service

import (
	"path/filepath"
	goruntime "runtime"
	"testing"
)

func TestBatchImportInitialDirectory(t *testing.T) {
	existingDirectory := t.TempDir()
	if got := batchImportInitialDirectory(existingDirectory, goruntime.GOOS); got != filepath.Clean(existingDirectory) {
		t.Fatalf("expected existing directory %q, got %q", existingDirectory, got)
	}

	if got := batchImportInitialDirectory(filepath.Join(existingDirectory, "missing"), goruntime.GOOS); got != "" {
		t.Fatalf("expected missing directory to be rejected, got %q", got)
	}

	if got := batchImportInitialDirectory(`/Users/example/Games`, "windows"); got != "" {
		t.Fatalf("expected Unix directory to be rejected on Windows, got %q", got)
	}

	if got := batchImportInitialDirectory(`C:\Games`, "darwin"); got != "" {
		t.Fatalf("expected Windows directory to be rejected on macOS, got %q", got)
	}
}
