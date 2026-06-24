package utils

import (
	"os"
	"testing"
)

func TestLoadEnvFilesIfExists(t *testing.T) {
	const (
		fromBuildKey  = "LUNABOX_TEST_FROM_BUILD"
		fromDotEnvKey = "LUNABOX_TEST_FROM_DOTENV"
		sharedKey     = "LUNABOX_TEST_SHARED"
		existingKey   = "LUNABOX_TEST_EXISTING"
	)
	preserveEnvKeys(t, fromBuildKey, fromDotEnvKey, sharedKey, existingKey)

	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile(".env.build", []byte(`
LUNABOX_TEST_FROM_BUILD='build-value'
LUNABOX_TEST_SHARED=build-shared
LUNABOX_TEST_EXISTING=file-value
`), 0o600); err != nil {
		t.Fatalf("写入 .env.build 失败: %v", err)
	}
	if err := os.WriteFile(".env", []byte(`
LUNABOX_TEST_FROM_DOTENV=dotenv-value
LUNABOX_TEST_SHARED=dotenv-shared
`), 0o600); err != nil {
		t.Fatalf("写入 .env 失败: %v", err)
	}
	if err := os.Setenv(existingKey, "existing-value"); err != nil {
		t.Fatalf("设置测试环境变量失败: %v", err)
	}

	if err := LoadEnvFilesIfExists("missing.env", ".env.build", ".env"); err != nil {
		t.Fatalf("加载 env 文件失败: %v", err)
	}

	assertEnvValue(t, fromBuildKey, "build-value")
	assertEnvValue(t, fromDotEnvKey, "dotenv-value")
	assertEnvValue(t, sharedKey, "build-shared")
	assertEnvValue(t, existingKey, "existing-value")
}

func preserveEnvKeys(t *testing.T, keys ...string) {
	t.Helper()

	type envValue struct {
		value  string
		exists bool
	}
	previous := make(map[string]envValue, len(keys))
	for _, key := range keys {
		value, exists := os.LookupEnv(key)
		previous[key] = envValue{value: value, exists: exists}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("清理测试环境变量 %s 失败: %v", key, err)
		}
	}

	t.Cleanup(func() {
		for _, key := range keys {
			value := previous[key]
			if value.exists {
				_ = os.Setenv(key, value.value)
				continue
			}
			_ = os.Unsetenv(key)
		}
	})
}

func assertEnvValue(t *testing.T, key, expected string) {
	t.Helper()

	if got := os.Getenv(key); got != expected {
		t.Fatalf("%s = %q, want %q", key, got, expected)
	}
}
