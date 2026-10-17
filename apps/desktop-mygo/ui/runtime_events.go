package ui

import (
	"strings"
	"time"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
)

// 应用级的服务事件，对应 hooks/useAppRuntimeEffects.ts 与 hooks/useExitSyncToast.ts。
const (
	eventQuitSyncRequested   = "app:quit-sync-requested" // bootstrap 在退出前同步时发出
	eventProtocolLaunchError = "protocol-launch:error"
)

// useExitSyncToast 的 EXIT_SYNC_TOAST_ID、EXIT_SUCCESS_DELAY_MS 与 EXIT_ERROR_DELAY_MS。
const (
	exitSyncToastID  = "exit-sync"
	exitSuccessDelay = 700 * time.Millisecond
	exitErrorDelay   = 1600 * time.Millisecond
)

// subscribe 订阅应用级事件；没有后端服务（只测游戏库的测试）时不订阅。
func (a *App) subscribe() {
	svc := a.env.Svc
	if svc == nil || svc.Events == nil {
		return
	}
	svc.Events.On(eventQuitSyncRequested, func(any) { a.runQuitSync() })
	svc.Events.On(eventProtocolLaunchError, a.protocolLaunchError)
}

// notify 在下一帧显示提示（事件回调里没有 ui.Context）。
func (a *App) notify(n env.Notice) {
	a.notices = append(a.notices, n)
	if a.env.Win != nil {
		a.env.Win.Invalidate()
	}
}

// runQuitSync 对应 useExitSyncToast：显示“正在备份数据库”，创建并上传数据库备份，成功或失败都提示后稍等再 SafeQuit。
func (a *App) runQuitSync() {
	if a.quitSyncing {
		return
	}
	a.quitSyncing = true
	a.notify(env.Notice{Type: "loading", ID: exitSyncToastID, Duration: -1,
		Msg: locales.ExitSyncToastSyncingTitle() + "\n" + locales.ExitSyncToastSyncingMessage()})
	svc := a.env.Svc
	a.env.Async(func() func() {
		_, err := svc.Backup.CreateAndUploadDBBackupForQuit()
		return func() {
			delay := exitSuccessDelay
			if err != nil {
				delay = exitErrorDelay
				a.notify(env.Notice{Type: "error", ID: exitSyncToastID, Duration: exitErrorDelay + 1200*time.Millisecond,
					Msg: locales.ExitSyncToastErrorTitle() + "\n" + locales.ExitSyncToastErrorMessage(err.Error())})
			} else {
				a.notify(env.Notice{Type: "success", ID: exitSyncToastID, Duration: exitSuccessDelay + time.Second,
					Msg: locales.ExitSyncToastSuccessTitle() + "\n" + locales.ExitSyncToastSuccessMessage()})
			}
			time.AfterFunc(delay, svc.AppConfig.SafeQuit)
		}
	})
}

// protocolLaunchError 对应 protocol-launch:error：显示窗口并提示快捷启动失败的原因。
// 原版缺少 Wine 运行器时提示里的“打开启动配置”只在 macOS、Linux 上出现，这里不需要。
func (a *App) protocolLaunchError(data any) {
	ev, ok := data.(vo.ProtocolLaunchErrorEvent)
	if !ok {
		return
	}
	msg := strings.TrimSpace(ev.Message)
	if detail := strings.TrimSpace(ev.Detail); detail != "" {
		msg += "\n" + detail
	}
	if a.env.Win != nil {
		a.env.Win.Show()
	}
	a.notify(env.Notice{Type: "error", ID: "protocol-launch-error", Msg: msg})
}
