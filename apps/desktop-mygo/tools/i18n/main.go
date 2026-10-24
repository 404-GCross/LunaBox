// i18n 把原版前端的语言包转换成 ui/locales 的 MyGo 目录，再运行 mygo generate 写出 catalog_gen.go；
// 同时把原版的 VNDB 标签译名（utils/vndbTagTranslations.<语言>.json）原样复制到 ui/tagname/data。
//
//	go run ./tools/i18n          # 转换并生成
//	go run ./tools/i18n -check   # 只检查 ui/locales 与 ui/tagname/data 是否与原版一致（CI、提交前）
//
// 在 apps/desktop-mygo 下运行。见 docs/i18n.md。
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"lunabox/desktop-mygo/tools/i18n/catalogs"
)

// tagLanguages 是有 VNDB 标签译名的语言（与 tagname.Languages 相同；不导入 tagname，它嵌入的数据由这里写出）。
var tagLanguages = []string{"ja-JP", "zh-CN", "zh-TW"}

func main() {
	check := flag.Bool("check", false, "只检查 ui/locales 与 ui/tagname/data 是否与原版一致，不写文件")
	flag.Parse()

	if err := syncTagNames(*check); err != nil {
		fail(err)
	}

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

// syncTagNames 把原版的 VNDB 标签译名复制到 ui/tagname/data/<语言>.json；check 时只比较。
func syncTagNames(check bool) error {
	for _, lang := range tagLanguages {
		src := filepath.Join("..", "desktop", "frontend", "src", "utils", "vndbTagTranslations."+lang+".json")
		dst := filepath.Join("ui", "tagname", "data", lang+".json")
		want, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		have, _ := os.ReadFile(dst)
		switch {
		case bytes.Equal(want, have):
		case check:
			return fmt.Errorf("%s 与原版 %s 不一致，运行 go run ./tools/i18n 重新复制", dst, src)
		default:
			if err := os.WriteFile(dst, want, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "i18n:", err)
	os.Exit(1)
}
