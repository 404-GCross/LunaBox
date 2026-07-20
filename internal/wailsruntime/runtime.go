package wailsruntime

import (
	"context"
	"errors"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var errNotInitialised = errors.New("Wails application runtime is not initialised")

var state struct {
	sync.RWMutex
	app    *application.App
	window *application.WebviewWindow
}

// FileFilter describes one native file-dialog filter.
type FileFilter struct {
	DisplayName string
	Pattern     string
}

// OpenDialogOptions keeps the service-facing dialog API independent from Wails.
type OpenDialogOptions struct {
	DefaultDirectory           string
	DefaultFilename            string
	Title                      string
	Filters                    []FileFilter
	ShowHiddenFiles            bool
	CanCreateDirectories       bool
	ResolvesAliases            bool
	TreatPackagesAsDirectories bool
}

// SaveDialogOptions keeps the service-facing dialog API independent from Wails.
type SaveDialogOptions struct {
	DefaultDirectory           string
	DefaultFilename            string
	Title                      string
	Filters                    []FileFilter
	ShowHiddenFiles            bool
	CanCreateDirectories       bool
	TreatPackagesAsDirectories bool
}

func Initialise(app *application.App, window *application.WebviewWindow) {
	state.Lock()
	defer state.Unlock()
	state.app = app
	state.window = window
}

func current() (*application.App, *application.WebviewWindow, error) {
	state.RLock()
	defer state.RUnlock()
	if state.app == nil {
		return nil, nil, errNotInitialised
	}
	return state.app, state.window, nil
}

func EventsEmit(_ context.Context, name string, data ...interface{}) {
	app, _, err := current()
	if err != nil {
		return
	}
	app.Event.Emit(name, data...)
}

func BrowserOpenURL(_ context.Context, targetURL string) {
	app, _, err := current()
	if err != nil {
		return
	}
	app.Browser.OpenURL(targetURL)
}

func WindowUnminimise(_ context.Context) {
	_, window, err := current()
	if err == nil && window != nil {
		window.Restore()
	}
}

func WindowShow(_ context.Context) {
	_, window, err := current()
	if err == nil && window != nil {
		window.Show()
	}
}

func OpenFileDialog(_ context.Context, options OpenDialogOptions) (string, error) {
	app, window, err := current()
	if err != nil {
		return "", err
	}
	dialog := configureOpenDialog(app.Dialog.OpenFile(), window, options)
	return dialog.PromptForSingleSelection()
}

func OpenDirectoryDialog(_ context.Context, options OpenDialogOptions) (string, error) {
	app, window, err := current()
	if err != nil {
		return "", err
	}
	dialog := configureOpenDialog(app.Dialog.OpenFile(), window, options)
	dialog.CanChooseDirectories(true).CanChooseFiles(false)
	return dialog.PromptForSingleSelection()
}

func SaveFileDialog(_ context.Context, options SaveDialogOptions) (string, error) {
	app, window, err := current()
	if err != nil {
		return "", err
	}

	filters := make([]application.FileFilter, 0, len(options.Filters))
	for _, filter := range options.Filters {
		filters = append(filters, application.FileFilter{
			DisplayName: filter.DisplayName,
			Pattern:     filter.Pattern,
		})
	}
	dialog := app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		CanCreateDirectories:            true,
		ShowHiddenFiles:                 options.ShowHiddenFiles,
		TreatsFilePackagesAsDirectories: options.TreatPackagesAsDirectories,
		Title:                           options.Title,
		Directory:                       options.DefaultDirectory,
		Filename:                        options.DefaultFilename,
		Filters:                         filters,
		Window:                          window,
	})
	return dialog.PromptForSingleSelection()
}

func configureOpenDialog(
	dialog *application.OpenFileDialogStruct,
	window *application.WebviewWindow,
	options OpenDialogOptions,
) *application.OpenFileDialogStruct {
	if window != nil {
		dialog.AttachToWindow(window)
	}
	if options.Title != "" {
		dialog.SetTitle(options.Title)
	}
	if options.DefaultDirectory != "" {
		dialog.SetDirectory(options.DefaultDirectory)
	}
	if options.ShowHiddenFiles {
		dialog.ShowHiddenFiles(true)
	}
	if options.CanCreateDirectories {
		dialog.CanCreateDirectories(true)
	}
	if options.ResolvesAliases {
		dialog.ResolvesAliases(true)
	}
	if options.TreatPackagesAsDirectories {
		dialog.TreatsFilePackagesAsDirectories(true)
	}
	for _, filter := range options.Filters {
		dialog.AddFilter(filter.DisplayName, filter.Pattern)
	}
	return dialog
}
