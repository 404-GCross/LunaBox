//go:build windows

package umbra

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
)

func TestTokenStoreProtectsAndRestoresCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.bin")
	store := &tokenStore{file: newProtectedFile(path)}
	want := &umbrsdk.TokenSet{
		AccessToken:  "access-secret",
		RefreshToken: "refresh-secret",
		TokenType:    "bearer",
	}

	if err := store.Save(context.Background(), want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if bytes.Contains(raw, []byte(want.RefreshToken)) {
		t.Fatal("credential file contains the plaintext refresh token")
	}

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got == nil || got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}

	if err := store.Clear(context.Background()); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	got, err = store.Load(context.Background())
	if err != nil || got != nil {
		t.Fatalf("Load() after Clear = %#v, %v", got, err)
	}
}
