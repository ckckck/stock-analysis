# Common Workflows

## 开发运行 `jcp`

1. 进入 `jcp/`，先安装前端依赖并下载 Go 依赖；仓库 README 明确给出 `cd frontend && npm install && cd ..` 与 `go mod download`，见 `jcp/README.md:92`。
2. 用 `wails dev` 启动开发模式；前端脚本定义在 `jcp/frontend/package.json:6`，README 运行命令见 `jcp/README.md:102`。
3. 若只需要前端验证，可先跑 `npm run test` 做前端纯函数回归，再执行 `npm run build` 做打包校验；两者都在 `jcp/frontend/package.json` 中定义，见 `jcp/frontend/package.json:6`。
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
5. 在设置页的 `模型基座` 列表中，点击整张 AI 配置卡片会直接把它切换成当前默认模型；右侧的独立“编辑”按钮才会进入配置详情页，因此“切换模型”和“编辑模型”不再共用同一个点击行为，见 `jcp/frontend/src/components/SettingsDialog.tsx`。

## 手动同步 AI 筛选数据库

1. 手动同步现在有两条入口：
   - 顶部栏“龙虎榜”前的同步状态按钮，适合直接看当前同步覆盖率并进入“仅同步”弹框，见 `jcp/frontend/src/App.tsx:1175`、`jcp/frontend/src/App.tsx:1527`。
   - 设置页 `AI筛选` 选项卡，适合调整市场范围、首次同步范围、保留策略、自动同步时间，以及手动同步区里的“立即同步 / 取消 / 只同步前 N 只股票测试”入口；左侧导航里已经不再保留“软件更新”菜单，见 `jcp/frontend/src/components/SettingsDialog.tsx:57`、`jcp/frontend/src/components/SettingsDialog.tsx:313`、`jcp/frontend/src/components/SettingsDialog.tsx:418`。
2. 首次同步前先确认市场范围。默认勾选沪市和深市，北交所和指数需要手动启用，见 `jcp/frontend/src/components/SettingsDialog.tsx:1317`。
3. 顶部栏入口会先根据本地同步状态显示三种视觉状态：全部完成时显示绿色“已同步”并禁用；部分完成时主文案仍是黄色“立即同步”，但右侧数字会优先显示当前已完成股票数 `(n/总数)`；未同步时显示红色“立即同步 (0/总数)” 或 `(--/--)`。如果用户在同步过程中取消，前端会保留当前已完成数，并在下一次开始时沿用断点进度显示，再继续调用同一个断点续传后端流程，见 `jcp/frontend/src/utils/screeningSync.ts:82`、`jcp/frontend/src/utils/screeningSync.ts:125`、`jcp/frontend/src/App.tsx:823`、`jcp/frontend/src/App.tsx:886`。
4. 点击“立即同步”后，前端调用 `RunScreeningSync({ mode, limitStocks })`；若勾选测试上限，只会处理前 N 只股票，未勾选则按完整市场范围执行。后端会先写基础股票列表，再按 `sync_state` 判断是首次窗口同步还是按最近交易日增量补齐，见 `jcp/app.go:373`、`jcp/internal/services/screening_sync_service.go:144`、`jcp/internal/services/screening_sync_service.go:194`。
5. 日线同步取数顺序是 `Baostock -> Sina`。如果 `Baostock` 登录、查询或返回空数据失败，才会回退到现有新浪日线接口；因此同步报错里可能同时带有两段来源上下文，见 `jcp/internal/services/market_service.go:502`、`jcp/internal/services/screening_daily_bar_source.go:38`、`jcp/internal/services/baostock_daily_bar_source.go:116`、`jcp/internal/services/market_service.go:573`。
6. 无论来自设置页还是顶部栏，同步过程中状态卡都会显示百分比、已完成股票数、当前股票、当前数据源和最近的数据源切换记录；点击“取消”后，本轮会在当前股票处理完成后停下，并把断点写入 `sync_jobs`，下次手动或自动同步都从断点继续，见 `jcp/frontend/src/App.tsx:1676`、`jcp/frontend/src/components/SettingsDialog.tsx:1419`、`jcp/app.go:397`、`jcp/internal/services/screening_store.go:118`、`jcp/internal/services/screening_sync_service.go:267`、`jcp/internal/services/screening_sync_service.go:946`。

## 首次从欢迎页同步并继续 AI 筛选

1. 欢迎页现在既会在“完全空白状态”下自动出现，也可以通过顶部模式切换里的 `首页` 随时主动进入；若首次直接输入自然语言筛选，不会立刻执行，而是先弹统一确认弹框。若当前本地已经有自选或筛选数据，欢迎页顶部还会额外给出“进入自选 / 进入 AI 筛选”两个快捷入口，避免进入首页后失去返回路径。
2. 这个确认弹框现在来自一套通用同步弹框：欢迎页和主工作区使用 `screening` 模式，顶部栏同步按钮使用 `sync-only` 模式。两种模式都共用“当前同步策略”“当前数据状态”和同步进度卡，且标题说明区统一左对齐；只有 `screening` 模式额外展示“本次筛选条件”和一次性的“测试范围”面板，决定本次是否只同步并筛选前 N 只股票。真正开始同步后，同一弹框会继续显示实时同步卡，包含百分比、当前股票、当前数据源、当前阶段和最近同步日志，见 `jcp/frontend/src/utils/screeningSync.ts:162`、`jcp/frontend/src/App.tsx:1527`。
3. 用户点击确认后，前端现在始终会先触发一次手动同步，再继续本次 AI 筛选；若勾选“只同步并筛选前 N 只股票”，同步完成后优先复用本次返回的 `syncedSymbols`，若返回值缺失，则额外调用 `GetScreeningUniverseSymbols(limit)` 从本地 SQLite 里读取当前可查询股票池的前 N 只 symbol，再把该子集继续传给 AI 筛选请求，见 `jcp/frontend/src/App.tsx`、`jcp/app.go`、`jcp/internal/services/screening_store.go`。
4. 后端在执行这类测试筛选时，会在同一条 SQLite 连接上创建临时 `screening_scope` 表，并额外用同名 TEMP VIEW 覆盖 `stocks_basic`、`daily_bars`、`daily_snapshots` 和 `v_stock_latest_daily`；因此即使 AI 生成的 SQL 没有显式写出 `screening_scope`，查询也只会落在本次测试股票子集里，不会再因为缺少该引用直接失败，见 `jcp/internal/services/screening_query_service.go`。

## 执行一次 AI 筛选并回放历史

1. 在主屏幕顶栏切到 `AI 筛选` 模式，左侧会切换成“结果区 + 工作区”的双区布局；结果区顶部再分成“当前筛选结果 / 历史筛选结果”两个页签，工作区固定贴在左下方。
2. 在左下工作区输入自然语言条件，选择“不限”或前 N 条结果，然后确认执行；前端会把结果模式、数量和可选测试股票子集一并传给后端。
3. 后端不会直接进入 SQL 生成，而是先用同一套 AI 配置输出一段连续自然语言思考摘要，再继续生成只读 SQL；这个思考阶段现在更像“边思考边说明”的一段流式文字，不再强制做成 3 到 5 条短句，但仍然只允许输出可控摘要，不允许输出 SQL，也不是模型原始推理链，见 `jcp/internal/services/screening_query_service.go:202`、`jcp/internal/services/screening_query_service.go:642`。
4. 之后查询服务再生成 SQL，校验来源表、列顺序、`ORDER BY` 和 `LIMIT` 规则后执行，并把 SQL 与结果快照保存到历史表；生成 SQL 的超时现在优先读取当前 AI 配置的 `timeout`，不再固定死为 45 秒。
5. 查询进行中，左上结果区会显示当前阶段、百分比和最近日志；当阶段为 `reasoning` 时，进度卡标题显示“AI 正在思考”，流式区域展示连续自然语言摘要；当阶段切到 `generate_sql` 时，同一块区域继续显示 SQL 草稿，见 `jcp/frontend/src/components/ScreeningResultList.tsx:125`、`jcp/frontend/src/components/ScreeningResultList.tsx:143`、`jcp/frontend/src/components/ScreeningResultList.tsx:272`。
6. 结果区的页签现在位于标题和数量说明上方，当前结果和历史结果都沿用同一套布局；如果本次筛选没有命中结果，左侧会明确显示“没有符合条件的结果”。如果有结果，前端仍会默认选中第一只股票并加载详情，见 `jcp/frontend/src/components/ScreeningResultList.tsx:51`、`jcp/frontend/src/components/ScreeningResultList.tsx:171`、`jcp/frontend/src/App.tsx:146`、`jcp/frontend/src/App.tsx:518`。
7. 切到“历史筛选结果”页签后，左侧整栏显示历史记录列表；点击任意一条记录，前端会先读取当次保存的结果快照，再切回“当前筛选结果”页签显示。
8. 如果当前结果来源于历史记录，且用户没有修改输入框，工作区按钮会显示“根据历史筛选方式重新筛选”；这时点击会直接走 `RerunScreeningHistoryRun` 复用历史 `generated_sql` 重跑，并写出一条新的历史记录。只要用户修改输入框文字，前端就会退出这个历史重跑态，按钮切回“开始筛选”，下一次执行重新生成 SQL，见 `jcp/frontend/src/components/ScreeningWorkspace.tsx:74`、`jcp/frontend/src/App.tsx:746`、`jcp/frontend/src/App.tsx:1290`。
9. 首页确认弹框中点击“设置”后，设置弹窗现在会覆盖在确认弹框之上，不再被遮挡。

## 在设置中调整 AI 筛选同步参数

1. `AppConfig.Screening` 保存 AI 筛选的市场范围、首次同步范围、保留策略、自动同步开关、自动同步时间和默认结果条数，见 `jcp/internal/models/config.go:140`。
2. SQL 生成超时现在也归 `AppConfig.Screening` 管理，用户需要在 `AI筛选` 设置页里调整，不再从 Provider 设置里推断；查询服务会优先读取这个筛选专用超时，只有缺失时才回退到 Provider `timeout`。
3. 设置页修改这些字段后，保存会走 `UpdateConfig`；如果应用已经运行，`UpdateConfig` 会额外调用 `screeningScheduler.Refresh` 让新调度立即生效，见 `jcp/app.go:332`、`jcp/app.go:364`。
4. 自动同步仅在桌面应用运行期间生效。关闭应用后不会继续调度，也不会额外启动后台守护进程，见 `jcp/internal/services/screening_scheduler.go:44`、`jcp/internal/services/screening_scheduler.go:54`。

## 使用 OpenClaw HTTP 服务

1. 在配置中启用 `OpenClaw` 并设置端口；字段定义见 `jcp/internal/models/config.go:106`。
2. 应用启动时若 `Enabled` 且端口大于 0，则启动 HTTP 服务，见 `jcp/app.go:216`。
3. 可访问 `/health`、`/status`，分析入口为 `/analyze`；路由注册见 `jcp/internal/openclaw/server.go:58`。
4. 若设置了 API Key，请求 `/analyze` 需要 `Authorization: Bearer <key>`，见 `jcp/internal/openclaw/handlers.go:9`。
5. 验证方式：
   - `/health` 返回 `status=ok`，见 `jcp/internal/openclaw/handlers.go:23`。
   - `/status` 返回 agent 数量和 AI 是否已配置，见 `jcp/internal/openclaw/handlers.go:27`。
