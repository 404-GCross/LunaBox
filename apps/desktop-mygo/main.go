// desktop-mygo 是 LunaBox 用 MyGo 原生 UI 实现的桌面应用：界面在 ui 包里一比一还原 Wails 版前端，
// 后端与 Wails 版共用 packages/backend（服务、启动、生命周期与退出在 lunabox/backend/bootstrap），
// app 包是 MyGo 的应用外壳，host 包把后端接到 MyGo。
package main

import (
	_ "embed"
	"flag"
	"log"
	"os"

	"github.com/egoist/mygo"

	"lunabox/backend/bootstrap"
	"lunabox/desktop-mygo/app"
	_ "lunabox/desktop-mygo/ui/locales" // 注册界面的多语言目录（ui/locales），它的语言就是应用的语言
)

// appIcon 是应用图标（与 Wails 版的 build/appicon.png 相同）。mygo dev、mygo build 把它嵌进 exe 的图标资源；
// go run、go build 构建的 exe 没有图标资源，启动后用它设置窗口与任务栏的图标。
//
//go:embed resources/icon.png
var appIcon []byte

var (
	flagRepeat    = flag.Int("repeat", 1, "把游戏列表复制 N 份，用于压测大列表")
	flagBench     = flag.Bool("bench", false, "启动后自动滚动游戏库并输出帧时间报告")
	flagBenchExit = flag.Bool("bench-exit", false, "压测结束后退出（配合 -bench）")
	flagHUD       = flag.Bool("hud", false, "显示性能 HUD（F2 切换）")
	flagShot      = flag.String("screenshot", "", "封面加载完成后把窗口截图保存到该 PNG 路径并退出")
	flagDataDir   = flag.String("data-dir", "", "数据目录；为空时开发运行用 build/data（首次运行从 %APPDATA%/LunaBox 复制），正式版与 Wails 版相同")
	flagReseed    = flag.Bool("reseed", false, "开发运行：重新从 %APPDATA%/LunaBox 复制设置、数据库、封面与备份，覆盖 build/data")
	flagWidth     = flag.Int("width", 0, "窗口宽度（DIP）；为 0 时用上次的窗口尺寸")
	flagHeight    = flag.Int("height", 0, "窗口高度（DIP）；为 0 时用上次的窗口尺寸")
	flagState     = flag.String("state", "", "截图调试：启动时进入的界面状态（drawer、addmenu、batch、collapsed，或详情页的 tab:edit 等）")
	flagRoute     = flag.String("route", "/library", "启动时显示的页面路径")
	flagLang      = flag.String("lang", "", "本次运行显示的语言（zh-CN、zh-TW、en-US、ja-JP，或伪语言 en-XA、ar-XB），不写进配置")
)

func main() {
	// 开机自启动项带的 --autostart（与 Wails 版相同）不是 flag 包的写法，先取出来。
	args, autostart := bootstrap.ExtractAutostartLaunchFlag(os.Args[1:])
	if err := flag.CommandLine.Parse(args); err != nil {
		os.Exit(2)
	}
	if *flagLang != "" {
		os.Setenv("MYGO_LOCALE", *flagLang) // 同 mygo dev -lang：只影响这次运行
	}
	err := app.Run(app.Options{
		Icon:       appIcon,
		Args:       flag.Args(),
		Autostart:  autostart,
		Dev:        mygo.IsDev(),
		DataDir:    *flagDataDir,
		Reseed:     *flagReseed,
		Width:      *flagWidth,
		Height:     *flagHeight,
		Screenshot: *flagShot,
		State:      *flagState,
		Route:      *flagRoute,
		Repeat:     *flagRepeat,
		HUD:        *flagHUD,
		Bench:      *flagBench,
		BenchExit:  *flagBenchExit,
	})
	if err != nil {
		log.Fatal(err)
	}
}
