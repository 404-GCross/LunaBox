package localeutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const DefaultLocale = "ja_JP.SJIS"

type LocaleSpec struct {
	Name    string
	Input   string
	Charmap string
}

type LaunchEnvironment struct {
	Locale  string
	LOCPATH string
}

var supportedLocales = []LocaleSpec{
	{Name: "ja_JP.SJIS", Input: "ja_JP", Charmap: "SHIFT_JIS"},
	{Name: "ja_JP.UTF-8", Input: "ja_JP", Charmap: "UTF-8"},
	{Name: "ja_JP.EUC-JP", Input: "ja_JP", Charmap: "EUC-JP"},
	{Name: "zh_CN.UTF-8", Input: "zh_CN", Charmap: "UTF-8"},
	{Name: "zh_CN.GB2312", Input: "zh_CN", Charmap: "GB2312"},
	{Name: "zh_CN.GBK", Input: "zh_CN", Charmap: "GBK"},
	{Name: "zh_CN.GB18030", Input: "zh_CN", Charmap: "GB18030"},
	{Name: "zh_HK.UTF-8", Input: "zh_HK", Charmap: "UTF-8"},
	{Name: "zh_HK.BIG5", Input: "zh_HK", Charmap: "BIG5"},
	{Name: "zh_TW.EUC-TW", Input: "zh_TW", Charmap: "EUC-TW"},
	{Name: "zh_TW.UTF-8", Input: "zh_TW", Charmap: "UTF-8"},
	{Name: "zh_TW.BIG5", Input: "zh_TW", Charmap: "BIG5"},
}

func SupportedLocales() []LocaleSpec {
	out := make([]LocaleSpec, len(supportedLocales))
	copy(out, supportedLocales)
	return out
}

func NormalizeLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return DefaultLocale
	}
	for _, spec := range supportedLocales {
		if strings.EqualFold(spec.Name, locale) {
			return spec.Name
		}
	}
	return DefaultLocale
}

func NormalizeLocaleForStorage(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return ""
	}
	return NormalizeLocale(locale)
}

func Spec(locale string) LocaleSpec {
	normalized := NormalizeLocale(locale)
	for _, spec := range supportedLocales {
		if spec.Name == normalized {
			return spec
		}
	}
	return supportedLocales[0]
}

func BuildLaunchEnvironment(gamePath string, locale string) (LaunchEnvironment, error) {
	gamePath = strings.TrimSpace(gamePath)
	if gamePath == "" {
		return LaunchEnvironment{}, fmt.Errorf("game path is empty")
	}
	launchDir := gamePath
	if info, err := os.Stat(gamePath); err == nil {
		if !info.IsDir() {
			launchDir = filepath.Dir(gamePath)
		}
	} else if ext := filepath.Ext(gamePath); ext != "" {
		launchDir = filepath.Dir(gamePath)
	}
	if strings.TrimSpace(launchDir) == "" || launchDir == "." {
		return LaunchEnvironment{}, fmt.Errorf("game directory is empty")
	}
	return LaunchEnvironment{
		Locale:  NormalizeLocale(locale),
		LOCPATH: filepath.Join(launchDir, "DWLE"),
	}, nil
}

func (e LaunchEnvironment) Env() []string {
	if strings.TrimSpace(e.Locale) == "" || strings.TrimSpace(e.LOCPATH) == "" {
		return nil
	}
	return []string{
		"LOCPATH=" + e.LOCPATH,
		"LANG=" + e.Locale,
	}
}

func (e LaunchEnvironment) SteamLaunchOptions() string {
	if strings.TrimSpace(e.Locale) == "" || strings.TrimSpace(e.LOCPATH) == "" {
		return "%command%"
	}
	return fmt.Sprintf("LOCPATH=%s LANG=%s %%command%%", shellQuote(e.LOCPATH), shellQuote(e.Locale))
}

func shellQuote(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "''"
	}
	if isShellSafe(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func isShellSafe(value string) bool {
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case strings.ContainsRune("@%_+=:,./-", r):
		default:
			return false
		}
	}
	return true
}
