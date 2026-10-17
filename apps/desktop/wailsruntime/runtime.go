// Package wailsruntime implements appruntime.Runtime with the Wails v3
// application and main window.
package wailsruntime

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"

	"lunabox/backend/appruntime"
)

const wailsDialogCancelledMessage = "cancelled by user"

// AutostartLaunchArgument lets LunaBox distinguish a login launch from a
// normal user-initiated launch.
const AutostartLaunchArgument = appruntime.AutostartLaunchArgument

type applicationRuntime struct {
	app    *application.App
	window *application.WebviewWindow
}

// New creates a runtime adapter backed by the Wails v3 application and main
// window objects.
func New(app *application.App, window *application.WebviewWindow) appruntime.Runtime {
	return &applicationRuntime{app: app, window: window}
}

func (r *applicationRuntime) Emit(name string, data ...any) bool {
	if r == nil || r.app == nil {
		return false
	}
	return r.app.Event.Emit(name, data...)
}

func (r *applicationRuntime) OpenURL(targetURL string) error {
	if r == nil || r.app == nil {
		return appruntime.ErrUnavailable
	}
	return r.app.Browser.OpenURL(targetURL)
}

func (r *applicationRuntime) SetAutostart(enabled bool) error {
	if r == nil || r.app == nil || r.app.Autostart == nil {
		return appruntime.ErrUnavailable
	}
	if !enabled {
		return r.app.Autostart.Disable()
	}
	return r.app.Autostart.EnableWithOptions(application.AutostartOptions{
		Arguments: []string{AutostartLaunchArgument},
	})
}

func (r *applicationRuntime) RestoreWindow() {
	if r != nil && r.window != nil {
		r.window.Restore()
	}
}

func (r *applicationRuntime) ShowWindow() {
	if r != nil && r.window != nil {
		r.window.Show()
	}
}

func (r *applicationRuntime) OpenFile(options appruntime.OpenDialogOptions) (string, error) {
	if r == nil || r.app == nil {
		return "", appruntime.ErrUnavailable
	}

	dialog := r.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		CanChooseFiles:                  true,
		CanChooseDirectories:            false,
		CanCreateDirectories:            false,
		ShowHiddenFiles:                 options.ShowHiddenFiles,
		ResolvesAliases:                 options.ResolvesAliases,
		TreatsFilePackagesAsDirectories: options.TreatsFilePackagesAsDirectories,
		Title:                           options.Title,
		Directory:                       options.Directory,
		Filters:                         applicationFileFilters(options.Filters),
		Window:                          r.window,
	})
	return normalizeDialogSelection(dialog.PromptForSingleSelection())
}

func (r *applicationRuntime) OpenDirectory(options appruntime.OpenDialogOptions) (string, error) {
	if r == nil || r.app == nil {
		return "", appruntime.ErrUnavailable
	}

	dialog := r.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		CanChooseFiles:                  false,
		CanChooseDirectories:            true,
		CanCreateDirectories:            true,
		ShowHiddenFiles:                 options.ShowHiddenFiles,
		ResolvesAliases:                 options.ResolvesAliases,
		TreatsFilePackagesAsDirectories: options.TreatsFilePackagesAsDirectories,
		Title:                           options.Title,
		Directory:                       options.Directory,
		Filters:                         applicationFileFilters(options.Filters),
		Window:                          r.window,
	})
	return normalizeDialogSelection(dialog.PromptForSingleSelection())
}

func (r *applicationRuntime) SaveFile(options appruntime.SaveDialogOptions) (string, error) {
	if r == nil || r.app == nil {
		return "", appruntime.ErrUnavailable
	}

	dialog := r.app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		CanCreateDirectories:            true,
		ShowHiddenFiles:                 options.ShowHiddenFiles,
		TreatsFilePackagesAsDirectories: options.TreatsFilePackagesAsDirectories,
		Title:                           options.Title,
		Directory:                       options.Directory,
		Filename:                        options.Filename,
		Filters:                         applicationFileFilters(options.Filters),
		Window:                          r.window,
	})
	return normalizeDialogSelection(dialog.PromptForSingleSelection())
}

func applicationFileFilters(filters []appruntime.FileFilter) []application.FileFilter {
	result := make([]application.FileFilter, 0, len(filters))
	for _, filter := range filters {
		result = append(result, application.FileFilter{
			DisplayName: filter.DisplayName,
			Pattern:     filter.Pattern,
		})
	}
	return result
}

func normalizeDialogSelection(selection string, err error) (string, error) {
	if err == nil {
		return selection, nil
	}

	// Wails v3 alpha keeps its cancellation sentinel in an internal package,
	// so the adapter cannot use errors.Is with the original value.
	for current := err; current != nil; current = errors.Unwrap(current) {
		if current.Error() == wailsDialogCancelledMessage {
			return "", nil
		}
	}

	return "", err
}
