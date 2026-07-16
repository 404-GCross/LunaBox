//go:build windows

package launcher

import (
	"context"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsLauncherStrategyNativePlan(t *testing.T) {
	game := &models.Game{Path: `C:\Games\Game.exe`}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, &appconf.AppConfig{})
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if plan.File != game.Path {
		t.Fatalf("expected file %q, got %q", game.Path, plan.File)
	}
	if plan.Dir != filepath.Dir(game.Path) || plan.DetectionDir != filepath.Dir(game.Path) {
		t.Fatalf("unexpected dirs: dir=%q detection=%q", plan.Dir, plan.DetectionDir)
	}
	if plan.DetectionMode != DetectionStaged {
		t.Fatalf("expected staged detection, got %v", plan.DetectionMode)
	}
}

func TestWindowsLauncherStrategyUsesGameDirectoryForDetection(t *testing.T) {
	gameDirectory := t.TempDir()
	launchDirectory := filepath.Join(gameDirectory, "launcher", "bin")
	if err := os.MkdirAll(launchDirectory, 0o755); err != nil {
		t.Fatalf("create launch directory: %v", err)
	}
	game := &models.Game{
		Path:          filepath.Join(launchDirectory, "Game.exe"),
		GameDirectory: gameDirectory,
	}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, &appconf.AppConfig{})
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if plan.Dir != launchDirectory {
		t.Fatalf("expected launch dir %q, got %q", launchDirectory, plan.Dir)
	}
	if plan.DetectionDir != gameDirectory {
		t.Fatalf("expected detection dir %q, got %q", gameDirectory, plan.DetectionDir)
	}
}

func TestWindowsLauncherStrategyLocaleEmulatorPlan(t *testing.T) {
	gameDirectory := t.TempDir()
	launchDirectory := filepath.Join(gameDirectory, "bin")
	if err := os.MkdirAll(launchDirectory, 0o755); err != nil {
		t.Fatalf("create launch directory: %v", err)
	}
	game := &models.Game{
		Path:              filepath.Join(launchDirectory, "Game.exe"),
		GameDirectory:     gameDirectory,
		UseLocaleEmulator: true,
	}
	cfg := &appconf.AppConfig{LocaleEmulatorPath: `C:\Tools\LEProc.exe`}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, cfg)
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if plan.File != cfg.LocaleEmulatorPath {
		t.Fatalf("expected LE file %q, got %q", cfg.LocaleEmulatorPath, plan.File)
	}
	if len(plan.Args) != 1 || plan.Args[0] != game.Path {
		t.Fatalf("unexpected args: %#v", plan.Args)
	}
	if plan.Dir != launchDirectory {
		t.Fatalf("expected launch dir %q, got %q", launchDirectory, plan.Dir)
	}
	if plan.DetectionDir != gameDirectory {
		t.Fatalf("expected detection dir %q, got %q", gameDirectory, plan.DetectionDir)
	}
	if plan.DetectionMode != DetectionStaged {
		t.Fatalf("expected staged detection, got %v", plan.DetectionMode)
	}
}

func TestEffectiveProcessDetectionDirFallsBackForUnrelatedDirectory(t *testing.T) {
	gameDirectory := t.TempDir()
	launchDirectory := filepath.Join(t.TempDir(), "bin")

	if got := EffectiveProcessDetectionDir(gameDirectory, launchDirectory); got != launchDirectory {
		t.Fatalf("expected unrelated game directory to fall back to %q, got %q", launchDirectory, got)
	}
	if got := EffectiveProcessDetectionDir(filepath.Join(t.TempDir(), "missing"), launchDirectory); got != launchDirectory {
		t.Fatalf("expected missing game directory to fall back to %q, got %q", launchDirectory, got)
	}
}

func TestWindowsLauncherStrategyAdminPlan(t *testing.T) {
	admin := true
	game := &models.Game{Path: `C:\Games\Game.exe`}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{RunAsAdmin: &admin}, &appconf.AppConfig{})
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{RunAsAdmin: &admin})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !plan.RunAsAdmin {
		t.Fatalf("expected RunAsAdmin=true")
	}
}
