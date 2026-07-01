package launcher

import (
	"context"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"lunabox/internal/utils/timerutils"
	"os"
	"path/filepath"
	"strings"
)

type DetectionMode int

const (
	DetectionStaged DetectionMode = iota
	DetectionLauncherOnly
	DetectionSteamDirectory
)

type ActiveTrack = timerutils.ActiveTrack

const (
	ActiveTrackDefault     = timerutils.ActiveTrackDefault
	ActiveTrackBundlePath  = timerutils.ActiveTrackBundlePath
	ActiveTrackWineRootPID = timerutils.ActiveTrackWineRootPID
	ActiveTrackLauncherPID = timerutils.ActiveTrackLauncherPID
)

type LaunchPlan struct {
	File          string
	Args          []string
	Dir           string
	Env           []string
	DetectionDir  string
	DetectionMode DetectionMode
	DisplayName   string
	ActiveTrack   ActiveTrack
	Magpie        bool
	RunAsAdmin    bool
}

// LaunchOptions defines optional game launch overrides.
type LaunchOptions struct {
	UseLocaleEmulator *bool
	UseMagpie         *bool
	RunAsAdmin        *bool
	WineRunner        *string
	WineArgs          *string
	WinePrefix        *string
	UseSteam          *bool
}

type LauncherStrategy interface {
	Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error)
}

type StrategyError struct {
	Kind        string
	ConfigKey   string
	UserMessage string
	Detail      string
}

func (e *StrategyError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Detail) != "" {
		return fmt.Sprintf("%s: %s", e.UserMessage, e.Detail)
	}
	return e.UserMessage
}

func newStrategyError(kind string, configKey string, userMessage string, detail string) *StrategyError {
	return &StrategyError{
		Kind:        strings.TrimSpace(kind),
		ConfigKey:   strings.TrimSpace(configKey),
		UserMessage: strings.TrimSpace(userMessage),
		Detail:      strings.TrimSpace(detail),
	}
}

func SelectLauncherStrategy(game *models.Game, opts LaunchOptions, cfg *appconf.AppConfig) (LauncherStrategy, error) {
	if game == nil {
		return nil, fmt.Errorf("game is nil")
	}
	return selectPlatformLauncherStrategy(game, opts, cfg)
}

func ShouldUseSteamLaunch(game *models.Game, opts LaunchOptions) bool {
	if game == nil {
		return false
	}
	if opts.UseSteam != nil {
		return *opts.UseSteam
	}
	if game.SourceType != "steam" || strings.TrimSpace(game.SourceID) == "" {
		return false
	}
	path := strings.TrimSpace(game.Path)
	if path == "" {
		return true
	}
	if info, err := os.Stat(path); err == nil {
		return info.IsDir()
	}
	return false
}

func EffectiveBool(option *bool, fallback bool) bool {
	if option != nil {
		return *option
	}
	return fallback
}

func EffectiveString(option *string, fallback string) string {
	if option != nil {
		return strings.TrimSpace(*option)
	}
	return strings.TrimSpace(fallback)
}

func ProcessDetectionDir(path string) string {
	return filepath.Dir(path)
}

func buildStagedWindowsPlan(file string, args []string, dir string, displayName string, useMagpie bool, runAsAdmin bool) LaunchPlan {
	if strings.TrimSpace(displayName) == "" {
		displayName = filepath.Base(file)
	}
	return LaunchPlan{
		File:          file,
		Args:          args,
		Dir:           dir,
		DetectionDir:  dir,
		DetectionMode: DetectionStaged,
		DisplayName:   displayName,
		Magpie:        useMagpie,
		RunAsAdmin:    runAsAdmin,
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackDefault,
		},
	}
}
