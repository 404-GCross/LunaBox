//go:build darwin

package launcher

import (
	"context"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"os"
	"path/filepath"
	"strings"
)

const (
	wineRunnerSystem    = "system"
	wineRunnerCrossover = "crossover"
	wineRunnerCustom    = "custom"
)

type nativeAppStrategy struct{}
type nativeExecutableStrategy struct{}
type wineSystemStrategy struct {
	cfg *appconf.AppConfig
}
type wineCrossoverStrategy struct {
	cfg *appconf.AppConfig
}

func selectPlatformLauncherStrategy(game *models.Game, opts LaunchOptions, cfg *appconf.AppConfig) (LauncherStrategy, error) {
	path := strings.TrimSpace(game.Path)
	ext := strings.ToLower(filepath.Ext(path))
	wineRunner := EffectiveString(opts.WineRunner, game.WineRunner)

	if ext == ".app" {
		if wineRunner != "" {
			return nil, newStrategyError("invalid-config", "wine_runner", "原生 macOS 应用不应启用 Wine 启动器", fmt.Sprintf("path=%s wine_runner=%s", path, wineRunner))
		}
		return nativeAppStrategy{}, nil
	}

	if ext == ".exe" || ext == ".bat" {
		switch wineRunner {
		case "":
			return nil, newStrategyError("missing-config", "wine_runner", "该游戏需要在 macOS 上启用 Wine 启动器", fmt.Sprintf("path=%s", path))
		case wineRunnerCrossover:
			return wineCrossoverStrategy{cfg: cfg}, nil
		case wineRunnerSystem, wineRunnerCustom:
			return wineSystemStrategy{cfg: cfg}, nil
		default:
			return nil, newStrategyError("invalid-config", "wine_runner", "未知的 Wine 启动器类型", fmt.Sprintf("wine_runner=%s", wineRunner))
		}
	}

	if wineRunner != "" {
		return nil, newStrategyError("invalid-config", "wine_runner", "原生 macOS 可执行文件不应启用 Wine 启动器", fmt.Sprintf("path=%s wine_runner=%s", path, wineRunner))
	}
	return nativeExecutableStrategy{}, nil
}

func (s nativeAppStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	return LaunchPlan{
		File:          game.Path,
		Dir:           filepath.Dir(game.Path),
		DetectionDir:  game.Path,
		DetectionMode: DetectionLauncherOnly,
		DisplayName:   filepath.Base(game.Path),
		ActiveTrack: ActiveTrack{
			Kind:       ActiveTrackBundlePath,
			BundlePath: game.Path,
		},
	}, nil
}

func (s nativeExecutableStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	return LaunchPlan{
		File:          game.Path,
		Dir:           filepath.Dir(game.Path),
		DetectionDir:  filepath.Dir(game.Path),
		DetectionMode: DetectionLauncherOnly,
		DisplayName:   filepath.Base(game.Path),
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackLauncherPID,
		},
	}, nil
}

func (s wineSystemStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	winePath, err := resolveWineBinaryPath(s.cfg)
	if err != nil {
		return LaunchPlan{}, err
	}

	prefix := EffectiveString(opts.WinePrefix, game.WinePrefix)
	if prefix == "" && s.cfg != nil {
		prefix = strings.TrimSpace(s.cfg.WinePrefix)
	}

	env := []string{"WINEDEBUG=-all"}
	if prefix != "" {
		env = append(env, "WINEPREFIX="+prefix)
	}

	args := append([]string{game.Path}, parseWineArgs(EffectiveString(opts.WineArgs, game.WineArgs))...)
	return LaunchPlan{
		File:          winePath,
		Args:          args,
		Dir:           filepath.Dir(game.Path),
		DetectionDir:  filepath.Dir(game.Path),
		Env:           env,
		DetectionMode: DetectionLauncherOnly,
		DisplayName:   filepath.Base(game.Path),
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackWineRootPID,
		},
	}, nil
}

func (s wineCrossoverStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	winePath := ""
	if s.cfg != nil {
		winePath = strings.TrimSpace(s.cfg.WineRunnerPath)
	}
	if strings.EqualFold(filepath.Ext(winePath), ".app") {
		return LaunchPlan{}, newStrategyError("invalid-config", "wine_runner_path", "CrossOver 启动器路径应选择 bundle 内的 bin/wine，而不是 .app 本身", fmt.Sprintf("path=%s", winePath))
	}
	var err error
	winePath, err = resolveWineBinaryPath(s.cfg)
	if err != nil {
		return LaunchPlan{}, err
	}

	bottle := EffectiveString(opts.WinePrefix, game.WinePrefix)
	if bottle == "" && s.cfg != nil {
		bottle = strings.TrimSpace(s.cfg.WinePrefix)
	}

	env := []string{"WINEDEBUG=-all"}
	if bottle != "" {
		env = append(env, "CX_BOTTLE="+bottle)
	}

	args := append([]string{game.Path}, parseWineArgs(EffectiveString(opts.WineArgs, game.WineArgs))...)
	return LaunchPlan{
		File:          winePath,
		Args:          args,
		Dir:           filepath.Dir(game.Path),
		DetectionDir:  filepath.Dir(game.Path),
		Env:           env,
		DetectionMode: DetectionLauncherOnly,
		DisplayName:   filepath.Base(game.Path),
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackWineRootPID,
		},
	}, nil
}

func resolveWineBinaryPath(cfg *appconf.AppConfig) (string, error) {
	if cfg == nil || strings.TrimSpace(cfg.WineRunnerPath) == "" {
		return "", newStrategyError("missing-config", "wine_runner_path", "请先在设置中配置 Wine 可执行文件路径", "WineRunnerPath is empty")
	}
	winePath := strings.TrimSpace(cfg.WineRunnerPath)
	info, err := os.Stat(winePath)
	if err != nil {
		return "", newStrategyError("missing-config", "wine_runner_path", fmt.Sprintf("Wine 可执行文件路径不存在：%s", winePath), err.Error())
	}
	if info.IsDir() {
		return "", newStrategyError("invalid-config", "wine_runner_path", fmt.Sprintf("Wine 路径必须是可执行文件而不是目录：%s", winePath), "wine runner path is a directory")
	}
	return winePath, nil
}

func parseWineArgs(args string) []string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	return strings.Fields(args)
}
