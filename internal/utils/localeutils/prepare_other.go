//go:build !linux

package localeutils

import (
	"context"
	"fmt"
)

func PrepareLaunchEnvironment(_ context.Context, _ string, _ string) (LaunchEnvironment, error) {
	return LaunchEnvironment{}, fmt.Errorf("Linux locale emulator is only supported on Linux")
}
