# Conventions

## 1. 核心摘要

- 当前主仓库根目录是 `stock-analysis/`；项目代码主体在 `jcp/`，而不是仓库根直接平铺。
- `jcp/` 已不再作为嵌套 Git 工作树使用，顶层仓库统一跟踪它的文件内容；上游 Git 信息被转移到 `.upstreams/` 并被忽略，见 `.gitignore:1`、`docs/upstream-sync.md:1`。
- `llmdoc/` 用于记录当前顶层项目视角，不应把历史上游仓库结构和当前主仓库结构混写。

## 2. 信息源

### 运行与架构

- Wails 应用入口与前后端绑定：`jcp/main.go:21`、`jcp/main.go:33`
- 后端主装配点：`jcp/app.go:52`
- 前端主界面：`jcp/frontend/src/App.tsx:46`

### 本地数据落盘

- 应用数据目录：`jcp/internal/pkg/paths/paths.go:8`
- AI 筛选 SQLite 目录与数据库：`jcp/internal/pkg/paths/paths.go:22`、`jcp/internal/pkg/paths/paths.go:32`
- 配置与自选股：`jcp/internal/services/config_service.go:23`
- 策略：`jcp/internal/services/strategy_service.go:110`
- 会话：`jcp/internal/services/session_service.go:23`
- 记忆：`jcp/internal/memory/manager.go:12`
- AI 筛选表与历史快照：`jcp/internal/services/screening_store.go:81`、`jcp/internal/services/screening_store.go:121`、`jcp/internal/services/screening_store.go:131`

### AI 与扩展

- AI / MCP / OpenClaw 配置模型：`jcp/internal/models/config.go:13`、`jcp/internal/models/config.go:44`、`jcp/internal/models/config.go:106`
- AI 筛选配置模型：`jcp/internal/models/config.go:124`、`jcp/internal/models/config.go:140`
- 内置工具清单：`jcp/internal/adk/tools/registry.go:51`
- 会议服务：`jcp/internal/meeting/service.go:128`
- AI 筛选 SQL 生成与校验：`jcp/internal/services/screening_query_service.go:79`、`jcp/internal/services/screening_query_service.go:131`、`jcp/internal/services/screening_query_service.go:253`
- OpenClaw HTTP 服务：`jcp/internal/openclaw/server.go:43`

### 仓库与同步约定

- 顶层忽略规则：`.gitignore:1`
- 上游同步说明：`docs/upstream-sync.md:1`
- 仓库迁移设计：`docs/plans/2026-03-18-repo-migration-design.md:1`

## 3. 约定补充

- 顶层仓库仍统一管理 `jcp/` 源码；AI 筛选的数据不进仓库，运行时固定落到用户配置目录下的 `jcp/screening/screening.db`，见 `jcp/internal/pkg/paths/paths.go:14`、`jcp/internal/pkg/paths/paths.go:39`。
- AI 筛选同步只保存日线相关数据，不保存分钟级别数据；默认市场范围是沪市和深市，可选补充北交所与指数，见 `jcp/internal/models/config.go:132`、`jcp/internal/services/screening_sync_service.go:67`。
- AI 筛选 SQL 必须是 `SELECT` 或 `WITH ... SELECT`，只能引用白名单表/视图，并遵守 `ORDER BY` 与 `LIMIT` 规则；不允许写操作或额外 SQLite 指令，见 `jcp/internal/services/screening_query_service.go:15`、`jcp/internal/services/screening_query_service.go:22`、`jcp/internal/services/screening_query_service.go:131`。
- 历史筛选记录保存的是执行时的 SQL 和结果快照；回放历史读取快照，不重新请求模型，也不重新跑 SQL，见 `jcp/internal/services/screening_store.go:121`、`jcp/internal/services/screening_store.go:131`、`jcp/internal/services/screening_query_service.go:217`。
- 当前环境里若 `wails` CLI 不可用，前后端新增绑定需要手工同步 `jcp/frontend/wailsjs/go/main/App.d.ts` 与 `jcp/frontend/wailsjs/go/main/App.js`，避免只改 Go 导致前端调用缺失。
