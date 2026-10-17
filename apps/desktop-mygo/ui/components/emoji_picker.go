package components

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// emoji-picker-element 的分组（allGroups.slice(1)）：分组号、导航图标、名称。
var emojiGroups = []struct {
	id          int
	emoji, name string
}{
	{0, "😀", "Smileys and emoticons"},
	{1, "👋", "People and body"},
	{3, "🐱", "Animals and nature"},
	{4, "🍎", "Food and drink"},
	{5, "🏠️", "Travel and places"},
	{6, "⚽", "Activities"},
	{7, "📝", "Objects"},
	{8, "⛔️", "Symbols"},
	{9, "🏁", "Flags"},
}

// emojiFavorites 是 MOST_COMMONLY_USED_EMOJI：底部常用栏（原版之后按使用次数排序，原型不记录次数）。
var emojiFavorites = []string{"😊", "😒", "❤️", "👍️", "😍", "😂", "😭", "☺️", "😔", "😩", "😏", "💕", "🙌", "😘"}

// emoji-picker-element 的尺寸：--emoji-size 1.375rem、--emoji-padding 0.5rem、--num-columns 8。
const (
	emojiSize    = 22
	emojiCell    = emojiSize + 2*8
	emojiColumns = 8
)

// EmojiPickerProps 是 EmojiPickerPopover 的属性。
type EmojiPickerProps struct {
	Value        string
	CanEdit      bool
	Compact      bool   // h-11 w-11（否则 h-12 w-12 且右侧 mr-4）
	FallbackIcon string // 没有表情时的 i-mdi-* 图标
	System       bool   // variant="system"：红色底
	OnChange     func(emoji string)
}

type emojiPickerState struct {
	open   bool
	group  int // emojiGroups 的下标
	query  string
	tone   int // 0 默认，1–5 浅 → 深
	toneUI bool
	scroll ScrollBar
}

// EmojiPicker 对应 EmojiPickerPopover：显示当前表情（或备用图标）的方块按钮，点击后在下方弹出 w-[24rem]
// rounded-xl border p-3 shadow-lg 的面板，标题“选择图标”与“移除”，下面是 emoji-picker-element（高 320px）。
func EmojiPicker(c *ui.Context, t *theme.Theme, key string, p EmojiPickerProps) {
	size := float32(48)
	if p.Compact {
		size = 44
	}
	btn := ui.ButtonBase(c.Key(key)).Size(size, size).Radius(8).Center().Shrink(0).FocusRing(false).Label(locales.CategoriesEmojiPickerTitle())
	if !p.Compact {
		btn.Margin(0, 16, 0, 0)
	}
	st := ui.Local(btn, "emoji-picker", func() *emojiPickerState { return &emojiPickerState{} })
	s := *st
	bg, fg := t.Pick(theme.Neutral100, ui.Hex("#0f172a").Alpha(0.3)), t.Pick(theme.Neutral600, theme.Neutral400)
	if p.System {
		bg, fg = t.Pick(ui.Hex("#fee2e2"), ui.Hex("#7f1d1d").Alpha(0.3)), t.Pick(theme.Error600, theme.Error400)
	}
	if p.CanEdit && btn.Hovered() {
		bg = t.Pick(ui.Hex("#e2e8f0"), ui.Hex("#1e293b").Alpha(0.5))
	}
	btn.Background(bg)
	if p.CanEdit {
		btn.OnClick(func() { s.open = !s.open })
	} else {
		btn.Cursor(ui.CursorDefault)
		s.open = false
	}
	btn.Children(func() {
		if v := strings.TrimSpace(p.Value); v != "" {
			EmojiText(c, v, 24)
		} else {
			Icon(c, p.FallbackIcon, 24, fg)
		}
	})
	choose := func(e string) {
		s.open = false
		if p.OnChange != nil {
			p.OnChange(e)
		}
	}
	ui.PopoverBase(c, btn, &s.open, func(panel ui.Element) {
		panel.AttachTo(btn, ui.AnchorBottomLeft, ui.AnchorTopLeft).Top(8).Width(384).Padding(12).Radius(12).
			Background(t.PanelBg).Border(t.HL, t.PanelBorder).
			Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1))
		panel.Children(func() {
			ui.Row(c).Margin(0, 0, 8, 0).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				Text(c, locales.CategoriesEmojiPickerTitle(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand300))
				clear := ui.ButtonBase(c.Key("clear")).FocusRing(false)
				color := t.Pick(theme.Brand500, theme.Brand400)
				if clear.Hovered() {
					color = t.Pick(theme.Neutral600, theme.Neutral400)
				}
				clear.OnClick(func() { choose("") }).Children(func() {
					Text(c, locales.CategoriesEmojiPickerClear(), theme.TextXS, theme.WeightNormal, color)
				})
			})
			// overflow-hidden rounded-lg bg-white dark:bg-brand-800，里面是 emoji-picker-element
			ui.Box(c).Radius(8).Clip().Background(t.PanelBg).Children(func() {
				emojiPickerElement(c, t, s, choose)
			})
		})
	})
}

// emojiPickerElement 是 emoji-picker-element（LunaBox 的样式：无边框，height 320px，--border-radius 10px）：
// 搜索框与肤色按钮、分组导航与 3px 指示条、当前分组（或搜索结果）的 8 列表情、底部常用表情。
func emojiPickerElement(c *ui.Context, t *theme.Theme, s *emojiPickerState, choose func(string)) {
	text := t.Pick(ui.Hex("#121416"), ui.Hex("#f3f4f6"))
	hover := t.Pick(ui.Hex("#f3f4f6"), ui.Hex("#44484e"))
	width := float32(emojiColumns*emojiCell) + t.Gutter()
	searching := len([]rune(strings.TrimSpace(s.query))) >= 2 // MIN_SEARCH_TEXT_LENGTH

	ui.Column(c.Key("picker")).Width(width).Height(320).Children(func() {
		ui.Box(c).Height(8) // .pad-top
		// .search-row：padding-inline-start 与 padding-bottom 都是 --emoji-padding
		ui.Row(c).Padding(0, 0, 8, 8).AlignItems(ui.Center).Children(func() {
			in := ui.TextInputBase(c.Key("search"), &s.query).Grow(1).Basis(0).MinWidth(0).Padding(4).Radius(8).
				FontSize(16).FixedLineHeight(24).TextColor(text).Placeholder("Search").Label("Search").
				Background(t.Pick(theme.White, ui.Hex("#303235"))).Border(1, t.Pick(ui.Hex("#d1d5db"), ui.Hex("#4b5563")))
			if theme.UIFont != "" {
				in.Font(theme.UIFont)
			}
			if in.Focused() {
				in.Shadow(0, 0, 0, 2, t.Pick(theme.Neutral600, theme.Neutral500))
			}
			skinToneButton(c, t, s, hover)
		})
		// .nav：九个分组按钮，下面 .indicator-wrapper 里 3px 的指示条（搜索时透明）
		ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
			for i, g := range emojiGroups {
				b := ui.ButtonBase(c.Key("nav-"+g.name)).Size(emojiCell, emojiCell).Center().FocusRing(false).Label(g.name)
				b.OnClick(func() { s.group, s.query, s.scroll = i, "", ScrollBar{} }).Children(func() {
					EmojiText(c, g.emoji, emojiSize)
				})
			}
		})
		ind := ui.Box(c.Key("indicator")).Height(3).FillWidth()
		x := ind.AnimateWith("x", float32(s.group), 250*time.Millisecond, theme.EaseCSS)
		opacity := ind.AnimateWith("opacity", theme.Bool(!searching), 100*time.Millisecond, ui.Linear)
		color := t.Pick(theme.Neutral500, theme.Neutral400)
		ind.Draw(func(pt *ui.Painter, r ui.Rect) {
			w := r.W / float32(len(emojiGroups))
			pt.Fill(ui.Rect{X: r.X + x*w, Y: r.Y, W: w, H: r.H}, color.Alpha(float32(color.A)/255*opacity), 0)
		})

		// .tabpanel：当前分组或搜索结果，scrollbar-gutter: stable
		ScrollArea(c, t, "tabpanel", &s.scroll, func(w ui.Element) { w.Grow(1).Basis(0).MinHeight(0) }, func(sc ui.Element) {
			sc.Padding(0, t.Gutter(), 0, 0).Children(func() {
				var list []assets.Emoji
				label := emojiGroups[s.group].name
				if searching {
					list, label = searchEmoji(s.query), ""
				} else {
					for _, e := range assets.Emojis() {
						if e.Group == emojiGroups[s.group].id {
							list = append(list, e)
						}
					}
				}
				if label != "" {
					Text(c, label, theme.TextBase, theme.WeightNormal, text).Padding(8)
				} else if len(list) == 0 {
					Text(c, "No results", theme.TextBase, theme.WeightNormal, text).Padding(8)
				}
				emojiGrid(c, s, list, hover, choose)
			})
		})
		// .favorites：border-top 透明的一行常用表情
		ui.Row(c.Key("favorites")).Padding(0, t.Gutter(), 0, 0).Justify(ui.SpaceAround).Children(func() {
			for _, e := range emojiFavorites[:emojiColumns] {
				emojiButton(c, e, hover, func() { choose(e) })
			}
		})
	})
}

// emojiGrid：grid-template-columns repeat(8, 38px) justify-content space-around。
func emojiGrid(c *ui.Context, s *emojiPickerState, list []assets.Emoji, hover ui.Color, choose func(string)) {
	ui.Grid(c.Key("grid")).Columns(emojiColumns).Children(func() {
		for _, e := range list {
			u := e.Unicode
			if s.tone > 0 && len(e.Skins) >= s.tone {
				u = e.Skins[s.tone-1]
			}
			emojiButton(c, u, hover, func() { choose(u) }).JustifySelf(ui.Center)
		}
	})
}

// emojiButton：button.emoji 圆形 38px，悬停 --button-hover-background。
func emojiButton(c *ui.Context, e string, hover ui.Color, onClick func()) ui.Element {
	b := ui.ButtonBase(c.Key(e)).Size(emojiCell, emojiCell).Radius(999).Center().FocusRing(false).Label(e)
	if b.Hovered() {
		b.Background(hover)
	}
	b.OnClick(onClick).Children(func() { EmojiText(c, e, emojiSize) })
	return b
}

// skinToneButton 是 .skintone-button：显示当前肤色的 🖐️，点击展开六种肤色的列表。
func skinToneButton(c *ui.Context, t *theme.Theme, s *emojiPickerState, hover ui.Color) {
	tones := []string{"🖐️", "🖐🏻", "🖐🏼", "🖐🏽", "🖐🏾", "🖐🏿"}
	names := []string{"Default", "Light", "Medium-Light", "Medium", "Medium-Dark", "Dark"}
	b := ui.ButtonBase(c.Key("skintone")).Size(emojiCell, emojiCell).Radius(999).Center().FocusRing(false).
		Label("Choose a skin tone (currently " + names[s.tone] + ")")
	if b.Hovered() {
		b.Background(hover)
	}
	b.OnClick(func() { s.toneUI = !s.toneUI }).Children(func() { EmojiText(c, tones[s.tone], emojiSize) })
	ui.PopoverBase(c, b, &s.toneUI, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorBottomLeft, ui.AnchorTopLeft).Radius(19).Background(t.PanelBg).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.1))
		panel.Children(func() {
			for i, tone := range tones {
				o := ui.ButtonBase(c.Key("tone-"+names[i])).Size(emojiCell, emojiCell).Radius(999).Center().FocusRing(false).Label(names[i])
				if o.Hovered() || i == s.tone {
					o.Background(hover)
				}
				o.OnClick(func() { s.tone, s.toneUI = i, false }).Children(func() { EmojiText(c, tone, emojiSize) })
			}
		})
	})
}

// searchEmoji 对应 getEmojiBySearchQuery：查询拆成词，每个词都要是名称或标签里某个词的前缀。
func searchEmoji(query string) []assets.Emoji {
	words := strings.Fields(strings.ToLower(query))
	var out []assets.Emoji
	for _, e := range assets.Emojis() {
		tokens := append(strings.Fields(strings.ToLower(e.Annotation)), e.Tags...)
		match := true
		for _, w := range words {
			found := false
			for _, tk := range tokens {
				if strings.HasPrefix(tk, w) {
					found = true
					break
				}
			}
			if !found {
				match = false
				break
			}
		}
		if match {
			out = append(out, e)
		}
	}
	return out
}

// EmojiText 用彩色表情字体显示表情（--emoji-font-family 在 Windows 上是 Segoe UI Emoji）。
func EmojiText(c *ui.Context, s string, size float32) ui.Element {
	return ui.Text(c, s).Font("Segoe UI Emoji").FontSize(size).FixedLineHeight(size).NoWrap()
}
