//go:build !linux

package wailsruntime

import "github.com/wailsapp/wails/v3/pkg/application"

func setAutostart(app *application.App, enabled bool) error {
	if app == nil || app.Autostart == nil {
		return ErrUnavailable
	}
	if !enabled {
		return app.Autostart.Disable()
	}
	return app.Autostart.EnableWithOptions(application.AutostartOptions{
		Arguments: []string{AutostartLaunchArgument},
	})
}
