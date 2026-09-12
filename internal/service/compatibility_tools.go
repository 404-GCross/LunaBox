package service

import (
	"fmt"
	"strings"
)

const (
	CompatibilityActionPrefixDir = "prefix_dir"
	CompatibilityActionDriveC    = "drive_c"
	CompatibilityActionRegedit   = "regedit"
	CompatibilityActionWinecfg   = "winecfg"
	CompatibilityActionExplorer  = "explorer"
	CompatibilityActionWinecmd   = "winecmd"
)

type GameCompatibilityToolsInfo struct {
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

func (s *IntegrationService) GetGameCompatibilityTools(gameID string) (GameCompatibilityToolsInfo, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return GameCompatibilityToolsInfo{}, err
	}
	return getPlatformGameCompatibilityTools(s.ctx, game, s.config)
}

func (s *IntegrationService) OpenGameCompatibilityTool(gameID string, action string) (string, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return "", err
	}
	action = strings.TrimSpace(action)
	if !isGameCompatibilityAction(action) {
		return "", fmt.Errorf("未知的兼容层工具动作: %s", action)
	}
	return openPlatformGameCompatibilityTool(s.ctx, game, s.config, action)
}

func isGameCompatibilityAction(action string) bool {
	switch action {
	case CompatibilityActionPrefixDir,
		CompatibilityActionDriveC,
		CompatibilityActionRegedit,
		CompatibilityActionWinecfg,
		CompatibilityActionExplorer,
		CompatibilityActionWinecmd:
		return true
	default:
		return false
	}
}

func compatibilityActionAvailable(actions []string, action string) bool {
	for _, item := range actions {
		if item == action {
			return true
		}
	}
	return false
}
