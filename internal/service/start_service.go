package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/service/gamehelper"
	launcherpkg "lunabox/internal/service/launcher"
	"lunabox/internal/utils/processutils"
	"lunabox/internal/utils/timerutils"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	homeRefreshRequestedEvent = "home:refresh-requested"
	gameRuntimeChangedEvent   = "game-runtime:changed"
	sessionHeartbeatInterval  = 15 * time.Second
)

type GameRuntimeState string

const (
	GameRuntimeStateLaunching GameRuntimeState = "launching"
	GameRuntimeStatePlaying   GameRuntimeState = "playing"
	GameRuntimeStateEnding    GameRuntimeState = "ending"
	GameRuntimeStateIdle      GameRuntimeState = "idle"
)

type GameRuntimeTimingMode string

const (
	GameRuntimeTimingModeWallClock GameRuntimeTimingMode = "wall-clock"
	GameRuntimeTimingModeActive    GameRuntimeTimingMode = "active"
)

type GameRuntimeChangedEvent struct {
	GameID        string                `json:"game_id"`
	Game          *models.Game          `json:"game,omitempty"`
	SessionID     string                `json:"session_id,omitempty"`
	StartTime     time.Time             `json:"start_time,omitempty"`
	State         GameRuntimeState      `json:"state"`
	Reason        string                `json:"reason,omitempty"`
	TimingMode    GameRuntimeTimingMode `json:"timing_mode,omitempty"`
	ActiveSeconds *int                  `json:"active_seconds,omitempty"`
	IsFocused     *bool                 `json:"is_focused,omitempty"`
}

type StartService struct {
	ctx               context.Context
	config            *appconf.AppConfig
	backupService     *BackupService
	gameService       *GameService
	sessionService    *SessionService
	activeTimeTracker *timerutils.ActiveTimeTracker

	// 进程选择相关
	pendingProcessSelect   map[string]chan string // gameID -> channel，用于接收用户选择的进程名
	pendingProcessSelectMu sync.RWMutex

	activeSessions   map[string]*activePlaySession
	activeSessionsMu sync.Mutex
}

type launchedProcess struct {
	PID      uint32
	Name     string
	Handle   uintptr
	ExitChan <-chan struct{}
}

// maxProcessHandoffs 限制单次会话内的进程接力次数，防止异常进程链导致会话永不结束。
const maxProcessHandoffs = 5

// processHandoffState 携带进程接力检测所需的上下文。
// 为 nil 时表示该监控路径不启用接力（如 DetectionLauncherOnly 模式）。
type processHandoffState struct {
	launchDir        string
	savedProcessName string
	handoffs         int
}

type activePlaySession struct {
	sessionID string
	gameID    string
	startTime time.Time
	game      models.Game
	done      chan struct{}
	finalOnce sync.Once
	// activeSeconds 由活跃窗口计时回调更新，供 15 秒心跳持久化读取。
	activeSeconds atomic.Int64
}

func intPtr(value int) *int {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func NewStartService() *StartService {
	return &StartService{
		pendingProcessSelect: make(map[string]chan string),
		activeSessions:       make(map[string]*activePlaySession),
		// activeTimeTracker 将在 Init 时创建
	}
}

func (s *StartService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	// db 不再使用，但保留参数以保持与其他服务的接口一致性
	s.config = config
	// 初始化内部服务
	s.activeTimeTracker = timerutils.NewActiveTimeTracker(ctx, db)
	s.activeTimeTracker.SetUpdateHandler(s.handleActiveTimeUpdate)
	// 确保 map 已初始化
	if s.pendingProcessSelect == nil {
		s.pendingProcessSelect = make(map[string]chan string)
	}
	if s.activeSessions == nil {
		s.activeSessions = make(map[string]*activePlaySession)
	}
}

// SetBackupService 设置备份服务（用于自动备份）
func (s *StartService) SetBackupService(backupService *BackupService) {
	s.backupService = backupService
}

// SetGameService 设置游戏服务（用于获取游戏信息）
func (s *StartService) SetGameService(gameService *GameService) {
	s.gameService = gameService
}

// SetSessionService 设置会话服务（用于管理游玩记录）
func (s *StartService) SetSessionService(sessionService *SessionService) {
	s.sessionService = sessionService
}

// StartGameWithTracking 启动游戏并自动追踪游玩时长
// 当游戏进程退出时，自动保存游玩记录到数据库
func (s *StartService) StartGameWithTracking(gameID string) (bool, error) {
	return s.startGame(gameID, launcherpkg.LaunchOptions{})
}

// StartGameWithOptions 使用指定选项启动游戏
// 供 CLI 调用，支持覆盖 LE 和 Magpie 设置
func (s *StartService) StartGameWithOptions(gameID string, options launcherpkg.LaunchOptions) (bool, error) {
	return s.startGame(gameID, options)
}

// HandleProtocolLaunch validates and dispatches a protocol-triggered game launch.
func (s *StartService) HandleProtocolLaunch(req vo.ProtocolLaunchRequest) error {
	gameID := strings.TrimSpace(req.GameID)
	if gameID == "" {
		err := fmt.Errorf("missing required parameter: game_id")
		s.emitProtocolLaunchError("快捷启动失败", err.Error(), "", "", "")
		return err
	}

	if s.gameService == nil {
		err := fmt.Errorf("game service is not initialized")
		s.emitProtocolLaunchError("快捷启动失败", err.Error(), gameID, "", "")
		return err
	}

	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		wrappedErr := fmt.Errorf("failed to resolve target game: %w", err)
		s.emitProtocolLaunchError("未找到该游戏快捷方式对应的游戏记录", err.Error(), gameID, "", "")
		return wrappedErr
	}

	started, err := s.StartGameWithTracking(gameID)
	if err != nil {
		wrappedErr := fmt.Errorf("start game via protocol: %w", err)
		s.emitProtocolLaunchErrorFromError(fmt.Sprintf("启动《%s》失败", game.Name), err, gameID)
		return wrappedErr
	}
	if !started {
		err := fmt.Errorf("game failed to start")
		s.emitProtocolLaunchError(fmt.Sprintf("启动《%s》失败", game.Name), err.Error(), gameID, "", "")
		return err
	}

	return nil
}

// startGame 内部启动方法，支持通过 options 覆盖配置
func (s *StartService) startGame(gameID string, options launcherpkg.LaunchOptions) (bool, error) {
	if s.gameService == nil {
		return false, fmt.Errorf("game service is not initialized")
	}

	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to get game path: %v", err)
		return false, fmt.Errorf("failed to get game path: %w", err)
	}
	path := game.Path
	processName := game.ProcessName
	useSteamLaunch := launcherpkg.ShouldUseSteamLaunch(&game, options)

	// 如果未配置路径或配置的是文件夹，则在首次启动时要求用户选择可执行文件并写回游戏路径
	if !useSteamLaunch {
		resolvedPath, resolvedProcessName, cancelled, err := s.resolveExecutablePath(gameID, path, processName)
		if err != nil {
			applog.LogErrorf(s.ctx, "failed to resolve executable path: %v", err)
			return false, fmt.Errorf("failed to resolve executable path: %w", err)
		}
		if cancelled {
			applog.LogInfof(s.ctx, "user cancelled executable selection for game: %s", gameID)
			return false, nil
		}
		path = resolvedPath
		if strings.TrimSpace(resolvedProcessName) != "" {
			processName = resolvedProcessName
		}
		if strings.TrimSpace(processName) == "" {
			processName = filepath.Base(path)
		}
	}
	game.Path = path
	game.ProcessName = processName

	strategy, err := launcherpkg.SelectLauncherStrategy(&game, options, s.config)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to select launcher strategy: %v", err)
		return false, err
	}
	plan, err := strategy.Plan(s.ctx, &game, options)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to build launch plan: %v", err)
		return false, err
	}
	if strings.TrimSpace(plan.DisplayName) == "" {
		plan.DisplayName = filepath.Base(plan.File)
	}
	if strings.TrimSpace(plan.DetectionDir) == "" {
		plan.DetectionDir = launcherpkg.EffectiveProcessDetectionDir(game.GameDirectory, filepath.Dir(path))
	}
	launcherExeName := filepath.Base(plan.File)

	var startedProcess *processutils.StartedProcess
	if plan.RunAsAdmin {
		applog.LogInfof(s.ctx, "Starting game as administrator: %s", gameID)
		startedProcess, err = processutils.StartProcessElevated(plan.File, plan.Args, plan.Dir)
	} else if len(plan.Env) > 0 {
		startedProcess, err = processutils.StartProcessWithEnv(plan.File, plan.Args, plan.Dir, plan.Env)
	} else {
		startedProcess, err = processutils.StartProcess(plan.File, plan.Args, plan.Dir)
	}
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to start game: %v", err)
		return false, fmt.Errorf("failed to start game: %w", err)
	}

	// 如果启用了 Magpie，在游戏启动后启动 Magpie
	if plan.Magpie && s.config.MagpiePath != "" {
		go s.startMagpie()
	}

	if plan.ActiveTrack.Kind == launcherpkg.ActiveTrackWineRootPID && plan.ActiveTrack.RootPID == 0 {
		plan.ActiveTrack.RootPID = startedProcess.PID
	}
	if plan.ActiveTrack.Kind == launcherpkg.ActiveTrackLauncherPID && plan.ActiveTrack.LauncherPID == 0 {
		plan.ActiveTrack.LauncherPID = startedProcess.PID
	}

	launcher := launchedProcess{
		PID:      startedProcess.PID,
		Name:     plan.DisplayName,
		Handle:   startedProcess.Handle,
		ExitChan: startedProcess.ExitChan,
	}

	startTime := time.Now()
	sessionID, err := s.sessionService.CreatePendingSession(gameID, startTime)
	if err != nil {
		processutils.CloseProcessHandle(startedProcess.Handle)
		return false, fmt.Errorf("failed to create play session: %w", err)
	}

	session := s.registerActiveSession(sessionID, gameID, startTime, game)
	s.emitGameRuntimeChanged(GameRuntimeChangedEvent{
		GameID:    gameID,
		Game:      &game,
		SessionID: sessionID,
		StartTime: startTime,
		State:     GameRuntimeStateLaunching,
		Reason:    "launched",
	})

	// 启动进程检测和监控 goroutine
	go s.detectAndMonitorProcess(session, launcher, launcherExeName, plan.DetectionDir, processName, plan)

	// pending session 已创建，Home 数据已发生变化，立即通知前端刷新
	s.requestHomeRefresh()

	// 启动成功，返回 true 给前端
	return true, nil
}

// detectAndMonitorProcess 检测实际游戏进程并开始监控。
func (s *StartService) detectAndMonitorProcess(session *activePlaySession, launcher launchedProcess, launcherExeName string, launchDir string, savedProcessName string, plan launcherpkg.LaunchPlan) {
	sessionID := session.sessionID
	gameID := session.gameID

	if plan.DetectionMode == launcherpkg.DetectionLauncherOnly {
		s.monitorLauncherOnly(session, launcher, plan)
		return
	}

	detectionInput := launcherpkg.StagedProcessDetectionInput{
		GameID: gameID,
		Launcher: launcherpkg.LaunchedProcessInfo{
			PID:  launcher.PID,
			Name: launcher.Name,
		},
		LauncherExeName:       launcherExeName,
		LaunchDir:             launchDir,
		SavedProcessName:      savedProcessName,
		AutoDetectGameProcess: s.config.AutoDetectGameProcess,
	}

	var result launcherpkg.StagedProcessDetectionResult
	if plan.DetectionMode == launcherpkg.DetectionSteamDirectory {
		result = launcherpkg.DetectSteamDirectoryProcess(detectionInput, serviceDetectionLogger{ctx: s.ctx})
	} else {
		result = launcherpkg.DetectStagedProcess(detectionInput, serviceDetectionLogger{ctx: s.ctx})
	}

	handoff := &processHandoffState{
		launchDir:        launchDir,
		savedProcessName: savedProcessName,
	}

	if strings.TrimSpace(result.PersistProcessName) != "" {
		if err := s.updateGameProcessName(gameID, result.PersistProcessName); err != nil {
			applog.LogWarningf(s.ctx, "Failed to update detected process name for game %s: %v", gameID, err)
		} else {
			handoff.savedProcessName = result.PersistProcessName
		}
	}
	if result.CloseLauncherHandle {
		s.closeLauncherHandle(launcher)
	}
	if result.RequireProcessSelection {
		s.promptUserToSelectProcess(session, launcherExeName, handoff)
		return
	}
	if result.ProcessID == 0 {
		applog.LogWarningf(s.ctx, "Staged process detection returned no process for game %s, requiring manual selection", gameID)
		s.promptUserToSelectProcess(session, launcherExeName, handoff)
		return
	}

	s.emitGameRuntimePlaying(session, "process-detected")
	s.startActiveTimeTracking(sessionID, gameID, result.ProcessID, launcherpkg.ActiveTrack{})

	if result.UseLauncherHandle && launcher.Handle != 0 {
		s.monitorProcessByHandle(session, result.ProcessID, result.ProcessName, launcher.Handle, handoff)
		return
	}
	s.monitorProcessByPID(session, result.ProcessID, result.ProcessName, handoff)
}

type serviceDetectionLogger struct {
	ctx context.Context
}

func (l serviceDetectionLogger) Infof(format string, args ...any) {
	applog.LogInfof(l.ctx, format, args...)
}

func (l serviceDetectionLogger) Warningf(format string, args ...any) {
	applog.LogWarningf(l.ctx, format, args...)
}

func (s *StartService) closeLauncherHandle(launcher launchedProcess) {
	if launcher.Handle == 0 {
		return
	}
	if err := processutils.CloseProcessHandle(launcher.Handle); err != nil {
		applog.LogWarningf(s.ctx, "Failed to close launcher process handle for %s (PID %d): %v", launcher.Name, launcher.PID, err)
	}
}

func (s *StartService) monitorLauncherOnly(session *activePlaySession, launcher launchedProcess, plan launcherpkg.LaunchPlan) {
	s.emitGameRuntimePlaying(session, "launcher-monitoring")
	s.startActiveTimeTracking(session.sessionID, session.gameID, launcher.PID, plan.ActiveTrack)
	// DetectionLauncherOnly 模式明确只监控启动进程本身，不做进程接力。
	if launcher.Handle != 0 {
		s.monitorProcessByHandle(session, launcher.PID, launcher.Name, launcher.Handle, nil)
		return
	}
	if launcher.ExitChan != nil {
		s.waitForProcessExit(session, launcher.Name, launcher.PID, launcher.ExitChan, nil)
		return
	}
	s.monitorProcessByPID(session, launcher.PID, launcher.Name, nil)
}

func (s *StartService) startActiveTimeTracking(sessionID string, gameID string, processID uint32, activeTrack launcherpkg.ActiveTrack) {
	if !s.config.RecordActiveTimeOnly {
		return
	}

	_, err := s.activeTimeTracker.StartTrackingWithActiveTrack(sessionID, gameID, processID, activeTrack)
	if err != nil {
		applog.LogWarningf(s.ctx, "Failed to start active time tracking: %v", err)
	}
}

func (s *StartService) persistSelectedProcessName(gameID string, selectedProcessName string) {
	if !launcherpkg.IsPersistableProcessName(selectedProcessName) {
		applog.LogInfof(s.ctx, "Selected non-exe process for game %s will not be persisted as process_name: %s", gameID, selectedProcessName)
		return
	}
	if err := s.updateGameProcessName(gameID, selectedProcessName); err != nil {
		applog.LogWarningf(s.ctx, "Failed to update selected process name for game %s: %v", gameID, err)
	}
}

func (s *StartService) emitProtocolLaunchError(message string, detail string, gameID string, kind string, configKey string) {
	if s.ctx == nil {
		return
	}
	runtime.EventsEmit(s.ctx, "protocol-launch:error", vo.ProtocolLaunchErrorEvent{
		Message:   strings.TrimSpace(message),
		Detail:    strings.TrimSpace(detail),
		GameID:    strings.TrimSpace(gameID),
		Kind:      strings.TrimSpace(kind),
		ConfigKey: strings.TrimSpace(configKey),
	})
}

func (s *StartService) emitProtocolLaunchErrorFromError(message string, err error, gameID string) {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	var strategyErr *launcherpkg.StrategyError
	if errors.As(err, &strategyErr) && strategyErr != nil {
		if strings.TrimSpace(strategyErr.UserMessage) != "" {
			message = strategyErr.UserMessage
		}
		s.emitProtocolLaunchError(message, detail, gameID, strategyErr.Kind, strategyErr.ConfigKey)
		return
	}
	s.emitProtocolLaunchError(message, detail, gameID, "", "")
}

// promptUserToSelectProcess 提示用户选择实际的游戏进程
func (s *StartService) promptUserToSelectProcess(session *activePlaySession, launcherExeName string, handoff *processHandoffState) {
	sessionID := session.sessionID
	gameID := session.gameID

	// 创建等待用户选择的 channel
	selectChan := make(chan string, 1)
	s.pendingProcessSelectMu.Lock()
	s.pendingProcessSelect[gameID] = selectChan
	s.pendingProcessSelectMu.Unlock()

	// 发送事件通知前端弹出进程选择窗口
	runtime.EventsEmit(s.ctx, "process-select-required", map[string]interface{}{
		"gameID":          gameID,
		"sessionID":       sessionID,
		"launcherExeName": launcherExeName,
	})

	// 等待用户选择（最多等待5分钟）
	var selectedProcess string
	var ok bool
	select {
	case selectedProcess, ok = <-selectChan:
		if !ok {
			// channel 被关闭，说明用户取消了选择
			applog.LogInfof(s.ctx, "User cancelled process selection for game %s", gameID)
			s.deleteShortOrCancelledSession(session, "process-selection-cancelled")
			return
		}
		// 用户已选择进程
		applog.LogInfof(s.ctx, "User selected process: %s for game %s", selectedProcess, gameID)
	case <-time.After(5 * time.Minute):
		// 超时未选择
		applog.LogWarningf(s.ctx, "Process selection timeout for game %s, cleaning up session", gameID)
		s.pendingProcessSelectMu.Lock()
		delete(s.pendingProcessSelect, gameID)
		s.pendingProcessSelectMu.Unlock()
		s.deleteShortOrCancelledSession(session, "process-selection-timeout")
		return
	}

	// 清理 channel
	s.pendingProcessSelectMu.Lock()
	delete(s.pendingProcessSelect, gameID)
	s.pendingProcessSelectMu.Unlock()

	// 获取选中进程的PID
	pid, err := processutils.GetProcessPIDByName(selectedProcess)
	if err != nil {
		applog.LogErrorf(s.ctx, "Failed to find selected process %s: %v", selectedProcess, err)
		s.deleteShortOrCancelledSession(session, "selected-process-not-found")
		return
	}

	// 保存用户选择的进程名
	s.persistSelectedProcessName(gameID, selectedProcess)
	if handoff != nil {
		handoff.savedProcessName = selectedProcess
	}

	s.emitGameRuntimePlaying(session, "process-selected")
	// 启动活跃时间追踪（如果启用）
	s.startActiveTimeTracking(sessionID, gameID, pid, launcherpkg.ActiveTrack{})

	// 监控选中的进程
	s.monitorProcessByPID(session, pid, selectedProcess, handoff)
}

// monitorProcessByPID 通过PID监控外部进程直到退出
// 优先使用 WaitForSingleObject；权限不足时退回进程快照轮询。
func (s *StartService) monitorProcessByPID(session *activePlaySession, processID uint32, processName string, handoff *processHandoffState) {
	applog.LogInfof(s.ctx, "Starting to monitor external process %s (PID %d) using WaitForSingleObject", processName, processID)

	// 创建进程监控器
	pm, exitChan, err := processutils.WaitForProcessExitAsync(processID)
	if err != nil {
		applog.LogWarningf(s.ctx, "Failed to open process monitor for %s (PID %d), falling back to process snapshot polling: %v", processName, processID, err)
		snapshotMonitor, snapshotExitChan := processutils.WaitForProcessExitBySnapshotAsync(processID)
		defer snapshotMonitor.Stop()
		s.waitForProcessExit(session, processName, processID, snapshotExitChan, handoff)
		return
	}
	defer pm.Stop()

	s.waitForProcessExit(session, processName, processID, exitChan, handoff)
}

func (s *StartService) monitorProcessByHandle(session *activePlaySession, processID uint32, processName string, processHandle uintptr, handoff *processHandoffState) {
	applog.LogInfof(s.ctx, "Starting to monitor launched process %s (PID %d) using ShellExecuteEx handle", processName, processID)

	pm, exitChan, err := processutils.WaitForProcessHandleExitAsync(processID, processHandle)
	if err != nil {
		applog.LogWarningf(s.ctx, "Failed to monitor process handle for %s (PID %d), falling back to PID monitor: %v", processName, processID, err)
		s.monitorProcessByPID(session, processID, processName, handoff)
		return
	}
	defer pm.Stop()

	s.waitForProcessExit(session, processName, processID, exitChan, handoff)
}

func (s *StartService) waitForProcessExit(session *activePlaySession, processName string, processID uint32, exitChan <-chan struct{}, handoff *processHandoffState) {
	// 等待进程退出或超时（24小时）
	select {
	case <-exitChan:
		applog.LogInfof(s.ctx, "Game process %s (PID %d) has exited", processName, processID)
		// 进程退出不一定是游戏结束：彩窗/启动器可能已把控制权交给了新进程
		// （spawn 子进程后自退、同名 re-exec 等），先做一轮继任者检测。
		if successor, ok := s.detectSuccessorProcess(session, processID, processName, handoff); ok {
			s.continueMonitoringSuccessor(session, successor, handoff)
			return
		}
	case <-session.done:
		applog.LogInfof(s.ctx, "Game runtime tracking for %s was stopped manually", session.gameID)
		return
	case <-time.After(24 * time.Hour):
		applog.LogWarningf(s.ctx, "Game %s exceeded maximum runtime (24h), forcing cleanup", session.gameID)
	}

	// 执行统一的会话清理逻辑
	s.finalizePlaySession(session, "process-exited")
}

// detectSuccessorProcess 在被监控进程退出后，于短暂宽限期内寻找接管的游戏进程。
func (s *StartService) detectSuccessorProcess(session *activePlaySession, exitedPID uint32, exitedName string, handoff *processHandoffState) (processutils.ProcessInfo, bool) {
	if handoff == nil {
		return processutils.ProcessInfo{}, false
	}
	if handoff.handoffs >= maxProcessHandoffs {
		applog.LogWarningf(s.ctx, "Game %s reached process hand-off limit (%d), finalizing session", session.gameID, maxProcessHandoffs)
		return processutils.ProcessInfo{}, false
	}

	input := launcherpkg.SuccessorDetectionInput{
		GameID:            session.gameID,
		ExitedPID:         exitedPID,
		ExitedProcessName: exitedName,
		LaunchDir:         handoff.launchDir,
		SavedProcessName:  handoff.savedProcessName,
		SessionStart:      session.startTime,
		SelfPID:           uint32(os.Getpid()),
	}
	return launcherpkg.DetectSuccessorProcess(input, serviceDetectionLogger{ctx: s.ctx})
}

// continueMonitoringSuccessor 把会话的追踪与监控切换到继任进程上。
func (s *StartService) continueMonitoringSuccessor(session *activePlaySession, successor processutils.ProcessInfo, handoff *processHandoffState) {
	// 继任检测有数秒宽限期，期间会话可能已被手动结束，此时不能再接力。
	select {
	case <-session.done:
		applog.LogInfof(s.ctx, "Game %s session ended during successor detection, skipping hand-off to %s (PID %d)", session.gameID, successor.Name, successor.PID)
		return
	default:
	}

	handoff.handoffs++
	applog.LogInfof(s.ctx, "Game %s process hand-off #%d: continuing session with %s (PID %d)", session.gameID, handoff.handoffs, successor.Name, successor.PID)

	// 只换绑已存在的追踪，不新建：若会话在此期间被结束，新建的追踪将无人回收。
	if s.config.RecordActiveTimeOnly {
		s.activeTimeTracker.RetargetTracking(session.gameID, successor.PID)
	}

	// 记住真实游戏进程名，下次启动可直接命中 saved process_name 快捷路径。
	if name := launcherpkg.ProcessNameForPersistence("", successor.Name); name != "" && !strings.EqualFold(name, handoff.savedProcessName) {
		if err := s.updateGameProcessName(session.gameID, name); err != nil {
			applog.LogWarningf(s.ctx, "Failed to persist successor process name for game %s: %v", session.gameID, err)
		} else {
			handoff.savedProcessName = name
		}
	}

	s.emitGameRuntimePlaying(session, "process-handoff")
	s.monitorProcessByPID(session, successor.PID, successor.Name, handoff)
}

// finalizePlaySession 完成游玩会话的最终处理
// 包括停止追踪、计算时长、更新数据库、自动备份等
func (s *StartService) finalizePlaySession(session *activePlaySession, reason string) {
	session.finalOnce.Do(func() {
		s.finalizePlaySessionOnce(session, reason)
	})
}

func (s *StartService) finalizePlaySessionOnce(session *activePlaySession, reason string) {
	sessionID := session.sessionID
	gameID := session.gameID
	startTime := session.startTime

	close(session.done)
	s.unregisterActiveSession(gameID, sessionID)

	// 确保停止追踪（无论如何都要执行）
	activeSeconds := s.activeTimeTracker.StopTracking(gameID)

	s.emitGameRuntimeChanged(GameRuntimeChangedEvent{
		GameID:        gameID,
		Game:          &session.game,
		SessionID:     sessionID,
		StartTime:     startTime,
		State:         GameRuntimeStateEnding,
		Reason:        reason,
		TimingMode:    s.runtimeTimingMode(),
		ActiveSeconds: s.runtimeActiveSeconds(activeSeconds),
	})

	endTime := time.Now()

	// 如果启用活跃时间追踪，使用累加的活跃时长
	// 否则使用整个运行时长
	var duration int
	if s.config.RecordActiveTimeOnly {
		duration = activeSeconds
		applog.LogInfof(s.ctx, "Game %s active play time: %d seconds", gameID, duration)
	} else {
		duration = int(endTime.Sub(startTime).Seconds())
		applog.LogInfof(s.ctx, "Game %s total runtime: %d seconds", gameID, duration)
	}

	// 如果游玩时长小于1分钟，删除临时会话记录
	if duration < 60 {
		err := s.sessionService.DeletePlaySession(sessionID)
		if err != nil {
			applog.LogErrorf(s.ctx, "Failed to delete short play session %s: %v", sessionID, err)
			s.emitGameRuntimeIdle(session, "short-session-delete-failed")
		} else {
			s.emitGameRuntimeIdle(session, "short-session-deleted")
			s.requestHomeRefresh()
		}
		return
	}

	// 更新会话记录
	playSession := models.PlaySession{
		ID:        sessionID,
		GameID:    gameID,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
	}
	err := s.sessionService.UpdatePlaySession(playSession)
	if err != nil {
		applog.LogErrorf(s.ctx, "Failed to update play session %s: %v", sessionID, err)
		s.emitGameRuntimeIdle(session, "session-finalize-failed")
		return
	}

	s.emitGameRuntimeIdle(session, "session-finalized")
	s.requestHomeRefresh()

	// 自动备份游戏存档
	if s.config.AutoBackupGameSave && s.backupService != nil {
		s.autoBackupGameSave(gameID)
	}
}

// EndCurrentPlaySession manually ends LunaBox tracking for the active game.
// It does not terminate the external game process; it finalizes the current
// play session and stops monitoring so later process exit cannot write twice.
func (s *StartService) EndCurrentPlaySession(gameID string) error {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return fmt.Errorf("game id is required")
	}

	session := s.getActiveSession(gameID)
	if session == nil {
		return fmt.Errorf("没有正在游玩的游戏: %s", gameID)
	}

	s.finalizePlaySession(session, "manual-ended")
	return nil
}

func (s *StartService) registerActiveSession(sessionID string, gameID string, startTime time.Time, game models.Game) *activePlaySession {
	session := &activePlaySession{
		sessionID: sessionID,
		gameID:    gameID,
		startTime: startTime,
		game:      game,
		done:      make(chan struct{}),
	}

	s.activeSessionsMu.Lock()
	s.activeSessions[gameID] = session
	s.activeSessionsMu.Unlock()

	go s.persistSessionHeartbeats(session)

	return session
}

func (s *StartService) persistSessionHeartbeats(session *activePlaySession) {
	ticker := time.NewTicker(sessionHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-session.done:
			return
		case heartbeatAt := <-ticker.C:
			duration := int(heartbeatAt.Sub(session.startTime).Seconds())
			if s.config != nil && s.config.RecordActiveTimeOnly {
				duration = int(session.activeSeconds.Load())
			}
			if duration < 0 {
				duration = 0
			}

			if s.sessionService == nil {
				continue
			}
			if err := s.sessionService.saveSessionHeartbeat(session.sessionID, duration, heartbeatAt); err != nil {
				applog.LogWarningf(s.ctx, "Failed to save play session heartbeat %s: %v", session.sessionID, err)
			}
		}
	}
}

func (s *StartService) getActiveSession(gameID string) *activePlaySession {
	s.activeSessionsMu.Lock()
	defer s.activeSessionsMu.Unlock()
	return s.activeSessions[gameID]
}

func (s *StartService) unregisterActiveSession(gameID string, sessionID string) {
	s.activeSessionsMu.Lock()
	defer s.activeSessionsMu.Unlock()

	current := s.activeSessions[gameID]
	if current != nil && current.sessionID == sessionID {
		delete(s.activeSessions, gameID)
	}
}

func (s *StartService) activeSessionSnapshot() []*activePlaySession {
	s.activeSessionsMu.Lock()
	defer s.activeSessionsMu.Unlock()

	sessions := make([]*activePlaySession, 0, len(s.activeSessions))
	for _, session := range s.activeSessions {
		sessions = append(sessions, session)
	}
	return sessions
}

func (s *StartService) deleteShortOrCancelledSession(session *activePlaySession, reason string) {
	session.finalOnce.Do(func() {
		close(session.done)
		s.unregisterActiveSession(session.gameID, session.sessionID)
		if err := s.sessionService.DeletePlaySession(session.sessionID); err != nil {
			applog.LogErrorf(s.ctx, "Failed to delete cancelled play session %s: %v", session.sessionID, err)
		}
		s.activeTimeTracker.StopTracking(session.gameID)
		s.emitGameRuntimeIdle(session, reason)
		s.requestHomeRefresh()
	})
}

func (s *StartService) emitGameRuntimePlaying(session *activePlaySession, reason string) {
	s.emitGameRuntimeChanged(GameRuntimeChangedEvent{
		GameID:        session.gameID,
		Game:          &session.game,
		SessionID:     session.sessionID,
		StartTime:     session.startTime,
		State:         GameRuntimeStatePlaying,
		Reason:        reason,
		TimingMode:    s.runtimeTimingMode(),
		ActiveSeconds: s.runtimeActiveSeconds(0),
	})
}

func (s *StartService) emitGameRuntimeIdle(session *activePlaySession, reason string) {
	s.emitGameRuntimeChanged(GameRuntimeChangedEvent{
		GameID:    session.gameID,
		Game:      &session.game,
		SessionID: session.sessionID,
		StartTime: session.startTime,
		State:     GameRuntimeStateIdle,
		Reason:    reason,
	})
}

func (s *StartService) emitGameRuntimeChanged(event GameRuntimeChangedEvent) {
	if s.ctx == nil {
		return
	}
	runtime.EventsEmit(s.ctx, gameRuntimeChangedEvent, event)
}

func (s *StartService) handleActiveTimeUpdate(update timerutils.ActiveTimeUpdate) {
	session := s.getActiveSession(update.GameID)
	if session == nil || session.sessionID != update.SessionID {
		return
	}
	session.activeSeconds.Store(int64(update.ActiveSeconds))

	s.emitGameRuntimeChanged(GameRuntimeChangedEvent{
		GameID:        session.gameID,
		SessionID:     session.sessionID,
		StartTime:     session.startTime,
		State:         GameRuntimeStatePlaying,
		Reason:        "active-time-updated",
		TimingMode:    GameRuntimeTimingModeActive,
		ActiveSeconds: intPtr(update.ActiveSeconds),
		IsFocused:     boolPtr(update.IsFocused),
	})
}

func (s *StartService) runtimeTimingMode() GameRuntimeTimingMode {
	if s.config != nil && s.config.RecordActiveTimeOnly {
		return GameRuntimeTimingModeActive
	}
	return GameRuntimeTimingModeWallClock
}

func (s *StartService) runtimeActiveSeconds(activeSeconds int) *int {
	if s.config != nil && s.config.RecordActiveTimeOnly {
		return intPtr(activeSeconds)
	}
	return nil
}

func (s *StartService) requestHomeRefresh() {
	if s.ctx == nil {
		return
	}

	runtime.EventsEmit(s.ctx, homeRefreshRequestedEvent)
}

// updateGameProcessName 更新游戏的进程名
func (s *StartService) updateGameProcessName(gameID string, processName string) error {
	return s.gameService.UpdateGameProcessName(gameID, processName)
}

// NotifyProcessSelected 用户选择了进程后调用此方法通知后端
// 这会唤醒等待的 goroutine，并在选择稳定 exe 进程时更新数据库
func (s *StartService) NotifyProcessSelected(gameID string, processName string) error {
	// 先更新数据库。非 exe 包装进程通常是随机/伪装名称，只用于本次监控。
	if launcherpkg.IsPersistableProcessName(processName) {
		if err := s.updateGameProcessName(gameID, processName); err != nil {
			return err
		}
	} else {
		applog.LogInfof(s.ctx, "Selected non-exe process for game %s will not be persisted as process_name: %s", gameID, processName)
	}

	// 通过 channel 通知等待的 goroutine
	s.pendingProcessSelectMu.RLock()
	selectChan, exists := s.pendingProcessSelect[gameID]
	s.pendingProcessSelectMu.RUnlock()

	if exists {
		// 非阻塞发送（如果 channel 已满或已关闭则跳过）
		select {
		case selectChan <- processName:
			applog.LogInfof(s.ctx, "Notified process selection for game %s: %s", gameID, processName)
		default:
			applog.LogWarningf(s.ctx, "Failed to notify process selection for game %s (channel full or closed)", gameID)
		}
	} else {
		applog.LogWarningf(s.ctx, "No pending process selection for game %s", gameID)
	}

	return nil
}

// CancelProcessSelection 用户取消了进程选择
// 关闭等待的 channel 并清理临时会话
func (s *StartService) CancelProcessSelection(gameID string) error {
	s.pendingProcessSelectMu.Lock()
	selectChan, exists := s.pendingProcessSelect[gameID]
	if exists {
		// 关闭 channel（让等待的 goroutine 知道用户取消了）
		close(selectChan)
		delete(s.pendingProcessSelect, gameID)
	}
	s.pendingProcessSelectMu.Unlock()

	if exists {
		applog.LogInfof(s.ctx, "User cancelled process selection for game %s", gameID)
	} else {
		applog.LogWarningf(s.ctx, "No pending process selection to cancel for game %s", gameID)
	}

	return nil
}

// CleanupPendingSessions 清理所有待定的进程选择会话
// 用于程序关闭时的清理，包括：
// 1. 关闭所有等待的进程选择 channels
// 2. 停止所有活跃时间追踪
// 3. 清理数据库中未完成的会话记录
func (s *StartService) CleanupPendingSessions() {
	// 1. 清理进程选择 channels
	s.pendingProcessSelectMu.Lock()
	if len(s.pendingProcessSelect) > 0 {
		applog.LogInfof(s.ctx, "Cleaning up %d pending process selections", len(s.pendingProcessSelect))
		// 关闭所有等待的 channels
		for gameID, selectChan := range s.pendingProcessSelect {
			close(selectChan)
			applog.LogInfof(s.ctx, "Cancelled pending process selection for game %s", gameID)
		}
		// 清空 map
		s.pendingProcessSelect = make(map[string]chan string)
	}
	s.pendingProcessSelectMu.Unlock()

	activeSessions := s.activeSessionSnapshot()
	activeDurations := make(map[string]int)

	// 2. 停止所有活跃时间追踪
	if s.activeTimeTracker != nil {
		activeDurations = s.activeTimeTracker.StopAllTracking()
		applog.LogInfof(s.ctx, "Stopped all active time tracking")
	}

	// 3. 结束当前进程内仍在追踪的会话
	if s.sessionService != nil && len(activeSessions) > 0 {
		endTime := time.Now()
		applog.LogInfof(s.ctx, "Completing %d active play sessions during shutdown", len(activeSessions))
		for _, session := range activeSessions {
			duration := int(endTime.Sub(session.startTime).Seconds())
			if s.config.RecordActiveTimeOnly {
				duration = activeDurations[session.gameID]
			}

			session.finalOnce.Do(func() {
				close(session.done)
				s.unregisterActiveSession(session.gameID, session.sessionID)
				if err := s.sessionService.completeUnfinishedSessionWithDuration(session.sessionID, endTime, duration); err != nil {
					applog.LogErrorf(s.ctx, "Failed to complete active session %s during shutdown: %v", session.sessionID, err)
				}
			})
		}
	}

	// 4. 清理数据库中未完成的会话
	if s.sessionService != nil {
		err := s.sessionService.CleanupUnfinishedSessions()
		if err != nil {
			applog.LogErrorf(s.ctx, "Failed to cleanup unfinished sessions: %v", err)
		} else {
			applog.LogInfof(s.ctx, "Successfully cleaned up unfinished sessions")
		}
	}
}

// autoBackupGameSave 自动备份游戏存档
func (s *StartService) autoBackupGameSave(gameID string) {
	// 检查是否设置了存档目录
	game, err := s.gameService.GetGameByID(gameID)
	if err != nil || game.SavePath == "" {
		applog.LogDebugf(s.ctx, "Game %s has no save path configured, skipping auto backup", gameID)
		return
	}

	// 执行备份
	applog.LogInfof(s.ctx, "Auto backing up game save for: %s", gameID)
	backup, err := s.backupService.CreateBackup(gameID)
	if err != nil {
		applog.LogErrorf(s.ctx, "Failed to auto backup game save: %v", err)
		return
	}

	// 如果启用了游戏存档自动上传到云端
	if s.config.AutoUploadSaveToCloud && s.config.CloudBackupEnabled && s.config.BackupUserID != "" {
		applog.LogInfof(s.ctx, "Auto uploading backup to cloud: %s", backup.Path)
		err = s.backupService.UploadGameBackupToCloud(gameID, backup.Path)
		if err != nil {
			applog.LogErrorf(s.ctx, "Failed to auto upload backup to cloud: %v", err)
		} else {
			applog.LogInfof(s.ctx, "Successfully uploaded backup to cloud: %s", backup.Path)
		}
	}
	applog.LogInfof(s.ctx, "Auto backup completed for game: %s", gameID)
}

// getGamePathAndProcess 获取游戏路径和已保存的进程名
func (s *StartService) getGamePathAndProcess(gameID string) (path string, processName string, err error) {
	if s.gameService == nil {
		return "", "", fmt.Errorf("game service is not initialized")
	}
	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		return "", "", err
	}
	return game.Path, game.ProcessName, nil
}

// resolveExecutablePath 当路径为空或路径是目录时，引导用户选择可执行文件并保存到游戏配置
func (s *StartService) resolveExecutablePath(gameID string, path string, processName string) (string, string, bool, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		applog.LogInfof(s.ctx, "game path is empty for game %s, prompting executable selection", gameID)
		selection, err := s.gameService.SelectGameExecutable("")
		if err != nil {
			return "", "", false, fmt.Errorf("open executable dialog failed: %w", err)
		}
		return s.saveSelectedExecutablePath(gameID, selection, processName)
	}

	normalizedPath, err := filepath.Abs(filepath.Clean(trimmedPath))
	if err != nil {
		return "", "", false, fmt.Errorf("normalize game path failed: %w", err)
	}

	info, err := os.Stat(normalizedPath)
	if err != nil {
		return "", "", false, fmt.Errorf("stat game path: %w", err)
	}
	if !info.IsDir() || gamehelper.IsMacAppBundlePath(normalizedPath) {
		return normalizedPath, strings.TrimSpace(processName), false, nil
	}

	selection, err := s.gameService.ResolveExecutablePathForImport(normalizedPath)
	if err != nil {
		return "", "", false, fmt.Errorf("open executable dialog failed: %w", err)
	}
	if selection == "" {
		return "", "", true, nil
	}

	return s.saveSelectedExecutablePath(gameID, selection, processName)
}

func (s *StartService) saveSelectedExecutablePath(gameID string, selection string, processName string) (string, string, bool, error) {
	if s.gameService == nil {
		return "", "", false, fmt.Errorf("game service is not initialized")
	}

	selection = strings.TrimSpace(selection)
	if selection == "" {
		return "", "", true, nil
	}

	resolvedSelection, err := filepath.Abs(filepath.Clean(selection))
	if err != nil {
		return "", "", false, fmt.Errorf("normalize selected executable failed: %w", err)
	}
	selectionInfo, err := os.Stat(resolvedSelection)
	if err != nil {
		return "", "", false, fmt.Errorf("stat selected executable failed: %w", err)
	}
	if selectionInfo.IsDir() && !gamehelper.IsMacAppBundlePath(resolvedSelection) {
		return "", "", false, fmt.Errorf("selected path is a directory, not executable")
	}

	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		return "", "", false, fmt.Errorf("failed to load game for path update: %w", err)
	}

	resolvedProcessName := strings.TrimSpace(processName)
	if resolvedProcessName == "" {
		resolvedProcessName = filepath.Base(resolvedSelection)
	}

	game.Path = resolvedSelection
	if strings.TrimSpace(game.ProcessName) == "" {
		game.ProcessName = resolvedProcessName
	}
	if err := s.gameService.UpdateGame(game); err != nil {
		return "", "", false, fmt.Errorf("failed to save selected executable: %w", err)
	}

	return resolvedSelection, resolvedProcessName, false, nil
}

// getGameLaunchConfig 获取游戏的启动配置
func (s *StartService) getGameLaunchConfig(gameID string) (useLE bool, useMagpie bool, err error) {
	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		return false, false, err
	}
	return game.UseLocaleEmulator, game.UseMagpie, nil
}

// startMagpie 启动 Magpie 程序
func (s *StartService) startMagpie() {
	// 延迟一小段时间，确保游戏窗口已经创建
	time.Sleep(1 * time.Second)

	// 检查 Magpie 是否已经在运行
	isRunning, err := processutils.CheckIfProcessRunning("Magpie.exe")
	if err != nil {
		applog.LogErrorf(s.ctx, "Failed to check Magpie process: %v", err)
		return
	}

	if isRunning {
		applog.LogInfof(s.ctx, "Magpie is already running")
		return
	}

	// 启动 Magpie (tray 模式)
	applog.LogInfof(s.ctx, "Starting Magpie in tray mode: %s", s.config.MagpiePath)
	cmd := exec.Command(s.config.MagpiePath, "-t")
	cmd.Dir = filepath.Dir(s.config.MagpiePath)

	if err := cmd.Start(); err != nil {
		applog.LogErrorf(s.ctx, "Failed to start Magpie: %v", err)
		return
	}

	// 分离进程，避免阻塞
	if cmd.Process != nil {
		cmd.Process.Release()
	}

	applog.LogInfof(s.ctx, "Magpie started successfully")
}
