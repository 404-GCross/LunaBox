//go:build !linux

package main

import (
	"errors"

	"lunabox/internal/applog"
)

// runHeadless 仅在 Linux 上可用：其它平台继续使用 GUI 模式。
func runHeadless(_ *applog.FileLogger) error {
	return errors.New("--headless 仅在 Linux 上支持")
}
