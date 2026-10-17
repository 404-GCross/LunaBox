# 关键文件落点（Anchors）

> 除特别标注外，本文路径均相对桌面应用目录 `apps/desktop/`。

## 前端

| 职责                                           | 路径                                                |
| ---------------------------------------------- | --------------------------------------------------- |
| 路由注册（routeTree 组装）与应用级运行时编排   | `frontend/src/App.tsx`                              |
| 根布局（TopBar/SideBar、data-glass、拖拽遮罩） | `frontend/src/routes/__root.tsx`                    |
| 页面路由文件                                   | `frontend/src/routes/*.tsx`（每个页面导出 `Route`） |
| 全局 Store（Zustand）                          | `frontend/src/store.ts`                             |
| 应用级运行时事件监听                           | `frontend/src/hooks/useAppRuntimeEffects.ts`        |
| 退出前云同步 / 退出提示                        | `frontend/src/hooks/useExitSyncToast.ts`            |
| UnoCSS 配置                                    | `frontend/uno.config.ts`                            |
| 全局样式（仅全局不可避免项）                   | `frontend/src/style.css`                            |
| Wails v3 绑定（自动生成，勿手改）              | `frontend/bindings/`                                |
| Wails 绑定与 runtime 兼容入口                  | `frontend/src/bindings/`                            |
| 工具函数                                       | `frontend/src/utils/`                               |
| 时间处理函数                                   | `frontend/src/utils/time.ts`                        |
| 可复用 UI 组件                                 | `frontend/src/components/ui/`                       |
| HeadlessUI 封装参考                            | `frontend/src/components/ui/BetterSelect.tsx`       |
| Radix 封装参考                                 | `frontend/src/components/ui/BetterSwitch.tsx`       |

## 后端

| 职责                              | 路径                                                                                                                                                |
| --------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| 启动、退出生命周期与依赖注入      | `packages/backend/bootstrap/`（Wails 外壳在 `main.go`）                                                                                                                                         |
| 初始建表（新安装时的完整 schema） | `packages/backend/migrations/init.go` → `InitSchema(...)`                                                                                                   |
| Migration 列表                    | `packages/backend/migrations/migrations.go`                                                                                                                 |
| Services                          | `packages/backend/service/*_service.go`                                                                                                                     |
| 游戏启动策略                      | `packages/backend/service/launcher/*.go`                                                                                                                    |
| 配置读写与运行时配置回写          | `packages/backend/service/config_service.go`                                                                                                                |
| 工具函数                          | `packages/backend/utils/`（分 `apputils` / `archiveutils` / `downloadutils` / `imageutils` / `metadata` / `processutils` / `proxyutils` / `timerutils` 等） |
| 工具函数索引                      | `docs/backend-utils.md`                                                                                                                             |
| Windows API 封装参考              | `packages/backend/utils/processutils/process_windows.go`                                                                                                    |
| 客户端更新协调                    | `packages/backend/updateclient/`                                                                                                                            |
| 独立更新器与更新资产生成器        | 仓库根 `packages/updater/`                                                                                                                                 |
| Cloudflare 更新服务               | 仓库根 `apps/update-server/`                                                                                                                                 |
