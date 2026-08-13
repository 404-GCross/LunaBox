//go:build linux

package umbra

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
	"lunabox/internal/utils/apputils"
)

var credentialFileLocks sync.Map

type credentialFile struct {
	path string
	mu   *sync.Mutex
}

type tokenStore struct{ file *credentialFile }
type deviceStore struct{ file *credentialFile }

func newCredentialStores(cfg Config) (umbrsdk.TokenStore, umbrsdk.DeviceStore, error) {
	dir, err := credentialDir(cfg)
	if err != nil {
		return nil, nil, err
	}
	return &tokenStore{file: newCredentialFile(filepath.Join(dir, "tokens.json"))},
		&deviceStore{file: newCredentialFile(filepath.Join(dir, "device.json"))}, nil
}

func credentialDir(cfg Config) (string, error) {
	configDir, err := apputils.GetConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取 Umbra 凭据目录失败: %w", err)
	}
	identity := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/") + "\x00" + strings.TrimSpace(cfg.ClientID)
	sum := sha256.Sum256([]byte(identity))
	return filepath.Join(configDir, "umbra", hex.EncodeToString(sum[:16])), nil
}

func installIDPath() (string, error) {
	configDir, err := apputils.GetConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "umbra")
	if err := ensurePrivateDir(dir); err != nil {
		return "", err
	}
	return filepath.Join(dir, "install-id"), nil
}

func newCredentialFile(path string) *credentialFile {
	lock, _ := credentialFileLocks.LoadOrStore(path, &sync.Mutex{})
	return &credentialFile{path: path, mu: lock.(*sync.Mutex)}
}

func (s *credentialFile) load(ctx context.Context, target any) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return false, fmt.Errorf("解析 Umbra 凭据失败: %w", err)
	}
	return true, nil
}

func (s *credentialFile) save(ctx context.Context, value any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := ensurePrivateDir(dir); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return err
	}
	return os.Chmod(s.path, 0o600)
}

func (s *credentialFile) clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *tokenStore) Load(ctx context.Context) (*umbrsdk.TokenSet, error) {
	var token umbrsdk.TokenSet
	found, err := s.file.load(ctx, &token)
	if err != nil || !found {
		return nil, err
	}
	return &token, nil
}

func (s *tokenStore) Save(ctx context.Context, token *umbrsdk.TokenSet) error {
	return s.file.save(ctx, token)
}

func (s *tokenStore) Clear(ctx context.Context) error { return s.file.clear(ctx) }

func (s *deviceStore) Load(ctx context.Context) (*umbrsdk.DeviceCredentials, error) {
	var credentials umbrsdk.DeviceCredentials
	found, err := s.file.load(ctx, &credentials)
	if err != nil || !found {
		return nil, err
	}
	return &credentials, nil
}

func (s *deviceStore) Save(ctx context.Context, credentials *umbrsdk.DeviceCredentials) error {
	return s.file.save(ctx, credentials)
}

func (s *deviceStore) Clear(ctx context.Context) error { return s.file.clear(ctx) }

func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700)
}
