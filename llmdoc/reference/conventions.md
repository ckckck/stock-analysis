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
- Wails 应用展示名是“散牛盘”，Windows 构建产物文件名也改为 `散牛盘.exe`；但内部工程目录、Go module 和用户数据目录仍保留 `jcp` 命名，见 `jcp/wails.json:3`、`jcp/wails.json:4`、`jcp/internal/pkg/paths/paths.go:14`。
- AI 筛选同步只保存日线相关数据，不保存分钟级别数据；默认市场范围是沪市和深市，可选补充北交所与指数。深市 `300`、`301` 开头的创业板个股会在股票池构建时被排除，不参与同步和筛选，但指数范围不受影响，见 `jcp/internal/models/config.go:132`、`jcp/internal/services/screening_sync_service.go:67`、`jcp/internal/services/market_service.go:632`。
- AI 筛选同步还会进一步排除 `is_active = false` 的股票；即使本地 SQLite 里还保留这类股票的旧日线，它们也不会再进入新的同步队列或筛选 universe。
- 对于仍被基础股票名录视为活跃、但连续 3 次同步都 `no new bars`，且源最新日期至少落后目标交易日 20 个交易日的 symbol，本地会写入 `sync_symbol_states.excluded=1`。这类股票会从后续同步队列、同步覆盖率统计和 `ListScreeningUniverseSymbols()` 返回的股票池中移除，但不会删除其历史日线数据。
- AI 筛选 SQL 必须是 `SELECT` 或 `WITH ... SELECT`，只能引用白名单表/视图，并遵守 `ORDER BY` 与 `LIMIT` 规则；不允许写操作或额外 SQLite 指令，见 `jcp/internal/services/screening_query_service.go:15`、`jcp/internal/services/screening_query_service.go:22`、`jcp/internal/services/screening_query_service.go:131`。
- AI 筛选同步的日线源优先走 `Baostock`，回退到新的 `money.finance.sina.com.cn` 日线接口；`Baostock` 对空成交量/空成交额按 0 容错，单只股票取数失败会记录诊断事件后继续后续队列，不再直接打断整轮同步，见 `jcp/internal/services/market_service.go:508`、`jcp/internal/services/baostock_daily_bar_source.go:319`、`jcp/internal/services/screening_sync_service.go:419`。
- 历史筛选记录保存的是执行时的 SQL、作用域和结果快照；回放历史读取快照，不重新请求模型，也不重新跑 SQL。若用户从历史记录再次执行，是否复用 SQL 由“提示词、市场范围、结果模式、结果条数、测试范围”共同决定；只要任一项变化，就重新请求模型生成 SQL，见 `jcp/internal/services/screening_store.go:121`、`jcp/internal/services/screening_query_service.go:217`、`jcp/frontend/src/utils/screeningHistoryReuse.ts:1`。
- 历史筛选记录支持显式删除；删除入口走前端确认弹框，后端删除 `screening_runs` 主记录后依赖外键级联清理结果快照，见 `jcp/frontend/src/App.tsx`、`jcp/internal/services/screening_store.go:599`。
- 当前环境里若 `wails` CLI 不可用，前后端新增绑定需要手工同步 `jcp/frontend/wailsjs/go/main/App.d.ts` 与 `jcp/frontend/wailsjs/go/main/App.js`，避免只改 Go 导致前端调用缺失。
