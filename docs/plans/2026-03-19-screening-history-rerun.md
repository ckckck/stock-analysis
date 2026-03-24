# AI 筛选历史结果重跑与查询超时 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复 AI 筛选生成 SQL 的固定超时问题，并把左侧结果区改成“当前筛选结果 / 历史筛选结果”双页签，支持基于历史 SQL 直接重跑并生成新历史。

**Architecture:** 后端把 SQL 生成超时改为读取当前 AI 配置，并新增“按历史 run 的 `generatedSql` 直接重跑”的服务接口；前端把左侧结果区分为当前结果页和历史结果页，历史记录点击后恢复结果并允许直接重跑，不再重新请求 AI 生成 SQL。设置弹窗层级单独提升，避免被确认弹框遮挡。

**Tech Stack:** Go, Wails v2, React, TypeScript, SQLite

---

### Task 1: 后端查询超时与历史 SQL 重跑

**Files:**
- Modify: `jcp/app.go`
- Modify: `jcp/internal/services/screening_query_service.go`
- Test: `jcp/internal/services/screening_query_service_test.go`

**Step 1: Write the failing test**

- 新增测试，断言：
  - 筛选查询超时会读取配置而不是固定 45 秒
  - 基于历史 `generatedSql` 重跑会生成新的 run 和新的结果快照

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run 'TestScreeningQuery.*(Timeout|Rerun)' -v`

**Step 3: Write minimal implementation**

- 给查询服务增加可配置 SQL 生成超时解析
- 将超时错误包装为明确提示
- 新增按历史 SQL 重跑的方法
- `App` 暴露新的 Wails 接口

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run 'TestScreeningQuery.*(Timeout|Rerun)' -v`

### Task 2: 前端双页签与历史 SQL 重跑入口

**Files:**
- Modify: `jcp/frontend/src/types.ts`
- Modify: `jcp/frontend/src/services/screeningService.ts`
- Modify: `jcp/frontend/src/App.tsx`
- Modify: `jcp/frontend/src/components/ScreeningResultList.tsx`
- Modify: `jcp/frontend/src/components/ScreeningWorkspace.tsx`

**Step 1: Write the failing test**

- 通过类型和构建约束先引入新的页签状态、历史来源状态和历史 SQL 重跑接口，预期前端构建失败直到所有 props 和调用点补齐。

**Step 2: Run test to verify it fails**

Run: `cd jcp/frontend && npm run build`

**Step 3: Write minimal implementation**

- `ScreeningResultList` 新增结果页签
- 历史结果页签显示整栏历史列表
- 点击历史记录后恢复当前结果并切换按钮文案
- 工作区按钮在历史来源时改走“根据当前条件重新筛选”

**Step 4: Run test to verify it passes**

Run: `cd jcp/frontend && npm run build`

### Task 3: 设置弹窗层级修复

**Files:**
- Modify: `jcp/frontend/src/components/SettingsDialog.tsx`
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: Write the failing check**

- 手工以 DOM 结构和类名为约束，确认设置弹窗当前会被确认框压住。

**Step 2: Implement minimal fix**

- 提升设置弹窗层级，使其高于确认框。

**Step 3: Run verification**

Run: `cd jcp/frontend && npm run build`

### Task 4: 全量验证与文档

**Files:**
- Modify: `llmdoc/architecture/system-map.md`
- Modify: `llmdoc/guides/common-workflows.md`
- Modify: `llmdoc/index.md`
- Modify: `llmdoc/overview/project-overview.md`

**Step 1: Run backend verification**

Run: `go test ./...`

**Step 2: Run frontend verification**

Run: `cd jcp/frontend && npm run build`

**Step 3: Run package verification**

Run: `cd jcp && ~/go/bin/wails build`

**Step 4: Update llmdoc**

- 补充历史结果双页签、历史 SQL 重跑、SQL 生成超时配置链与设置弹窗层级说明。
