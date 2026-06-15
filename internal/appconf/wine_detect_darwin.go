//go:build darwin

package appconf

import (
	"lunabox/internal/applog"
	"os"
	"strings"
)

var crossoverWineCandidates = []string{
	"/Applications/CrossOver.app/Contents/SharedSupport/CrossOver/bin/wine",
	"/Applications/CrossOver.app/Contents/SharedSupport/CrossOver/bin/wine64",
}

func detectDefaultWineRunnerPath(config *AppConfig) bool {
	if config == nil || strings.TrimSpace(config.WineRunnerPath) != "" {
		return false
	}

	for _, candidate := range crossoverWineCandidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			config.WineRunnerPath = candidate
			applog.LogInfof(nil, "Detected CrossOver wine binary at %s", candidate)
			return true
		}
	}
	return false
}
