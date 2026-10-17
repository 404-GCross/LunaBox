# LunaBox Agent 地图

> 这是一份**地图**，不是手册。先读这里，按需跳转专题文档。

## 项目概况

Wails v3 Alpha 桌面应用（仅 Windows）。
前端：React + TypeScript + UnoCSS（presetWind3）+ Zustand + TanStack Router。
后端：Go + DuckDB + 自研 migrations。

## 仓库结构（monorepo）

| 目录 | 内容 |
|------|------|
| `apps/desktop/` | 桌面应用（Go module `lunabox`：`main.go`、`internal/`、`frontend/`、`build/`、`cmd/`、`scripts/`） |
| `apps/update-server/` | Cloudflare Worker 更新服务（独立 pnpm 项目） |
| `apps/desktop-mygo/` | MyGo 原生 UI 原型（Go module `lunabox/desktop-mygo`；`ui/` 按原版 `frontend/src` 分为 `pages`、`components`、`theme` 等包，`ui.Router` 路由；`replace` 引用 `apps/desktop` 的真实服务；与原版逐像素对照见 `docs/parity.md`；仅顶栏/侧边栏/游戏库/游戏详情/收藏/设置/下载，不参与发布） |
| `packages/updater/` | 独立更新器与更新资产生成器（Go module `lunabox/updater`，桌面应用通过 `replace` 引用） |
| `sync/` | 版本源 `version.json`（发布流程与 Pages/Netlify 依赖此路径，不要移动） |
| `docs/` | 专题文档 |

- MUST 各子项目独立管理依赖：Go 不使用 `go.work`，JS 不共用根级 pnpm workspace（根 `package.json` 只放脚本，不加依赖）。
- 本文件与 `docs/` 中未标注前缀的路径（如 `internal/...`、`frontend/...`），均相对 `apps/desktop/`；桌面应用相关命令在 `apps/desktop/` 下执行。
- 根目录 `package.json` 是常用命令入口，可直接在仓库根执行：`pnpm setup`（安装各子项目依赖）、`pnpm dev`、`pnpm build`、`pnpm bindings`、`pnpm check`（桌面编译 + 更新器测试 + 更新服务检查）等，完整列表见其 `scripts`。

## 关键词优先级

- **MUST**：必须遵守（违反即视为实现不合格）
- **SHOULD**：强烈建议（有明确原因可偏离，需说明）
- **MAY**：可选

当本文件与用户需求冲突时：**以用户需求为准**。

---

## 核心约束（每次任务都适用）

1. **最小可行改动**：优先改动已有文件，复用已有模式，不新增架构性模块。
2. **先搜再新建**：新增组件/函数/SQL 前，先搜索对应目录是否已有实现。
3. **跟随仓库风格**：路由组织、service 注入、migration 形态等，沿用当前写法。
4. **一次只解决一个问题域**：不顺手重构或格式化不相关代码。
5. **统一 User-Agent（MUST）**：后端 HTTP 请求不得硬编码应用 UA，默认值统一调用 `internal/version.UserAgent()`；仅当上游明确要求时才允许传入完整自定义 UA，版本号不得单独维护。

---

## 何时必须问用户

仅以下情况才追问（否则按最简单解释执行）：

- 需求影响数据结构/迁移策略（是否需要数据回填、是否允许破坏性迁移）
- UI/交互存在多种合理方案且影响用户使用习惯（如新增入口位置）
- 需要引入新依赖或大改目录结构

---

## 关键文件落点（快速定位）

详见 → [docs/anchors.md](docs/anchors.md)

| 类型 | 文件 |
|------|------|
| 路由注册 | `frontend/src/App.tsx` |
| 根布局 | `frontend/src/routes/__root.tsx` |
| 全局 Store | `frontend/src/store.ts` |
| UnoCSS 配置 | `frontend/uno.config.ts` |
| 后端启动/注入 | `main.go` |
| 初始建表 | `internal/migrations/init.go` |
| Migrations | `internal/migrations/migrations.go` |
| Services | `internal/service/*_service.go` |
| 后端工具函数 | `internal/utils/*`（按场景细分 package） |

---

## 渐进式披露阅读顺序

按任务复杂度逐层展开，不要一上来通读全部文档：

1. 先看本文件，只确定任务属于前端、后端还是流程问题
2. 涉及后端 service / DB / migration：先读 [backend.md](docs/backend.md)
3. 涉及文件、压缩包、图片、进程、代理、元数据抓取等辅助能力：再从 [backend-utils.md](docs/backend-utils.md) 进入对应 `internal/utils/*` 子包
4. 仍不确定文件落点：回到 [anchors.md](docs/anchors.md)

---

## 专题文档索引

按任务类型，读取对应文档：

| 任务类型 | 读取文档 |
|----------|----------|
| 新增/修改前端页面、组件、样式 | [frontend.md](docs/frontend.md) |
| 新增/修改后端 service、DB、migration | [backend.md](docs/backend.md) |
| 需要复用后端工具函数（`internal/utils/*`） | [backend-utils.md](docs/backend-utils.md) |
| 不确定文件落点 | [anchors.md](docs/anchors.md) |
| 提交前自检、变更流程 | [workflow.md](docs/workflow.md) |

---

## 交付要求

- 后端改动：在 `apps/desktop/` 下 `go build -tags dev` 不新增编译错误，`wails3 generate bindings -ts` 可运行。
- 前端改动：在 `apps/desktop/frontend/` 下 `pnpm build` 可通过。
- 更新器改动：在 `packages/updater/` 下 `go test ./...` 通过，并确认 `apps/desktop/` 仍可编译。
- 更新服务改动：在 `apps/update-server/` 下 `pnpm typecheck` 与 `pnpm test` 通过。

## IMPORTANT

- MUST `frontend/bindings/` 是 Wails v3 自动生成的绑定，不要手改；业务代码通过具体 service 文件或 `frontend/src/bindings/` 兼容入口使用后端类型，不要依赖可能产生重复导出的 package 聚合 `index.ts`。
