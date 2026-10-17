package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/env/envtest"
	"lunabox/desktop-mygo/ui/locales"
)

// newEventApp 返回订阅了应用级事件的 App；没有窗口时事件在发出它的 goroutine 上同步处理。
func newEventApp(t *testing.T) (*App, *ui.Tester, *host.Backend) {
	t.Helper()
	svc := hosttest.NewServices(t)
	a := New(envtest.NewBackend(0), svc.API(), Options{})
	a.SetScale(1.5)
	a.subscribe()
	tt := ui.NewTester(a.View, 1280, 800)
	tt.SetScale(1.5)
	tt.Frame()
	return a, tt, svc
}

func hasTextContaining(tt *ui.Tester, s string) bool {
	for _, text := range tt.Texts() {
		if strings.Contains(text, s) {
			return true
		}
	}
	return false
}

func TestQuitSyncBacksUpThenQuits(t *testing.T) {
	_, tt, svc := newEventApp(t)
	quit := make(chan struct{}, 2)
	svc.Services.Config.SetQuitHandler(func() { quit <- struct{}{} })

	svc.Events.Emit(eventQuitSyncRequested, map[string]string{"reason": "window-close"})
	tt.Frame()
	if !hasTextContaining(tt, locales.ExitSyncToastSuccessTitle()) {
		t.Fatalf("missing success toast in %q", tt.Texts())
	}
	select {
	case <-quit:
	case <-time.After(5 * time.Second):
		t.Fatal("SafeQuit was not called after the backup")
	}
	status, err := svc.Backup.GetDBBackups()
	if err != nil {
		t.Fatal(err)
	}
	if status == nil || len(status.Backups) == 0 {
		t.Fatalf("no database backup created: %+v", status)
	}

	// 再次请求不会重复备份与退出。
	svc.Events.Emit(eventQuitSyncRequested, nil)
	select {
	case <-quit:
		t.Fatal("quit sync ran twice")
	case <-time.After(exitSuccessDelay + 300*time.Millisecond):
	}
}

func TestProtocolLaunchErrorToast(t *testing.T) {
	_, tt, svc := newEventApp(t)
	svc.Events.Emit(eventProtocolLaunchError, vo.ProtocolLaunchErrorEvent{Message: "快捷启动失败", Detail: "未找到游戏"})
	tt.Frame()
	if !hasTextContaining(tt, "快捷启动失败") {
		t.Fatalf("missing protocol launch error toast in %q", tt.Texts())
	}
}
