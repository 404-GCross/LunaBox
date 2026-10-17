package better

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// EdgePlacement 对应 BetterEdgeIconButton 的 placement。
type EdgePlacement int

const (
	EdgeBottom EdgePlacement = iota // h-8 w-12 rounded-t-lg，图标 text-xl
	EdgeLeft                        // h-16 w-10 rounded-r-xl，图标 text-3xl
	EdgeRight                       // h-16 w-10 rounded-l-xl，图标 text-3xl
)

// EdgeIconButtonProps 是 BetterEdgeIconButton 的属性。
type EdgeIconButtonProps struct {
	Placement EdgePlacement
	Icon      string
	Label     string // aria-label
	// Blur 是 blurClassName 的背景模糊半径，0 是默认的 backdrop-blur-md（12px）。
	Blur float32
	// Surface、SurfaceHover 是 surfaceClassName 的背景色；Surface 为零值时是默认的
	// bg-white/40 hover:bg-white/65 dark:bg-black/35 dark:hover:bg-black/55。
	Surface, SurfaceHover ui.Color
	OnClick               func()
}

// EdgeIconButton 对应 BetterEdgeIconButton：贴在容器边缘的半透明毛玻璃图标按钮，border border-white/30
// dark:border-white/15，text-brand-700 dark:text-brand-200，opacity-75 hover:opacity-100 active:scale-95
// （transition-all duration-200）。返回按钮，调用方负责定位（相当于传 className）。
func EdgeIconButton(c *ui.Context, t *theme.Theme, key string, p EdgeIconButtonProps) ui.Element {
	w, h, icon := float32(48), float32(32), float32(20)
	if p.Placement != EdgeBottom {
		w, h, icon = 40, 64, 30
	}
	blur := p.Blur
	if blur == 0 {
		blur = 12
	}
	btn := ui.ButtonBase(c.Key(key)).Size(w, h).Center().Shrink(0).FocusRing(false).Label(p.Label).
		Material(glass.Blur{Radius: blur}).Border(t.HL, t.Pick(theme.White.Alpha(0.3), theme.White.Alpha(0.15)))
	switch p.Placement {
	case EdgeLeft:
		btn.Radius(0, 12, 12, 0)
	case EdgeRight:
		btn.Radius(12, 0, 0, 12)
	default:
		btn.Radius(8, 8, 0, 0)
	}
	hovered := btn.Hovered()
	bg, bgHover := p.Surface, p.SurfaceHover
	if bg == (ui.Color{}) {
		bg, bgHover = t.Pick(theme.White.Alpha(0.4), theme.Black.Alpha(0.35)), t.Pick(theme.White.Alpha(0.65), theme.Black.Alpha(0.55))
	}
	opacity := float32(0.75)
	if hovered {
		bg, opacity = bgHover, 1
	}
	scale := float32(1)
	if btn.Pressed() {
		scale = 0.95
	}
	sc := btn.AnimateWith("press", scale, 200*time.Millisecond, theme.EaseTailwind)
	btn.Background(bg).Opacity(btn.AnimateWith("opacity", opacity, 200*time.Millisecond, theme.EaseTailwind)).Scale(sc, sc).
		Transition(theme.ColorsTransition(200 * time.Millisecond))
	if p.OnClick != nil {
		btn.OnClick(p.OnClick)
	}
	btn.Children(func() {
		components.Icon(c, p.Icon, icon, t.Pick(theme.Brand700, theme.Brand200))
	})
	return btn
}
