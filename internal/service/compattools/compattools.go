// Package compattools resolves and opens the Wine/Proton helper tools offered
// for a game: the prefix and drive_c directories, regedit, winecfg, explorer
// and a command prompt. Steam Proton games go through protontricks, other
// Proton setups launch Proton directly.
package compattools

import (
	"context"
	"fmt"
	"strings"

	"lunabox/internal/appconf"
	"lunabox/internal/models"
)

const (
	ActionPrefixDir = "prefix_dir"
	ActionDriveC    = "drive_c"
	ActionRegedit   = "regedit"
	ActionWinecfg   = "winecfg"
	ActionExplorer  = "explorer"
	ActionWinecmd   = "winecmd"
)

// Info describes the Wine/Proton helper tools available for a game.
type Info struct {
	Supported             bool     `json:"supported"`
	RunnerKind            string   `json:"runner_kind"`
	PrefixPath            string   `json:"prefix_path"`
	DriveCPath            string   `json:"drive_c_path"`
	AppID                 string   `json:"app_id"`
	WinetricksPath        string   `json:"winetricks_path"`
	WinetricksSource      string   `json:"winetricks_source"`
	WinetricksAvailable   bool     `json:"winetricks_available"`
	WinetricksError       string   `json:"winetricks_error"`
	ProtontricksPath      string   `json:"protontricks_path"`
	ProtontricksSource    string   `json:"protontricks_source"`
	ProtontricksAvailable bool     `json:"protontricks_available"`
	ProtontricksError     string   `json:"protontricks_error"`
	Actions               []string `json:"actions"`
	Message               string   `json:"message"`
}

// IsAction reports whether action is one of the supported helper actions.
func IsAction(action string) bool {
	switch action {
	case ActionPrefixDir,
		ActionDriveC,
		ActionRegedit,
		ActionWinecfg,
		ActionExplorer,
		ActionWinecmd:
		return true
	default:
		return false
	}
}

// Get resolves the Wine/Proton helper tools available for game.
func Get(ctx context.Context, game models.Game, cfg *appconf.AppConfig) (Info, error) {
	return getPlatformTools(ctx, game, cfg)
}

// Open launches the helper action for game and returns the affected path when
// the action opens a directory.
func Open(ctx context.Context, game models.Game, cfg *appconf.AppConfig, action string) (string, error) {
	action = strings.TrimSpace(action)
	if !IsAction(action) {
		return "", fmt.Errorf("未知的兼容层工具动作: %s", action)
	}
	return openPlatformTool(ctx, game, cfg, action)
}

func actionAvailable(actions []string, action string) bool {
	for _, item := range actions {
		if item == action {
			return true
		}
	}
	return false
}
