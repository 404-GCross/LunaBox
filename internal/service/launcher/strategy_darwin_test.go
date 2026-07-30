//go:build darwin

package launcher

import (
	"context"
	"errors"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"os"
	"path/filepath"
	"testing"
)

func tempWineBinary(t *testing.T) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "wine")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}

func TestDarwinLauncherStrategyNativeApp(t *testing.T) {
	game := &models.Game{Path: "/Applications/Game.app"}
	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, &appconf.AppConfig{})
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}

	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.DetectionMode != DetectionLauncherOnly {
		t.Fatalf("expected launcher-only detection, got %v", plan.DetectionMode)
	}
	if plan.ActiveTrack.Kind != ActiveTrackBundlePath || plan.ActiveTrack.BundlePath != game.Path {
		t.Fatalf("unexpected active track: %+v", plan.ActiveTrack)
	}
}

func TestDarwinLauncherStrategySteamNativePlan(t *testing.T) {
	steamExecutable := tempWineBinary(t)
	gameDirectory := t.TempDir()
	game := &models.Game{
		Path:            gameDirectory,
		GameDirectory:   gameDirectory,
		LaunchMode:      "steam",
		SteamLaunchKind: "native",
		SteamLaunchID:   "123456",
	}

	if !SupportsSteamLaunch(game, LaunchOptions{}) {
		t.Fatal("expected native Steam game to be supported on macOS")
	}
	plan, err := (steamDarwinStrategy{steamExecutable: steamExecutable}).Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan Steam launch: %v", err)
	}
	if plan.File != steamExecutable {
		t.Fatalf("expected Steam executable %q, got %q", steamExecutable, plan.File)
	}
	if len(plan.Args) != 2 || plan.Args[0] != "-silent" || plan.Args[1] != "steam://rungameid/123456" {
		t.Fatalf("unexpected Steam arguments: %#v", plan.Args)
	}
	if plan.DetectionMode != DetectionSteamDirectory || plan.DetectionDir != gameDirectory {
		t.Fatalf("unexpected Steam detection plan: %+v", plan)
	}
}

func TestDarwinLauncherStrategyRejectsSteamShortcut(t *testing.T) {
	game := &models.Game{
		Path:            t.TempDir(),
		LaunchMode:      "steam",
		SteamLaunchKind: "shortcut",
		SteamLaunchID:   "123456",
	}

	if SupportsSteamLaunch(game, LaunchOptions{}) {
		t.Fatal("expected non-Steam shortcut to remain unsupported on macOS")
	}
	_, err := SelectLauncherStrategy(game, LaunchOptions{}, &appconf.AppConfig{})
	var strategyErr *StrategyError
	if !errors.As(err, &strategyErr) || strategyErr.Kind != "unsupported" {
		t.Fatalf("expected unsupported StrategyError, got %v", err)
	}
}

func TestDarwinLauncherStrategyExeRequiresWineRunner(t *testing.T) {
	game := &models.Game{Path: "/tmp/Game.exe"}
	_, err := SelectLauncherStrategy(game, LaunchOptions{}, &appconf.AppConfig{})
	var strategyErr *StrategyError
	if !errors.As(err, &strategyErr) {
		t.Fatalf("expected StrategyError, got %v", err)
	}
	if strategyErr.Kind != "missing-config" || strategyErr.ConfigKey != "wine_runner" {
		t.Fatalf("unexpected error metadata: %+v", strategyErr)
	}
}

func TestDarwinLauncherStrategyWineSystemPlan(t *testing.T) {
	winePath := tempWineBinary(t)
	game := &models.Game{
		Path:       "/Users/u/games/Game.exe",
		WineRunner: "system",
		WineArgs:   "--no-d3d11 -windowed",
	}
	cfg := &appconf.AppConfig{
		WineRunnerPath: winePath,
		WinePrefix:     "/Users/u/.wine_default",
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
	if got, want := plan.Args, []string{game.Path, "--no-d3d11", "-windowed"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("unexpected args: %#v", got)
	}
	if plan.Dir != filepath.Dir(game.Path) {
		t.Fatalf("unexpected dir: %s", plan.Dir)
	}
	if plan.DetectionMode != DetectionLauncherOnly || plan.ActiveTrack.Kind != ActiveTrackWineRootPID {
		t.Fatalf("unexpected detection/track: %v %+v", plan.DetectionMode, plan.ActiveTrack)
	}
	if plan.ActiveTrack.ExecutablePath != game.Path || plan.ActiveTrack.Bottle != "" {
		t.Fatalf("unexpected Wine target identity: %+v", plan.ActiveTrack)
	}
	assertEnvContains(t, plan.Env, "WINEDEBUG=-all")
	assertEnvContains(t, plan.Env, "WINEPREFIX=/Users/u/.wine_default")
}

func TestDarwinLauncherStrategyWineCrossoverPlan(t *testing.T) {
	winePath := tempWineBinary(t)
	game := &models.Game{
		Path:       "/Users/u/games/Game.exe",
		WineRunner: "crossover",
		WinePrefix: "Bottle",
	}
	cfg := &appconf.AppConfig{WineRunnerPath: winePath}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, cfg)
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	assertEnvContains(t, plan.Env, "WINEDEBUG=-all")
	assertEnvContains(t, plan.Env, "CX_BOTTLE=Bottle")
	assertEnvNotContainsPrefix(t, plan.Env, "WINEPREFIX=")
	if plan.ActiveTrack.ExecutablePath != game.Path || plan.ActiveTrack.Bottle != "Bottle" {
		t.Fatalf("unexpected CrossOver target identity: %+v", plan.ActiveTrack)
	}
}

func TestDarwinLauncherStrategyWineMissingBinary(t *testing.T) {
	game := &models.Game{Path: "/tmp/Game.exe", WineRunner: "system"}
	cfg := &appconf.AppConfig{WineRunnerPath: filepath.Join(t.TempDir(), "missing-wine")}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, cfg)
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

func TestDarwinLauncherStrategyCrossoverRejectsAppPath(t *testing.T) {
	game := &models.Game{Path: "/tmp/Game.exe", WineRunner: "crossover"}
	cfg := &appconf.AppConfig{WineRunnerPath: "/Applications/CrossOver.app"}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, cfg)
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	_, err = strategy.Plan(context.Background(), game, LaunchOptions{})
	var strategyErr *StrategyError
	if !errors.As(err, &strategyErr) {
		t.Fatalf("expected StrategyError, got %v", err)
	}
	if strategyErr.Kind != "invalid-config" || strategyErr.ConfigKey != "wine_runner_path" {
		t.Fatalf("unexpected error metadata: %+v", strategyErr)
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

func assertEnvNotContainsPrefix(t *testing.T, env []string, prefix string) {
	t.Helper()
	for _, item := range env {
		if len(item) >= len(prefix) && item[:len(prefix)] == prefix {
			t.Fatalf("expected env not to contain prefix %q, got %#v", prefix, env)
		}
	}
}
