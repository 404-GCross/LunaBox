//go:build !linux

package service

import (
	"context"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
)

func getPlatformGameCompatibilityTools(_ context.Context, _ models.Game, _ *appconf.AppConfig) (GameCompatibilityToolsInfo, error) {
	return GameCompatibilityToolsInfo{
		Supported: false,
		Message:   "Wine/Proton 快捷工具仅支持 Linux",
	}, nil
}

func openPlatformGameCompatibilityTool(_ context.Context, _ models.Game, _ *appconf.AppConfig, _ string) (string, error) {
	return "", fmt.Errorf("Wine/Proton 快捷工具仅支持 Linux")
}
