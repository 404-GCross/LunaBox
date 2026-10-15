// Package backend 把 apps/desktop 的真实服务搬进原型：自己的数据目录、数据库迁移、服务的构造与相互注入，
// 以及用 MyGo 实现的 wailsruntime.Runtime（文件对话框、事件、打开网址）。
package backend

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"lunabox/internal/applog"
	"lunabox/internal/version"
)

// seedEntries 是首次运行时从 LunaBox 安装版数据目录复制过来的内容（日志与云同步身份不复制）。
var seedEntries = []string{"appconf.json", "lunabox.db", "lunabox.db.wal", "covers", "backgrounds", "avatars", "templates", "backups"}

// DefaultDataDir 是原型自己的数据目录：可执行文件在 build/ 下时用 build/data，否则用当前目录下的 build/data
// （go run 在 apps/desktop-mygo 下执行）。
func DefaultDataDir() string {
	if exe, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exe); filepath.Base(dir) == "build" {
			return filepath.Join(dir, "data")
		}
	}
	return filepath.Join("build", "data")
}

// InstalledDataDir 是 LunaBox 安装版的数据目录（%APPDATA%/LunaBox），用作种子数据的来源。
func InstalledDataDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "LunaBox"), nil
}

// UseDataDir 让 apps/desktop 的 apputils 把 dir 当作便携版数据目录，服务日志写到 dir/logs/app.log
// （与主程序相同）。必须在任何服务读取路径之前调用。
func UseDataDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", err
	}
	version.BuildMode = "portable"
	if err := os.Setenv("LUNABOX_PORTABLE_ROOT", abs); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(abs, "logs"), 0o755); err != nil {
		return "", err
	}
	applog.SetMode(applog.ModeGUI)
	applog.SetLogger(applog.NewFileLogger(filepath.Join(abs, "logs", "app.log"), slog.LevelInfo))
	return abs, nil
}

// Seed 在 dir 还没有数据库（或 force）时，从 from 复制设置、数据库、封面与备份。只复制，不改动 from。
func Seed(dir, from string, force bool) (bool, error) {
	if _, err := os.Stat(filepath.Join(dir, "lunabox.db")); err == nil && !force {
		return false, nil
	}
	if _, err := os.Stat(filepath.Join(from, "lunabox.db")); err != nil {
		return false, nil // 没有安装版数据：从空数据库开始
	}
	if force {
		_ = os.Remove(filepath.Join(dir, "lunabox.db.wal"))
	}
	for _, name := range seedEntries {
		src, dst := filepath.Join(from, name), filepath.Join(dir, name)
		info, err := os.Stat(src)
		if err != nil {
			continue
		}
		if info.IsDir() {
			err = copyDir(src, dst)
		} else {
			err = copyFile(src, dst)
		}
		if err != nil {
			return false, fmt.Errorf("copy %s: %w", name, err)
		}
	}
	return true, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
