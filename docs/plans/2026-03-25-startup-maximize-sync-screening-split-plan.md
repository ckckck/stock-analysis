# 启动最大化与同步/筛选拆分 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让散牛盘启动时自动最大化到工作区，并把 AI 筛选确认弹框的“同步”和“筛选”动作拆开。

**Architecture:** 后端把 Wails 窗口配置抽成可测试函数并设置为 `options.Maximised`。前端继续复用 `SyncActionDialog`，但在 `screening` 模式下拆成双按钮和双回调，保留 `sync-only` 模式单按钮不变。

**Tech Stack:** Go, Wails v2, React 18, TypeScript, Vitest

---

### Task 1: 窗口启动最大化

**Files:**
- Modify: `jcp/main.go`
- Create: `jcp/main_test.go`

**Step 1: 写失败测试**

给窗口配置提取函数写测试，断言 `WindowStartState == options.Maximised`。

**Step 2: 运行测试确认失败**

Run: `go test ./... -run TestBuildAppOptionsStartsMaximised`

**Step 3: 写最小实现**

提取 `buildAppOptions(app *App) *options.App`，在其中设置 `WindowStartState: options.Maximised`，`main()` 直接调用。

**Step 4: 运行测试确认通过**

Run: `go test ./... -run TestBuildAppOptionsStartsMaximised`

### Task 2: 弹框文案与动作拆分

**Files:**
- Modify: `jcp/frontend/src/utils/screeningSync.ts`
- Modify: `jcp/frontend/src/utils/screeningSync.test.ts`

**Step 1: 写失败测试**

扩展 `resolveSyncDialogCopy('screening')` 的断言，要求返回双按钮相关文案。

**Step 2: 运行测试确认失败**

Run: `npm test -- screeningSync.test.ts`

**Step 3: 写最小实现**

扩展 `ScreeningSyncDialogCopy`，让 `screening` 模式返回标题、说明、主按钮文案 `开始筛选` 和次按钮文案 `开始同步`，`sync-only` 保持单按钮。

**Step 4: 运行测试确认通过**

Run: `npm test -- screeningSync.test.ts`

### Task 3: 弹框双按钮接线

**Files:**
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: 写失败测试或先补纯函数断言**

如果不引入组件测试，则通过现有工具函数测试锁定文案，再在代码里最小改动接线：
- 新增“只同步”处理函数
- 新增“直接筛选”处理函数
- `screening` 模式渲染双按钮

**Step 2: 写最小实现**

将当前 `handleWelcomeSyncAndContinue` 拆成：
- `handleWelcomeSyncOnly`
- `handleWelcomeScreeningConfirm`

并在 `SyncActionDialog` 中仅对 `screening` 模式显示次级 `开始同步` 按钮和主 `开始筛选` 按钮。

**Step 3: 运行前端测试**

Run: `npm test -- screeningSync.test.ts`

### Task 4: 全量验证

**Files:**
- Verify only

**Step 1: 运行 Go 测试**

Run: `go test ./...`

**Step 2: 运行前端测试**

Run: `npm test`

**Step 3: 构建前端**

Run: `npm run build`
