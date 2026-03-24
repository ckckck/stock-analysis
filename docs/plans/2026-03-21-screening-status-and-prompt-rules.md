# Screening Status And Prompt Rules Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修正 AI 筛选确认弹框中的同步状态展示，并为自然语言筛选补充更通用的涨停/连续性提示词规则。

**Architecture:** 前端通过保留已有同步覆盖统计字段，避免“开始同步”时用残缺状态覆盖卡片内容；同时移除低价值的“已入库日线”展示。后端在 SQL 生成提示词中加入统一的时间窗口、事件定义、连续性与涨停阈值规则，让模型能根据“连续三天都涨停”或“最近三天至少一天涨停”这类条件稳定产出 SQL。

**Tech Stack:** React + TypeScript, Go, Wails, SQLite

---

### Task 1: 锁定提示词规则的测试

**Files:**
- Modify: `jcp/internal/services/screening_query_service_test.go`

**Step 1: Write the failing test**

新增测试，断言 `buildPrompt()` 包含以下规则片段：
- 最近N天指最近 N 个交易日
- 上涨/下跌定义
- ST / 主板 / 创业板科创板北交所的涨停阈值
- 连续N天、至少K天、恰好K天、N天内有一天 的语义
- 先生成 `is_up` / `is_limit_up` 等日级标记，再聚合

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run TestScreeningQueryBuildPromptIncludesTradingWindowAndLimitUpRules -count=1`

Expected: FAIL，因为提示词里还没有这些规则片段。

**Step 3: Write minimal implementation**

在 `buildPrompt()` 中加入统一规则块，覆盖时间窗口、日级事件、涨停判定、连续性和组合条件。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run TestScreeningQueryBuildPromptIncludesTradingWindowAndLimitUpRules -count=1`

Expected: PASS

### Task 2: 修正同步中数据状态卡片

**Files:**
- Modify: `jcp/frontend/src/App.tsx`
- Modify: `jcp/frontend/src/utils/screeningSync.ts`

**Step 1: Preserve summary fields when sync starts**

将“开始同步并继续”时的临时 `screeningSyncStatus` 改为基于旧状态合并，只更新运行态字段，保留：
- `targetTradeDate`
- `latestSyncedTradeDate`
- `marketStockCount`
- `pendingSyncStocks`
- `syncedToLatestStocks`

**Step 2: Remove low-value data item**

从“当前数据状态”卡片中移除“已入库日线”。

**Step 3: Keep health summary stable during running**

确认同步进度事件合并逻辑不会把静态汇总字段清空。

**Step 4: Verify**

Run: `npm --prefix frontend run build`

Expected: PASS

### Task 3: 全量验证

**Files:**
- Verify only

**Step 1: Run backend tests**

Run: `go test ./internal/services -count=1`

Expected: PASS

**Step 2: Run full app tests**

Run: `go test ./... -count=1`

Expected: PASS

**Step 3: Run frontend build**

Run: `npm --prefix frontend run build`

Expected: PASS
