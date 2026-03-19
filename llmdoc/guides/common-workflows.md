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

## 手动同步 AI 筛选数据库

1. 打开设置页，进入 `AI筛选` 选项卡；这里可以看到市场范围、首次同步范围、保留策略、自动同步时间和“立即同步”按钮，见 `jcp/frontend/src/components/SettingsDialog.tsx:382`、`jcp/frontend/src/components/SettingsDialog.tsx:1249`。
2. 首次同步前先确认市场范围。默认勾选沪市和深市，北交所和指数需要手动启用，见 `jcp/frontend/src/components/SettingsDialog.tsx:1317`。
3. 点击“立即同步”后，前端调用 `RunScreeningSync`；后端会先写基础股票列表，再按 `sync_state` 判断是首次窗口同步还是按最近交易日增量补齐，见 `jcp/app.go:370`、`jcp/internal/services/screening_sync_service.go:67`、`jcp/internal/services/screening_sync_service.go:90`。
4. 同步完成后，状态卡会显示最近交易日、最近同步时间、本次股票数和本次日线数；若配置了仅保留近 30 天或 60 天，清理在同一次同步里完成，见 `jcp/frontend/src/components/SettingsDialog.tsx:1304`、`jcp/internal/services/screening_sync_service.go:164`。

## 执行一次 AI 筛选并回放历史

1. 在主屏幕顶栏切到 `AI 筛选` 模式，左侧会切换成“结果列表 + 工作区”的双区布局，见 `jcp/frontend/src/App.tsx:543`、`jcp/frontend/src/App.tsx:675`。
2. 在左下工作区输入自然语言条件，选择“不限”或前 N 条结果，然后执行筛选；前端会把结果模式和数量一并传给后端，见 `jcp/frontend/src/App.tsx:384`。
3. 后端把自然语言转成只读 SQL，校验来源表、列顺序、`ORDER BY` 和 `LIMIT` 规则后执行，并把 SQL 与结果快照保存到历史表，见 `jcp/internal/services/screening_query_service.go:100`、`jcp/internal/services/screening_query_service.go:131`、`jcp/internal/services/screening_query_service.go:157`。
4. 结果列表中的股票可以直接加入自选，也可以直接切换右侧详情和 AI 讨论室上下文，见 `jcp/frontend/src/App.tsx:309`、`jcp/frontend/src/App.tsx:359`、`jcp/frontend/src/App.tsx:688`。
5. 点击历史记录时，前端调用 `GetScreeningHistoryRun` 读取当次保存的结果快照，并在当前 AI 筛选页替换现有结果，见 `jcp/app.go:422`、`jcp/frontend/src/App.tsx:409`。

## 在设置中调整 AI 筛选同步参数

1. `AppConfig.Screening` 保存 AI 筛选的市场范围、首次同步范围、保留策略、自动同步开关、自动同步时间和默认结果条数，见 `jcp/internal/models/config.go:140`。
2. 设置页修改这些字段后，保存会走 `UpdateConfig`；如果应用已经运行，`UpdateConfig` 会额外调用 `screeningScheduler.Refresh` 让新调度立即生效，见 `jcp/app.go:332`、`jcp/app.go:364`。
3. 自动同步仅在桌面应用运行期间生效。关闭应用后不会继续调度，也不会额外启动后台守护进程，见 `jcp/internal/services/screening_scheduler.go:44`、`jcp/internal/services/screening_scheduler.go:54`。

## 使用 OpenClaw HTTP 服务

1. 在配置中启用 `OpenClaw` 并设置端口；字段定义见 `jcp/internal/models/config.go:106`。
2. 应用启动时若 `Enabled` 且端口大于 0，则启动 HTTP 服务，见 `jcp/app.go:216`。
3. 可访问 `/health`、`/status`，分析入口为 `/analyze`；路由注册见 `jcp/internal/openclaw/server.go:58`。
4. 若设置了 API Key，请求 `/analyze` 需要 `Authorization: Bearer <key>`，见 `jcp/internal/openclaw/handlers.go:9`。
5. 验证方式：
   - `/health` 返回 `status=ok`，见 `jcp/internal/openclaw/handlers.go:23`。
   - `/status` 返回 agent 数量和 AI 是否已配置，见 `jcp/internal/openclaw/handlers.go:27`。
