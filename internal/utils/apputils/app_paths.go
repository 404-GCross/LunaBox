package apputils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"lunabox/internal/version"
)

const appName = "LunaBox"

var (
	dataDir   string
	cacheDir  string
	configDir string
	initOnce  sync.Once
	initErr   error
)

// initDirs 初始化所有目录路径
func initDirs() error {
	initOnce.Do(func() {
		if version.BuildMode == "portable" {
			// 便携版：使用程序目录
			initErr = initPortableDirs()
		} else {
			// 安装版 / AppImage：使用系统标准目录
			initErr = initInstallerDirs()
		}
	})
	return initErr
}

// initPortableDirs 初始化便携版目录（程序目录）
func initPortableDirs() error {
	if portableRoot := os.Getenv("LUNABOX_PORTABLE_ROOT"); portableRoot != "" {
		portableRoot, err := filepath.Abs(filepath.Clean(portableRoot))
		if err != nil {
			return err
		}
		dataDir = portableRoot
		cacheDir = portableRoot
		configDir = portableRoot
		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	execDir := filepath.Dir(execPath)
	dataDir = execDir
	cacheDir = execDir
	configDir = execDir
	return nil
}

// initInstallerDirs 初始化安装版目录（系统标准目录）
func initInstallerDirs() error {
	// 配置目录: %APPDATA%\LunaBox (Windows) 或 ~/.config/LunaBox (Linux/Mac)
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	configDir = filepath.Join(userConfigDir, appName)

	// 缓存目录: %LOCALAPPDATA%\LunaBox (Windows) 或 ~/.cache/LunaBox (Linux/Mac)
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	cacheDir = filepath.Join(userCacheDir, appName)

	// 数据目录：使用配置目录（数据库、备份等重要数据）
	dataDir = configDir

	return nil
}

// GetDataDir 获取数据目录（数据库、备份、上传的封面图片等）
func GetDataDir() (string, error) {
	if err := initDirs(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", err
	}
	return dataDir, nil
}

// GetCacheDir 获取缓存目录
func GetCacheDir() (string, error) {
	if err := initDirs(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	return cacheDir, nil
}

// GetConfigDir 获取配置目录
func GetConfigDir() (string, error) {
	if err := initDirs(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return configDir, nil
}

// GetSubDir 获取子目录并确保目录存在
func GetSubDir(subPath string) (string, error) {
	base, err := GetDataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, subPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// GetCacheSubDir 获取缓存子目录并确保目录存在
func GetCacheSubDir(subPath string) (string, error) {
	base, err := GetCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, subPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// GetTemplatesDir 获取用户模板目录
func GetTemplatesDir() (string, error) {
	return GetSubDir("templates")
}

// GetDesktopDir 获取当前用户桌面目录
func GetDesktopDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "Desktop"), nil
}

// IsPortableMode 返回是否为便携模式
func IsPortableMode() bool {
	return version.BuildMode == "portable"
}

// IsAppImageMode 返回是否为 AppImage 模式
func IsAppImageMode() bool {
	return version.BuildMode == "appimage"
}

// GetLaunchExecutablePath returns the stable user-facing executable path.
// AppImage builds run from a temporary SquashFS mount, so os.Executable points
// at an unstable internal path. Prefer the outer AppImage path provided by the
// AppImage runtime in that mode.
func GetLaunchExecutablePath() (string, error) {
	if IsAppImageMode() {
		return GetAppImagePath()
	}

	if IsPortableMode() && runtime.GOOS == "linux" {
		if portableRoot := strings.TrimSpace(os.Getenv("LUNABOX_PORTABLE_ROOT")); portableRoot != "" {
			launcherPath, err := filepath.Abs(filepath.Join(filepath.Clean(portableRoot), appName))
			if err != nil {
				return "", fmt.Errorf("resolve portable launcher path: %w", err)
			}
			if info, err := os.Stat(launcherPath); err == nil && !info.IsDir() {
				return launcherPath, nil
			}
		}
	}

	return currentExecutablePath()
}

// GetAppImagePath returns the outer AppImage file path for AppImage builds.
func GetAppImagePath() (string, error) {
	for _, envName := range []string{"LUNABOX_APPIMAGE_PATH", "APPIMAGE"} {
		if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
			appImagePath, err := filepath.Abs(filepath.Clean(value))
			if err != nil {
				return "", fmt.Errorf("resolve AppImage path from %s: %w", envName, err)
			}
			if info, err := os.Stat(appImagePath); err != nil {
				return "", fmt.Errorf("stat AppImage path %s: %w", appImagePath, err)
			} else if info.IsDir() {
				return "", fmt.Errorf("AppImage path is a directory: %s", appImagePath)
			}
			return appImagePath, nil
		}
	}
	return "", fmt.Errorf("APPIMAGE is not set; run through the AppImage file to register integrations")
}

func currentExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return abs, nil
}

// GetBuildMode 返回当前构建模式
func GetBuildMode() string {
	return version.BuildMode
}
