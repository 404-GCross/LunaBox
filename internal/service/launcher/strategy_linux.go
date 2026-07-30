//go:build linux

package launcher

import (
	"context"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	wineRunnerSystem    = "system"
	wineRunnerCrossover = "crossover"
	wineRunnerCustom    = "custom"
)

type nativeLinuxStrategy struct{}
type wineLinuxStrategy struct {
	cfg *appconf.AppConfig
}
type steamLinuxStrategy struct{}

func selectPlatformLauncherStrategy(game *models.Game, opts LaunchOptions, cfg *appconf.AppConfig) (LauncherStrategy, error) {
	if ShouldUseSteamLaunch(game, opts) {
		return steamLinuxStrategy{}, nil
	}

	path := strings.TrimSpace(game.Path)
	ext := strings.ToLower(filepath.Ext(path))
	wineRunner := EffectiveString(opts.WineRunner, game.WineRunner)

	if ext == ".exe" || ext == ".bat" || ext == ".cmd" {
		switch wineRunner {
		case "":
			return nil, newStrategyError("missing-config", "wine_runner", "该游戏需要在 Linux 上启用 Wine 启动器", fmt.Sprintf("path=%s", path))
		case wineRunnerSystem, wineRunnerCrossover, wineRunnerCustom:
			return wineLinuxStrategy{cfg: cfg}, nil
		default:
			return nil, newStrategyError("invalid-config", "wine_runner", "未知的 Wine 启动器类型", fmt.Sprintf("wine_runner=%s", wineRunner))
		}
	}

	if wineRunner != "" {
		return nil, newStrategyError("invalid-config", "wine_runner", "原生 Linux 可执行文件不应启用 Wine 启动器", fmt.Sprintf("path=%s wine_runner=%s", path, wineRunner))
	}
	return nativeLinuxStrategy{}, nil
}

func (s nativeLinuxStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	launchDir := filepath.Dir(game.Path)
	return LaunchPlan{
		File:          game.Path,
		Dir:           launchDir,
		DetectionDir:  EffectiveProcessDetectionDir(game.GameDirectory, launchDir),
		DetectionMode: DetectionLauncherOnly,
		DisplayName:   filepath.Base(game.Path),
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackDefault,
		},
	}, nil
}

func (s wineLinuxStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	wineRunner := EffectiveString(opts.WineRunner, game.WineRunner)
	winePath, err := resolveLinuxWineBinaryPath(s.cfg, wineRunner)
	if err != nil {
		return LaunchPlan{}, err
	}

	prefix := EffectiveString(opts.WinePrefix, game.WinePrefix)
	if prefix == "" && s.cfg != nil {
		prefix = strings.TrimSpace(s.cfg.WinePrefix)
	}

	env := []string{"WINEDEBUG=-all"}
	if prefix != "" {
		if wineRunner == wineRunnerCrossover {
			env = append(env, "CX_BOTTLE="+prefix)
		} else {
			env = append(env, "WINEPREFIX="+prefix)
		}
	}

	args := append([]string{game.Path}, parseWineArgs(EffectiveString(opts.WineArgs, game.WineArgs))...)
	launchDir := filepath.Dir(game.Path)
	return LaunchPlan{
		File:          winePath,
		Args:          args,
		Dir:           launchDir,
		DetectionDir:  EffectiveProcessDetectionDir(game.GameDirectory, launchDir),
		Env:           env,
		DetectionMode: DetectionLauncherOnly,
		DisplayName:   filepath.Base(game.Path),
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackDefault,
		},
	}, nil
}

func (s steamLinuxStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	if !isSteamLaunchSource(game.SourceType) || strings.TrimSpace(game.SourceID) == "" {
		return LaunchPlan{}, fmt.Errorf("Steam launch requires a Steam source and launch id")
	}

	file, args, displayName, err := resolveLinuxSteamCommand(strings.TrimSpace(game.SourceID))
	if err != nil {
		return LaunchPlan{}, err
	}

	installDir := strings.TrimSpace(game.Path)
	if installDir == "" {
		return LaunchPlan{}, fmt.Errorf("Steam 启动需要游戏安装目录用于进程检测")
	}
	detectionDir := EffectiveProcessDetectionDir(game.GameDirectory, installDir)
	workingDir := detectionDir
	if info, err := os.Stat(workingDir); err != nil || !info.IsDir() {
		workingDir = ""
	}

	return LaunchPlan{
		File:          file,
		Args:          args,
		Dir:           workingDir,
		DetectionDir:  detectionDir,
		DetectionMode: DetectionSteamDirectory,
		DisplayName:   displayName,
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackDefault,
		},
	}, nil
}

func resolveLinuxWineBinaryPath(cfg *appconf.AppConfig, wineRunner string) (string, error) {
	configured := ""
	if cfg != nil {
		configured = strings.TrimSpace(cfg.WineRunnerPath)
	}
	if configured != "" {
		info, err := os.Stat(configured)
		if err != nil {
			return "", newStrategyError("missing-config", "wine_runner_path", fmt.Sprintf("Wine 可执行文件路径不存在：%s", configured), err.Error())
		}
		if info.IsDir() {
			return "", newStrategyError("invalid-config", "wine_runner_path", fmt.Sprintf("Wine 路径必须是可执行文件而不是目录：%s", configured), "wine runner path is a directory")
		}
		return configured, nil
	}

	if wineRunner == wineRunnerCustom || wineRunner == wineRunnerCrossover {
		return "", newStrategyError("missing-config", "wine_runner_path", "请先在设置中配置 Wine 可执行文件路径", "WineRunnerPath is empty")
	}

	for _, candidate := range []string{"wine", "wine64"} {
		if path, err := exec.LookPath(candidate); err == nil && strings.TrimSpace(path) != "" {
			return path, nil
		}
	}
	return "", newStrategyError("missing-config", "wine_runner_path", "未在 PATH 中找到 wine，请先安装 Wine 或在设置中配置 Wine 路径", "wine executable not found")
}

func resolveLinuxSteamCommand(sourceID string) (string, []string, string, error) {
	launchURL := "steam://rungameid/" + strings.TrimSpace(sourceID)
	if steamPath, err := exec.LookPath("steam"); err == nil && strings.TrimSpace(steamPath) != "" {
		return steamPath, []string{launchURL}, filepath.Base(steamPath), nil
	}
	if flatpakPath, err := exec.LookPath("flatpak"); err == nil && strings.TrimSpace(flatpakPath) != "" {
		return flatpakPath, []string{"run", "com.valvesoftware.Steam", launchURL}, "flatpak", nil
	}
	return "", nil, "", fmt.Errorf("未找到 Steam 启动命令：请安装 steam 命令，或安装 com.valvesoftware.Steam Flatpak")
}

func isSteamLaunchSource(source enums.SourceType) bool {
	return source == enums.Steam || source == enums.SteamShortcut
}

func parseWineArgs(args string) []string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	return strings.Fields(args)
}
