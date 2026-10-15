package components

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/models"
)

// DESKTOP_MYGO_PNG=<目录> 时把表情选择器、标签、图片查看器渲染成图片，用于目测。
func TestComponentDump(t *testing.T) {
	dir := os.Getenv("DESKTOP_MYGO_PNG")
	if dir == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	save := func(tt *ui.Tester, name string) {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		_ = png.Encode(f, tt.Image())
	}
	th := theme.New(true, 1.5)
	editor := TagEditor{}
	tags := []models.GameTag{
		{ID: "1", Name: "纯爱", Source: "bangumi"},
		{ID: "2", Name: "剧透内容", Source: "vndb", IsSpoiler: true},
		{ID: "3", Name: "我的标签", Source: "user"},
	}
	value := ""
	tt := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(th.AppBg)
		ui.Column(c).Padding(24).Gap(24).Children(func() {
			GameTags(c, th, GameTagsProps{Tags: tags, Editor: &editor})
			EmojiPicker(c, th, "emoji", EmojiPickerProps{Value: value, CanEdit: true, Compact: true, FallbackIcon: "folder-outline",
				OnChange: func(e string) { value = e }})
		})
	}, 640, 600)
	tt.SetScale(1.5)
	tt.SetDark(true)
	tt.Frame()
	if err := tt.Click("选择图标"); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		tt.Frame()
	}
	save(tt, "components-emoji.png")

	img := image.NewRGBA(image.Rect(0, 0, 300, 420))
	for y := range 420 {
		for x := range 300 {
			img.Set(x, y, color.RGBA{uint8(x * 255 / 300), 120, uint8(y * 255 / 420), 255})
		}
	}
	open := true
	vt := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(th.AppBg)
		ImageViewer(c, th, "viewer", ImageViewerProps{Open: &open, Title: "夏日口袋 REFLECTION BLUE", Image: ui.NewBitmap(img), OnSave: func() {}})
	}, 1000, 700)
	vt.SetScale(1.5)
	vt.SetDark(true)
	vt.Frame()
	save(vt, "components-viewer.png")
}
