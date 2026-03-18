# Project Overview

## 1. 身份

`jcp/` 是一个基于 Wails 的桌面股票分析应用：Go 后端创建桌面壳并绑定 `App`，前端使用 React + TypeScript 渲染主界面。入口和绑定关系见 `jcp/main.go:21`、`jcp/main.go:33`、`jcp/app.go:52`。

## 2. 概述

- 主界面围绕三块能力组织：自选股与市场指数、K 线与盘口、AI 讨论室；前端主组件直接引入这些模块并维护布局、行情、会话和弹窗状态，见 `jcp/frontend/src/App.tsx:2`、`jcp/frontend/src/App.tsx:46`、`jcp/frontend/src/App.tsx:72`。
- 后端 `App` 在启动时组装配置、行情、新闻、热点、龙虎榜、会议、会话、策略、工具注册、MCP、记忆、更新和 OpenClaw 服务，说明项目是“桌面 UI + 本地服务编排”的结构，而不是单一行情看板，见 `jcp/app.go:28`、`jcp/app.go:52`、`jcp/app.go:82`、`jcp/app.go:91`、`jcp/app.go:129`、`jcp/app.go:139`、`jcp/app.go:142`。
- 暴露给前端的 API 覆盖自选股、实时行情、K 线、盘口、会话消息、持仓、策略、AI 会议、MCP、热点、更新和龙虎榜，见 `jcp/app.go:335`、`jcp/app.go:396`、`jcp/app.go:402`、`jcp/app.go:452`、`jcp/app.go:498`、`jcp/app.go:614`、`jcp/app.go:708`、`jcp/app.go:783`、`jcp/app.go:1092`、`jcp/app.go:1230`、`jcp/app.go:1253`、`jcp/app.go:1313`。
- 配置模型支持多 AI 提供商、MCP Server、记忆、代理、布局、OpenClaw 和技术指标，说明产品核心不是固定单模型问答，而是可配置的分析工作台，见 `jcp/internal/models/config.go:13`、`jcp/internal/models/config.go:44`、`jcp/internal/models/config.go:56`。
- 当前技术栈由 Go 1.24、Wails v2、React 18、Vite、TypeScript、Tailwind、Lightweight Charts 组成；运行和构建命令在仓库 README 中明确给出，见 `jcp/go.mod:1`、`jcp/frontend/package.json:6`、`jcp/frontend/package.json:11`、`jcp/README.md:71`。

## 3. 当前已确认的功能面

- 行情能力：支持实时行情、K 线、盘口、交易日历和交易状态；行情层自带短 TTL 缓存与定时清理，见 `jcp/app.go:396`、`jcp/app.go:402`、`jcp/app.go:408`、`jcp/app.go:1292`、`jcp/app.go:1304`、`jcp/internal/services/market_service.go:34`、`jcp/internal/services/market_service.go:108`、`jcp/internal/services/market_service.go:159`。
- 自选与会话能力：自选股保存在配置服务中，单股票会话和持仓信息落到 `sessions/` 目录，见 `jcp/app.go:335`、`jcp/app.go:368`、`jcp/app.go:378`、`jcp/internal/services/config_service.go:29`、`jcp/internal/services/session_service.go:24`、`jcp/internal/services/session_service.go:45`。
- AI 分析能力：会议支持智能串行模式和直接 @ 专家模式，并包含进度事件、失败重试和继续执行，见 `jcp/internal/meeting/service.go:128`、`jcp/internal/meeting/service.go:171`、`jcp/internal/meeting/service.go:183`、`jcp/app.go:783`、`jcp/app.go:900`、`jcp/app.go:949`、`jcp/app.go:1007`。
- 策略与专家能力：内置默认策略“均衡分析”，默认含 6 类专家并为不同角色分配工具集，见 `jcp/internal/services/strategy_service.go:23`、`jcp/internal/services/strategy_service.go:36`。
- 工具与外部扩展：内置工具覆盖实时行情、K 线、盘口、快讯、搜索、研报、热点和龙虎榜；同时支持 MCP Server 配置与测试，见 `jcp/internal/adk/tools/registry.go:28`、`jcp/internal/adk/tools/registry.go:51`、`jcp/app.go:1099`、`jcp/app.go:1164`。
- OpenClaw 能力：可选启动本地 HTTP 服务，对外提供 `/health`、`/status`、`/analyze`，并支持 Bearer 鉴权，见 `jcp/app.go:216`、`jcp/internal/openclaw/server.go:43`、`jcp/internal/openclaw/server.go:58`、`jcp/internal/openclaw/handlers.go:9`。

## 4. 目前缺口

- `llmdoc` 目前只覆盖概览级信息，还没有展开到单个热点源、MCP 管理器和 OpenClaw `analyze` 请求/响应模型。
- 若后续需要排查行为或同步上游变更，优先补 `architecture/` 下的会议执行链和 `reference/` 下的数据落盘约定。
