//go:build linux

package localeutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultI18NRoot = "/usr/share/i18n"

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
	if err := ensureLocaledefDataAvailable(spec); err != nil {
		return LaunchEnvironment{}, err
	}

	cmd := exec.CommandContext(ctx, localedefPath, "-f", spec.Charmap, "-i", spec.Input, outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return LaunchEnvironment{}, localedefError(spec, err, out)
	}
	return env, nil
}

func ensureLocaledefDataAvailable(spec LocaleSpec) error {
	missing := missingLocaledefData(spec, localedefI18NRoots())
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"系统 localedef 缺少 locale/charmap 数据（缺少 %s）。请安装 glibc locale 源数据包，确保 %s/locales 和 %s/charmaps 可用",
		strings.Join(missing, "、"),
		defaultI18NRoot,
		defaultI18NRoot,
	)
}

func localedefI18NRoots() []string {
	roots := make([]string, 0, 2)
	seen := make(map[string]bool)
	add := func(root string) {
		root = strings.TrimSpace(root)
		if root == "" || seen[root] {
			return
		}
		seen[root] = true
		roots = append(roots, root)
	}
	for _, root := range filepath.SplitList(os.Getenv("I18NPATH")) {
		add(root)
	}
	add(defaultI18NRoot)
	return roots
}

func missingLocaledefData(spec LocaleSpec, roots []string) []string {
	missing := make([]string, 0, 2)
	if !localedefDataFileExists(roots, "locales", spec.Input, nil) {
		missing = append(missing, "locale 输入文件 "+spec.Input)
	}
	if !localedefDataFileExists(roots, "charmaps", spec.Charmap, []string{".gz"}) {
		missing = append(missing, "字符映射 "+spec.Charmap)
	}
	return missing
}

func localedefDataFileExists(roots []string, dir string, name string, suffixes []string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	candidates := []string{name}
	for _, suffix := range suffixes {
		candidates = append(candidates, name+suffix)
	}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(filepath.Join(root, dir, candidate)); err == nil {
				return true
			}
		}
	}
	return false
}

func localedefError(spec LocaleSpec, err error, output []byte) error {
	message := strings.TrimSpace(string(output))
	if message == "" {
		return fmt.Errorf("生成 locale %s 失败: %w", spec.Name, err)
	}
	if strings.Contains(message, "not found") &&
		(strings.Contains(message, "character map") ||
			strings.Contains(message, "locale definition") ||
			strings.Contains(message, "input")) {
		return fmt.Errorf(
			"生成 locale %s 失败：系统 localedef 缺少 locale/charmap 数据。请安装 glibc locale 源数据包后重试: %w (%s)",
			spec.Name,
			err,
			message,
		)
	}
	return fmt.Errorf("生成 locale %s 失败: %w (%s)", spec.Name, err, message)
}
