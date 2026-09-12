//go:build !linux

package compattools

import (
	"context"
	"fmt"

	"lunabox/internal/appconf"
	"lunabox/internal/models"
)

func getPlatformTools(_ context.Context, _ models.Game, _ *appconf.AppConfig) (Info, error) {
	return Info{
		Supported: false,
		Message:   "Wine/Proton 快捷工具仅支持 Linux",
	}, nil
}

func openPlatformTool(_ context.Context, _ models.Game, _ *appconf.AppConfig, _ string) (string, error) {
	return "", fmt.Errorf("Wine/Proton 快捷工具仅支持 Linux")
}
