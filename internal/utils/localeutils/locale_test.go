package localeutils

import (
	"strings"
	"testing"
)

func TestBuildLaunchEnvironmentDefaultsLocaleAndQuotesSteamOptions(t *testing.T) {
	env, err := BuildLaunchEnvironment("/home/u/Games/My Game/game.exe", "")
	if err != nil {
		t.Fatalf("build launch environment: %v", err)
	}
	if env.Locale != DefaultLocale {
		t.Fatalf("expected default locale %q, got %q", DefaultLocale, env.Locale)
	}
	if env.LOCPATH != "/home/u/Games/My Game/DWLE" {
		t.Fatalf("unexpected LOCPATH: %q", env.LOCPATH)
	}

	options := env.SteamLaunchOptions()
	if !strings.Contains(options, "LOCPATH='/home/u/Games/My Game/DWLE'") {
		t.Fatalf("expected quoted LOCPATH in %q", options)
	}
	if !strings.Contains(options, "LANG=ja_JP.SJIS") || !strings.HasSuffix(options, " %command%") {
		t.Fatalf("unexpected Steam launch options: %q", options)
	}
}

func TestNormalizeLocaleFallsBackToDefault(t *testing.T) {
	if got := NormalizeLocale("zh_cn.gbk"); got != "zh_CN.GBK" {
		t.Fatalf("expected canonical zh_CN.GBK, got %q", got)
	}
	if got := NormalizeLocale("unknown"); got != DefaultLocale {
		t.Fatalf("expected default locale for unknown value, got %q", got)
	}
}
