// desktop-mygo 是 LunaBox 用 MyGo 原生 UI 重写的原型：界面在 ui 包里一比一还原顶栏、侧边栏、游戏库与游戏详情，
// 数据通过 apps/desktop 的真实服务读写原型自己的数据目录（默认 build/data，首次运行从安装版复制）。
package main

import (
	"context"
	"flag"
	"log"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans" // MyGo 自带的文字（输入框菜单、对话框标题等）的简体中文
	mygoui "github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/backend"
	"lunabox/desktop-mygo/ui"
)

var (
	flagRepeat    = flag.Int("repeat", 1, "把游戏列表复制 N 份，用于压测大列表")
	flagBench     = flag.Bool("bench", false, "启动后自动滚动游戏库并输出帧时间报告")
	flagBenchExit = flag.Bool("bench-exit", false, "压测结束后退出（配合 -bench）")
	flagHUD       = flag.Bool("hud", false, "显示性能 HUD（F2 切换）")
	flagShot      = flag.String("screenshot", "", "封面加载完成后把窗口截图保存到该 PNG 路径并退出")
	flagDataDir   = flag.String("data-dir", "", "原型的数据目录；为空时用 build/data（首次运行从 %APPDATA%/LunaBox 复制）")
	flagReseed    = flag.Bool("reseed", false, "重新从 %APPDATA%/LunaBox 复制设置、数据库、封面与备份，覆盖原型的数据")
	flagWidth     = flag.Int("width", 1280, "窗口宽度（DIP）")
	flagHeight    = flag.Int("height", 800, "窗口高度（DIP）")
	flagState     = flag.String("state", "", "截图调试：启动时进入的界面状态（drawer、addmenu、batch、collapsed，或详情页的 tab:edit 等）")
	flagRoute     = flag.String("route", "/library", "启动时显示的页面路径")
)

func main() {
	flag.Parse()
	mygo.App.SetLanguages("zh-Hans") // 同原版只有简体中文界面
	svc, err := openBackend(context.Background(), *flagDataDir, *flagReseed)
	if err != nil {
		log.Fatal(err)
	}
	var list ui.Backend = listBackend{svc}
	if *flagRepeat > 1 {
		list = repeatedBackend{listBackend: listBackend{svc}, n: *flagRepeat}
	}
	app := ui.New(list, svc, ui.Options{Route: *flagRoute, HUD: *flagHUD, Bench: *flagBench, BenchExit: *flagBenchExit})
	app.SetDebugState(*flagState)

	mygo.App.WhenReady(func() {
		bg := "#f3f4f6"
		if mygo.Theme.IsDark() {
			bg = "#121416"
		}
		opts := mygo.WindowOptions{
			Title:           "LunaBox",
			Width:           *flagWidth,
			Height:          *flagHeight,
			MinWidth:        960,
			MinHeight:       600,
			Frameless:       true,
			BackgroundColor: bg,
			Content:         mygoui.View(app.View),
		}
		if *flagShot == "" {
			opts.StateKey = "main" // 截图时固定尺寸，不恢复上次的窗口大小
		}
		win := mygo.NewWindow(opts)
		svc.Events.SetDispatcher(win.Update)
		svc.SetRuntime(&backend.Runtime{Events: svc.Events, Win: win})
		app.Attach(win)
		if *flagShot != "" {
			go app.Screenshot(*flagShot)
		}
	})
	mygo.App.OnQuit(func() { svc.Close() })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
