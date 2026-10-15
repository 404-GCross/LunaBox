// Package assets 内嵌原版前端用到的图标与图片：icons/ 是从 @iconify-json/mdi 导出的 i-mdi-* 图标，
// 其余 PNG 复制自 apps/desktop/frontend/src/assets。
package assets

import (
	"bytes"
	"embed"
	"image"
	_ "image/png"
	"sync"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"
)

//go:embed icons importers *.png
var files embed.FS

var (
	iconsMu sync.Mutex
	icons   = map[string]*ui.SVG{}
)

// Icon 按 i-mdi-* 的名字（不含前缀）返回图标，例如 Icon("gamepad-variant")。
func Icon(name string) *ui.SVG {
	iconsMu.Lock()
	defer iconsMu.Unlock()
	s, ok := icons[name]
	if !ok {
		data, err := files.ReadFile("icons/" + name + ".svg")
		if err != nil {
			panic("assets: missing icon " + name + "，从 @iconify-json/mdi 导出到 ui/assets/icons")
		}
		s = ui.MustParseSVG(data)
		icons[name] = s
	}
	return s
}

// 窗口控制按钮的图标，沿用 TopBar.tsx 里的内联 SVG。
var (
	Minimise = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none"><path d="M0 6h12" stroke="currentColor" stroke-width="1.5"/></svg>`))
	Maximise = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none"><path d="M1 1h10v10H1V1z" stroke="currentColor" stroke-width="1.5"/></svg>`))
	Restore  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none"><path d="M3 3h6v6H3V3z" stroke="currentColor" stroke-width="1.5"/><path d="M5 1h6v6" stroke="currentColor" stroke-width="1.5"/></svg>`))
	Close    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none"><path d="M1 1l10 10M11 1L1 11" stroke="currentColor" stroke-width="1.5"/></svg>`))
)

type bitmaps struct {
	appIcon, appIconDark  *ui.Bitmap
	titleLight, titleDark *ui.Bitmap // titleLight 用于浅色主题（深色文字）
	importers             map[string]*ui.Bitmap
}

var loadBitmaps = sync.OnceValue(func() *bitmaps {
	b := &bitmaps{
		appIcon:     bitmap("appicon.png", 96),
		appIconDark: bitmap("appicon-dark.png", 96),
		titleLight:  bitmap("topbar-title-dark.png", 260),
		titleDark:   bitmap("topbar-title.png", 260),
		importers:   map[string]*ui.Bitmap{},
	}
	for _, name := range []string{"potatovn", "playnite", "yukihub", "vnite", "reinamanager"} {
		b.importers[name] = bitmap("importers/"+name+".png", 72)
	}
	return b
})

// AppIcon 是侧边栏的应用图标。
func AppIcon(dark bool) *ui.Bitmap {
	if dark {
		return loadBitmaps().appIconDark
	}
	return loadBitmaps().appIcon
}

// Title 是顶栏与侧边栏的“LunaBox”字标。
func Title(dark bool) *ui.Bitmap {
	if dark {
		return loadBitmaps().titleDark
	}
	return loadBitmaps().titleLight
}

// Importer 返回导入来源的图标：potatovn、playnite、yukihub、vnite、reinamanager。
func Importer(name string) *ui.Bitmap { return loadBitmaps().importers[name] }

// bitmap 解码内嵌图片并缩到 maxWidth 像素宽，避免 1024px 的图标常驻显存。
func bitmap(name string, maxWidth int) *ui.Bitmap {
	data, err := files.ReadFile(name)
	if err != nil {
		panic(err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	return ui.NewBitmap(Downscale(img, maxWidth))
}

// Downscale 等比缩小到最大宽度（双线性）。
func Downscale(img image.Image, maxWidth int) image.Image {
	b := img.Bounds()
	if b.Dx() <= maxWidth {
		return img
	}
	h := b.Dy() * maxWidth / b.Dx()
	dst := image.NewRGBA(image.Rect(0, 0, maxWidth, h))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}
