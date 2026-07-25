package integrator

import (
	"context"
	"lunabox/internal/models"
)

const (
	SteamLaunchStateReady              = "ready"
	SteamLaunchStateNeedsImport        = "needs_import"
	SteamLaunchStateSteamNotInstalled  = "steam_not_installed"
	SteamLaunchStateSteamRunning       = "steam_running"
	SteamLaunchStateExecutableRequired = "executable_required"
	SteamLaunchStateUserUnavailable    = "user_unavailable"
)

type SteamLaunchStatus struct {
	State          string
	Ready          bool
	SteamInstalled bool
	SteamRunning   bool
	LaunchID       string
	LaunchKind     string
	UserID         string
}

type SteamResult struct {
	Status     SteamLaunchStatus
	Imported   bool
	BackupPath string
}

func ResolveSteamTarget(ctx context.Context, game models.Game) (SteamResult, error) {
	return resolveSteamPlatformTarget(ctx, game)
}

func ImportSteamShortcut(ctx context.Context, game models.Game) (SteamResult, error) {
	return importSteamPlatformShortcut(ctx, game)
}
