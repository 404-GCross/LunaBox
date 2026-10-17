// desktop-mygo 是 LunaBox 用 MyGo 原生 UI 重写的原型：界面在 ui 包里一比一还原顶栏、侧边栏、游戏库与游戏详情，
// 数据通过 apps/desktop 的真实服务读写原型自己的数据目录（默认 build/data，首次运行从安装版复制）。
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/backend"
	"lunabox/desktop-mygo/ui"
	"lunabox/desktop-mygo/ui/env"
	_ "lunabox/desktop-mygo/ui/locales" // 注册界面的多语言目录（ui/locales），它的语言就是应用的语言
	"lunabox/internal/appconf"
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
	flagLang      = flag.String("lang", "", "本次运行显示的语言（zh-CN、zh-TW、en-US、ja-JP，或伪语言 en-XA、ar-XB），不写进配置")
)

func main() {
	flag.Parse()
	if *flagLang != "" {
		os.Setenv("MYGO_LOCALE", *flagLang) // 同 mygo dev -lang：只影响这次运行
	}
	svc, err := openBackend(context.Background(), *flagDataDir, *flagReseed)
	if err != nil {
		log.Fatal(err)
	}
	if svc.Config != nil {
		env.ApplyLanguage(svc.Config.Language)
		env.ApplyTheme(svc.Config.Theme)
	}
	// 设置页安排恢复后调用 SafeQuit 退出，下次启动时执行恢复（原版的 SetQuitHandler 还要先跑退出前的云同步，原型不启动云同步）。
	svc.AppConfig.SetQuitHandler(mygo.App.Quit)
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
	mygo.App.OnQuit(func() {
		svc.Close()
		// 同 apps/desktop/main.go 退出时的 save final config：设置页安排的恢复（PendingDBRestore、PendingFullRestore）
		// 与最近一次备份时间等只写在内存里的配置，退出时才保存。
		if err := appconf.SaveConfig(svc.Config); err != nil {
			log.Printf("save config: %v", err)
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
