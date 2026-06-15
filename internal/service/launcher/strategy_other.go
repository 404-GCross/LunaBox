//go:build !windows && !darwin

package launcher

import (
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
)

func selectPlatformLauncherStrategy(game *models.Game, opts LaunchOptions, cfg *appconf.AppConfig) (LauncherStrategy, error) {
	return nil, fmt.Errorf("launcher strategies are only supported on Windows and macOS")
}
