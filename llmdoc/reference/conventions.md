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
- 配置与自选股：`jcp/internal/services/config_service.go:23`
- 策略：`jcp/internal/services/strategy_service.go:110`
- 会话：`jcp/internal/services/session_service.go:23`
- 记忆：`jcp/internal/memory/manager.go:12`

### AI 与扩展

- AI / MCP / OpenClaw 配置模型：`jcp/internal/models/config.go:13`、`jcp/internal/models/config.go:44`、`jcp/internal/models/config.go:106`
- 内置工具清单：`jcp/internal/adk/tools/registry.go:51`
- 会议服务：`jcp/internal/meeting/service.go:128`
- OpenClaw HTTP 服务：`jcp/internal/openclaw/server.go:43`

### 仓库与同步约定

- 顶层忽略规则：`.gitignore:1`
- 上游同步说明：`docs/upstream-sync.md:1`
- 仓库迁移设计：`docs/plans/2026-03-18-repo-migration-design.md:1`
