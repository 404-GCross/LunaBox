//go:build linux

package localeutils

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingLocaledefDataReportsInputAndCharmap(t *testing.T) {
	root := t.TempDir()
	spec := LocaleSpec{Name: "ja_JP.SJIS", Input: "ja_JP", Charmap: "SHIFT_JIS"}

	missing := strings.Join(missingLocaledefData(spec, []string{root}), "\n")
	if !strings.Contains(missing, "locale 输入文件 ja_JP") {
		t.Fatalf("expected missing locale input, got %q", missing)
	}
	if !strings.Contains(missing, "字符映射 SHIFT_JIS") {
		t.Fatalf("expected missing charmap, got %q", missing)
	}
}

func TestMissingLocaledefDataAcceptsGzippedCharmap(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "locales"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "charmaps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "locales", "ja_JP"), []byte("locale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "charmaps", "SHIFT_JIS.gz"), []byte("charmap"), 0o644); err != nil {
		t.Fatal(err)
	}

	spec := LocaleSpec{Name: "ja_JP.SJIS", Input: "ja_JP", Charmap: "SHIFT_JIS"}
	if missing := missingLocaledefData(spec, []string{root}); len(missing) != 0 {
		t.Fatalf("expected all localedef data to be present, missing %v", missing)
	}
}

func TestLocaledefErrorExplainsMissingLocaleData(t *testing.T) {
	err := localedefError(
		LocaleSpec{Name: "ja_JP.SJIS"},
		errors.New("exit status 4"),
		[]byte("[error] default character map file `ANSI_X3.4-1968' not found: No such file or directory"),
	)
	message := err.Error()
	if !strings.Contains(message, "缺少 locale/charmap 数据") {
		t.Fatalf("expected user friendly missing data error, got %q", message)
	}
}
