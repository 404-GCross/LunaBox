package app

import (
	_ "embed"

	"github.com/egoist/mygo"

	"lunabox/desktop-mygo/ui/locales"
)

// trayIcon 是 Wails 版的 build/windows/tray.png（32x32）。
//
//go:embed tray.png
var trayIcon []byte

// setupTray 创建通知区域图标，对应 Wails 版的 configureTray：单击显示主窗口，右键菜单是“显示主窗口”“退出”。
// 必须在 WhenReady 里调用。
func (a *app) setupTray() {
	show := &mygo.MenuItem{Label: locales.NativeTrayShowMainWindow(), Click: func(*mygo.MenuItem, *mygo.Window) { a.core.ShowMainWindow() }}
	quit := &mygo.MenuItem{Label: locales.NativeTrayQuit(), Click: func(*mygo.MenuItem, *mygo.Window) { a.core.RequestQuit("tray-menu") }}
	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:    trayIcon,
		ToolTip: "LunaBox",
		Menu:    mygo.NewMenu([]*mygo.MenuItem{show, mygo.Separator(), quit}),
	})
	if err != nil {
		a.logger.Error("failed to create tray icon: " + err.Error())
		return
	}
	tray.OnClick(a.core.ShowMainWindow)
	// 设置页切换语言后菜单随之更新（Wails 版的菜单文字是写死的中文）。
	a.relabelTray = func() {
		show.SetLabel(locales.NativeTrayShowMainWindow())
		quit.SetLabel(locales.NativeTrayQuit())
	}
	a.tray = tray
	a.core.SetTrayAvailable()
}
