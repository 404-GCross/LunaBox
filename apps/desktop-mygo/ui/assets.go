package ui

import (
	"bytes"
	"embed"
	"image"
	_ "image/png"

	"github.com/egoist/mygo/ui"
)

//go:embed assets
var assetsFS embed.FS

func svgAsset(name string) *ui.SVG {
	data, err := assetsFS.ReadFile("assets/icons/" + name + ".svg")
	if err != nil {
		panic(err)
	}
	return ui.MustParseSVG(data)
}

// bitmapAsset 解码内嵌图片并缩到 maxWidth 像素宽，避免 1024px 的图标常驻显存。
func bitmapAsset(name string, maxWidth int) *ui.Bitmap {
	data, err := assetsFS.ReadFile("assets/" + name)
	if err != nil {
		panic(err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	return ui.NewBitmap(downscale(img, maxWidth))
}

// 窗口控制按钮的图标，沿用 TopBar.tsx 里的内联 SVG。
var (
	iconMinimise = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none"><path d="M0 6h12" stroke="currentColor" stroke-width="1.5"/></svg>`))
	iconMaximise = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none"><path d="M1 1h10v10H1V1z" stroke="currentColor" stroke-width="1.5"/></svg>`))
	iconRestore  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none"><path d="M3 3h6v6H3V3z" stroke="currentColor" stroke-width="1.5"/><path d="M5 1h6v6" stroke="currentColor" stroke-width="1.5"/></svg>`))
	iconClose    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none"><path d="M1 1l10 10M11 1L1 11" stroke="currentColor" stroke-width="1.5"/></svg>`))
)

// icons 按 i-mdi-* 的名字取图标。
type icons map[string]*ui.SVG

func (m icons) get(name string) *ui.SVG {
	s, ok := m[name]
	if !ok {
		s = svgAsset(name)
		m[name] = s
	}
	return s
}

type assets struct {
	appIcon, appIconDark  *ui.Bitmap
	titleLight, titleDark *ui.Bitmap // titleLight 用于浅色主题（深色文字）
	importers             map[string]*ui.Bitmap
	icons                 icons
}

func loadAssets() *assets {
	a := &assets{
		appIcon:     bitmapAsset("appicon.png", 96),
		appIconDark: bitmapAsset("appicon-dark.png", 96),
		titleLight:  bitmapAsset("topbar-title-dark.png", 260),
		titleDark:   bitmapAsset("topbar-title.png", 260),
		importers:   map[string]*ui.Bitmap{},
		icons:       icons{},
	}
	for _, name := range []string{"potatovn", "playnite", "yukihub", "vnite", "reinamanager"} {
		a.importers[name] = bitmapAsset("importers/"+name+".png", 72)
	}
	return a
}

func (a *App) icon(name string) *ui.SVG { return a.assets.icons.get(name) }

// downscale 等比缩小到最大宽度。
func downscale(img image.Image, maxWidth int) image.Image {
	b := img.Bounds()
	if b.Dx() <= maxWidth {
		return img
	}
	h := b.Dy() * maxWidth / b.Dx()
	dst := image.NewRGBA(image.Rect(0, 0, maxWidth, h))
	scaleBilinear(dst, img)
	return dst
}
