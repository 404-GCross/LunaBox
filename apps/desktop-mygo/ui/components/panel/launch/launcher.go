package launch

import (
	"reflect"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/service"
	launcherpkg "lunabox/internal/service/launcher"
)

// runtimeEvent 是 StartService 发出的运行状态事件（useAppRuntimeEffects 订阅的 game-runtime:changed）。
const runtimeEvent = "game-runtime:changed"

// runtimeInfo 对应 store 的 GameRuntimeInfo（只留详情页用到的字段）。
type runtimeInfo struct {
	game      *models.Game
	sessionID string
	startTime time.Time
	state     service.GameRuntimeState
	// optimistic 是 startGame 乐观写入时的序号（原版用乐观的 startTime 认出自己）；后端事件带来真实的
	// start_time 后清零，启动失败时只回滚仍是自己写入的那一条。
	optimistic int
}

// steamAction 对应 SteamPendingAction：Steam 状态就绪后要继续做的事。
type steamAction string

const (
	steamNone        steamAction = ""
	steamSaveDefault steamAction = "save-default"
	steamLaunch      steamAction = "launch"
)

// steamFlow 是 Steam 导入弹窗与 pendingSteamAction（routes/game.tsx 的 isSteamModalOpen、isCheckingSteam、
// isImportingSteam、steamStatus）。
type steamFlow struct {
	open      bool
	checking  bool
	importing bool
	status    *service.SteamLaunchStatus
	action    steamAction
	gameID    string
}

// Launcher 启动游戏并跟踪运行状态（store.gameRuntimes），整个应用一个，由详情页持有。
// Steam 导入流程（resolveSteamGame 与 SteamImportModal）也在这里：页头的启动按钮与“启动配置”里
// 选择 Steam 默认启动方式共用它。
type Launcher struct {
	env      *env.Env
	runtimes map[string]*runtimeInfo
	seq      int
	steam    steamFlow
	// latest 是最近一帧页面传来的内容（Modals 与 View 每帧更新）。后台结果回到主线程时，
	// 在它的游戏上合并改动，而不是在发起操作时的旧副本上。
	latest panel.Game
}

// launchers 让“启动配置”标签页（View 只拿到 Env）找到页面创建的 Launcher。
var launchers sync.Map // *env.Env → *Launcher

// NewLauncher 创建启动器；订阅服务的运行状态事件（事件在主线程上分发）。
func NewLauncher(e *env.Env) *Launcher {
	l := &Launcher{env: e, runtimes: map[string]*runtimeInfo{}}
	if e != nil && e.Svc != nil && e.Svc.Events != nil {
		e.Svc.Events.On(runtimeEvent, l.applyEvent)
	}
	launchers.Store(e, l)
	return l
}

// launcherFor 返回 e 的 Launcher；没有时（单独测试标签页）创建一个。
func launcherFor(e *env.Env) *Launcher {
	if v, ok := launchers.Load(e); ok {
		return v.(*Launcher)
	}
	return NewLauncher(e)
}

// Status 是游戏的运行状态：running 对应 gameRuntime 存在，ending 对应 state === "ending"。
func (l *Launcher) Status(gameID string) (running, ending bool) {
	r := l.runtimes[gameID]
	if r == nil {
		return false, false
	}
	return true, r.state == service.GameRuntimeStateEnding
}

// applyEvent 对应 store 的 applyGameRuntimeEvent：idle 删除，其余按会话合并；同一会话的状态不回退。
func (l *Launcher) applyEvent(data any) {
	var ev service.GameRuntimeChangedEvent
	switch v := data.(type) {
	case service.GameRuntimeChangedEvent:
		ev = v
	case *service.GameRuntimeChangedEvent:
		if v == nil {
			return
		}
		ev = *v
	default:
		return
	}
	state := ev.State
	if state == "" {
		state = service.GameRuntimeStateIdle
	}
	id := ev.GameID
	if id == "" && ev.Game != nil {
		id = ev.Game.ID
	}
	if id == "" {
		if state == service.GameRuntimeStateIdle {
			clear(l.runtimes)
		}
		return
	}
	cur := l.runtimes[id]
	if state == service.GameRuntimeStateIdle {
		if ev.SessionID != "" && cur != nil && cur.sessionID != "" && cur.sessionID != ev.SessionID {
			return
		}
		delete(l.runtimes, id)
		return
	}
	if stateRegression(cur, ev.SessionID, state) {
		return
	}
	next := &runtimeInfo{}
	if cur != nil {
		*next = *cur
	}
	next.state = state
	if ev.Game != nil {
		next.game = ev.Game
	}
	if ev.SessionID != "" {
		next.sessionID = ev.SessionID
	}
	if !ev.StartTime.IsZero() {
		next.startTime, next.optimistic = ev.StartTime, 0
	}
	l.runtimes[id] = next
}

// stateRegression 对应 isSameSessionStateRegression：同一会话里 playing/ending 之后的 launching、ending 之后的 playing 是迟到的事件。
func stateRegression(cur *runtimeInfo, sessionID string, next service.GameRuntimeState) bool {
	if cur == nil || cur.sessionID == "" || sessionID == "" || cur.sessionID != sessionID {
		return false
	}
	switch next {
	case service.GameRuntimeStateLaunching:
		return cur.state == service.GameRuntimeStatePlaying || cur.state == service.GameRuntimeStateEnding
	case service.GameRuntimeStatePlaying:
		return cur.state == service.GameRuntimeStateEnding
	}
	return false
}

// Start 对应详情页的 handleStartGame：正在运行时什么也不做；Steam 启动先确认 Steam 关联（resolveSteamGame）。
func (l *Launcher) Start(c *ui.Context, p panel.Game, mode enums.LaunchMode) {
	if p.Game == nil || p.Game.ID == "" || l.runtimes[p.Game.ID] != nil {
		return
	}
	l.latest = p
	if mode == enums.LaunchModeSteam {
		l.resolveSteam(c, steamLaunch, *p.Game, func(models.Game) {
			l.steam.action = steamNone
			l.performStart(c, l.gameOr(p.Game), enums.LaunchModeSteam)
		})
		return
	}
	l.performStart(c, *p.Game, mode)
}

// gameOr 返回页面当前的游戏（与 fallback 同一个 ID 时），否则 fallback。
func (l *Launcher) gameOr(fallback *models.Game) models.Game {
	if g := l.latest.Game; g != nil && g.ID == fallback.ID {
		return *g
	}
	return *fallback
}

// launchOptions 对应 performStartGame 传给 startGame 的选项（原型只在 Windows 上运行：管理员启动可用，没有兼容层）。
func launchOptions(mode enums.LaunchMode) launcherpkg.LaunchOptions {
	f, tr := false, true
	switch mode {
	case enums.LaunchModeAdmin:
		return launcherpkg.LaunchOptions{RunAsAdmin: &tr, UseSteam: &f, UseCompatibility: &f}
	case enums.LaunchModeSteam:
		return launcherpkg.LaunchOptions{UseSteam: &tr}
	}
	return launcherpkg.LaunchOptions{RunAsAdmin: &f, UseSteam: &f, UseCompatibility: &f}
}

// performStart 对应 performStartGame 与 store.startGame：先乐观地记为 launching，StartGameWithOptions 失败或
// 未启动时回滚；启动后重新读取游戏（页面的 OnSaved）。
func (l *Launcher) performStart(c *ui.Context, g models.Game, mode enums.LaunchMode) {
	id, name := g.ID, g.Name
	if id == "" {
		return
	}
	if l.runtimes[id] != nil { // startGame：已经在运行，直接算启动成功
		l.afterStart(c, id, name, true, nil)
		return
	}
	l.seq++
	seq := l.seq
	l.runtimes[id] = &runtimeInfo{game: &g, startTime: time.Now(), state: service.GameRuntimeStateLaunching, optimistic: seq}
	opts := launchOptions(mode)
	e := l.env
	e.Async(func() func() {
		started, err := e.Svc.Start.StartGameWithOptions(id, opts)
		return func() {
			if !started || err != nil {
				if r := l.runtimes[id]; r != nil && r.optimistic == seq && r.state == service.GameRuntimeStateLaunching {
					delete(l.runtimes, id)
				}
			}
			l.afterStart(c, id, name, started, err)
		}
	})
}

func (l *Launcher) afterStart(c *ui.Context, id, name string, started bool, err error) {
	switch {
	case err != nil:
		l.env.Toast(c, locales.GameCardStartFailedLog(name))
	case !started:
		l.env.Toast(c, locales.GameCardStartFailedNotLaunched(name))
	default:
		if p := l.latest; p.Game != nil && p.Game.ID == id && p.OnSaved != nil {
			p.OnSaved()
		}
	}
}

// change 在页面当前的游戏上做改动并交给页面（updateGameState）；游戏已换或没有变化时不提交。
func (l *Launcher) change(id string, edit func(g *models.Game)) {
	p := l.latest
	if p.Game == nil || p.Game.ID != id || p.OnChange == nil {
		return
	}
	g := *p.Game
	edit(&g)
	if reflect.DeepEqual(g, *p.Game) {
		return
	}
	p.OnChange(g)
	l.latest.Game = &g
}

// withSteamStatus 对应 gameWithSteamStatus：写入 Steam 启动身份，wine_prefix 为空时用 Proton prefix。
func withSteamStatus(g *models.Game, status service.SteamLaunchStatus) {
	g.SteamLaunchID, g.SteamLaunchKind, g.SteamUserID = status.LaunchID, status.LaunchKind, status.UserID
	if g.WinePrefix == "" {
		g.WinePrefix = status.ProtonPrefix
	}
}

// mergeSteamSettings 对应 mergeSteamSettings：只取重新读到的 Steam 相关字段，保留页面里其余未保存的修改。
func mergeSteamSettings(g *models.Game, refreshed models.Game) {
	g.SteamLaunchID, g.SteamLaunchKind, g.SteamUserID = refreshed.SteamLaunchID, refreshed.SteamLaunchKind, refreshed.SteamUserID
	g.SteamLaunchOptions = refreshed.SteamLaunchOptions
	g.UseLocaleEmulator = refreshed.UseLocaleEmulator
	g.WinePrefix = refreshed.WinePrefix
}

// resolveSteam 对应 resolveSteamGame：查询 Steam 关联；未就绪时打开导入弹窗，就绪时继续 then。
// GetGameSteamStatus 就绪时会把启动身份写进数据库，这里同步到页面里的游戏，免得之后的自动保存用旧值覆盖。
func (l *Launcher) resolveSteam(c *ui.Context, action steamAction, g models.Game, then func(models.Game)) {
	l.steam.action, l.steam.checking, l.steam.gameID = action, true, g.ID
	e := l.env
	e.Async(func() func() {
		status, err := e.Svc.Integration.GetGameSteamStatus(g.ID)
		return func() {
			l.steam.checking = false
			if err != nil {
				l.steam.action = steamNone
				e.Toast(c, locales.SteamImportCheckFailed(err.Error()))
				return
			}
			l.steam.status = &status
			if !status.Ready {
				l.steam.open = true
				return
			}
			l.change(g.ID, func(pg *models.Game) { withSteamStatus(pg, status) })
			withSteamStatus(&g, status)
			then(g)
		}
	})
}

// saveSteamAsDefault 对应 saveSteamAsDefault：默认启动方式改为 Steam。原版立即 UpdateGame；这里交给页面的自动保存，
// 与页面里尚未保存的修改一起写入，不与之竞争。
func (l *Launcher) saveSteamAsDefault(g models.Game) {
	l.change(g.ID, func(pg *models.Game) {
		pg.SteamLaunchID, pg.SteamLaunchKind, pg.SteamUserID = g.SteamLaunchID, g.SteamLaunchKind, g.SteamUserID
		if pg.WinePrefix == "" {
			pg.WinePrefix = g.WinePrefix
		}
		pg.LaunchMode = enums.LaunchModeSteam
	})
}

// finishSteam 在 Steam 关联就绪后关闭弹窗并继续挂起的操作（handleRetrySteamStatus 等的共同结尾）。
func (l *Launcher) finishSteam(c *ui.Context, action steamAction, g models.Game) {
	l.steam.open = false
	l.steam.action = steamNone
	if action == steamSaveDefault {
		l.saveSteamAsDefault(g)
	} else {
		l.performStart(c, l.gameOr(&g), enums.LaunchModeSteam)
	}
}

// retrySteam 对应 handleRetrySteamStatus。
func (l *Launcher) retrySteam(c *ui.Context, g models.Game) {
	action := l.steam.action
	if action == steamNone {
		return
	}
	l.resolveSteam(c, action, g, func(sg models.Game) { l.finishSteam(c, action, sg) })
}

// importSteam 对应 handleImportGameToSteam：写入 Steam 快捷方式；刚导入时 Steam 要重启，只提示不启动。
func (l *Launcher) importSteam(c *ui.Context, g models.Game) {
	action := l.steam.action
	if action == steamNone {
		return
	}
	l.steam.importing = true
	e := l.env
	e.Async(func() func() {
		result, err := e.Svc.Integration.ImportGameToSteam(g.ID)
		return func() {
			l.steam.importing = false
			if err != nil {
				e.Toast(c, locales.SteamImportImportFailed(err.Error()))
				return
			}
			l.steam.status = &result.Status
			if !result.Status.Ready {
				return
			}
			l.change(g.ID, func(pg *models.Game) { withSteamStatus(pg, result.Status) })
			withSteamStatus(&g, result.Status)
			l.steam.open = false
			l.steam.action = steamNone
			if action == steamSaveDefault {
				l.saveSteamAsDefault(g)
			}
			// action 为 launch 时原版还会 setLaunchMode(Steam)：页头的临时启动方式由页面持有，而发起这次启动时它已经是 Steam。
			switch {
			case result.Imported:
				c.AddToast(ui.Toast{ID: "steam-import-success", Title: locales.SteamImportImportSuccess(), Timeout: 6 * time.Second})
			case action == steamLaunch:
				l.performStart(c, l.gameOr(&g), enums.LaunchModeSteam)
			}
		}
	})
}

// steamSelectExecutable 对应 handleSteamSelectExecutable：选择可执行文件，保存路径后重新检查 Steam 关联。
func (l *Launcher) steamSelectExecutable(c *ui.Context, g models.Game) {
	e := l.env
	e.Async(func() func() {
		path, err := e.Svc.Game.SelectGameExecutable(g.Path)
		return func() {
			if err != nil {
				e.Toast(c, locales.GameToastSelectExecutableFailed())
				return
			}
			if path == "" {
				return
			}
			updated := l.gameOr(&g)
			updated.Path = path
			e.Async(func() func() {
				err := e.Svc.Game.UpdateGame(updated)
				return func() {
					if err != nil {
						e.Toast(c, locales.GameToastSelectExecutableFailed())
						return
					}
					l.change(updated.ID, func(pg *models.Game) { pg.Path = path })
					action := l.steam.action
					if action == steamNone {
						return
					}
					l.resolveSteam(c, action, updated, func(sg models.Game) { l.finishSteam(c, action, sg) })
				}
			})
		}
	})
}

// closeSteam 对应 handleCloseSteamModal。
func (l *Launcher) closeSteam() {
	l.steam.open = false
	l.steam.action = steamNone
}

// Modals 构建启动相关的弹窗（SteamImportModal），详情页在最后调用。
func (l *Launcher) Modals(c *ui.Context, t *theme.Theme, p panel.Game) {
	l.latest = p
	if p.Game == nil {
		return
	}
	if l.steam.open && l.steam.gameID != p.Game.ID { // 已换到另一个游戏：原版的弹窗随页面卸载
		l.closeSteam()
	}
	l.steamModal(c, t, *p.Game)
}
