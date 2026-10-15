//go:build linux

package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lunabox/internal/applog"
	"lunabox/internal/cli"
	"lunabox/internal/cli/ipcserver"
	"lunabox/internal/service"
	"lunabox/internal/utils/dbutils"
	"lunabox/internal/wailsruntime"
)

// runHeadless 以无界面模式运行 LunaBox：不创建窗口与托盘，只装配核心 service 并启动本地
// IPC/HTTP 服务，供 SteamOS 上的 Decky 插件调用。收到 SIGINT/SIGTERM 后按 OnShutdown 的
// 职责做无交互清理。
//
// 与 GUI 还不互斥：两种模式会争抢同一个本地端口与数据库，不要同时运行。
func runHeadless(appLogger *applog.FileLogger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := bootstrapCore(ctx, appLogger); err != nil {
		return err
	}
	defer closeHeadlessDatabase(appLogger)

	coreApp := newHeadlessCoreApp(ctx)
	if err := coreApp.SessionService.CleanupUnfinishedSessions(); err != nil {
		appLogger.Error("headless: cleanup unfinished sessions failed: " + err.Error())
	}

	server := ipcserver.StartServer(coreApp, wailsruntime.Unavailable())
	if server == nil {
		return errors.New("启动本地服务失败：没有可用端口")
	}
	defer func() {
		if err := ipcserver.ShutdownServer(server); err != nil {
			appLogger.Error("headless: shutdown local server failed: " + err.Error())
		}
	}()

	appLogger.Info("headless mode started")
	<-ctx.Done()
	appLogger.Info("headless mode shutdown requested")

	return nil
}

// newHeadlessCoreApp 装配 headless 需要的核心 service。复用 cli.CoreApp 这一既有容器，
// 并与 GUI 保持同一套 Init / SetXxxService 注入顺序。
func newHeadlessCoreApp(ctx context.Context) *cli.CoreApp {
	gameService := service.NewGameService()
	backupService := service.NewBackupService()
	sessionService := service.NewSessionService()
	startService := service.NewStartService()
	versionService := service.NewVersionService()

	gameService.Init(ctx, db, config)
	backupService.Init(ctx, db, config)
	sessionService.Init(ctx, db, config)
	startService.Init(ctx, db, config)
	versionService.Init(ctx)

	startService.SetBackupService(backupService)
	startService.SetGameService(gameService)
	startService.SetSessionService(sessionService)

	runtime := wailsruntime.Unavailable()
	gameService.SetRuntime(runtime)
	startService.SetRuntime(runtime)
	backupService.SetRuntime(runtime)

	return &cli.CoreApp{
		Config: config, DB: db, Ctx: ctx, GameService: gameService,
		StartService: startService, SessionService: sessionService,
		BackupService: backupService, VersionService: versionService,
	}
}

// closeHeadlessDatabase 退出前关闭数据库，保证 DuckDB 正常落盘。
func closeHeadlessDatabase(appLogger *applog.FileLogger) {
	if db == nil {
		return
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := dbutils.SafeCloseDuckDB(closeCtx, db, appLogger); err != nil {
		appLogger.Error("headless: close database failed: " + err.Error())
	}
	db = nil
}
