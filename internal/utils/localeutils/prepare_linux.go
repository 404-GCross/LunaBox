//go:build linux

package localeutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func PrepareLaunchEnvironment(ctx context.Context, gamePath string, locale string) (LaunchEnvironment, error) {
	env, err := BuildLaunchEnvironment(gamePath, locale)
	if err != nil {
		return LaunchEnvironment{}, err
	}

	spec := Spec(env.Locale)
	if err := os.MkdirAll(env.LOCPATH, 0o755); err != nil {
		return LaunchEnvironment{}, fmt.Errorf("create locale directory: %w", err)
	}

	outputPath := filepath.Join(env.LOCPATH, spec.Name)
	if _, err := os.Stat(outputPath); err == nil {
		return env, nil
	}

	localedefPath, err := exec.LookPath("localedef")
	if err != nil {
		return LaunchEnvironment{}, fmt.Errorf("未找到 localedef，请先安装系统 locales/glibc-locale 工具: %w", err)
	}

	cmd := exec.CommandContext(ctx, localedefPath, "-f", spec.Charmap, "-i", spec.Input, outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return LaunchEnvironment{}, fmt.Errorf("生成 locale %s 失败: %w (%s)", spec.Name, err, string(out))
	}
	return env, nil
}
