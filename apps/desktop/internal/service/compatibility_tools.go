package service

import "lunabox/internal/service/compattools"

// GameCompatibilityToolsInfo 描述某个游戏可用的 Wine/Proton 快捷工具。
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

// GetGameCompatibilityTools 返回指定游戏可用的 Wine/Proton 快捷工具。
func (s *IntegrationService) GetGameCompatibilityTools(gameID string) (GameCompatibilityToolsInfo, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return GameCompatibilityToolsInfo{}, err
	}
	info, err := compattools.Get(s.ctx, game, s.config)
	if err != nil {
		return GameCompatibilityToolsInfo{}, err
	}
	return gameCompatibilityToolsInfoFromCompattools(info), nil
}

// OpenGameCompatibilityTool 打开指定游戏的 Wine/Proton 快捷工具。
func (s *IntegrationService) OpenGameCompatibilityTool(gameID string, action string) (string, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return "", err
	}
	return compattools.Open(s.ctx, game, s.config, action)
}

func gameCompatibilityToolsInfoFromCompattools(info compattools.Info) GameCompatibilityToolsInfo {
	return GameCompatibilityToolsInfo{
		Supported:             info.Supported,
		RunnerKind:            info.RunnerKind,
		PrefixPath:            info.PrefixPath,
		DriveCPath:            info.DriveCPath,
		AppID:                 info.AppID,
		WinetricksPath:        info.WinetricksPath,
		WinetricksSource:      info.WinetricksSource,
		WinetricksAvailable:   info.WinetricksAvailable,
		WinetricksError:       info.WinetricksError,
		ProtontricksPath:      info.ProtontricksPath,
		ProtontricksSource:    info.ProtontricksSource,
		ProtontricksAvailable: info.ProtontricksAvailable,
		ProtontricksError:     info.ProtontricksError,
		Actions:               info.Actions,
		Message:               info.Message,
	}
}
