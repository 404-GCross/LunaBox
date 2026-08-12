//go:build linux

package wailsruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const linuxDesktopID = "io.github.saramanda9988.lunabox"

func setAutostart(_ *application.App, enabled bool) error {
	path, err := linuxAutostartDesktopPath()
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove Linux autostart desktop entry: %w", err)
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	exe, err = filepath.Abs(filepath.Clean(exe))
	if err != nil {
		return fmt.Errorf("normalize executable path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create autostart directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(linuxAutostartDesktopEntry(exe)), 0o644); err != nil {
		return fmt.Errorf("write Linux autostart desktop entry: %w", err)
	}
	return nil
}

func linuxAutostartDesktopPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "autostart", "lunabox.desktop"), nil
}

func linuxAutostartDesktopEntry(exePath string) string {
	return strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Version=1.0",
		"Name=LunaBox",
		"Comment=LunaBox game library manager",
		"Exec=" + quoteDesktopExecArg(exePath) + " " + AutostartLaunchArgument,
		"Icon=" + linuxDesktopIcon(exePath),
		"Terminal=false",
		"Categories=Utility;Game;",
		"StartupWMClass=" + linuxDesktopID,
		"X-GNOME-Autostart-enabled=true",
		"X-GNOME-Autostart-Delay=5",
		"X-KDE-autostart-after=panel",
		"Hidden=false",
		"NoDisplay=false",
		"",
	}, "\n")
}

func linuxDesktopIcon(exePath string) string {
	portableIcon := filepath.Join(filepath.Dir(exePath), "appicon.png")
	if info, err := os.Stat(portableIcon); err == nil && !info.IsDir() {
		return portableIcon
	}
	return linuxDesktopID
}

func quoteDesktopExecArg(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "\"\""
	}
	if !strings.ContainsAny(value, " \t\n\"'\\$") {
		return value
	}
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"$", "\\$",
	)
	return "\"" + replacer.Replace(value) + "\""
}
