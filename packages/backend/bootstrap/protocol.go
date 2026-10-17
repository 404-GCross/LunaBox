package bootstrap

import (
	"fmt"
	goruntime "runtime"
	"strings"
	"time"

	"lunabox/backend/applog"
	"lunabox/backend/appruntime"
	"lunabox/backend/cli/ipcclient"
	"lunabox/backend/common/vo"
	"lunabox/backend/protocol"
	"lunabox/backend/utils/apputils"
)

// ProtocolRequest 是 lunabox:// 链接解析后的请求。
type ProtocolRequest struct {
	rawURL  string
	install *vo.InstallRequest
	launch  *vo.ProtocolLaunchRequest
}

// ExtractAutostartLaunchFlag 去掉命令行里的 --autostart，报告是否有它（由开机自启动启动）。
func ExtractAutostartLaunchFlag(args []string) ([]string, bool) {
	cleanArgs := make([]string, 0, len(args))
	launchedByAutostart := false

	for _, arg := range args {
		if strings.EqualFold(strings.TrimSpace(arg), appruntime.AutostartLaunchArgument) {
			launchedByAutostart = true
			continue
		}
		cleanArgs = append(cleanArgs, arg)
	}

	return cleanArgs, launchedByAutostart
}

// ProtocolArg 返回命令行里唯一的 lunabox:// 参数（系统打开链接时这样启动程序）。
func ProtocolArg(args []string) (string, bool) {
	if len(args) == 1 && protocol.IsProtocolURL(args[0]) {
		return args[0], true
	}
	return "", false
}

// ParseProtocolRequest 解析 lunabox:// 链接。macOS 暂不支持 lunabox://launch。
func ParseProtocolRequest(rawURL string) (*ProtocolRequest, error) {
	action, err := protocol.ParseAction(rawURL)
	if err != nil {
		return nil, err
	}

	req := &ProtocolRequest{rawURL: rawURL}
	switch action {
	case protocol.ActionInstall:
		installReq, err := protocol.ParseInstallURL(rawURL)
		if err != nil {
			return nil, err
		}
		req.install = installReq
	case protocol.ActionLaunch:
		if goruntime.GOOS == "darwin" {
			return nil, fmt.Errorf("lunabox://launch is not supported on macOS yet")
		}
		launchReq, err := protocol.ParseLaunchURL(rawURL)
		if err != nil {
			return nil, err
		}
		req.launch = launchReq
	default:
		return nil, fmt.Errorf("unsupported URL action: %s", action)
	}

	return req, nil
}

// Forward 把请求交给已在运行的 LunaBox（通过 CLI 的 IPC 服务）。
func (r *ProtocolRequest) Forward() error {
	switch {
	case r == nil:
		return nil
	case r.install != nil:
		return ipcclient.RemoteInstall(r.install)
	case r.launch != nil:
		return ipcclient.RemoteLaunch(r.launch)
	default:
		return fmt.Errorf("unsupported protocol request: %s", r.rawURL)
	}
}

// TakeProtocolRequest 处理启动参数里的 lunabox:// 链接：已有 LunaBox 在运行时转交给它（forwarded 为 true，
// 本进程随后退出），否则返回请求，由本进程启动完成后处理（Options.InitialRequest）。没有链接时返回 nil。
func TakeProtocolRequest(args []string, logger *applog.FileLogger) (req *ProtocolRequest, forwarded bool, err error) {
	rawURL, ok := ProtocolArg(args)
	if !ok {
		return nil, false, nil
	}
	req, err = ParseProtocolRequest(rawURL)
	if err != nil {
		logger.Error("failed to parse protocol URL: " + err.Error())
		return nil, false, fmt.Errorf("parsing protocol URL: %w", err)
	}
	if !ipcclient.IsServerRunning() {
		return req, false, nil
	}
	if err := req.Forward(); err != nil {
		logger.Error("failed to forward protocol request to running instance: " + err.Error())
		return nil, false, fmt.Errorf("forwarding protocol request to LunaBox: %w", err)
	}
	logger.Info("protocol request forwarded to running instance")
	return req, true, nil
}

// DispatchProtocolRequest 在本进程里处理请求：安装请求交给下载服务并显示主窗口，启动请求稍后启动游戏。
func (a *App) DispatchProtocolRequest(req *ProtocolRequest) {
	if req == nil {
		return
	}

	if req.install != nil {
		a.Services.Download.SetPendingInstall(req.install)
		a.ShowMainWindow()
		a.emit(EventInstallPending, req.install)
		return
	}

	if req.launch != nil {
		launchReq := *req.launch
		go func() {
			// 启动事件可能在界面订阅 protocol-launch:error 之前到达，先等界面就绪。
			time.Sleep(1200 * time.Millisecond)
			if err := a.Services.Start.HandleProtocolLaunch(launchReq); err != nil {
				a.logger.Error("protocol launch failed: " + err.Error())
			}
		}()
	}
}

// HandleLaunchedWithURL 处理系统在运行中交来的 lunabox:// 链接（例如 macOS 打开链接），启动完成后再分发。
// 与启动参数相同的链接在启动后一段时间内只处理一次。
func (a *App) HandleLaunchedWithURL(rawURL string) {
	req, err := ParseProtocolRequest(rawURL)
	if err != nil {
		a.logger.Error("failed to handle protocol URL: " + err.Error())
		return
	}
	initial := a.o.InitialRequest
	if initial != nil && req.rawURL == initial.rawURL && time.Now().Before(a.initialDuplicateSuppressUntil) {
		if !a.initialRequestHandled.CompareAndSwap(false, true) {
			a.logger.Info("ignored duplicate initial protocol URL event")
			return
		}
		a.logger.Info("dispatching initial protocol request from URL event")
	}
	go func() {
		<-a.startupDone
		if a.startupReady.Load() {
			a.DispatchProtocolRequest(req)
		}
	}()
}

// repairStaleAppImageProtocolRegistration 在 AppImage 换了位置后修复 lunabox:// 的注册。
func repairStaleAppImageProtocolRegistration(logger *applog.FileLogger) {
	if goruntime.GOOS != "linux" || !apputils.IsAppImageMode() {
		return
	}

	currentPath, err := apputils.GetLaunchExecutablePath()
	if err != nil {
		logger.Warning("failed to resolve AppImage path for protocol repair: " + err.Error())
		return
	}
	registeredPath, err := protocol.GetRegisteredURLSchemeExe()
	if err != nil {
		logger.Warning("failed to query protocol registration for AppImage repair: " + err.Error())
		return
	}
	registeredPath = strings.TrimSpace(registeredPath)
	if !protocol.RegistrationNeedsRepair(registeredPath, currentPath) {
		return
	}

	if err := protocol.RegisterPortableURLScheme(currentPath); err != nil {
		logger.Warning("failed to repair stale AppImage protocol registration: " + err.Error())
		return
	}
	logger.Info(fmt.Sprintf("repaired stale AppImage protocol registration: %s -> %s", registeredPath, currentPath))
}
