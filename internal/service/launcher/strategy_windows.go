//go:build windows

package launcher

import (
	"context"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

type nativeWindowsStrategy struct {
	cfg *appconf.AppConfig
}

type localeEmulatorStrategy struct {
	cfg *appconf.AppConfig
}

type steamWindowsStrategy struct{}

func selectPlatformLauncherStrategy(game *models.Game, opts LaunchOptions, cfg *appconf.AppConfig) (LauncherStrategy, error) {
	if ShouldUseSteamLaunch(game, opts) {
		return steamWindowsStrategy{}, nil
	}
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
	launchDir := filepath.Dir(path)
	plan := buildStagedWindowsPlan(path, nil, launchDir, filepath.Base(path), useMagpie, runAsAdmin)
	plan.DetectionDir = EffectiveProcessDetectionDir(game.GameDirectory, launchDir)
	return plan, nil
}

func (s localeEmulatorStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	useMagpie := EffectiveBool(opts.UseMagpie, game.UseMagpie)
	runAsAdmin := EffectiveBool(opts.RunAsAdmin, false)
	path := game.Path
	launchDir := filepath.Dir(path)
	plan := buildStagedWindowsPlan(s.cfg.LocaleEmulatorPath, []string{path}, launchDir, filepath.Base(s.cfg.LocaleEmulatorPath), useMagpie, runAsAdmin)
	plan.DetectionDir = EffectiveProcessDetectionDir(game.GameDirectory, launchDir)
	return plan, nil
}

func (s steamWindowsStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	if !isSteamLaunchSource(game.SourceType) || strings.TrimSpace(game.SourceID) == "" {
		return LaunchPlan{}, fmt.Errorf("Steam launch requires a Steam source and launch id")
	}

	steamPath, err := findSteamInstallPath()
	if err != nil {
		return LaunchPlan{}, err
	}
	steamExe := filepath.Join(steamPath, "steam.exe")
	if info, err := os.Stat(steamExe); err != nil || info.IsDir() {
		return LaunchPlan{}, fmt.Errorf("未找到 steam.exe: %s", steamExe)
	}

	installDir := strings.TrimSpace(game.Path)
	if installDir == "" {
		return LaunchPlan{}, fmt.Errorf("Steam 启动需要游戏安装目录用于进程检测")
	}
	detectionDir := EffectiveProcessDetectionDir(game.GameDirectory, installDir)

	return LaunchPlan{
		File:          steamExe,
		Args:          []string{"-silent", "steam://rungameid/" + strings.TrimSpace(game.SourceID)},
		Dir:           steamPath,
		DetectionDir:  detectionDir,
		DetectionMode: DetectionSteamDirectory,
		DisplayName:   "steam.exe",
		Magpie:        EffectiveBool(opts.UseMagpie, game.UseMagpie),
		RunAsAdmin:    false,
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackDefault,
		},
	}, nil
}

func isSteamLaunchSource(source enums.SourceType) bool {
	return source == enums.Steam || source == enums.SteamShortcut
}

func findSteamInstallPath() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("未找到 Steam 安装信息: %w", err)
	}
	defer key.Close()

	for _, valueName := range []string{"SteamPath", "InstallPath"} {
		value, _, err := key.GetStringValue(valueName)
		if err != nil || strings.TrimSpace(value) == "" {
			continue
		}
		path, err := filepath.Abs(filepath.Clean(strings.ReplaceAll(value, "/", string(os.PathSeparator))))
		if err != nil {
			continue
		}
		if info, err := os.Stat(filepath.Join(path, "steam.exe")); err == nil && !info.IsDir() {
			return path, nil
		}
	}

	return "", fmt.Errorf("未找到有效的 Steam 安装目录")
}
