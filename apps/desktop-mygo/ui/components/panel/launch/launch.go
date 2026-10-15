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
	"lunabox/desktop-mygo/ui/components/panel"
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

// 原版 Windows 上的默认启动方式选项（launchModeOptions）。
var launchModeOptions = []components.SelectOption{
	{Label: "普通启动", Value: string(enums.LaunchModeNormal)},
	{Label: "管理员启动", Value: string(enums.LaunchModeAdmin)},
	{Label: "Steam 启动", Value: string(enums.LaunchModeSteam)},
}

// steamLaunchOptionPresets 对应 steamLaunchOptionPresets（Windows 上也显示，与原版一致）。
var steamLaunchOptionPresets = []struct{ key, label, value string }{
	{"chineseLocale", "强制中文环境", "LANG=zh_CN.UTF-8 %command%"},
	{"fullChineseLocale", "完整中文 Locale", "LC_ALL=zh_CN.UTF-8 LANG=zh_CN.UTF-8 %command%"},
	{"protonLog", "开启 Proton 日志", "PROTON_LOG=1 %command%"},
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
		Title:       "重启 Steam 使配置生效？",
		Message:     "Steam 正在运行，LunaBox 已写入配置。是否立即重启 Steam，让刚保存的启动参数或 Proton 设置生效？",
		ConfirmText: "重启 Steam",
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
		cardTitle(c, t, "进程监控", 0)

		ui.Column(c).Children(func() {
			fieldLabel(c, t, "默认启动方式", 4)
			components.Select(c, t, "launch-mode", components.SelectProps{
				Value:    string(effectiveLaunchMode(g.LaunchMode)),
				Options:  launchModeOptions,
				OnChange: func(v string) { h.launchModeChange(c, enums.LaunchMode(v)) },
			})
		})

		// glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-brand-50
		// dark:bg-brand-700 text-brand-900 dark:text-white font-mono break-all text-sm；提示 mt-1 text-xs text-brand-500。
		ui.Column(c).Children(func() {
			fieldLabel(c, t, "可执行程序", 4)
			name := "未设置游戏路径"
			if g.Path != "" {
				name = fileName(g.Path)
			}
			ui.Column(c.Key("executable")).Padding(8, 12).Radius(6).Border(t.HL, t.Pick(theme.Brand300, theme.Brand600)).
				Background(t.Pick(theme.Brand50, theme.Brand700)).Children(func() {
				monoText(c, name, theme.TextSM, theme.WeightNormal, t.TextStrong)
			})
			components.TextMargin(c, "LunaBox 使用此可执行文件启动游戏。这里是您一开始选择的可执行文件路径",
				theme.TextXS, theme.WeightNormal, theme.Brand500, 4, 0, 0, 0)
		})

		// flex flex-col gap-2 sm:flex-row：BetterActionInput（flex-1）与“选择运行进程”。
		ui.Column(c).Children(func() {
			fieldLabel(c, t, "实际游戏进程", 4)
			wide := winW >= 640
			var row ui.Element
			if wide {
				row = ui.Row(c.Key("process-row")).Gap(8).AlignItems(ui.Start)
			} else {
				row = ui.Column(c.Key("process-row")).Gap(8)
			}
			row.Children(func() {
				value := g.ProcessName
				in := actionInput(c, t, "process-name", &value, "实际游戏进程", "", func(r ui.Element) {
					if wide {
						r.Grow(1).Basis(0)
					} else {
						r.FillWidth()
					}
				}, inputAction{icon: "file-search-outline", label: "选择进程文件", onClick: func() { h.selectProcessExecutable(c) }})
				if in.Changed() {
					h.l.change(g.ID, func(pg *models.Game) { pg.ProcessName = value })
				}
				emptyIconButton(c, t, "select-running-process", "选择运行进程").OnClick(func() { h.s.process.show(c, h.p) })
			})
			components.TextMargin(c, "指定实际游戏的进程名称（包含 .exe 后缀）。如果启动文件属于启动器，可先启动游戏，再通过“选择运行进程”从当前进程列表中指定实际游戏，保存后重新启动游戏。",
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
					components.Text(c, "导出快捷启动方式", theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand200))
					components.TextMargin(c, "导出一个 `.url` 快捷方式文件。双击后会唤起 LunaBox，并按当前游戏配置启动和监控该游戏。",
						textXSRelax, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
				})
				components.Button(c, t, "export-shortcut-button", components.ButtonProps{
					Variant: components.ButtonPrimary, Icon: "link-variant", Label: "导出快捷启动方式",
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
				cardTitle(c, t, "Steam 启动器", 0)
				components.TextMargin(c, "读取 Steam 的兼容工具设置，可为此游戏指定 Proton 版本。", theme.TextXS, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
			})
			// isLoading 跟随 Steam 兼容层的读取，只在 Linux 上发生。
			components.Button(c, t, "steam-refresh", components.ButtonProps{
				Variant: components.ButtonGhost, Size: components.ButtonSM, Icon: "refresh", Disabled: h.s.steamRestarting,
			}).Label("刷新 Steam 设置").OnClick(func() { h.refreshSteamSettings() })
		})
		ui.Column(c).Gap(12).Children(func() {
			fieldLabel(c, t, "Steam 启动参数", 0)
			value := g.SteamLaunchOptions
			in := actionInput(c, t, "steam-launch-options", &value, "Steam 启动参数", "例如 LANG=zh_CN.UTF-8 %command%", func(r ui.Element) { r.FillWidth() },
				inputAction{icon: "content-save-outline", label: "保存到 Steam", loading: h.s.launchOptionsSaving, onClick: func() { h.saveSteamLaunchOptions(c) }})
			if in.Changed() {
				h.l.change(g.ID, func(pg *models.Game) { pg.SteamLaunchOptions = value })
			}
			components.Text(c, "写入 Steam 的“启动选项”。导入到 Steam 的非 Steam 游戏会同步到 shortcuts.vdf；原生 Steam 游戏请在 Steam 属性中设置。修改后建议重启 Steam。",
				textXSRelax, theme.WeightNormal, t.TextMuted)
			ui.Row(c).Wrap().GapX(8).GapY(8).Children(func() {
				for _, preset := range steamLaunchOptionPresets {
					components.Button(c, t, "preset-"+preset.key, components.ButtonProps{
						Size: components.ButtonSM, Icon: "plus-circle-outline", Label: preset.label,
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
		cardTitle(c, t, "启动增强工具", 8)
		if !steam {
			h.toolRow(c, t, "use_locale_emulator", "Locale Emulator", "使用日文环境模拟启动游戏，解决乱码和区域限制问题。",
				"请先在设置中配置 LEProc.exe 路径", hasLE, g.UseLocaleEmulator, func(on bool) {
					if on && !hasLE {
						h.toast(c, "请先在设置中配置 Locale Emulator 路径")
						return
					}
					h.l.change(g.ID, func(pg *models.Game) { pg.UseLocaleEmulator = on })
				})
		}
		h.toolRow(c, t, "use_magpie", "Magpie", "游戏启动后自动启动 Magpie 进行全屏超分辨率缩放。",
			"请先在设置中配置 Magpie.exe 路径", hasMagpie, g.UseMagpie, func(on bool) {
				if on && !hasMagpie {
					h.toast(c, "请先在设置中配置 Magpie 路径")
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
			components.Switch(c, t, "switch", checked, !configured, onChange)
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
				h.toast(c, "选择文件失败")
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
				h.toast(c, "导出快捷启动方式失败: "+err.Error())
			case path != "":
				h.toast(c, "快捷启动方式已导出\n"+path)
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
		return "Steam 正在运行，请先完全退出 Steam 后再保存启动参数。"
	case "steam_not_installed":
		return "未检测到 Linux Steam 客户端。"
	case "executable_required":
		return "添加非 Steam 游戏需要一个可执行文件。请选择该游戏实际使用的启动程序。"
	case "user_unavailable":
		return "请启动 Steam 并登录目标账号，然后退出 Steam 再重试。"
	}
	state := status.State
	if state == "" {
		state = "unknown"
	}
	return "Steam 快捷方式尚未准备好：" + state
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
				h.toast(c, "保存 Steam 启动参数失败: "+err.Error())
				return
			}
			h.l.change(id, func(pg *models.Game) { mergeSteamSettings(pg, refreshed) })
			h.toast(c, "Steam 启动参数已保存")
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
				h.toast(c, "重启 Steam 失败: "+err.Error())
				return
			}
			h.toast(c, "Steam 已重启")
			h.refreshSteamSettings()
		}
	})
}
