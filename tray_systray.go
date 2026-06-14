//go:build !darwin

package main

import (
	goruntime "runtime"

	"github.com/energye/systray"
)

func (s *lifecycleState) StartTray() {
	go func() {
		goruntime.LockOSThread()
		defer goruntime.UnlockOSThread()
		systray.Run(onSystrayReady, onSystrayExit)
	}()
}

func (s *lifecycleState) RequestTrayQuit() {
	s.trayQuitOnce.Do(func() {
		systray.Quit()
	})
}

func onSystrayReady() {
	systray.SetIcon(icon)
	systray.SetTitle("LunaBox")
	systray.SetTooltip("LunaBox")

	systray.SetOnClick(func(menu systray.IMenu) {
		appState.ShowMainWindow()
	})

	systray.SetOnDClick(func(menu systray.IMenu) {
		appState.ShowMainWindow()
	})

	mShow := systray.AddMenuItem("显示主窗口", "显示 LunaBox 主窗口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出 LunaBox")

	mShow.Click(func() {
		appState.ShowMainWindow()
	})

	mQuit.Click(func() {
		if shouldRunFrontendQuitSync(config) {
			appState.RequestFrontendQuitSync("tray-menu")
			return
		}

		appState.QuitApplication()
	})

	appState.MarkTrayReady()
}

func onSystrayExit() {
	appState.MarkTrayExit()
}
