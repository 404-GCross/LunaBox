//go:build windows

package launcher

import (
	"context"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
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

func TestWindowsLauncherStrategyLocaleEmulatorPlan(t *testing.T) {
	game := &models.Game{Path: `C:\Games\Game.exe`, UseLocaleEmulator: true}
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
	if plan.DetectionMode != DetectionStaged {
		t.Fatalf("expected staged detection, got %v", plan.DetectionMode)
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
