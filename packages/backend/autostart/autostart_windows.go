// Package autostart 设置 LunaBox 的开机自启动，供没有自带开机自启的桌面框架（MyGo）使用。
package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"

	"lunabox/backend/appruntime"
)

// Wails 的 Autostart（application.autostartSlug("LunaBox")）在用户的 Run 键下写 lunabox 这一项，
// 值是 `"<程序>" --autostart`。这里写同一项，Wails 版与 MyGo 版共用一个开机启动设置。
const (
	autostartRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
	autostartName   = "lunabox"
)

// Set 打开或关闭当前程序的开机自启动。
func Set(enabled bool) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("autostart: get executable path: %w", err)
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	if !enabled {
		key, err := registry.OpenKey(registry.CURRENT_USER, autostartRunKey, registry.QUERY_VALUE|registry.SET_VALUE)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("autostart: open registry key: %w", err)
		}
		defer key.Close()
		// 只删除指向本程序的启动项：它指向另一个安装位置时属于那个程序。
		if cmd, _, err := key.GetStringValue(autostartName); err == nil && strings.EqualFold(commandExe(cmd), exe) {
			if err := key.DeleteValue(autostartName); err != nil {
				return fmt.Errorf("autostart: delete registry value: %w", err)
			}
		}
		return nil
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, autostartRunKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("autostart: open registry key: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue(autostartName, `"`+exe+`" `+appruntime.AutostartLaunchArgument); err != nil {
		return fmt.Errorf("autostart: write registry value: %w", err)
	}
	return nil
}

// commandExe 返回启动命令里的程序路径（带引号或不带引号的第一段）。
func commandExe(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if rest, ok := strings.CutPrefix(cmd, `"`); ok {
		exe, _, _ := strings.Cut(rest, `"`)
		return exe
	}
	exe, _, _ := strings.Cut(cmd, " ")
	return exe
}
