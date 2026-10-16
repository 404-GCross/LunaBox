// Package launch 对应原版 components/panel/GameLaunchPanel.tsx（详情页“启动配置”标签页），它在 routes/game.tsx 里
// 接收的回调（默认启动方式、Steam 设置、进程选择与 ProcessSelectModal、导出快捷方式），
// 以及详情页的启动游戏（handleStartGame、Steam 导入流程与 SteamImportModal，store 的 startGame、gameRuntimes）。
// 原型只在 Windows 上运行：只还原 goos === "windows" 时的界面与行为，macOS、Linux 专属的 Wine、Proton 部分不迁移。
package launch

import (
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/service"
)

// State 是面板跨帧保留的状态，由详情页持有；进入另一个游戏时清零。
type State struct {
	gameID string // 后台结果回来时游戏已换就丢弃

	launchOptionsSaving bool // isSteamLaunchOptionsSaving
	steamRestarting     bool // isSteamRestarting
	restartConfirm      bool // isSteamRestartConfirmOpen

	process processModal // ProcessSelectModal（原版由详情页持有）
}

// launchModeOptions 是原版 Windows 上的默认启动方式选项（launchModeOptions）。
func launchModeOptions() []better.SelectOption {
	return []better.SelectOption{
		{Label: locales.GameLaunchLaunchModeNormal(), Value: string(enums.LaunchModeNormal)},
		{Label: locales.GameLaunchLaunchModeAdmin(), Value: string(enums.LaunchModeAdmin)},
		{Label: locales.GameLaunchLaunchModeSteam(), Value: string(enums.LaunchModeSteam)},
	}
}

// steamLaunchOptionPresets 对应 steamLaunchOptionPresets（Windows 上也显示，与原版一致）；
// 按钮文字是 gameLaunch.steamLaunchOptionsPresets.<key>。
var steamLaunchOptionPresets = []struct{ key, value string }{
	{"chineseLocale", "LANG=zh_CN.UTF-8 %command%"},
	{"fullChineseLocale", "LC_ALL=zh_CN.UTF-8 LANG=zh_CN.UTF-8 %command%"},
	{"protonLog", "PROTON_LOG=1 %command%"},
}

// effectiveLaunchMode 对应面板里的 launchMode：Windows 上没有兼容层启动，按普通启动显示。
func effectiveLaunchMode(m enums.LaunchMode) enums.LaunchMode {
	switch m {
	case enums.LaunchModeAdmin, enums.LaunchModeSteam:
		return m
	}
	return enums.LaunchModeNormal
}

// fileName 对应 path.split(/[\\/]/).pop()。
func fileName(path string) string {
	if i := strings.LastIndexAny(path, `\/`); i >= 0 {
		return path[i+1:]
	}
	return path
}

// View 构建面板：space-y-6 的三张卡片——进程监控、Steam 启动器（Steam 启动时）、启动增强工具。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Game) {
	if p.Game == nil {
		return
	}
	if s.gameID != p.Game.ID {
		*s = State{gameID: p.Game.ID}
	}
	l := launcherFor(p.Env)
	l.latest = p
	h := &handlers{s: s, p: p, l: l}
	steam := effectiveLaunchMode(p.Game.LaunchMode) == enums.LaunchModeSteam

	ui.Column(c.Key("launch-panel")).Gap(24).Children(func() {
		h.processCard(c, t)
		if steam {
			h.steamCard(c, t)
		}
		h.enhancementCard(c, t, steam)
	})

	components.ConfirmModal(c, t, "steam-restart", components.ConfirmProps{
		Open:        &s.restartConfirm,
		Title:       locales.NativeSteamRestartConfirmTitle(),
		Message:     locales.NativeSteamRestartConfirmMessage(),
		ConfirmText: locales.GameLaunchSteamRestart(),
		TopOffset:   panel.TopOffset,
		OnConfirm:   func() { h.restartSteam(c) },
	})
	s.process.view(c, t, l, p)
}

// handlers 是面板与详情页传给它的回调（handleDefaultLaunchModeChange 等）。
type handlers struct {
	s *State
	p panel.Game
	l *Launcher
}

func (h *handlers) toast(c *ui.Context, msg string) { h.p.Env.Toast(c, msg) }

// current 报告后台结果回来时面板是否仍是同一个游戏。
func (h *handlers) current(id string) bool { return h.s.gameID == id }

// processCard：进程监控卡片（space-y-6）：默认启动方式、可执行程序、实际游戏进程、导出快捷启动方式。
func (h *handlers) processCard(c *ui.Context, t *theme.Theme) {
	g := h.p.Game
	winW, _ := c.Size()
	card(c, t, "process-card", 24).Children(func() {
		// 标题下的空 <p className="mt-1"> 没有高度，外边距与 space-y-6 合并，不占位置。
		cardTitle(c, t, locales.GameLaunchProcessMonitor(), 0)

		ui.Column(c).Children(func() {
			fieldLabel(c, t, locales.GameLaunchLaunchMode(), 4)
			better.Select(c, t, "launch-mode", better.SelectProps{
				Value:    string(effectiveLaunchMode(g.LaunchMode)),
				Options:  launchModeOptions(),
				OnChange: func(v string) { h.launchModeChange(c, enums.LaunchMode(v)) },
			})
		})

		// glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-brand-50
		// dark:bg-brand-700 text-brand-900 dark:text-white font-mono break-all text-sm；提示 mt-1 text-xs text-brand-500。
		ui.Column(c).Children(func() {
			fieldLabel(c, t, locales.GameLaunchExecutable(), 4)
			name := locales.GameLaunchNoPathSet()
			if g.Path != "" {
				name = fileName(g.Path)
			}
			ui.Column(c.Key("executable")).Padding(8, 12).Radius(6).Border(t.HL, t.Pick(theme.Brand300, theme.Brand600)).
				Background(t.Pick(theme.Brand50, theme.Brand700)).Children(func() {
				monoText(c, name, theme.TextSM, theme.WeightNormal, t.TextStrong)
			})
			components.TextMargin(c, locales.GameLaunchExecutableHint(),
				theme.TextXS, theme.WeightNormal, theme.Brand500, 4, 0, 0, 0)
		})

		// flex flex-col gap-2 sm:flex-row：BetterActionInput（flex-1）与“选择运行进程”。
		ui.Column(c).Children(func() {
			fieldLabel(c, t, locales.GameLaunchActualProcess(), 4)
			wide := winW >= 640
			var row ui.Element
			if wide {
				row = ui.Row(c.Key("process-row")).Gap(8).AlignItems(ui.Start)
			} else {
				row = ui.Column(c.Key("process-row")).Gap(8)
			}
			row.Children(func() {
				value := g.ProcessName
				in := actionInput(c, t, "process-name", &value, locales.GameLaunchActualProcess(), "", func(r ui.Element) {
					if wide {
						r.Grow(1).Basis(0)
					} else {
						r.FillWidth()
					}
				}, inputAction{icon: "file-search-outline", label: locales.GameLaunchSelectProcessFile(), onClick: func() { h.selectProcessExecutable(c) }})
				if in.Changed() {
					h.l.change(g.ID, func(pg *models.Game) { pg.ProcessName = value })
				}
				emptyIconButton(c, t, "select-running-process", locales.GameLaunchSelectRunningProcess()).OnClick(func() { h.s.process.show(c, h.p) })
			})
			components.TextMargin(c, locales.GameLaunchProcessHint(),
				theme.TextXS, theme.WeightNormal, theme.Brand500, 4, 0, 0, 0)
		})

		// glass-panel 里 flex flex-col gap-3 md:flex-row md:items-center md:justify-between。
		glassPanel(c, t, "export-shortcut").Children(func() {
			wide := winW >= 768
			var row ui.Element
			if wide {
				row = ui.Row(c).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween)
			} else {
				row = ui.Column(c).Gap(12)
			}
			row.Children(func() {
				text := ui.Column(c).MinWidth(0)
				if wide {
					text.Grow(1).Basis(0)
				}
				text.Children(func() {
					components.Text(c, locales.GameLaunchExportShortcut(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand200))
					components.TextMargin(c, locales.GameLaunchExportShortcutHint(),
						textXSRelax, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
				})
				better.Button(c, t, "export-shortcut-button", better.ButtonProps{
					Variant: better.ButtonPrimary, Icon: "link-variant", Label: locales.GameLaunchExportShortcut(),
				}).OnClick(func() { h.exportShortcut(c) })
			})
		})
	})
}

// steamCard：Steam 启动器卡片（space-y-5）。标题行 pb-2 flex items-center justify-between gap-3，右侧 ghost sm 的刷新按钮；
// 下面 space-y-3 的 Steam 启动参数、提示与预设按钮（flex-wrap gap-2）。Windows 上没有 Proton 部分。
func (h *handlers) steamCard(c *ui.Context, t *theme.Theme) {
	g := h.p.Game
	card(c, t, "steam-card", 20).Children(func() {
		ui.Row(c).Padding(0, 0, 8, 0).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				cardTitle(c, t, locales.GameLaunchSteamTools(), 0)
				components.TextMargin(c, locales.GameLaunchSteamToolsHint(), theme.TextXS, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
			})
			// isLoading 跟随 Steam 兼容层的读取，只在 Linux 上发生。
			better.Button(c, t, "steam-refresh", better.ButtonProps{
				Variant: better.ButtonGhost, Size: better.ButtonSM, Icon: "refresh", Disabled: h.s.steamRestarting,
			}).Label(locales.GameLaunchSteamProtonRefresh()).OnClick(func() { h.refreshSteamSettings() })
		})
		ui.Column(c).Gap(12).Children(func() {
			fieldLabel(c, t, locales.GameLaunchSteamLaunchOptions(), 0)
			value := g.SteamLaunchOptions
			in := actionInput(c, t, "steam-launch-options", &value, locales.GameLaunchSteamLaunchOptions(), locales.GameLaunchSteamLaunchOptionsPlaceholder(), func(r ui.Element) { r.FillWidth() },
				inputAction{icon: "content-save-outline", label: locales.GameLaunchSteamLaunchOptionsSave(), loading: h.s.launchOptionsSaving, onClick: func() { h.saveSteamLaunchOptions(c) }})
			if in.Changed() {
				h.l.change(g.ID, func(pg *models.Game) { pg.SteamLaunchOptions = value })
			}
			components.Text(c, locales.GameLaunchSteamLaunchOptionsHint(),
				textXSRelax, theme.WeightNormal, t.TextMuted)
			ui.Row(c).Wrap().GapX(8).GapY(8).Children(func() {
				for _, preset := range steamLaunchOptionPresets {
					better.Button(c, t, "preset-"+preset.key, better.ButtonProps{
						Size: better.ButtonSM, Icon: "plus-circle-outline", Label: locales.Key("gameLaunch.steamLaunchOptionsPresets." + preset.key),
					}).OnClick(func() {
						h.l.change(g.ID, func(pg *models.Game) { pg.SteamLaunchOptions = preset.value })
					})
				}
			})
		})
	})
}

// enhancementCard：启动增强工具卡片（space-y-6），Steam 启动时没有 Locale Emulator。
func (h *handlers) enhancementCard(c *ui.Context, t *theme.Theme, steam bool) {
	g := h.p.Game
	cfg := h.p.Env.Config()
	hasLE := cfg != nil && cfg.LocaleEmulatorPath != ""
	hasMagpie := cfg != nil && cfg.MagpiePath != ""
	card(c, t, "enhancement-card", 24).Children(func() {
		cardTitle(c, t, locales.GameLaunchEnhancementTools(), 8)
		if !steam {
			h.toolRow(c, t, "use_locale_emulator", "Locale Emulator", locales.GameLaunchLeDesc(),
				locales.GameLaunchLeNotConfigured(), hasLE, g.UseLocaleEmulator, func(on bool) {
					if on && !hasLE {
						h.toast(c, locales.GameLaunchToastLePathRequired())
						return
					}
					h.l.change(g.ID, func(pg *models.Game) { pg.UseLocaleEmulator = on })
				})
		}
		h.toolRow(c, t, "use_magpie", "Magpie", locales.GameLaunchMagpieDesc(),
			locales.GameLaunchMagpieNotConfigured(), hasMagpie, g.UseMagpie, func(on bool) {
				if on && !hasMagpie {
					h.toast(c, locales.GameLaunchToastMagpiePathRequired())
					return
				}
				h.l.change(g.ID, func(pg *models.Game) { pg.UseMagpie = on })
			})
	})
}

// toolRow：flex items-center justify-between；左侧 min-w-0 pr-4 的名称（text-sm font-medium）、说明（mt-1 text-xs）
// 与未配置提示（mt-1 flex items-center gap-1 text-xs text-error-500，i-mdi-alert-circle text-sm），右侧 BetterSwitch。
func (h *handlers) toolRow(c *ui.Context, t *theme.Theme, key, name, desc, missing string, configured, checked bool, onChange func(bool)) {
	ui.Row(c.Key(key)).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Padding(0, 16, 0, 0).Children(func() {
			components.Text(c, name, theme.TextSM, theme.WeightMedium, t.SectionLabel)
			components.TextMargin(c, desc, theme.TextXS, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
			if !configured {
				ui.Row(c).Margin(4, 0, 0, 0).Gap(4).AlignItems(ui.Center).Children(func() {
					components.Icon(c, "alert-circle", 14, theme.Error500)
					components.Text(c, missing, theme.TextXS, theme.WeightNormal, theme.Error500)
				})
			}
		})
		ui.Box(c).Shrink(0).Children(func() {
			better.Switch(c, t, "switch", checked, !configured, onChange)
		})
	})
}

// launchModeChange 对应 handleDefaultLaunchModeChange：Steam 要先确认 Steam 关联，其余直接改游戏。
func (h *handlers) launchModeChange(c *ui.Context, mode enums.LaunchMode) {
	g := h.p.Game
	if mode != enums.LaunchModeSteam {
		h.l.change(g.ID, func(pg *models.Game) { pg.LaunchMode = mode })
		return
	}
	h.l.resolveSteam(c, steamSaveDefault, *g, func(sg models.Game) {
		h.l.steam.action = steamNone
		h.l.saveSteamAsDefault(sg)
	})
}

// selectProcessExecutable 对应 handleSelectProcessExecutable：选择文件，取文件名作为进程名。
func (h *handlers) selectProcessExecutable(c *ui.Context) {
	id, path, e := h.p.Game.ID, h.p.Game.Path, h.p.Env
	e.Async(func() func() {
		selection, err := e.Svc.Game.SelectGameExecutable(path)
		return func() {
			if err != nil {
				h.toast(c, locales.GameToastSelectFileFailed())
				return
			}
			if name := fileName(selection); selection != "" && name != "" {
				h.l.change(id, func(pg *models.Game) { pg.ProcessName = name })
			}
		}
	})
}

// exportShortcut 对应 handleExportLaunchShortcut：保存对话框取消时什么也不提示。
func (h *handlers) exportShortcut(c *ui.Context) {
	id, e := h.p.Game.ID, h.p.Env
	e.Async(func() func() {
		path, err := e.Svc.Game.ExportLaunchShortcut(id)
		return func() {
			switch {
			case err != nil:
				h.toast(c, locales.GameLaunchToastShortcutExportFailed(err.Error()))
			case path != "":
				h.toast(c, locales.GameLaunchToastShortcutExportSuccess(path))
			}
		}
	})
}

// refreshSteamSettings 对应面板与详情页的 handleRefreshSteamSettings：重新读取游戏，只合并 Steam 相关字段。
// Windows 上不读取 Steam 兼容层，失败时的 steamCompatibilityError 也不显示。
func (h *handlers) refreshSteamSettings() {
	id, e := h.p.Game.ID, h.p.Env
	e.Async(func() func() {
		refreshed, err := e.Svc.Game.GetGameByID(id)
		return func() {
			if err == nil {
				h.l.change(id, func(pg *models.Game) { mergeSteamSettings(pg, refreshed) })
			}
		}
	})
}

// steamLaunchOptionsStatusError 对应详情页的 steamLaunchOptionsStatusError。
func steamLaunchOptionsStatusError(status service.SteamLaunchStatus) string {
	switch status.State {
	case "steam_running":
		return locales.NativeSteamLaunchOptionsSteamRunning()
	case "steam_not_installed":
		return locales.GameLaunchSteamProtonNotInstalled()
	case "executable_required":
		return locales.SteamImportExecutableRequired()
	case "user_unavailable":
		return locales.SteamImportUserUnavailable()
	}
	state := status.State
	if state == "" {
		state = "unknown"
	}
	return locales.NativeSteamLaunchOptionsNotReady(state)
}

// saveSteamLaunchOptions 对应面板的 handleSaveSteamLaunchOptions 与详情页的同名回调：SetGameSteamLaunchOptions，
// 未就绪时按状态报错；成功后合并重新读到的 Steam 设置，Steam 正在运行时询问是否重启。
func (h *handlers) saveSteamLaunchOptions(c *ui.Context) {
	s := h.s
	if s.launchOptionsSaving {
		return
	}
	s.launchOptionsSaving = true
	id, options, e := h.p.Game.ID, h.p.Game.SteamLaunchOptions, h.p.Env
	e.Async(func() func() {
		status, err := e.Svc.Integration.SetGameSteamLaunchOptions(id, options)
		statusSet := err == nil
		if err == nil && !status.Ready {
			err = errors.New(steamLaunchOptionsStatusError(status))
		}
		var refreshed models.Game
		if err == nil {
			refreshed, err = e.Svc.Game.GetGameByID(id)
		}
		return func() {
			if statusSet {
				h.l.steam.status = &status
			}
			if !h.current(id) {
				return
			}
			s.launchOptionsSaving = false
			if err != nil {
				h.toast(c, locales.GameLaunchToastSteamSettingsSaveFailed(err.Error()))
				return
			}
			h.l.change(id, func(pg *models.Game) { mergeSteamSettings(pg, refreshed) })
			h.toast(c, locales.GameLaunchToastSteamSettingsSaved())
			if status.SteamRunning {
				s.restartConfirm = true
			}
		}
	})
}

// restartSteam 对应 handleRestartSteam：RestartSteamClient 后刷新 Steam 设置。
func (h *handlers) restartSteam(c *ui.Context) {
	s := h.s
	if s.steamRestarting {
		return
	}
	s.steamRestarting = true
	id, e := h.p.Game.ID, h.p.Env
	e.Async(func() func() {
		err := e.Svc.Integration.RestartSteamClient()
		return func() {
			if !h.current(id) {
				return
			}
			s.steamRestarting = false
			if err != nil {
				h.toast(c, locales.GameLaunchToastSteamRestartFailed(err.Error()))
				return
			}
			h.toast(c, locales.GameLaunchToastSteamRestarted())
			h.refreshSteamSettings()
		}
	})
}
