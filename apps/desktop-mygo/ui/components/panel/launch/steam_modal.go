package launch

import (
	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// steamContent 是 SteamImportModal 按 status.state 显示的图标、标题与说明。
type steamContent struct {
	icon, title, message string
}

// steamModalContent 对应 SteamImportModal 的 content 表；canRestartSteam 只在 Linux 为真，这里按 Windows 取值。
func steamModalContent(state, gameName string) steamContent {
	switch state {
	case "checking":
		return steamContent{"loading", locales.SteamImportCheckingTitle(), locales.SteamImportCheckingMessage()}
	case "needs_import":
		return steamContent{"steam", locales.SteamImportTitle(), locales.SteamImportNeedsImport(gameName)}
	case "steam_running":
		return steamContent{"steam", locales.SteamImportSteamRunningTitle(), locales.SteamImportSteamRunning()}
	case "steam_not_installed":
		return steamContent{"alert-circle-outline", locales.SteamImportSteamMissingTitle(), locales.SteamImportSteamMissing()}
	case "executable_required":
		return steamContent{"file-alert-outline", locales.SteamImportExecutableRequiredTitle(), locales.SteamImportExecutableRequired()}
	case "user_unavailable":
		return steamContent{"account-alert-outline", locales.SteamImportUserUnavailableTitle(), locales.SteamImportUserUnavailable()}
	case "ready":
		return steamContent{"check-circle-outline", locales.SteamImportReadyTitle(), locales.SteamImportReady()}
	}
	return steamContent{"alert-circle-outline", locales.SteamImportTitle(), locales.SteamImportUnknownState()}
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
					components.Text(c, locales.SteamImportLocalIdentityNote(),
						textXSRelax, theme.WeightNormal, t.TextMuted).Grow(1).Basis(0).MinWidth(0)
				})
			}
			ui.Row(c).Margin(28, 0, 0, 0).Wrap().GapX(12).GapY(12).Justify(ui.End).Children(func() {
				better.Button(c, t, "steam-cancel", better.ButtonProps{Label: locales.CommonCancel(), Disabled: busy}).OnClick(l.closeSteam)
				switch state {
				case "needs_import":
					better.Button(c, t, "steam-import", better.ButtonProps{
						Variant: better.ButtonPrimary, Icon: "plus", Label: locales.SteamImportImport(), Loading: f.importing,
					}).OnClick(func() { l.importSteam(c, l.gameOr(&g)) })
				case "executable_required":
					better.Button(c, t, "steam-select-exe", better.ButtonProps{
						Variant: better.ButtonPrimary, Icon: "file-search-outline", Label: locales.SteamImportSelectExecutable(), Disabled: busy,
					}).OnClick(func() { l.steamSelectExecutable(c, l.gameOr(&g)) })
				case "steam_running", "user_unavailable":
					better.Button(c, t, "steam-retry", better.ButtonProps{
						Variant: better.ButtonPrimary, Icon: "refresh", Label: locales.SteamImportRetry(), Loading: f.checking,
					}).OnClick(func() { l.retrySteam(c, l.gameOr(&g)) })
				}
			})
		})
	})
}
