# Common Workflows

## 开发运行 `jcp`

1. 进入 `jcp/`，先安装前端依赖并下载 Go 依赖；仓库 README 明确给出 `cd frontend && npm install && cd ..` 与 `go mod download`，见 `jcp/README.md:92`。
2. 用 `wails dev` 启动开发模式；前端脚本定义在 `jcp/frontend/package.json:6`，README 运行命令见 `jcp/README.md:102`。
3. 若只需要前端验证，可先跑 `npm run test` 做前端纯函数回归，再执行 `npm run build` 做打包校验；两者都在 `jcp/frontend/package.json` 中定义，见 `jcp/frontend/package.json:6`。
4. 验证启动是否完整：
   - 桌面窗口能打开且未崩溃；窗口由 `jcp/main.go:33` 配置。
   - `startup()` 已启动市场推送；对应逻辑见 `jcp/app.go:211`。
   - 设置页可读取和保存配置；配置接口起点见 `jcp/app.go:243`、`jcp/app.go:248`。
5. 若要排查“启动是否最大化 / 前端是否覆盖了窗口状态”，优先看当日日志中的 `frontend: [window] startup layout restore evaluated`；其中 `isMaximized`、`restoreWindowSizeSkipped` 和 `restoreReason` 能直接说明启动时窗口状态判定与布局恢复决策。
6. 若要排查“为什么 curl 直接调新浪 K 线接口成功，但程序里 K 线加载失败”，优先看当日日志中的 `module=market action=kline.fetch.fallback.start` / `kline.fetch.fallback.success` / `kline.fetch.fallback.retry` / `kline.fetch.stale_cache_used` / `kline.fetch.cooldown_stale_cache_used`。普通 K 线链路现在会在 `INFO`/`WARN` 下记录 `requestId`、`sinaUrl`、`responsePreview`、`fallbackUrl`、Eastmoney 重试次数，以及是否命中 stale/cooldown 兜底，因此可以区分“上游恢复成功”“仍然失败但旧缓存顶住了”“短时间内直接跳过重复远端请求”这三类现场结果。前端风控 toast 只会在 `frontend retries exhausted` 且该股票从未成功拿到过任何 K 线时出现；如果之前已经成功过，就算后续远端失败也只应看到旧图，不应再弹这个提示。

## 验证 Windows 打包图标

1. Windows 安装包与 exe 使用 `jcp/build/windows/icon.ico`；NSIS 安装器也引用同一份图标资源，见 `jcp/build/windows/installer/project.nsi:53`、`jcp/build/windows/installer/project.nsi:54`。
2. 现在仓库里有一个回归测试会直接比对 `build/appicon.png` 和 `build/windows/icon.ico` 的图像内容，避免只替换了主图标却漏掉 Windows 专用 ico，见 `jcp/icon_test.go:14`。
3. 本地验证时直接在 `jcp/` 下执行 `go test ./...`；如果 `icon.ico` 和 `appicon.png` 偏差过大，测试会以 `windows icon diverges from appicon` 失败。

## 新建一次股票分析会话

1. 先确保股票已加入自选；相关接口见 `jcp/app.go:368`、`jcp/app.go:378`。
2. 前端根据股票代码获取或创建 Session；后端入口见 `jcp/app.go:452`，落盘逻辑见 `jcp/internal/services/session_service.go:45`。
3. 用户发送消息后，后端进入 `SendMeetingMessage`，根据上下文选择智能会议或直接专家模式，见 `jcp/app.go:783`、`jcp/app.go:846`、`jcp/app.go:900`。
4. 结果会被写入 Session，并通过 `meeting:message:<stockCode>` 事件推送给前端，见 `jcp/app.go:925`、`jcp/app.go:941`、`jcp/app.go:943`。
5. 验证方式：
   - `sessions/<stock>.json` 出现新增消息，见 `jcp/internal/services/session_service.go:92`、`jcp/internal/services/session_service.go:123`。
   - 若启用记忆，相关上下文会进入记忆管理器，见 `jcp/app.go:94`、`jcp/internal/memory/manager.go:101`。
6. 若指定专家失败后点击“重试”，当天文件日志除了 `retry_agent.request` / `retry_agent.success` 外，还应出现 `frontend: [meeting] retry feedback resolved`，其中 `resolvedBy=latest|fallback|current` 会直接告诉你 UI 最终使用了哪一种消息来源做回填。

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
2. 首次同步前先确认市场范围。默认勾选沪市和深市，北交所和指数需要手动启用；其中深市创业板个股已在后端股票池构建时被过滤，不会进入同步或筛选，但创业板指数仍可在启用“指数”时保留，见 `jcp/frontend/src/components/SettingsDialog.tsx:1317`、`jcp/internal/services/market_service.go:632`。
3. 顶部栏入口会先根据本地同步状态显示三种视觉状态：全部完成时显示绿色“已同步”并禁用；部分完成时主文案仍是黄色“立即同步”，但右侧数字会优先显示当前已完成股票数 `(n/总数)`；未同步时显示红色“立即同步 (0/总数)” 或 `(--/--)`。如果用户在同步过程中取消，只有“同一测试范围且确实存在断点”的下一次运行才会继续沿用已完成数；一旦切换成新的范围，前端会把 `completedStocks`、`progressPercent` 和旧事件列表全部清零，避免把上一次失败任务的覆盖率误当成新一轮同步进度，见 `jcp/frontend/src/utils/screeningSync.ts:96`。
4. 点击“立即同步”后，前端调用 `RunScreeningSync({ mode, limitStocks })`；若勾选测试上限，只会处理前 N 只股票，未勾选则按完整市场范围执行。后端会先写基础股票列表，再按 `sync_state` 判断是首次窗口同步还是按最近交易日增量补齐，见 `jcp/app.go:373`、`jcp/internal/services/screening_sync_service.go:144`、`jcp/internal/services/screening_sync_service.go:194`。
5. 同步候选会先排除 `is_active = false` 的股票，因此退市/终止上市个股不会再进入后续同步队列；它们旧的历史日线仍可留在库里，但不会继续追最新交易日。
6. 若某只股票连续 3 次同步都出现 `no new bars`，并且本次数据源返回的最新交易日仍至少落后目标交易日 20 个交易日，同步服务会把它写入本地 `sync_symbol_states` 排除名单。之后它既不会再进入后续同步队列，也不会继续计入顶部同步覆盖率 `(已同步/总数)` 的总数。
7. 日线同步取数顺序是 `Baostock -> 新 Sina 日线接口`。如果 `Baostock` 登录、查询或返回空数据失败，才会回退到 `money.finance.sina.com.cn` 的日线接口；`Baostock` 遇到空成交量/空成交额时会按 0 处理，不再因为空字符串直接报错。
8. 无论来自设置页还是顶部栏，同步过程中状态卡都会显示百分比、已完成股票数、当前股票、当前数据源和最近事件；最近事件现在除了数据源切换外，还会额外记录 `queue`、`fetch`、`skip`、`stored`、`error` 和 `excluded` 六类诊断消息，并把 `target`、`localLatest`、`lookback`、`sourceBars`、`completed` 这类字段写进消息体，方便直接判断是断点续传、无增量数据、单票失败，还是因为长期无新增而被排除。
9. 点击“取消”后，本轮会在当前股票处理完成后停下，并把断点写入 `sync_jobs`；下次手动或自动同步都从该断点继续，但新的同步范围不会继承旧事件列表，见 `jcp/app.go:397`、`jcp/internal/services/screening_store.go:118`、`jcp/internal/services/screening_sync_service.go:267`、`jcp/internal/services/screening_sync_service.go:946`。

## 首次从欢迎页同步并继续 AI 筛选

1. 欢迎页现在既会在“完全空白状态”下自动出现，也可以通过顶部模式切换里的 `首页` 随时主动进入；若首次直接输入自然语言筛选，不会立刻执行，而是先弹统一确认弹框。若当前本地已经有自选或筛选数据，欢迎页顶部还会额外给出“进入自选 / 进入 AI 筛选”两个快捷入口，避免进入首页后失去返回路径。
2. 这个确认弹框现在来自一套通用同步弹框：欢迎页和主工作区使用 `screening` 模式，顶部栏同步按钮使用 `sync-only` 模式。两种模式都共用“当前同步策略”“当前数据状态”和同步进度卡，且标题说明区统一左对齐；只有 `screening` 模式额外展示“本次筛选条件”和一次性的“测试范围”面板，决定本次是否只同步并筛选前 N 只股票。真正开始同步后，同一弹框会继续显示实时同步卡，包含百分比、当前股票、当前数据源、当前阶段和最近同步日志，见 `jcp/frontend/src/utils/screeningSync.ts:162`、`jcp/frontend/src/App.tsx:1527`。
3. 用户点击确认后，前端现在始终会先触发一次手动同步，再继续本次 AI 筛选；若勾选“只同步并筛选前 N 只股票”，同步完成后优先复用本次返回的 `syncedSymbols`，若返回值缺失，则额外调用 `GetScreeningUniverseSymbols(limit)` 从本地 SQLite 里读取当前可查询股票池的前 N 只 symbol，再把该子集继续传给 AI 筛选请求，见 `jcp/frontend/src/App.tsx`、`jcp/app.go`、`jcp/internal/services/screening_store.go`。
4. 后端在执行这类测试筛选时，会在同一条 SQLite 连接上创建临时 `screening_scope` 表，并额外用同名 TEMP VIEW 覆盖 `stocks_basic`、`daily_bars`、`daily_snapshots` 和 `v_stock_latest_daily`；同时基础 `v_stock_latest_daily` 和筛选 universe 本身也会排除 `is_active = false` 的股票，以及已被本地 `sync_symbol_states` 标记为长期无新增的 symbol，因此即使旧库里还残留退市股票或长期无新增股票的历史日线，它们也不会再进入新的测试股票池，见 `jcp/internal/services/screening_store.go`、`jcp/internal/services/screening_query_service.go`。

## 执行一次 AI 筛选并回放历史

1. 在主屏幕顶栏切到 `AI 筛选` 模式，左侧会切换成“结果区 + 工作区”的双区布局；结果区顶部再分成“当前筛选结果 / 历史筛选结果”两个页签，工作区固定贴在左下方。
2. 在左下工作区输入自然语言条件，选择“不限”或前 N 条结果，然后确认执行；前端会把结果模式、数量和可选测试股票子集一并传给后端。
3. 后端不会直接进入 SQL 生成，而是先用同一套 AI 配置输出一段连续自然语言思考摘要，再继续生成只读 SQL；这个思考阶段现在更像“边思考边说明”的一段流式文字，不再强制做成 3 到 5 条短句，但仍然只允许输出可控摘要，不允许输出 SQL，也不是模型原始推理链，见 `jcp/internal/services/screening_query_service.go:202`、`jcp/internal/services/screening_query_service.go:642`。
4. 之后查询服务再生成 SQL，校验来源表、列顺序、`ORDER BY` 和 `LIMIT` 规则后执行，并把 SQL 与结果快照保存到历史表；生成 SQL 的超时现在优先读取当前 AI 配置的 `timeout`，不再固定死为 45 秒。
5. 查询进行中，左上结果区会显示当前阶段、百分比和最近日志；当阶段为 `reasoning` 时，进度卡标题显示“AI 正在思考”，流式区域展示连续自然语言摘要；当阶段切到 `generate_sql` 时，同一块区域继续显示 SQL 草稿，见 `jcp/frontend/src/components/ScreeningResultList.tsx:125`、`jcp/frontend/src/components/ScreeningResultList.tsx:143`、`jcp/frontend/src/components/ScreeningResultList.tsx:272`。
6. 结果区的页签现在位于标题和数量说明上方，当前结果和历史结果都沿用同一套布局；如果本次筛选没有命中结果，左侧会明确显示“没有符合条件的结果”。如果有结果，前端仍会默认选中第一只股票并加载详情，见 `jcp/frontend/src/components/ScreeningResultList.tsx:51`、`jcp/frontend/src/components/ScreeningResultList.tsx:171`、`jcp/frontend/src/App.tsx:146`、`jcp/frontend/src/App.tsx:518`。
7. 切到“历史筛选结果”页签后，左侧整栏显示历史记录列表；点击任意一条记录，前端会先读取当次保存的结果快照，再切回“当前筛选结果”页签显示。每条历史右上角有 `X` 按钮，点击后会弹出确认框；确认后删除该条历史及其结果快照。
8. 如果当前结果来源于历史记录，且用户没有修改输入框，工作区按钮会显示“根据历史筛选方式重新筛选”；一旦提示词被改动，按钮立即切回“开始筛选”。
9. 无论按钮文案是什么，真正点击执行时都会先弹出范围确认框。只有提示词、市场范围、结果模式、结果条数和测试范围都未变化时，前端才调用 `RerunScreeningHistoryRun` 或 `RerunScreeningHistoryRunWithUniverse` 复用历史 `generated_sql`；只要范围或筛选条件变了，就重新调用 `RunScreeningQuery` 让模型生成新 SQL。复用 SQL 时，查询仍会在当前 SQLite 数据库和本次确认后的股票范围上执行，因此命中的是最新股票数据，不是历史缓存结果。
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
