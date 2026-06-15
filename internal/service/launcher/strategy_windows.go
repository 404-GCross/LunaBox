//go:build windows

package launcher

import (
	"context"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"path/filepath"
)

type nativeWindowsStrategy struct {
	cfg *appconf.AppConfig
}

type localeEmulatorStrategy struct {
	cfg *appconf.AppConfig
}

func selectPlatformLauncherStrategy(game *models.Game, opts LaunchOptions, cfg *appconf.AppConfig) (LauncherStrategy, error) {
	useLE := EffectiveBool(opts.UseLocaleEmulator, game.UseLocaleEmulator)
	if useLE && cfg != nil && cfg.LocaleEmulatorPath != "" {
		return localeEmulatorStrategy{cfg: cfg}, nil
	}
	return nativeWindowsStrategy{cfg: cfg}, nil
}

func (s nativeWindowsStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	useMagpie := EffectiveBool(opts.UseMagpie, game.UseMagpie)
	runAsAdmin := EffectiveBool(opts.RunAsAdmin, false)
	path := game.Path
	return buildStagedWindowsPlan(path, nil, filepath.Dir(path), filepath.Base(path), useMagpie, runAsAdmin), nil
}

func (s localeEmulatorStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	useMagpie := EffectiveBool(opts.UseMagpie, game.UseMagpie)
	runAsAdmin := EffectiveBool(opts.RunAsAdmin, false)
	path := game.Path
	return buildStagedWindowsPlan(s.cfg.LocaleEmulatorPath, []string{path}, filepath.Dir(path), filepath.Base(s.cfg.LocaleEmulatorPath), useMagpie, runAsAdmin), nil
}
