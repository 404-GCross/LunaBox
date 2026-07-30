//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	appName   = "LunaBox"
	launchArg = "--autostart"
)

func ExtractLaunchFlag(args []string) ([]string, bool) {
	if len(args) == 0 {
		return []string{}, false
	}

	cleanArgs := make([]string, 0, len(args))
	launchedByAutostart := false
	for _, arg := range args {
		if strings.EqualFold(strings.TrimSpace(arg), launchArg) {
			launchedByAutostart = true
			continue
		}
		cleanArgs = append(cleanArgs, arg)
	}
	return cleanArgs, launchedByAutostart
}

func Sync(enabled bool) error {
	if enabled {
		return enable()
	}
	return disable()
}

func enable() error {
	desktopPath, err := autostartDesktopFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(desktopPath), 0755); err != nil {
		return fmt.Errorf("create autostart directory: %w", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	exePath, err = filepath.Abs(filepath.Clean(exePath))
	if err != nil {
		return fmt.Errorf("normalize executable path: %w", err)
	}

	entry := strings.Join([]string{
		"[Desktop Entry]",
		"Version=1.0",
		"Type=Application",
		"Name=" + appName,
		"Exec=" + quoteDesktopExecArg(exePath) + " " + quoteDesktopExecArg(launchArg),
		"Terminal=false",
		"X-GNOME-Autostart-enabled=true",
		"",
	}, "\n")
	if err := os.WriteFile(desktopPath, []byte(entry), 0644); err != nil {
		return fmt.Errorf("write autostart desktop entry: %w", err)
	}
	return nil
}

func disable() error {
	desktopPath, err := autostartDesktopFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(desktopPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove autostart desktop entry: %w", err)
	}
	return nil
}

func autostartDesktopFilePath() (string, error) {
	configHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get user home: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "autostart", appName+".desktop"), nil
}

func quoteDesktopExecArg(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", "\\$")
	return `"` + replacer.Replace(value) + `"`
}
