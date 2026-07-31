//go:build linux

package launcher

import (
	"context"
	"errors"
	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"os"
	"path/filepath"
	"testing"
)

func tempLinuxExecutable(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLinuxLauncherStrategyExeDefaultsToConfiguredWine(t *testing.T) {
	winePath := tempLinuxExecutable(t, "wine")
	game := &models.Game{
		Path:     "/home/u/games/Game.exe",
		WineArgs: "--no-d3d11 -windowed",
	}
	cfg := &appconf.AppConfig{
		WineRunnerPath: winePath,
		WinePrefix:     "/home/u/.wine_lunabox",
	}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, cfg)
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if plan.File != winePath {
		t.Fatalf("expected wine path %q, got %q", winePath, plan.File)
	}
	assertStringSliceEqual(t, plan.Args, []string{game.Path, "--no-d3d11", "-windowed"})
	if plan.Dir != filepath.Dir(game.Path) {
		t.Fatalf("unexpected dir: %s", plan.Dir)
	}
	if plan.DetectionMode != DetectionLauncherOnly {
		t.Fatalf("expected launcher-only detection, got %v", plan.DetectionMode)
	}
	assertEnvContains(t, plan.Env, "WINEDEBUG=-all")
	assertEnvContains(t, plan.Env, "WINEPREFIX=/home/u/.wine_lunabox")
}

func TestLinuxLauncherStrategyExeDefaultsToPathWine(t *testing.T) {
	winePath := tempLinuxExecutable(t, "wine")
	t.Setenv("PATH", filepath.Dir(winePath))
	game := &models.Game{Path: "/home/u/games/Game.exe"}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, &appconf.AppConfig{})
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if plan.File != winePath {
		t.Fatalf("expected PATH wine %q, got %q", winePath, plan.File)
	}
	assertStringSliceEqual(t, plan.Args, []string{game.Path})
}

func TestLinuxLauncherStrategyExeReportsMissingWinePath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	game := &models.Game{Path: "/home/u/games/Game.exe"}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, &appconf.AppConfig{})
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	_, err = strategy.Plan(context.Background(), game, LaunchOptions{})
	var strategyErr *StrategyError
	if !errors.As(err, &strategyErr) {
		t.Fatalf("expected StrategyError, got %v", err)
	}
	if strategyErr.Kind != "missing-config" || strategyErr.ConfigKey != "wine_runner_path" {
		t.Fatalf("unexpected error metadata: %+v", strategyErr)
	}
}

func TestLinuxSteamStrategyUsesSteamLaunchID(t *testing.T) {
	steamPath := tempLinuxExecutable(t, "steam")
	t.Setenv("PATH", filepath.Dir(steamPath))
	installDir := t.TempDir()
	game := &models.Game{
		Path:          installDir,
		GameDirectory: installDir,
		LaunchMode:    enums.LaunchModeSteam,
		SteamLaunchID: "123456",
	}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, &appconf.AppConfig{})
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if plan.File != steamPath {
		t.Fatalf("expected steam path %q, got %q", steamPath, plan.File)
	}
	assertStringSliceEqual(t, plan.Args, []string{"steam://rungameid/123456"})
	if plan.DetectionMode != DetectionSteamDirectory {
		t.Fatalf("expected Steam directory detection, got %v", plan.DetectionMode)
	}
}

func assertEnvContains(t *testing.T, env []string, expected string) {
	t.Helper()
	for _, item := range env {
		if item == expected {
			return
		}
	}
	t.Fatalf("expected env to contain %q, got %#v", expected, env)
}

func assertStringSliceEqual(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected %#v, got %#v", want, got)
		}
	}
}
