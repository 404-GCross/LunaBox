package bootstrap

import (
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"lunabox/backend/appconf"
	"lunabox/backend/appruntime"
)

type fakeWindow struct {
	shown, hidden int
	minimised     bool
	maximised     bool
	w, h          int
}

func (w *fakeWindow) Show()                     { w.shown++ }
func (w *fakeWindow) Hide()                     { w.hidden++ }
func (w *fakeWindow) Focus()                    {}
func (w *fakeWindow) UnMinimise()               { w.minimised = false }
func (w *fakeWindow) IsMinimised() bool         { return w.minimised }
func (w *fakeWindow) IsMaximised() bool         { return w.maximised }
func (w *fakeWindow) Size() (width, height int) { return w.w, w.h }

type fakeRuntime struct {
	appruntime.Runtime
	events []string
}

func (r *fakeRuntime) Emit(name string, _ ...any) bool {
	r.events = append(r.events, name)
	return true
}

// newAttachedApp 返回已接上假窗口的 App（不打开数据库），quits 记录框架退出的次数。
func newAttachedApp(config appconf.AppConfig) (*App, *fakeWindow, *fakeRuntime, *atomic.Int32) {
	a := New(Options{})
	a.Config = &config
	w := &fakeWindow{w: 1200, h: 800}
	rt := &fakeRuntime{Runtime: appruntime.Unavailable()}
	quits := new(atomic.Int32)
	a.mu.Lock()
	a.window, a.rt, a.quit = w, rt, func() { quits.Add(1) }
	a.mu.Unlock()
	return a, w, rt, quits
}

func TestMainWindowClosingHidesToTray(t *testing.T) {
	a, w, _, quits := newAttachedApp(appconf.AppConfig{CloseToTray: true})
	a.SetTrayAvailable()
	if a.MainWindowClosing() {
		t.Fatal("close-to-tray let the window close")
	}
	if w.hidden != 1 || quits.Load() != 0 {
		t.Fatalf("hidden %d, quits %d", w.hidden, quits.Load())
	}
	if a.Config.WindowWidth != 1200 || a.Config.WindowHeight != 800 {
		t.Fatalf("window state not captured: %+v", a.Config)
	}
}

func TestMainWindowClosingRequestsQuitSync(t *testing.T) {
	a, w, rt, quits := newAttachedApp(appconf.AppConfig{AutoBackupDB: true})
	w.minimised = true
	if a.MainWindowClosing() {
		t.Fatal("quit sync let the window close")
	}
	if !slices.Equal(rt.events, []string{EventQuitSyncRequested}) || w.shown != 1 || w.minimised || quits.Load() != 0 {
		t.Fatalf("events %q, shown %d, minimised %v, quits %d", rt.events, w.shown, w.minimised, quits.Load())
	}
	// 同步进行中再关闭窗口不重复请求；界面完成后 SafeQuit 直接退出。
	if a.MainWindowClosing() || len(rt.events) != 1 {
		t.Fatalf("second close: events %q", rt.events)
	}
	a.QuitApplication()
	if quits.Load() != 1 || !a.MainWindowClosing() {
		t.Fatalf("quits %d after QuitApplication", quits.Load())
	}
}

func TestMainWindowClosingQuitsWithoutSync(t *testing.T) {
	a, _, _, quits := newAttachedApp(appconf.AppConfig{})
	if a.MainWindowClosing() {
		t.Fatal("window closed before the application quit")
	}
	deadline := time.Now().Add(time.Second)
	for quits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if quits.Load() != 1 {
		t.Fatalf("quits %d", quits.Load())
	}
}

func TestKeepWindowState(t *testing.T) {
	a, _, _, _ := newAttachedApp(appconf.AppConfig{WindowWidth: 1000, WindowHeight: 700})
	a.o.KeepWindowState = true
	a.CaptureWindowState()
	if a.Config.WindowWidth != 1000 || a.Config.WindowHeight != 700 {
		t.Fatalf("window state overwritten: %+v", a.Config)
	}
}

func TestExtractAutostartLaunchFlag(t *testing.T) {
	args, ok := ExtractAutostartLaunchFlag([]string{"-hud", "--AutoStart ", "lunabox://launch?game_id=1"})
	if !ok || !slices.Equal(args, []string{"-hud", "lunabox://launch?game_id=1"}) {
		t.Fatalf("got %q, %v", args, ok)
	}
	if _, ok := ExtractAutostartLaunchFlag([]string{"-hud"}); ok {
		t.Fatal("reported --autostart without it")
	}
}

func TestProtocolArg(t *testing.T) {
	if raw, ok := ProtocolArg([]string{"lunabox://launch?game_id=g1"}); !ok || raw != "lunabox://launch?game_id=g1" {
		t.Fatalf("ProtocolArg = %q, %v", raw, ok)
	}
	for _, args := range [][]string{nil, {"-hud"}, {"lunabox://launch?game_id=g1", "extra"}} {
		if _, ok := ProtocolArg(args); ok {
			t.Errorf("ProtocolArg(%q) reported a protocol URL", args)
		}
	}
}

func TestParseProtocolRequest(t *testing.T) {
	req, err := ParseProtocolRequest("lunabox://launch?game_id=g1")
	if err != nil || req.launch == nil || req.launch.GameID != "g1" || req.install != nil {
		t.Fatalf("launch = %+v, %v", req, err)
	}
	install := fmt.Sprintf("lunabox://install?url=https%%3A%%2F%%2Fexample.com%%2Fa.zip&file_name=a.zip&archive_format=zip&size=1024&expires_at=%d",
		time.Now().Add(time.Hour).Unix())
	req, err = ParseProtocolRequest(install)
	if err != nil || req.install == nil || req.install.URL != "https://example.com/a.zip" {
		t.Fatalf("install = %+v, %v", req, err)
	}
	if _, err := ParseProtocolRequest("lunabox://launch"); err == nil {
		t.Error("launch without game_id parsed")
	}
	if _, err := ParseProtocolRequest("lunabox://unknown"); err == nil {
		t.Error("unknown action parsed")
	}
}
