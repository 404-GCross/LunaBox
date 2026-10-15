package tricksutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Tool struct {
	Name      string
	Path      string
	Source    string
	Available bool
	Error     string
}

func DetectWinetricks(configuredPath string) Tool {
	return detectExecutable("winetricks", configuredPath)
}

func DetectProtontricks(configuredPath string) Tool {
	return detectExecutable("protontricks", configuredPath)
}

func detectExecutable(name string, configuredPath string) Tool {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath != "" {
		if err := validateExecutable(configuredPath); err != nil {
			return Tool{
				Name:   name,
				Path:   configuredPath,
				Source: "config",
				Error:  err.Error(),
			}
		}
		return Tool{
			Name:      name,
			Path:      configuredPath,
			Source:    "config",
			Available: true,
		}
	}

	if path, err := exec.LookPath(name); err == nil {
		return Tool{
			Name:      name,
			Path:      path,
			Source:    "path",
			Available: true,
		}
	}

	for _, path := range commonExecutablePaths(name) {
		if err := validateExecutable(path); err == nil {
			return Tool{
				Name:      name,
				Path:      path,
				Source:    "common",
				Available: true,
			}
		}
	}

	return Tool{
		Name:   name,
		Source: "not_found",
		Error:  fmt.Sprintf("未找到 %s，请在设置中手动填写可执行文件路径", name),
	}
}

func validateExecutable(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("路径为空")
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("路径不可用：%w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("路径是目录而不是可执行文件")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("文件没有可执行权限")
	}
	return nil
}

func commonExecutablePaths(name string) []string {
	result := []string{
		filepath.Join("/usr", "local", "bin", name),
		filepath.Join("/usr", "bin", name),
		filepath.Join("/bin", name),
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		result = append([]string{filepath.Join(home, ".local", "bin", name)}, result...)
	}
	return result
}
