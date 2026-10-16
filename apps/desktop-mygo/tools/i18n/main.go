// i18n 把原版前端的语言包转换成 ui/locales 的 MyGo 目录，再运行 mygo generate 写出 catalog_gen.go。
//
//	go run ./tools/i18n          # 转换并生成
//	go run ./tools/i18n -check   # 只检查 ui/locales 是否与原版一致（CI、提交前）
//
// 在 apps/desktop-mygo 下运行。见 README 的“多语言”一节。
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"lunabox/desktop-mygo/tools/i18n/catalogs"
)

func main() {
	check := flag.Bool("check", false, "只检查 ui/locales 是否与原版语言包一致，不写文件")
	flag.Parse()

	res, err := catalogs.Convert(catalogs.Options{
		Frontend: filepath.Join("..", "desktop", "frontend", "src", "locales"),
		Extra:    filepath.Join("tools", "i18n", "extra"),
	})
	if err != nil {
		fail(err)
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	dir := filepath.Join("ui", "locales")
	if *check {
		if diff := res.Diff(dir); len(diff) > 0 {
			fail(fmt.Errorf("ui/locales 与原版语言包不一致：%v，运行 go run ./tools/i18n 重新生成", diff))
		}
		return
	}
	if err := res.Write(dir); err != nil {
		fail(err)
	}
	// mygo generate 检查目录并写出 catalog_gen.go；加载应用的包需要 cgo（DuckDB）。
	cmd := exec.Command("go", "tool", "mygo", "generate")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fail(fmt.Errorf("mygo generate：%w", err))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "i18n:", err)
	os.Exit(1)
}
