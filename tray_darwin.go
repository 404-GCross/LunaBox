//go:build darwin

package main

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa

void lunaboxTrayStart(const char *iconBytes, int iconLength);
void lunaboxTrayStop(void);
*/
import "C"

import "unsafe"

func (s *lifecycleState) StartTray() {
	var iconPtr *C.char
	iconBytes := appIcon
	if len(iconBytes) == 0 {
		iconBytes = icon
	}
	if len(iconBytes) > 0 {
		iconPtr = (*C.char)(unsafe.Pointer(&iconBytes[0]))
	}
	C.lunaboxTrayStart(iconPtr, C.int(len(iconBytes)))
}

func (s *lifecycleState) RequestTrayQuit() {
	s.trayQuitOnce.Do(func() {
		C.lunaboxTrayStop()
	})
}

//export lunaboxTrayReady
func lunaboxTrayReady() {
	appState.MarkTrayReady()
}

//export lunaboxTrayExit
func lunaboxTrayExit() {
	appState.MarkTrayExit()
}

//export lunaboxTrayShowMainWindow
func lunaboxTrayShowMainWindow() {
	appState.ShowMainWindow()
}

//export lunaboxTrayQuitApplication
func lunaboxTrayQuitApplication() {
	if shouldRunFrontendQuitSync(config) {
		appState.RequestFrontendQuitSync("tray-menu")
		return
	}

	appState.QuitApplication()
}
