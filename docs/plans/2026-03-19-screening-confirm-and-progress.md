# AI 筛选确认与进度 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让 AI 筛选在任意入口都先弹出确认框，并在执行时展示真实阶段进度、日志和进度条。

**Architecture:** 使用统一的前端确认状态拦截所有筛选提交；后端在 `ScreeningQueryService` 中发出阶段事件，`App` 负责转成 Wails 事件，前端订阅并渲染进度面板。测试 universe 继续沿用已有 `UniverseSymbols` 链路。

**Tech Stack:** Go, Wails v2, React, TypeScript, SQLite

---

### Task 1: 查询进度事件后端

**Files:**
- Modify: `jcp/internal/services/screening_query_service.go`
- Test: `jcp/internal/services/screening_query_service_test.go`
- Modify: `jcp/app.go`

**Step 1: Write the failing test**

- 新增测试，断言查询执行时会按阶段回调进度，并包含日志。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run 'TestScreeningQueryRunReportsProgressStages' -v`

**Step 3: Write minimal implementation**

- 新增 `ScreeningQueryProgress` / `ScreeningQueryLog`
- 新增 `RunWithProgress`
- `App.RunScreeningQuery` 转发 `screening:query:progress`

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run 'TestScreeningQueryRunReportsProgressStages|TestScreeningQuery' -v`

### Task 2: 统一确认入口

**Files:**
- Modify: `jcp/frontend/src/App.tsx`
- Modify: `jcp/frontend/src/components/WelcomePage.tsx`
- Modify: `jcp/frontend/src/components/ScreeningWorkspace.tsx`

**Step 1: Write the failing test**

- 以前端构建为约束，先加入统一确认状态和 props，预期初次编译失败直到所有调用点补齐。

**Step 2: Run test to verify it fails**

Run: `cd jcp/frontend && npm run build`

**Step 3: Write minimal implementation**

- 统一由 `App.tsx` 管理待执行筛选请求和确认弹框状态
- 欢迎页和工作区都改成先打开确认，不直接执行
- 确认后再执行 `executeScreening`

**Step 4: Run test to verify it passes**

Run: `cd jcp/frontend && npm run build`

### Task 3: 前端进度面板

**Files:**
- Modify: `jcp/frontend/src/types.ts`
- Modify: `jcp/frontend/src/services/screeningService.ts`
- Modify: `jcp/frontend/src/App.tsx`
- Modify: `jcp/frontend/src/components/ScreeningWorkspace.tsx`
- Modify: `jcp/frontend/src/components/ScreeningResultList.tsx`

**Step 1: Write the failing test**

- 通过类型与构建约束先加入 `ScreeningQueryProgress` 类型和事件订阅，直到 UI 全部接线完成。

**Step 2: Run test to verify it fails**

Run: `cd jcp/frontend && npm run build`

**Step 3: Write minimal implementation**

- 订阅 `screening:query:progress`
- 工作区显示阶段、进度条、日志
- 结果列表加载态显示当前阶段

**Step 4: Run test to verify it passes**

Run: `cd jcp/frontend && npm run build`

### Task 4: 全量验证与文档

**Files:**
- Modify: `llmdoc/guides/common-workflows.md`
- Modify: `llmdoc/architecture/system-map.md`
- Modify: `llmdoc/index.md`

**Step 1: Run backend verification**

Run: `go test ./...`

**Step 2: Run frontend verification**

Run: `cd jcp/frontend && npm run build`

**Step 3: Run package verification**

Run: `~/go/bin/wails build`

**Step 4: Update llmdoc**

- 补充 AI 筛选“先确认后执行”和查询进度事件流说明。
