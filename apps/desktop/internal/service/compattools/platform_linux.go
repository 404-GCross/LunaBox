//go:build linux

package compattools

import (
	"context"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/service/integrator"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/protonutils"
	"lunabox/internal/utils/tricksutils"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	compatibilityRunnerWine        = "wine"
	compatibilityRunnerProton      = "proton"
	compatibilityRunnerSteamProton = "steam-proton"
)

type gameCompatibilityContext struct {
	info                Info
	winePath            string
	protonPath          string
	protonClientInstall string
	steamRoot           string
	compatDataPath      string
}

func getPlatformTools(ctx context.Context, game models.Game, cfg *appconf.AppConfig) (Info, error) {
	resolved, err := resolveGameCompatibilityContext(ctx, game, cfg)
	if err != nil {
		return Info{}, err
	}
	return resolved.info, nil
}

func openPlatformTool(ctx context.Context, game models.Game, cfg *appconf.AppConfig, action string) (string, error) {
	resolved, err := resolveGameCompatibilityContext(ctx, game, cfg)
	if err != nil {
		return "", err
	}
	if !resolved.info.Supported {
		return "", errors.New(resolved.info.Message)
	}
	if !actionAvailable(resolved.info.Actions, action) {
		return "", fmt.Errorf("当前兼容层不支持该动作: %s", action)
	}

	switch action {
	case ActionPrefixDir:
		return resolved.info.PrefixPath, apputils.OpenDirectory(resolved.info.PrefixPath)
	case ActionDriveC:
		return resolved.info.DriveCPath, apputils.OpenDirectory(resolved.info.DriveCPath)
	}

	switch resolved.info.RunnerKind {
	case compatibilityRunnerWine:
		return action, startWineCompatibilityAction(resolved, action)
	case compatibilityRunnerSteamProton:
		return action, startProtontricksCompatibilityAction(resolved, action)
	case compatibilityRunnerProton:
		return action, startDirectProtonCompatibilityAction(resolved, action)
	default:
		return "", fmt.Errorf("当前游戏不是 Wine/Proton 启动")
	}
}

func resolveGameCompatibilityContext(ctx context.Context, game models.Game, cfg *appconf.AppConfig) (gameCompatibilityContext, error) {
	winetricks := tricksutils.DetectWinetricks(configString(cfg, func(config *appconf.AppConfig) string {
		return config.WinetricksPath
	}))
	protontricks := tricksutils.DetectProtontricks(configString(cfg, func(config *appconf.AppConfig) string {
		return config.ProtontricksPath
	}))

	base := gameCompatibilityContext{
		info: Info{
			WinetricksPath:        winetricks.Path,
			WinetricksSource:      winetricks.Source,
			WinetricksAvailable:   winetricks.Available,
			WinetricksError:       winetricks.Error,
			ProtontricksPath:      protontricks.Path,
			ProtontricksSource:    protontricks.Source,
			ProtontricksAvailable: protontricks.Available,
			ProtontricksError:     protontricks.Error,
		},
	}

	if enums.NormalizeLaunchMode(game.LaunchMode) == enums.LaunchModeSteam {
		return resolveSteamProtonCompatibilityContext(ctx, game, cfg, base)
	}

	if !isWindowsCompatibilityExecutable(game.Path) {
		base.info.Message = "当前游戏不是 Windows 可执行文件，不需要 Wine/Proton 工具"
		return base, nil
	}

	runner := strings.TrimSpace(game.WineRunner)
	if runner == "" {
		runner = "system"
	}
	switch {
	case runner == "system" || runner == "custom":
		return resolveWineCompatibilityContext(game, cfg, base)
	case protonutils.IsProtonRunner(runner):
		return resolveDirectProtonCompatibilityContext(game, cfg, runner, base)
	default:
		base.info.Message = fmt.Sprintf("当前兼容层暂不支持快捷工具: %s", runner)
		return base, nil
	}
}

func resolveWineCompatibilityContext(game models.Game, cfg *appconf.AppConfig, base gameCompatibilityContext) (gameCompatibilityContext, error) {
	prefix := strings.TrimSpace(game.WinePrefix)
	if prefix == "" {
		prefix = configString(cfg, func(config *appconf.AppConfig) string {
			return config.WinePrefix
		})
	}
	if prefix == "" {
		prefix = defaultWinePrefix()
	}
	if prefix != "" {
		prefix = filepath.Clean(prefix)
	}

	base.winePath = resolveConfiguredWinePath(cfg)
	base.info.Supported = true
	base.info.RunnerKind = compatibilityRunnerWine
	base.info.PrefixPath = prefix
	if prefix != "" {
		base.info.DriveCPath = filepath.Join(prefix, "drive_c")
	}
	base.info.Actions = directoryCompatibilityActions(base.info)
	if base.winePath != "" {
		base.info.Actions = append(base.info.Actions,
			ActionRegedit,
			ActionWinecfg,
			ActionExplorer,
			ActionWinecmd,
		)
	}
	if base.winePath == "" {
		base.info.Message = "未找到 Wine，可先在设置中填写路径"
	}
	return base, nil
}

func resolveDirectProtonCompatibilityContext(game models.Game, cfg *appconf.AppConfig, runner string, base gameCompatibilityContext) (gameCompatibilityContext, error) {
	tool, err := protonutils.SelectTool(protonutils.RunnerSelector(runner))
	if err != nil {
		base.info.Message = err.Error()
		return base, nil
	}
	compatDataPath := strings.TrimSpace(game.WinePrefix)
	if compatDataPath == "" {
		compatDataPath = configString(cfg, func(config *appconf.AppConfig) string {
			return config.WinePrefix
		})
	}
	compatDataPath, err = protonutils.ResolveCompatDataPath(compatDataPath, game.ID)
	if err != nil {
		return gameCompatibilityContext{}, fmt.Errorf("获取 Proton compatdata 目录失败: %w", err)
	}

	appID := protonutils.StableAppID(game.ID, game.SteamLaunchID)
	base.protonPath = tool.ProtonPath
	base.protonClientInstall = protonutils.ClientInstallPath(tool)
	base.compatDataPath = compatDataPath
	base.info.Supported = true
	base.info.RunnerKind = compatibilityRunnerProton
	base.info.PrefixPath = filepath.Join(compatDataPath, "pfx")
	base.info.DriveCPath = filepath.Join(compatDataPath, "pfx", "drive_c")
	base.info.AppID = appID
	base.info.Actions = directoryCompatibilityActions(base.info)
	if base.protonPath != "" || base.info.ProtontricksAvailable {
		base.info.Actions = append(base.info.Actions,
			ActionRegedit,
			ActionWinecfg,
			ActionExplorer,
			ActionWinecmd,
		)
	}
	return base, nil
}

func resolveSteamProtonCompatibilityContext(ctx context.Context, game models.Game, cfg *appconf.AppConfig, base gameCompatibilityContext) (gameCompatibilityContext, error) {
	info, err := integrator.GetSteamCompatibilityInfo(ctx, game)
	if err != nil {
		return gameCompatibilityContext{}, err
	}
	base.info.Supported = info.Supported
	base.info.RunnerKind = compatibilityRunnerSteamProton
	base.info.PrefixPath = strings.TrimSpace(info.ProtonPrefix)
	if base.info.PrefixPath != "" {
		base.info.DriveCPath = filepath.Join(base.info.PrefixPath, "drive_c")
	}
	base.info.AppID = strings.TrimSpace(info.AppID)
	base.steamRoot = strings.TrimSpace(info.SteamRoot)

	if !info.Supported {
		base.info.Message = "Steam Proton 快捷工具仅支持 Linux"
		return base, nil
	}
	if !info.SteamInstalled {
		base.info.Message = "未检测到 Linux Steam 客户端"
		return base, nil
	}
	if base.info.AppID == "" {
		base.info.Message = "该游戏尚未关联 Steam"
		return base, nil
	}

	base.info.Actions = directoryCompatibilityActions(base.info)
	if base.info.ProtontricksAvailable {
		base.info.Actions = append(base.info.Actions,
			ActionRegedit,
			ActionWinecfg,
			ActionExplorer,
			ActionWinecmd,
		)
	} else {
		base.info.Message = "未找到 protontricks，注册表/Wine 配置快捷入口不可用"
	}
	return base, nil
}

func directoryCompatibilityActions(info Info) []string {
	actions := make([]string, 0, 2)
	if isExistingDirectory(info.PrefixPath) {
		actions = append(actions, ActionPrefixDir)
	}
	if isExistingDirectory(info.DriveCPath) {
		actions = append(actions, ActionDriveC)
	}
	return actions
}

func startWineCompatibilityAction(resolved gameCompatibilityContext, action string) error {
	if strings.TrimSpace(resolved.winePath) == "" {
		return fmt.Errorf("未找到 Wine，请先在设置中填写路径")
	}
	env := []string{"WINEPREFIX=" + resolved.info.PrefixPath}
	if action == ActionWinecmd {
		if err := startCompatibilityCommandInTerminal(resolved.winePath, []string{"cmd.exe"}, env, resolved.info.PrefixPath); err == nil {
			return nil
		}
		if wineconsolePath := resolveWineSiblingExecutable(resolved.winePath, "wineconsole"); wineconsolePath != "" {
			return startCompatibilityCommand(wineconsolePath, []string{"cmd.exe"}, env, resolved.info.PrefixPath)
		}
	}
	return startCompatibilityCommand(resolved.winePath, []string{wineProgramForAction(action)}, env, resolved.info.PrefixPath)
}

func startProtontricksCompatibilityAction(resolved gameCompatibilityContext, action string) error {
	if !resolved.info.ProtontricksAvailable {
		return fmt.Errorf("未找到 protontricks，请先在设置中填写路径")
	}
	env := []string{}
	if resolved.steamRoot != "" {
		env = append(env, "STEAM_DIR="+resolved.steamRoot)
	}
	if resolved.compatDataPath != "" {
		env = append(env, "STEAM_COMPAT_DATA_PATH="+resolved.compatDataPath)
	}
	if resolved.protonClientInstall != "" {
		env = append(env, "STEAM_COMPAT_CLIENT_INSTALL_PATH="+resolved.protonClientInstall)
	}
	if resolved.info.WinetricksAvailable {
		env = append(env, "WINETRICKS="+resolved.info.WinetricksPath)
	}
	if action == ActionWinecmd {
		args := protontricksCommandArgs(resolved.info.AppID, action, false, true)
		if err := startCompatibilityCommandInTerminal(resolved.info.ProtontricksPath, args, env, resolved.info.PrefixPath); err == nil {
			return nil
		}
	}
	return startCompatibilityCommand(resolved.info.ProtontricksPath, protontricksCommandArgs(resolved.info.AppID, action, true, false), env, resolved.info.PrefixPath)
}

func startDirectProtonCompatibilityAction(resolved gameCompatibilityContext, action string) error {
	if strings.TrimSpace(resolved.protonPath) == "" {
		return fmt.Errorf("未找到当前 Proton 可执行文件")
	}
	if resolved.compatDataPath == "" {
		return fmt.Errorf("未找到 Proton compatdata 目录")
	}
	if err := os.MkdirAll(resolved.compatDataPath, 0o755); err != nil {
		return fmt.Errorf("创建 Proton compatdata 目录失败: %w", err)
	}

	env := []string{
		"WINEDEBUG=-all",
		"STEAM_COMPAT_DATA_PATH=" + resolved.compatDataPath,
		"STEAM_COMPAT_APP_ID=" + resolved.info.AppID,
		"SteamAppId=" + resolved.info.AppID,
		"SteamGameId=" + resolved.info.AppID,
	}
	if resolved.protonClientInstall != "" {
		env = append(env, "STEAM_COMPAT_CLIENT_INSTALL_PATH="+resolved.protonClientInstall)
	}
	if action == ActionWinecmd {
		if err := startCompatibilityCommandInTerminal(resolved.protonPath, []string{"run", "cmd.exe"}, env, resolved.info.PrefixPath); err == nil {
			return nil
		}
	}
	return startCompatibilityCommand(resolved.protonPath, append([]string{"run"}, protonProgramArgsForAction(action)...), env, resolved.info.PrefixPath)
}

type compatibilityLaunchCommand struct {
	path string
	args []string
}

func startCompatibilityCommand(path string, args []string, env []string, dir string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("兼容层工具路径为空")
	}
	cmd := exec.Command(path, args...)
	if strings.TrimSpace(dir) != "" && isExistingDirectory(dir) {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动兼容层工具失败: %w", err)
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func startCompatibilityCommandInTerminal(commandPath string, commandArgs []string, env []string, dir string) error {
	commands := terminalLaunchCommands(commandPath, commandArgs)
	if len(commands) == 0 {
		return fmt.Errorf("未找到可用的 Linux 终端模拟器")
	}

	errors := make([]string, 0, len(commands))
	for _, command := range commands {
		if err := startCompatibilityCommand(command.path, command.args, env, dir); err == nil {
			return nil
		} else {
			errors = append(errors, fmt.Sprintf("%s: %v", command.path, err))
		}
	}
	return fmt.Errorf("启动 Linux 终端失败: %s", strings.Join(errors, "; "))
}

func terminalLaunchCommands(commandPath string, commandArgs []string) []compatibilityLaunchCommand {
	commandPath = strings.TrimSpace(commandPath)
	if commandPath == "" {
		return nil
	}

	candidates := terminalCandidates()
	commands := make([]compatibilityLaunchCommand, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		fields := strings.Fields(strings.TrimSpace(candidate))
		if len(fields) == 0 {
			continue
		}

		terminalPath := strings.TrimSpace(fields[0])
		extraArgs := append([]string{}, fields[1:]...)
		resolvedTerminalPath := terminalPath
		if !strings.ContainsRune(terminalPath, os.PathSeparator) {
			path, err := exec.LookPath(terminalPath)
			if err != nil {
				continue
			}
			resolvedTerminalPath = path
		}
		if !isExecutableFile(resolvedTerminalPath) || seen[resolvedTerminalPath] {
			continue
		}

		seen[resolvedTerminalPath] = true
		commands = append(commands, compatibilityLaunchCommand{
			path: resolvedTerminalPath,
			args: terminalArgsForCommand(filepath.Base(resolvedTerminalPath), extraArgs, commandPath, commandArgs),
		})
	}
	return commands
}

func terminalCandidates() []string {
	candidates := make([]string, 0, 24)
	if terminal := strings.TrimSpace(os.Getenv("TERMINAL")); terminal != "" {
		candidates = append(candidates, terminal)
	}
	return append(candidates,
		"konsole",
		"gnome-terminal",
		"kgx",
		"ptyxis",
		"xfce4-terminal",
		"qterminal",
		"x-terminal-emulator",
		"mate-terminal",
		"tilix",
		"terminator",
		"alacritty",
		"kitty",
		"wezterm",
		"wezterm-gui",
		"foot",
		"ghostty",
		"cosmic-term",
		"rio",
		"lxterminal",
		"xterm",
		"uxterm",
	)
}

func terminalArgsForCommand(terminalName string, terminalExtraArgs []string, commandPath string, commandArgs []string) []string {
	command := append([]string{commandPath}, commandArgs...)
	args := append([]string{}, terminalExtraArgs...)

	switch strings.ToLower(strings.TrimSpace(terminalName)) {
	case "konsole", "qterminal", "x-terminal-emulator", "alacritty", "ghostty", "cosmic-term", "rio", "lxterminal", "xterm", "uxterm":
		return append(args, append([]string{"-e"}, command...)...)
	case "gnome-terminal", "kgx", "ptyxis", "mate-terminal", "tilix", "terminator":
		return append(args, append([]string{"--"}, command...)...)
	case "xfce4-terminal":
		return append(args, append([]string{"--execute"}, command...)...)
	case "wezterm", "wezterm-gui":
		return append(args, append([]string{"start", "--"}, command...)...)
	case "kitty", "foot":
		return append(args, command...)
	default:
		return append(args, append([]string{"-e"}, command...)...)
	}
}

func isExistingDirectory(path string) bool {
	info, err := os.Stat(strings.TrimSpace(path))
	return err == nil && info.IsDir()
}

func configString(cfg *appconf.AppConfig, pick func(*appconf.AppConfig) string) string {
	if cfg == nil || pick == nil {
		return ""
	}
	return strings.TrimSpace(pick(cfg))
}

func defaultWinePrefix() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".wine")
}

func resolveConfiguredWinePath(cfg *appconf.AppConfig) string {
	if cfg != nil {
		path := strings.TrimSpace(cfg.WineRunnerPath)
		if path != "" {
			if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
				return path
			}
		}
	}
	if path, err := exec.LookPath("wine"); err == nil {
		return path
	}
	return ""
}

func isWindowsCompatibilityExecutable(path string) bool {
	// Keep this set aligned with the Linux launcher strategy in
	// internal/service/launcher/strategy_linux.go, which only treats
	// .exe/.bat as Windows executables it can start through Wine/Proton.
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".exe", ".bat":
		return true
	default:
		return false
	}
}

func wineProgramForAction(action string) string {
	switch action {
	case ActionRegedit:
		return "regedit.exe"
	case ActionExplorer:
		return "explorer.exe"
	case ActionWinecmd:
		return "cmd.exe"
	default:
		return action
	}
}

func protonProgramArgsForAction(action string) []string {
	return protonProgramArgsForActionWithTerminal(action, false)
}

func protonProgramArgsForActionWithTerminal(action string, nativeTerminal bool) []string {
	if action == ActionWinecmd {
		if nativeTerminal {
			return []string{"cmd.exe"}
		}
		return []string{"wineconsole", "cmd.exe"}
	}
	return []string{wineProgramForAction(action)}
}

func protontricksCommandArgs(appID string, action string, noTerm bool, nativeTerminal bool) []string {
	args := make([]string, 0, 5)
	if noTerm {
		args = append(args, "--no-term")
	}
	return append(args, "-c", protontricksShellCommandForAction(action, nativeTerminal), appID)
}

func protontricksShellCommandForAction(action string, nativeTerminal bool) string {
	args := protonProgramArgsForActionWithTerminal(action, nativeTerminal)
	parts := []string{"\"${WINE:-wine}\""}
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func resolveWineSiblingExecutable(winePath string, name string) string {
	winePath = strings.TrimSpace(winePath)
	if winePath != "" {
		path := filepath.Join(filepath.Dir(winePath), name)
		if isExecutableFile(path) {
			return path
		}
	}
	if path, err := exec.LookPath(name); err == nil && isExecutableFile(path) {
		return path
	}
	return ""
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(strings.TrimSpace(path))
	return err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0
}
