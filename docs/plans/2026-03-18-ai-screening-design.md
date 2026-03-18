# AI Screening Design

**Date:** 2026-03-18

## 1. Goal

在主屏幕增加一个独立的 `AI 筛选` 工作区，让用户可以用自然语言描述选股条件，由 AI 生成只读 SQL 查询本地 `SQLite` 分析库，返回沪市/深市股票筛选结果；结果可直接查看详情、加入自选，并支持历史筛选记录回放。

## 2. Confirmed Scope

- 只覆盖 A 股沪市、深市。
- 本地分析库使用 `SQLite`。
- 分析库只保存三类数据：股票基础信息、实时行情快照、日线。
- 首次同步默认最近 30 天。
- 支持手动同步。
- 支持自动同步，但仅在应用运行时生效。
- 自动/手动同步均支持增量同步。
- 数据保留策略可配置：永久、近 30 天、近 60 天。
- 顶部新增两个模式按钮：`自选`、`AI 筛选`。
- `AI 筛选` 模式下，左侧栏改为上下分区：
  - 左上：当前筛选结果列表
  - 左下：AI 筛选工作区（自然语言输入、结果模式、历史记录）

## 3. User Experience

### 3.1 Screen Modes

- `自选`：保持当前左侧自选股列表逻辑不变，搜索框继续用于搜索股票并加入自选。
- `AI 筛选`：左侧不再展示自选，而是展示“筛选结果 + AI 工作区”。

### 3.2 AI Screening Workspace

- 输入自然语言条件，例如“找近 30 天放量上涨但回撤不大的沪深股票”。
- 可选结果模式：
  - 不限
  - 前 50 条
  - 前 100 条
  - 前 200 条
- 执行后：
  - 左上结果区替换为本次筛选命中股票
  - 中间详情区和右侧 AI 分析区可直接查看所选股票
  - 每只结果支持“加入自选”

### 3.3 History

- 左下显示历史筛选记录。
- 每条记录至少显示：
  - 原始自然语言
  - 创建时间
  - 命中数量
- 点击历史记录时：
  - 不重新询问 AI
  - 直接读取历史保存结果
  - 左上结果区替换为该次筛选结果

## 4. Architecture

### 4.1 Data Layers

拆成两类本地数据：

- 市场分析数据
  - 给 AI 生成 SQL 后查询使用
  - 包括股票基础信息、日线、日快照、同步状态
- 筛选历史数据
  - 给产品交互使用
  - 包括自然语言输入、AI 生成 SQL、命中数量、命中股票列表

### 4.2 Database

本地新增一个 `SQLite` 数据库，例如 `screening.db`。

建议核心表：

- `stocks_basic`
  - `symbol`
  - `name`
  - `market`
  - `industry`
  - `list_date`
  - `is_st`
  - `is_active`
- `daily_bars`
  - `symbol`
  - `trade_date`
  - `open`
  - `high`
  - `low`
  - `close`
  - `volume`
  - `amount`
- `daily_snapshots`
  - `symbol`
  - `trade_date`
  - `change`
  - `change_percent`
  - `amplitude`
  - `turnover_rate`
  - `price`
- `sync_state`
  - `dataset`
  - `market_scope`
  - `last_trade_date`
  - `updated_at`
- `screening_runs`
  - `id`
  - `prompt`
  - `market_scope`
  - `result_mode`
  - `result_limit`
  - `generated_sql`
  - `matched_count`
  - `created_at`
- `screening_run_results`
  - `run_id`
  - `symbol`
  - `name`
  - `rank`
  - `score`
  - `snapshot_trade_date`
  - `price`
  - `change_percent`
  - `volume`
  - `amount`

### 4.3 Query Views

AI 不直接查原始表，优先查面向筛选的白名单视图，例如：

- `v_stock_latest_daily`
- `v_stock_momentum_30d`
- `v_stock_volume_breakout`

这样可以把复杂计算预先整理好，降低 AI 生成 SQL 的难度和不稳定性。

## 5. AI SQL Generation

### 5.1 Prompt Inputs

后端会把以下约束作为提示词传给 AI：

- 当前允许的表/视图名单
- 当前市场范围（默认沪市、深市）
- 当前只支持日线数据
- 当前结果模式：
  - 不限
  - 或前 N 条
- SQL 只允许只读查询
- 返回必须是单条 SQL

### 5.2 SQL Rules

允许：

- `SELECT`
- `WITH ... SELECT`
- `JOIN`
- `GROUP BY`
- `HAVING`
- 子查询
- 窗口函数

禁止：

- `INSERT`
- `UPDATE`
- `DELETE`
- `DROP`
- `ALTER`
- `ATTACH`
- `PRAGMA`
- 多语句

### 5.3 Result Mode

- 当结果模式为“前 N 条”：
  - AI 生成 SQL 必须带 `ORDER BY`
  - AI 生成 SQL 必须带 `LIMIT N`
- 当结果模式为“不限”：
  - AI 不写 `LIMIT`
  - 但必须带 `ORDER BY`
  - 后端仍然会分页返回，并先做 `COUNT(*)`

## 6. Execution Safety

提示词不是唯一约束，后端必须做执行前校验：

- 只能是一条语句
- 开头必须是 `SELECT` 或 `WITH`
- 只能访问白名单表/视图
- 拒绝所有写操作和危险语句
- 设置查询超时
- 限制单页最大返回条数

## 7. Sync Model

### 7.1 Manual Sync

- 用户点击后执行同步。
- 首次同步：按设置范围拉取最近 N 天。
- 非首次同步：根据 `sync_state` 只补缺失日期。

### 7.2 Auto Sync

- 仅在应用运行时生效。
- 开启后，应用内启动定时任务。
- 到达设置时间后执行增量同步。
- 同步完成后执行一次保留策略清理。

### 7.3 Data Retention

- `market_data` 相关表：
  - 永久保留
  - 仅保留近 30 天
  - 仅保留近 60 天
- `screening_history` 默认永久保留。

## 8. UI Behavior

### 8.1 Result Interaction

- 点击筛选结果中的股票：
  - 中间详情区切换到该股票
  - 右侧 `AgentRoom` 复用现有分析能力
- 点击“加入自选”：
  - 调用现有自选逻辑
  - 不影响当前筛选结果列表

### 8.2 State Separation

- 自选列表状态与 AI 筛选结果状态分离保存。
- 模式切换不互相覆盖。
- 当前 AI 筛选结果应单独维护，不污染现有 `watchlist`。

## 9. Recommended Rollout

第一版只做：

- 沪市 / 深市
- 日线数据
- 手动同步 + 应用运行时自动同步
- AI -> 受限 SQL -> SQLite 查询
- 历史记录回放
- 左侧结果列表 + 左下 AI 工作区

第一版不做：

- 分钟线入库
- 北交所和指数默认同步
- 热点 / 研报 / 龙虎榜入库
- AI 任意脚本执行
- 应用外系统计划任务

## 10. Main Risks

- 若视图设计不足，AI 会频繁生成过于复杂或不稳定的 SQL。
- 若把“不限”实现成全量一次返回，UI 和查询性能会受影响。
- 若历史记录只保存提示词而不保存结果，点击历史记录会变成重新查询，体验不稳定。

## 11. Recommendation

采用“`AI -> 受限 SQL -> SQLite 白名单视图查询`”方案，并将 `AI 筛选` 作为主屏的独立模式入口。该方案在体验、控制力和工程复杂度之间最平衡，适合当前项目的第一版实现。
