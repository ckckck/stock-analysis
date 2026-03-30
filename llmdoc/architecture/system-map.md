# System Map

## 1. 身份

该文档描述 `jcp/` 当前可确认的系统边界、核心组件和主执行链，重点覆盖桌面壳、前端界面、服务编排、AI 会议和外部 HTTP 暴露。

## 2. 核心组件

- 桌面入口：Wails 使用嵌入的 `frontend/dist` 资源启动桌面窗口，并把 `App` 作为前后端桥接对象绑定到运行时，见 `jcp/main.go:15`、`jcp/main.go:33`、`jcp/main.go:48`。
- 后端编排层：`App` 聚合配置、行情、AI 筛选存储/同步/调度/查询、新闻、热点、龙虎榜、策略、会话、会议、MCP、记忆、更新和 OpenClaw 服务，是系统主装配点，见 `jcp/app.go:73`、`jcp/app.go:129`、`jcp/app.go:137`、`jcp/app.go:139`、`jcp/app.go:237`。
- 前端应用层：`frontend/src/App.tsx` 组织首页/自选/AI 筛选三态切换、筛选结果列表、筛选工作区、图表、盘口、会议室、设置和多个业务弹窗，并处理推送更新、统一筛选确认、历史结果双页签、历史删除确认和布局持久化，见 `jcp/frontend/src/App.tsx`。
- 统一日志与现场排障层：前端通过 `appLog` 桥接文件日志，普通行情链路、会议入口、会议重试回填和启动窗口恢复都补齐了稳定摘要；Go 侧 `app` / `frontend` / `market` logger 共同承担 request/failed/success 证据链，见 `jcp/frontend/src/utils/appLog.ts`、`jcp/frontend/src/components/AgentRoom.tsx`、`jcp/frontend/src/services/sessionService.ts`、`jcp/app.go`、`jcp/internal/services/market_service.go`。
- 设置页模型管理：`frontend/src/components/SettingsDialog.tsx` 中的 Provider 列表把“切换当前默认模型”和“进入编辑页”拆成两种交互，卡片点击只负责切换模型，右侧按钮区负责编辑、复制、设默认和删除。
- 配置与落盘层：应用数据默认位于用户配置目录下的 `jcp` 子目录；配置、自选股、策略、会话仍通过本地 JSON 文件持久化，AI 筛选新增独立 SQLite 库 `screening/screening.db`，见 `jcp/internal/pkg/paths/paths.go:8`、`jcp/internal/pkg/paths/paths.go:22`、`jcp/internal/services/config_service.go:23`、`jcp/internal/services/session_service.go:23`、`jcp/internal/services/strategy_service.go:110`、`jcp/internal/services/screening_store.go:51`。
- AI 筛选数据层：`ScreeningStore` 维护 `stocks_basic`、`daily_bars`、`daily_snapshots`、`sync_state`、`screening_runs`、`screening_run_results` 和视图 `v_stock_latest_daily`，供同步链与查询链共享，见 `jcp/internal/services/screening_store.go:81`、`jcp/internal/services/screening_store.go:121`、`jcp/internal/services/screening_store.go:131`、`jcp/internal/services/screening_store.go:150`。
- AI 会议层：`meeting.Service` 管理模型创建、超时、重试、智能/直接两种会议模式、中断状态和进度事件，见 `jcp/internal/meeting/service.go:29`、`jcp/internal/meeting/service.go:37`、`jcp/internal/meeting/service.go:109`、`jcp/internal/meeting/service.go:128`、`jcp/internal/meeting/service.go:183`、`jcp/internal/meeting/service.go:224`、`jcp/internal/meeting/service.go:236`。
- 记忆层：股票记忆按股票隔离，支持上下文构建、关键事实累积、轮次压缩和异步保存，见 `jcp/internal/memory/manager.go:12`、`jcp/internal/memory/manager.go:52`、`jcp/internal/memory/manager.go:101`、`jcp/internal/memory/manager.go:136`、`jcp/internal/memory/manager.go:161`。
- 外部服务层：OpenClaw 在启用时启动单独 HTTP 服务，对外暴露健康检查、状态和分析接口，见 `jcp/internal/openclaw/server.go:21`、`jcp/internal/openclaw/server.go:43`、`jcp/internal/openclaw/handlers.go:23`、`jcp/internal/openclaw/handlers.go:27`。

## 3. 执行流程

### 启动链

1. `main()` 创建 `App` 并调用 `wails.Run`，见 `jcp/main.go:21`、`jcp/main.go:30`、`jcp/main.go:33`。
2. `NewApp()` 创建核心服务并按依赖顺序装配 AI 筛选存储/同步/调度/查询、工具注册、MCP、会议、记忆、策略和 OpenClaw，见 `jcp/app.go:102`、`jcp/app.go:129`、`jcp/app.go:137`、`jcp/app.go:139`、`jcp/app.go:152`、`jcp/app.go:162`、`jcp/app.go:212`。
3. `startup()` 绑定代理配置、初始化 MCP、启动更新服务、市场数据推送、AI 筛选调度器和可选的 OpenClaw 服务，见 `jcp/app.go:262`、`jcp/app.go:268`、`jcp/app.go:280`、`jcp/app.go:285`、`jcp/app.go:290`、`jcp/app.go:294`。

### 普通股票分析链

1. 前端维护选中股票、K 线、盘口和会话状态，并通过服务层调用后端绑定接口，见 `jcp/frontend/src/App.tsx:49`、`jcp/frontend/src/App.tsx:52`、`jcp/frontend/src/App.tsx:55`、`jcp/frontend/src/App.tsx:69`。
2. 行情数据通过 `GetWatchlist`、`GetStockRealTimeData`、`GetKLineData`、`GetOrderBook` 等接口进入前端；市场服务对行情与 K 线做短期缓存，见 `jcp/app.go:335`、`jcp/app.go:396`、`jcp/app.go:402`、`jcp/app.go:408`、`jcp/internal/services/market_service.go:93`、`jcp/internal/services/market_service.go:108`。
3. 用户发送问题后，`SendMeetingMessage` 根据场景进入智能模式或直接专家模式；响应再被保存进 Session 并通过 Wails 事件推送回前端，见 `jcp/app.go:783`、`jcp/app.go:846`、`jcp/app.go:900`、`jcp/app.go:925`、`jcp/app.go:943`。
4. 若会议失败，可重试单专家或继续中断会议；前端重试成功后会主动拉取最新 Session 消息，并额外写入 `retry_feedback.resolved` 摘要，明确记录这次 UI 回填是来自最新消息、fallback 结果还是保持当前列表；中断状态在会议服务中有 TTL 缓存，见 `jcp/frontend/src/utils/meetingRetry.ts`、`jcp/frontend/src/components/AgentRoom.tsx`、`jcp/app.go:949`、`jcp/app.go:1007`、`jcp/internal/meeting/service.go:109`、`jcp/internal/meeting/service.go:125`。

### AI 筛选链

1. 设置页 `AI筛选` 选项卡除了维护市场范围、首次同步窗口、保留策略、应用运行期自动同步时间和默认结果条数外，还提供手动测试上限、百分比进度条、数据源切换记录和取消入口；同步过程中前端订阅 `screening:sync:progress` 实时刷新状态，见 `jcp/frontend/src/components/SettingsDialog.tsx:113`、`jcp/frontend/src/components/SettingsDialog.tsx:163`、`jcp/frontend/src/components/SettingsDialog.tsx:1401`。
2. 手动同步调用 `RunScreeningSync(options)`，取消走 `CancelScreeningSync()`；`App` 把同步服务的进度回调转成 Wails 事件，服务内部保存最近一次进度快照和运行级 cancel，从而支持手动/自动同步共用断点续传与取消入口。前端在发起“新的同步范围”时会清空旧事件和旧进度，只有“同一 limit 的失败/取消任务”才会继承断点计数，避免把上一轮市场级覆盖率混进新的测试同步状态，见 `jcp/app.go:373`、`jcp/app.go:397`、`jcp/frontend/src/utils/screeningSync.ts:96`、`jcp/internal/services/screening_sync_service.go:118`、`jcp/internal/services/screening_sync_service.go:144`、`jcp/internal/services/screening_sync_service.go:944`。
3. AI 筛选同步的日线来源不再直接等同于通用 `GetKLineData("1d")`：`MarketService.GetScreeningDailyBars()` 现在走同步专用 provider 链，优先使用 `Baostock`，失败或空数据时回退到新的 `money.finance.sina.com.cn` 日线接口；同时 `Baostock` 对空成交量/空成交额做零值容错，避免停牌/退市区间的空字段直接打断同步。对于连续多次 `no new bars` 且源最新日期明显早于目标交易日的个股，同步服务还会把它们写入本地 `sync_symbol_states` 排除名单，后续同步队列和覆盖率统计都会跳过这些 symbol，见 `jcp/internal/services/market_service.go:508`、`jcp/internal/services/baostock_daily_bar_source.go:53`、`jcp/internal/services/baostock_daily_bar_source.go:319`、`jcp/internal/services/screening_sync_service.go`、`jcp/internal/services/screening_store.go`。
4. 自动同步由 `ScreeningScheduler` 在应用运行期间按配置时间触发；配置关闭或应用退出时调度结束，不会脱离桌面应用单独常驻，见 `jcp/internal/services/screening_scheduler.go:19`、`jcp/internal/services/screening_scheduler.go:44`、`jcp/internal/services/screening_scheduler.go:54`、`jcp/internal/services/screening_scheduler.go:91`。
5. 用户在欢迎页或顶栏 `AI 筛选` 工作区输入自然语言时，不会立刻执行；两条入口都会先走 `ScreeningConfirmDialog`，由用户确认是否先同步、是否只测前 N 只，然后再进入真正的查询链。确认弹框本身现在也订阅 `screening:sync:progress`，同步期间直接显示实时百分比、当前股票、当前数据源、阶段和最近事件；事件流除了 provider fallback 以外，还会补充队列建立、单只股票 fetch 参数、无增量跳过、写入完成和失败诊断，因此远端排查时不再只能看到“同步失败”这一层。单只股票取数失败现在只会记录 `error` 事件并跳过，不再把整轮同步直接跑成失败，见 `jcp/frontend/src/App.tsx:214`、`jcp/frontend/src/App.tsx:648`、`jcp/frontend/src/App.tsx:1233`、`jcp/frontend/src/App.tsx:1365`、`jcp/internal/services/screening_sync_service.go:384`、`jcp/internal/services/screening_sync_service.go:419`、`jcp/internal/services/screening_sync_service.go:527`。
6. 用户在顶栏切到 `AI 筛选` 后，左侧上半区显示结果区，并额外分成“当前筛选结果 / 历史筛选结果”两个页签；页签现在位于标题和数量说明的上方，左下工作区固定贴底，只保留自然语言输入、结果条数选择和执行按钮，不再显示独立标题区与额外说明卡。点击历史记录后，结果会恢复到当前结果页，若提示词未改动，工作区按钮文案保留为“根据历史筛选方式重新筛选”；历史列表每条记录右上角还带独立删除入口，删除需二次确认。若本次筛选没有命中结果，左侧会明确显示“没有符合条件的结果”，而不是继续沿用旧结果的泛化提示，见 `jcp/frontend/src/components/ScreeningResultList.tsx:7`、`jcp/frontend/src/components/ScreeningWorkspace.tsx:74`、`jcp/frontend/src/App.tsx`。
7. `RunScreeningQuery` 调用查询服务，后端把自然语言、limit 选项和可选测试子集写入 prompt，让模型只生成白名单来源上的只读 SQL；如果前端开启“只测试前 N 只股票”，它会优先复用本次手动同步返回的 `syncedSymbols`，否则通过 `GetScreeningUniverseSymbols(limit)` 从当前 SQLite 股票池取前 N 只 symbol，再把它们带进查询请求。执行前，查询服务会在独立 SQLite 连接上创建 `screening_scope` 临时表，并用同名 TEMP VIEW 覆盖白名单源表/视图，把查询自动限制在测试股票子集内，因此模型即使没显式写出 `screening_scope` 也不会扫全市场。SQL 生成超时优先读取 `ScreeningConfig.sqlTimeoutSeconds`，只有缺失时才回退到 Provider `timeout`。查询服务会先走一次 `reasoning` 阶段，再进入 SQL 生成流：`RunWithProgress` 现在上报 `prepare -> reasoning -> generate_sql -> validate_sql -> execute_query -> store_results -> completed`，其中 `reasoning` 和 `generate_sql` 都复用 `streamingText` 做增量展示，但 `reasoning` 已从分条结构化步骤改成连续自然语言摘要，仍然只显示可控摘要，不透出模型原始思维链，见 `jcp/internal/services/screening_query_service.go:104`、`jcp/internal/services/screening_query_service.go:135`、`jcp/internal/services/screening_query_service.go:202`、`jcp/internal/services/screening_query_service.go:253`、`jcp/internal/services/screening_query_service.go:642`、`jcp/app.go:63`。
8. 历史 SQL 重跑通过 `RerunScreeningHistoryRun` / `RerunScreeningHistoryRunWithUniverse` 走独立链路：前端每次都会重新弹出范围确认框；只有提示词、市场范围、结果模式、结果条数和测试范围都未变化时，才复用历史 `generated_sql`，否则重新走 AI 生成 SQL。即使复用 SQL，执行时仍会重新校验白名单，并按本次确认后的 `screening_scope` 在当前数据库上重跑，因此结果始终基于最新股票池和最新日线快照，而不是历史命中集合。删除历史记录则通过 `DeleteScreeningHistoryRun` 直接移除 `screening_runs` 及其结果快照。

### 策略和工具链

1. `StrategyService` 加载内置/用户策略，并把活跃策略中的专家加载进 `agent.Container`，见 `jcp/internal/services/strategy_service.go:102`、`jcp/internal/services/strategy_service.go:119`、`jcp/app.go:135`。
2. `tools.Registry` 注册股票、K 线、盘口、新闻、研报、热点和龙虎榜工具；策略生成也会读取这些工具元数据和 MCP 工具列表，见 `jcp/internal/adk/tools/registry.go:16`、`jcp/internal/adk/tools/registry.go:51`、`jcp/app.go:614`、`jcp/app.go:648`、`jcp/app.go:656`。

## 4. 设计要点

- 桌面应用不是纯离线工具，核心功能依赖外部行情和模型服务，因此配置、代理和连接测试是一级能力，见 `jcp/internal/models/config.go:56`、`jcp/app.go:1171`。
- 记忆、Session 和策略按本地文件存储，AI 筛选单独落 SQLite；因此多机同步和冲突处理当前仍不在应用内解决，见 `jcp/internal/services/config_service.go:29`、`jcp/internal/services/session_service.go:26`、`jcp/internal/services/strategy_service.go:113`、`jcp/internal/services/screening_store.go:51`。
- AI 筛选历史同时承担三种角色：一是“结果快照”回放，二是“历史 SQL 模板”重跑，三是可删除的历史索引。回放仍保留当时命中集合；重跑则直接基于保存的 `generated_sql` 和当前数据库重新生成一条新历史；删除会移除这条历史及其结果快照，因此三者语义不同，见 `jcp/internal/services/screening_store.go`、`jcp/internal/services/screening_query_service.go`。
- AI 筛选同步与普通 K 线获取已分层：同步链路优先追求日线稳定性，因此单独走 `Baostock -> 新 Sina 日线接口`；普通图表与其它 K 线入口仍走 `GetKLineData()`，但当 Sina 返回 `456` / `blocked` / `拒绝访问` 时，会在同一条请求链路里自动回退到 Eastmoney K 线接口。回退诊断日志会在默认 `INFO` 下记录 `requestId`、Sina URL、响应摘要和 Eastmoney fallback URL，方便直接对比“curl 直调接口”和“程序调用”两条路径，见 `jcp/internal/services/market_service.go:473`、`jcp/internal/services/market_service.go:517`、`jcp/internal/services/market_service.go:670`、`jcp/internal/services/market_service_test.go:54`、`jcp/internal/services/market_service_test.go:204`、`jcp/internal/services/screening_daily_bar_source.go:38`。
- 桌面窗口启动时会先读取运行时最大化状态，再决定是否恢复持久化窗口尺寸，并把判定写入 `startup.restore_layout` 文件日志；因此现场可以区分“Wails 启动即最大化”与“后续按普通窗口尺寸恢复布局”这两种情况，见 `jcp/frontend/src/utils/windowLayout.ts`、`jcp/frontend/src/App.tsx`。
- AI 筛选同步候选和本地筛选 universe 都会排除 `is_active = false` 的股票；另外，对于连续 3 次 `no new bars` 且源最新日期至少落后目标交易日 20 个交易日的 symbol，本地还会额外写入 `sync_symbol_states.excluded=1`，把它们从后续同步队列、覆盖率总数和筛选 universe 中移除。这样退市/终止上市或长期无新增日线的股票仍可保留历史数据，但不会再永久拖住“已同步/总数”状态，见 `jcp/internal/services/screening_sync_service.go`、`jcp/internal/services/screening_store.go`。
- AI 筛选股票池里的深市创业板股票不再参与同步和筛选。`300`、`301` 开头的 `.SZ` 个股会在股票池构建阶段被过滤，但创业板指数仍可在启用“指数”范围时保留，见 `jcp/internal/services/market_service.go:632`、`jcp/internal/services/market_service_test.go:187`。
- OpenClaw 复用同一套会议与专家体系，不是完全独立的分析引擎；它是在桌面应用上额外开出的 HTTP 面，见 `jcp/app.go:142`、`jcp/internal/openclaw/server.go:27`。
