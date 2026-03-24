# AI 筛选工作区与 SQL 超时 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 精简 AI 筛选左下工作区 UI，并把 SQL 超时配置迁移到 `AI筛选` 设置中，由筛选查询优先读取。

**Architecture:** 后端在 `ScreeningConfig` 中新增独立 `sqlTimeoutSeconds` 字段，并在查询服务里优先使用该值；前端删掉左下工作区的冗余标题和说明卡，同时在 `AI筛选` 设置页新增 SQL 超时选项与摘要显示。

**Tech Stack:** Go, Wails, React, TypeScript, SQLite

---

### Task 1: 为 AI 筛选超时新增后端配置字段

**Files:**
- Modify: `jcp/internal/models/config.go`
- Modify: `jcp/internal/services/config_service.go`
- Test: `jcp/internal/services/screening_query_service_test.go`

**Step 1: Write the failing test**

给 `screening_query_service_test.go` 新增用例，断言当 `Screening.SQLTimeoutSeconds=123` 且默认 AI `Timeout=77` 时，实际 SQL 生成超时约为 `123s`。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run TestScreeningQueryRunUsesScreeningConfigTimeoutBeforeAIProvider -count=1`

Expected: FAIL，提示配置字段或行为不存在。

**Step 3: Write minimal implementation**

- 在 `ScreeningConfig` 增加 `SQLTimeoutSeconds int`
- 在默认配置与旧配置补全逻辑里加入该字段，默认 `45`
- 在 `resolveSQLGenerationTimeout()` 里先读 `Screening.SQLTimeoutSeconds`

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run 'TestScreeningQueryRunUsesScreeningConfigTimeoutBeforeAIProvider|TestScreeningQueryRunUsesAIConfigTimeoutForSQLGeneration' -count=1`

Expected: PASS

### Task 2: 把 SQL 超时配置接到前端 AI筛选设置页

**Files:**
- Modify: `jcp/frontend/src/types.ts`
- Modify: `jcp/frontend/src/components/SettingsDialog.tsx`
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: Extend frontend types**

为 `ScreeningConfig` 增加 `sqlTimeoutSeconds` 字段，并补齐设置页初始值。

**Step 2: Add AI筛选 setting control**

在 `AI筛选` 设置页中新增 `SQL 超时` 下拉项，固定值 `45/60/90/120/180` 秒。

**Step 3: Update summary source**

确认弹框与摘要不再从默认 Provider 推导超时，改为直接读取 `screening.sqlTimeoutSeconds`。

**Step 4: Run build to verify**

Run: `npm --prefix frontend run build`

Expected: PASS

### Task 3: 精简 AI 筛选工作区 UI

**Files:**
- Modify: `jcp/frontend/src/components/ScreeningWorkspace.tsx`

**Step 1: Remove redundant sections**

- 删除顶部“AI 筛选工作区”标题区
- 删除底部说明卡

**Step 2: Replace label copy**

把原 `筛选条件` 标题改为 `输入自然语言，描述筛选方式`

**Step 3: Keep core actions**

保留输入框、结果条数选择、执行按钮与历史重跑按钮文案逻辑。

**Step 4: Run build to verify**

Run: `npm --prefix frontend run build`

Expected: PASS

### Task 4: 同步文档并做全量验证

**Files:**
- Modify: `llmdoc/guides/common-workflows.md`
- Modify: `llmdoc/architecture/system-map.md`

**Step 1: Update docs**

记录新的 SQL 超时配置位置和工作区布局变化。

**Step 2: Run full verification**

Run: `go test ./...`

Expected: PASS

Run: `npm --prefix frontend run build`

Expected: PASS

Run: `~/go/bin/wails build`

Expected: PASS
