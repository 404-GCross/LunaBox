// desktop-mygo 是 LunaBox 用 MyGo 原生 UI 重写的原型：界面在 ui 包里一比一还原顶栏、侧边栏和游戏库，
// 数据通过 apps/desktop 的真实服务从数据库临时拷贝读取。
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui"
	"lunabox/internal/version"
)

var (
	flagRepeat    = flag.Int("repeat", 1, "把游戏列表复制 N 份，用于压测大列表")
	flagBench     = flag.Bool("bench", false, "启动后自动滚动游戏库并输出帧时间报告")
	flagBenchExit = flag.Bool("bench-exit", false, "压测结束后退出（配合 -bench）")
	flagHUD       = flag.Bool("hud", false, "显示性能 HUD（F2 切换）")
	flagShot      = flag.String("screenshot", "", "封面加载完成后把窗口截图保存到该 PNG 路径并退出")
	flagDataDir   = flag.String("data-dir", "", "便携版数据目录；为空时读取安装版数据目录（%APPDATA%/LunaBox）")
	flagWidth     = flag.Int("width", 1280, "窗口宽度（DIP）")
	flagHeight    = flag.Int("height", 800, "窗口高度（DIP）")
	flagState     = flag.String("state", "", "截图调试：启动时进入的界面状态（drawer、addmenu、batch、collapsed）")
)

func main() {
	flag.Parse()
	if *flagDataDir != "" {
		os.Setenv("LUNABOX_PORTABLE_ROOT", *flagDataDir)
	} else {
		version.BuildMode = "installer"
	}

	b, err := openBackend(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	var backend ui.Backend = b
	if *flagRepeat > 1 {
		backend = repeatedBackend{backend: b, n: *flagRepeat}
	}
	app := ui.New(backend, ui.Options{HUD: *flagHUD, Bench: *flagBench, BenchExit: *flagBenchExit})
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
		app.Attach(win)
		if *flagShot != "" {
			go app.Screenshot(*flagShot)
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
