package update

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/backend"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/env/envtest"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/appconf"
	"lunabox/internal/service"
)

// fakeCheck 用真实的 UpdateService，只替换联网的检查。
type fakeCheck struct {
	*service.UpdateService
	result *service.UpdateCheckResult
	err    error
	calls  int
}

func (f *fakeCheck) CheckForUpdates() (*service.UpdateCheckResult, error) {
	f.calls++
	return f.result, f.err
}

type harness struct {
	svc     *backend.Services
	env     *env.Env
	cfg     appconf.AppConfig
	changes int
	state   State
	fake    *fakeCheck
	tt      *ui.Tester
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{svc: envtest.NewServices(t)}
	h.cfg = *h.svc.Config
	h.env = &env.Env{Svc: h.svc, Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.fake = &fakeCheck{UpdateService: h.svc.Update}
	h.state.api = h.fake
	h.tt = ui.NewTester(h.view, 1024, 900)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.tt.Frame()
	return h
}

func (h *harness) view(c *ui.Context) {
	t := h.env.Theme
	p := panel.Settings{
		Env:       h.env,
		Config:    &h.cfg,
		Change:    func(fn func(*appconf.AppConfig)) { fn(&h.cfg); h.changes++ },
		PatchLive: func(fn func(*appconf.AppConfig)) { fn(&h.cfg) },
		Refresh:   func() {},
	}
	ui.Column(c).Fill().Padding(20).Gap(16).Background(t.AppBg).Children(func() { View(c, t, &h.state, p) })
}

func (h *harness) frames(n int) {
	for range n {
		h.tt.Frame()
	}
}

func (h *harness) click(t *testing.T, s string) {
	t.Helper()
	if err := h.tt.Click(s); err != nil {
		t.Fatalf("%v; texts %q", err, h.tt.Texts())
	}
	h.frames(2)
}

func (h *harness) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

func TestAutoCheckSwitch(t *testing.T) {
	h := newHarness(t)
	h.expect(t, locales.SettingsUpdateAutoCheckLabel(), locales.SettingsUpdateAutoCheckHint(), locales.SettingsUpdateManualCheckBtn())
	before := h.cfg.CheckUpdateOnStartup
	h.click(t, locales.SettingsUpdateAutoCheckLabel())
	if h.cfg.CheckUpdateOnStartup == before || h.changes != 1 {
		t.Fatalf("label click: check_update_on_startup %v, changes %d", h.cfg.CheckUpdateOnStartup, h.changes)
	}
}

func TestUpToDateAndError(t *testing.T) {
	h := newHarness(t)
	h.fake.result = &service.UpdateCheckResult{CurrentVer: "1.2.3", LatestVer: "1.2.3"}
	h.click(t, locales.SettingsUpdateManualCheckBtn())
	h.expect(t, locales.SettingsUpdateUpToDate(), "1.2.3")
	if h.state.showDialog || h.state.checking {
		t.Errorf("dialog %v, checking %v", h.state.showDialog, h.state.checking)
	}

	h.fake.result, h.fake.err = nil, errors.New("all update URLs failed")
	h.click(t, locales.SettingsUpdateManualCheckBtn())
	h.expect(t, locales.SettingsUpdateCheckFailedTitle(), "all update URLs failed")
	if h.tt.HasText(locales.SettingsUpdateUpToDate()) {
		t.Error("up-to-date hint kept after a failed check")
	}
}

func TestNewVersionDialog(t *testing.T) {
	h := newHarness(t)
	h.fake.result = &service.UpdateCheckResult{
		HasUpdate: true, CurrentVer: "1.2.3", LatestVer: "1.3.0", ReleaseDate: "2026-10-01",
		Changelog:         []string{"新增设置页", "修复若干问题"}, // i18n:keep：测试数据
		Downloads:         map[string]string{"github": "https://github.com/x", "gitee": "https://gitee.com/x"},
		UpdateManifestURL: "https://example.invalid/manifest.json",
	}
	h.click(t, locales.SettingsUpdateManualCheckBtn())
	if !h.state.showDialog {
		t.Fatal("dialog not shown")
	}
	h.tt.Advance(400 * time.Millisecond)
	h.expect(t, locales.UpdateDialogNewVersion(), locales.UpdateDialogAvailable(), "v1.2.3", "v1.3.0", "2026-10-01",
		locales.UpdateDialogChangelog(), "新增设置页", "修复若干问题", locales.UpdateDialogSkipVersion()) // i18n:keep：测试数据
	if h.tt.HasText(locales.SettingsUpdateNewVersion() + " ") {
		t.Error("hint shown while the dialog is open")
	}
	// Windows 是应用内更新（原型只提示尚未迁移），macOS、Linux 才有手动下载按钮。
	if runtime.GOOS == "windows" {
		if h.tt.HasText(locales.UpdateDialogGithubDownload()) {
			t.Error("manual download shown on Windows")
		}
		h.click(t, locales.UpdateDialogUpdateNow())
		h.expect(t, locales.NativeFeatureNotMigrated(locales.UpdateDialogUpdateNow()))
	}

	// 关闭：淡出 200ms 后关闭，面板里留下新版本提示，点击再次打开。
	h.click(t, locales.UpdateDialogClose())
	if !h.state.showDialog {
		t.Fatal("dialog closed before fading out")
	}
	h.tt.Advance(300 * time.Millisecond)
	h.frames(1)
	if h.state.showDialog {
		t.Fatalf("dialog still open after fading out: %+v %q", h.state.dialog, h.tt.Texts())
	}
	h.expect(t, locales.SettingsUpdateNewVersion(), "v1.3.0")
	h.click(t, locales.SettingsUpdateNewVersion())
	if !h.state.showDialog {
		t.Fatal("hint did not reopen the dialog")
	}
	h.tt.Advance(400 * time.Millisecond)

	// 点击遮罩同样关闭；再从提示打开。
	h.tt.ClickAt(4, 100)
	h.tt.Advance(300 * time.Millisecond)
	h.frames(1)
	if h.state.showDialog {
		t.Fatal("backdrop click did not close the dialog")
	}
	h.click(t, locales.SettingsUpdateNewVersion())
	h.tt.Advance(400 * time.Millisecond)

	// 跳过此版本：写进配置，提示换成已是最新。
	h.click(t, locales.UpdateDialogSkipVersion())
	if got := h.svc.Config.SkipVersion; got != "1.3.0" {
		t.Errorf("skip_version = %q", got)
	}
	h.tt.Advance(300 * time.Millisecond)
	h.frames(1)
	if h.state.showDialog || h.state.info.HasUpdate {
		t.Errorf("dialog %v, has update %v after skip", h.state.showDialog, h.state.info.HasUpdate)
	}
	h.expect(t, locales.SettingsUpdateUpToDate(), "1.2.3")
}
