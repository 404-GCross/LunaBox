package launch

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/models"
)

// steamContent 是 SteamImportModal 按 status.state 显示的图标、标题与说明。
type steamContent struct {
	icon, title, message string
}

// steamModalContent 对应 SteamImportModal 的 content 表；canRestartSteam 只在 Linux 为真，这里按 Windows 取值。
func steamModalContent(state, gameName string) steamContent {
	switch state {
	case "checking":
		return steamContent{"loading", "正在检查 Steam", "正在查询本机 Steam 库和当前账号的快捷方式。"}
	case "needs_import":
		return steamContent{"steam", "添加到 Steam", "《" + gameName + "》尚未添加到当前 Steam 账号。LunaBox 可以将该游戏写入 Steam 的非 Steam 游戏列表。"}
	case "steam_running":
		return steamContent{"steam", "请先退出 Steam", "Steam 当前正在运行。请从托盘完整退出 Steam 后重试，避免 Steam 退出时覆盖快捷方式文件。"}
	case "steam_not_installed":
		return steamContent{"alert-circle-outline", "未检测到 Steam", "请先安装 Steam，或确认当前系统账号能够访问 Steam 安装信息。"}
	case "executable_required":
		return steamContent{"file-alert-outline", "需要游戏可执行程序", "添加非 Steam 游戏需要一个可执行文件。请选择该游戏实际使用的启动程序。"}
	case "user_unavailable":
		return steamContent{"account-alert-outline", "无法识别 Steam 账号", "请启动 Steam 并登录目标账号，然后退出 Steam 再重试。"}
	case "ready":
		return steamContent{"check-circle-outline", "Steam 关联可用", "该游戏已经可以通过 Steam 启动。"}
	}
	return steamContent{"alert-circle-outline", "添加到 Steam", "Steam 返回了无法识别的状态，请稍后重试。"}
}

// steamModal 对应 SteamImportModal：glass-card w-full max-w-lg rounded-2xl border border-brand-200 bg-white p-6
// shadow-2xl dark:border-brand-700 dark:bg-brand-800。左侧 h-11 w-11 rounded-xl bg-neutral-100 text-neutral-700
// dark:bg-neutral-900/30 dark:text-neutral-300 的图标块，标题 text-xl font-bold，说明 mt-2 text-sm leading-relaxed；
// 需要导入时有 mt-5 rounded-xl border bg-brand-50/80 p-4 的说明框；按钮 mt-7 flex-wrap justify-end gap-3。
// 遮罩不响应点击与 Esc（原版没有）。
func (l *Launcher) steamModal(c *ui.Context, t *theme.Theme, g models.Game) {
	f := &l.steam
	state := "checking"
	if f.status != nil && f.status.State != "" {
		state = f.status.State
	}
	content := steamModalContent(state, g.Name)
	busy := f.checking || f.importing
	components.Modal(c, t, "steam-import", components.ModalProps{Open: f.open, TopOffset: panel.TopOffset, MaxWidth: 512}, func(p ui.Element) {
		p.Radius(16).Padding(24).Background(t.Pick(theme.White, theme.Brand800)).Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).
			Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25))
		p.Children(func() {
			ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
				fg := t.Pick(theme.Neutral700, theme.Neutral300)
				ui.Box(c).Size(44, 44).Shrink(0).Center().Radius(12).Background(t.Pick(theme.Neutral100, neutral900.Alpha(0.3))).Children(func() {
					if content.icon == "loading" {
						components.Spinner(c, 24, fg)
					} else {
						components.Icon(c, content.icon, 24, fg)
					}
				})
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					components.Text(c, content.title, theme.TextXL, theme.WeightBold, t.TextStrong)
					components.TextMargin(c, content.message, textSMRelax, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400), 8, 0, 0, 0)
				})
			})
			if state == "needs_import" {
				ui.Row(c).Margin(20, 0, 0, 0).Padding(16).Gap(12).Radius(12).
					Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).
					Background(t.Pick(theme.Brand50.Alpha(0.8), theme.Brand900.Alpha(0.3))).Children(func() {
					components.Icon(c, "information-outline", 18, theme.Brand500).Margin(2, 0, 0, 0)
					components.Text(c, "本机 Steam 关联与游戏元数据来源相互独立，并且仅保存在此设备。原有封面、标签和元数据来源保持原样。",
						textXSRelax, theme.WeightNormal, t.TextMuted).Grow(1).Basis(0).MinWidth(0)
				})
			}
			ui.Row(c).Margin(28, 0, 0, 0).Wrap().GapX(12).GapY(12).Justify(ui.End).Children(func() {
				components.Button(c, t, "steam-cancel", components.ButtonProps{Label: "取消", Disabled: busy}).OnClick(l.closeSteam)
				switch state {
				case "needs_import":
					components.Button(c, t, "steam-import", components.ButtonProps{
						Variant: components.ButtonPrimary, Icon: "plus", Label: "添加到 Steam", Loading: f.importing,
					}).OnClick(func() { l.importSteam(c, l.gameOr(&g)) })
				case "executable_required":
					components.Button(c, t, "steam-select-exe", components.ButtonProps{
						Variant: components.ButtonPrimary, Icon: "file-search-outline", Label: "选择可执行程序", Disabled: busy,
					}).OnClick(func() { l.steamSelectExecutable(c, l.gameOr(&g)) })
				case "steam_running", "user_unavailable":
					components.Button(c, t, "steam-retry", components.ButtonProps{
						Variant: components.ButtonPrimary, Icon: "refresh", Label: "重新检查", Loading: f.checking,
					}).OnClick(func() { l.retrySteam(c, l.gameOr(&g)) })
				}
			})
		})
	})
}
