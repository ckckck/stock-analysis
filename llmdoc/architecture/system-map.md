# System Map

## 1. 身份

该文档描述 `jcp/` 当前可确认的系统边界、核心组件和主执行链，重点覆盖桌面壳、前端界面、服务编排、AI 会议和外部 HTTP 暴露。

## 2. 核心组件

- 桌面入口：Wails 使用嵌入的 `frontend/dist` 资源启动桌面窗口，并把 `App` 作为前后端桥接对象绑定到运行时，见 `jcp/main.go:15`、`jcp/main.go:33`、`jcp/main.go:48`。
- 后端编排层：`App` 聚合配置、行情、新闻、热点、龙虎榜、策略、会话、会议、MCP、记忆、更新和 OpenClaw 服务，是系统主装配点，见 `jcp/app.go:28`、`jcp/app.go:61`、`jcp/app.go:76`、`jcp/app.go:91`、`jcp/app.go:129`、`jcp/app.go:142`。
- 前端应用层：`frontend/src/App.tsx` 组织股票列表、图表、盘口、会议室、设置和多个业务弹窗，并处理推送更新与布局持久化，见 `jcp/frontend/src/App.tsx:2`、`jcp/frontend/src/App.tsx:46`、`jcp/frontend/src/App.tsx:82`、`jcp/frontend/src/App.tsx:160`。
- 配置与落盘层：应用数据默认位于用户配置目录下的 `jcp` 子目录；配置、自选股、策略、会话都通过本地 JSON 文件持久化，见 `jcp/internal/pkg/paths/paths.go:8`、`jcp/internal/services/config_service.go:23`、`jcp/internal/services/config_service.go:149`、`jcp/internal/services/strategy_service.go:110`、`jcp/internal/services/session_service.go:23`。
- AI 会议层：`meeting.Service` 管理模型创建、超时、重试、智能/直接两种会议模式、中断状态和进度事件，见 `jcp/internal/meeting/service.go:29`、`jcp/internal/meeting/service.go:37`、`jcp/internal/meeting/service.go:109`、`jcp/internal/meeting/service.go:128`、`jcp/internal/meeting/service.go:183`、`jcp/internal/meeting/service.go:224`、`jcp/internal/meeting/service.go:236`。
- 记忆层：股票记忆按股票隔离，支持上下文构建、关键事实累积、轮次压缩和异步保存，见 `jcp/internal/memory/manager.go:12`、`jcp/internal/memory/manager.go:52`、`jcp/internal/memory/manager.go:101`、`jcp/internal/memory/manager.go:136`、`jcp/internal/memory/manager.go:161`。
- 外部服务层：OpenClaw 在启用时启动单独 HTTP 服务，对外暴露健康检查、状态和分析接口，见 `jcp/internal/openclaw/server.go:21`、`jcp/internal/openclaw/server.go:43`、`jcp/internal/openclaw/handlers.go:23`、`jcp/internal/openclaw/handlers.go:27`。

## 3. 执行流程

### 启动链

1. `main()` 创建 `App` 并调用 `wails.Run`，见 `jcp/main.go:21`、`jcp/main.go:30`、`jcp/main.go:33`。
2. `NewApp()` 创建核心服务并按依赖顺序装配工具注册、MCP、会议、记忆、策略和 OpenClaw，见 `jcp/app.go:52`、`jcp/app.go:82`、`jcp/app.go:85`、`jcp/app.go:91`、`jcp/app.go:94`、`jcp/app.go:132`、`jcp/app.go:142`。
3. `startup()` 绑定代理配置、初始化 MCP、启动更新服务、市场数据推送和可选的 OpenClaw 服务，见 `jcp/app.go:188`、`jcp/app.go:191`、`jcp/app.go:194`、`jcp/app.go:206`、`jcp/app.go:211`、`jcp/app.go:216`。

### 普通股票分析链

1. 前端维护选中股票、K 线、盘口和会话状态，并通过服务层调用后端绑定接口，见 `jcp/frontend/src/App.tsx:49`、`jcp/frontend/src/App.tsx:52`、`jcp/frontend/src/App.tsx:55`、`jcp/frontend/src/App.tsx:69`。
2. 行情数据通过 `GetWatchlist`、`GetStockRealTimeData`、`GetKLineData`、`GetOrderBook` 等接口进入前端；市场服务对行情与 K 线做短期缓存，见 `jcp/app.go:335`、`jcp/app.go:396`、`jcp/app.go:402`、`jcp/app.go:408`、`jcp/internal/services/market_service.go:93`、`jcp/internal/services/market_service.go:108`。
3. 用户发送问题后，`SendMeetingMessage` 根据场景进入智能模式或直接专家模式；响应再被保存进 Session 并通过 Wails 事件推送回前端，见 `jcp/app.go:783`、`jcp/app.go:846`、`jcp/app.go:900`、`jcp/app.go:925`、`jcp/app.go:943`。
4. 若会议失败，可重试单专家或继续中断会议；中断状态在会议服务中有 TTL 缓存，见 `jcp/app.go:949`、`jcp/app.go:1007`、`jcp/internal/meeting/service.go:109`、`jcp/internal/meeting/service.go:125`。

### 策略和工具链

1. `StrategyService` 加载内置/用户策略，并把活跃策略中的专家加载进 `agent.Container`，见 `jcp/internal/services/strategy_service.go:102`、`jcp/internal/services/strategy_service.go:119`、`jcp/app.go:135`。
2. `tools.Registry` 注册股票、K 线、盘口、新闻、研报、热点和龙虎榜工具；策略生成也会读取这些工具元数据和 MCP 工具列表，见 `jcp/internal/adk/tools/registry.go:16`、`jcp/internal/adk/tools/registry.go:51`、`jcp/app.go:614`、`jcp/app.go:648`、`jcp/app.go:656`。

## 4. 设计要点

- 桌面应用不是纯离线工具，核心功能依赖外部行情和模型服务，因此配置、代理和连接测试是一级能力，见 `jcp/internal/models/config.go:56`、`jcp/app.go:1171`。
- 记忆、Session 和策略都按本地文件存储，意味着多机同步和冲突处理当前不在应用内解决，见 `jcp/internal/services/config_service.go:29`、`jcp/internal/services/session_service.go:26`、`jcp/internal/services/strategy_service.go:113`。
- OpenClaw 复用同一套会议与专家体系，不是完全独立的分析引擎；它是在桌面应用上额外开出的 HTTP 面，见 `jcp/app.go:142`、`jcp/internal/openclaw/server.go:27`。
