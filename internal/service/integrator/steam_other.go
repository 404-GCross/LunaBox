//go:build !windows

package integrator

import (
	"context"
	"fmt"
	"lunabox/internal/models"
)

func resolveSteamPlatformTarget(_ context.Context, _ models.Game) (SteamResult, error) {
	return SteamResult{
		Status: SteamLaunchStatus{
			State: SteamLaunchStateSteamNotInstalled,
		},
	}, nil
}

func importSteamPlatformShortcut(_ context.Context, _ models.Game) (SteamResult, error) {
	return SteamResult{}, fmt.Errorf("Steam integration is only supported on Windows")
}
