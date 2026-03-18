# Common Workflows

## 开发运行 `jcp`

1. 进入 `jcp/`，先安装前端依赖并下载 Go 依赖；仓库 README 明确给出 `cd frontend && npm install && cd ..` 与 `go mod download`，见 `jcp/README.md:92`。
2. 用 `wails dev` 启动开发模式；前端脚本定义在 `jcp/frontend/package.json:6`，README 运行命令见 `jcp/README.md:102`。
3. 若只需要前端打包，使用 `npm run build`，其实际命令是 `tsc && vite build`，见 `jcp/frontend/package.json:6`。
4. 验证启动是否完整：
   - 桌面窗口能打开且未崩溃；窗口由 `jcp/main.go:33` 配置。
   - `startup()` 已启动市场推送；对应逻辑见 `jcp/app.go:211`。
   - 设置页可读取和保存配置；配置接口起点见 `jcp/app.go:243`、`jcp/app.go:248`。

## 新建一次股票分析会话

1. 先确保股票已加入自选；相关接口见 `jcp/app.go:368`、`jcp/app.go:378`。
2. 前端根据股票代码获取或创建 Session；后端入口见 `jcp/app.go:452`，落盘逻辑见 `jcp/internal/services/session_service.go:45`。
3. 用户发送消息后，后端进入 `SendMeetingMessage`，根据上下文选择智能会议或直接专家模式，见 `jcp/app.go:783`、`jcp/app.go:846`、`jcp/app.go:900`。
4. 结果会被写入 Session，并通过 `meeting:message:<stockCode>` 事件推送给前端，见 `jcp/app.go:925`、`jcp/app.go:941`、`jcp/app.go:943`。
5. 验证方式：
   - `sessions/<stock>.json` 出现新增消息，见 `jcp/internal/services/session_service.go:92`、`jcp/internal/services/session_service.go:123`。
   - 若启用记忆，相关上下文会进入记忆管理器，见 `jcp/app.go:94`、`jcp/internal/memory/manager.go:101`。

## 调整策略、专家或 AI 配置

1. AI 提供商、MCP、记忆、布局和 OpenClaw 都在 `AppConfig` 中；模型字段定义见 `jcp/internal/models/config.go:13`、`jcp/internal/models/config.go:56`。
2. 保存配置时调用 `UpdateConfig`，会把 JSON 写回本地配置文件；实现见 `jcp/app.go:248`、`jcp/internal/services/config_service.go:190`。
3. 策略切换和专家编辑走 `StrategyService`，并在修改后重新加载 `agent.Container`，见 `jcp/app.go:498`、`jcp/app.go:523`、`jcp/app.go:565`、`jcp/app.go:569`。
4. 验证方式：
   - 本地 `config.json`、`strategies.json` 已更新，见 `jcp/internal/services/config_service.go:30`、`jcp/internal/services/strategy_service.go:113`。
   - MCP 配置变更后会触发重新加载，见 `jcp/app.go:1108`、`jcp/app.go:1123`、`jcp/app.go:1141`。

## 使用 OpenClaw HTTP 服务

1. 在配置中启用 `OpenClaw` 并设置端口；字段定义见 `jcp/internal/models/config.go:106`。
2. 应用启动时若 `Enabled` 且端口大于 0，则启动 HTTP 服务，见 `jcp/app.go:216`。
3. 可访问 `/health`、`/status`，分析入口为 `/analyze`；路由注册见 `jcp/internal/openclaw/server.go:58`。
4. 若设置了 API Key，请求 `/analyze` 需要 `Authorization: Bearer <key>`，见 `jcp/internal/openclaw/handlers.go:9`。
5. 验证方式：
   - `/health` 返回 `status=ok`，见 `jcp/internal/openclaw/handlers.go:23`。
   - `/status` 返回 agent 数量和 AI 是否已配置，见 `jcp/internal/openclaw/handlers.go:27`。
