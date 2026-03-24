# Screening Sync Progress Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add manual test-limited screening sync, live progress with source visibility, cancel support, and checkpoint resume shared by manual and automatic sync.

**Architecture:** Extend screening sync with explicit run options, persisted job/checkpoint state, and Wails progress events. Keep manual-only test limiting out of global config, but make checkpoint resume a shared backend capability for both manual and automatic sync paths.

**Tech Stack:** Go, Wails runtime events, SQLite screening store, React, TypeScript

---

### Task 1: Add sync run models and failing backend tests

**Files:**
- Modify: `jcp/internal/services/screening_sync_service.go`
- Modify: `jcp/internal/services/screening_sync_service_test.go`
- Modify: `jcp/internal/services/screening_store.go`
- Modify: `jcp/internal/services/screening_store_test.go`

**Step 1: Write the failing tests**

Add tests for:
- manual run option limiting stocks
- checkpoint resuming from the next unfinished stock
- canceled runs persisting resumable state
- automatic runs ignoring manual test limits

**Step 2: Run test to verify it fails**

Run: `cd jcp && go test ./internal/services -run 'TestScreeningSync' -v`

Expected: FAIL because run options and checkpoint state do not exist yet.

**Step 3: Write minimal implementation**

Add:
- sync run options structs
- progress state structs
- checkpoint/job persistence schema

Do not add event emission yet.

**Step 4: Run test to verify it passes**

Run: `cd jcp && go test ./internal/services -run 'TestScreeningSync' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add jcp/internal/services/screening_sync_service.go jcp/internal/services/screening_sync_service_test.go jcp/internal/services/screening_store.go jcp/internal/services/screening_store_test.go
git commit -m "feat: add screening sync checkpoint state"
```

### Task 2: Add progress callbacks and source-attempt reporting

**Files:**
- Modify: `jcp/internal/services/screening_sync_service.go`
- Modify: `jcp/internal/services/screening_daily_bar_source.go`
- Modify: `jcp/internal/services/baostock_daily_bar_source.go`
- Modify: `jcp/internal/services/market_service_test.go`

**Step 1: Write the failing tests**

Add tests for:
- progress updates per stock
- current active source in progress payload
- fallback events recorded when switching `Baostock -> Sina`

**Step 2: Run test to verify it fails**

Run: `cd jcp && go test ./internal/services -run 'Test(ScreeningSync|GetScreeningDailyBars)' -v`

Expected: FAIL because source-attempt callbacks and progress reporting are missing.

**Step 3: Write minimal implementation**

Add:
- progress callback hooks to sync service
- source-attempt event callback from screening daily bar chain
- bounded event history for recent source records

**Step 4: Run test to verify it passes**

Run: `cd jcp && go test ./internal/services -run 'Test(ScreeningSync|GetScreeningDailyBars)' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add jcp/internal/services/screening_sync_service.go jcp/internal/services/screening_daily_bar_source.go jcp/internal/services/baostock_daily_bar_source.go jcp/internal/services/market_service_test.go
git commit -m "feat: add screening sync progress source events"
```

### Task 3: Add app-level sync start, cancel, and Wails progress events

**Files:**
- Modify: `jcp/app.go`
- Modify: `jcp/frontend/wailsjs/go/main/App.d.ts`
- Modify: `jcp/frontend/wailsjs/go/main/App.js`
- Modify: `jcp/frontend/wailsjs/go/models.ts`

**Step 1: Write the failing integration expectation mentally**

Current missing behaviors:
- no manual sync options input
- no cancel entry point
- no `screening:sync:progress` event emission

**Step 2: Add app methods**

Add:
- start manual sync with options
- cancel active manual sync
- get current sync progress/status if needed

Store manual sync cancel function on `App`, similar to meeting cancellation.

**Step 3: Emit Wails progress events**

Emit `screening:sync:progress` during manual and automatic runs.

**Step 4: Regenerate bindings if needed**

Run the Wails binding generation path via normal frontend/backend build flow.

**Step 5: Run verification**

Run: `cd jcp && go test ./...`

Expected: PASS

**Step 6: Commit**

```bash
git add jcp/app.go jcp/frontend/wailsjs/go/main/App.d.ts jcp/frontend/wailsjs/go/main/App.js jcp/frontend/wailsjs/go/models.ts
git commit -m "feat: expose screening sync progress and cancel api"
```

### Task 4: Add settings UI for manual test limit, progress, and cancel

**Files:**
- Modify: `jcp/frontend/src/components/SettingsDialog.tsx`
- Modify: `jcp/frontend/src/services/configService.ts`
- Modify: `jcp/frontend/src/types.ts`

**Step 1: Write the failing UI checklist**

Current failures:
- no manual test limit toggle
- no limit count input
- no progress bar
- no active source display
- no recent source/fallback history
- no cancel button

**Step 2: Add frontend types and service calls**

Add types for:
- sync run options
- sync progress payload
- sync event item

Wire service calls for:
- start manual sync with options
- cancel manual sync
- progress event subscription

**Step 3: Implement settings UI**

Add under the manual sync card:
- `测试模式` 开关
- `只同步前 N 只股票` 输入/选择
- progress bar
- current stock/source text
- recent event list
- cancel button while syncing

**Step 4: Run frontend build**

Run: `cd jcp/frontend && npm run build`

Expected: PASS

**Step 5: Commit**

```bash
git add jcp/frontend/src/components/SettingsDialog.tsx jcp/frontend/src/services/configService.ts jcp/frontend/src/types.ts
git commit -m "feat: add screening sync progress ui"
```

### Task 5: Verify end-to-end manual sync flow and docs

**Files:**
- Modify: `llmdoc/guides/common-workflows.md`
- Modify: `llmdoc/architecture/system-map.md`
- Modify: `llmdoc/overview/project-overview.md`
- Modify: `llmdoc/index.md`

**Step 1: Verify backend and frontend together**

Run:

```bash
cd /Users/caike/3.工具/stock-analysis/.worktrees/ai-screening/jcp && go test ./...
cd /Users/caike/3.工具/stock-analysis/.worktrees/ai-screening/jcp/frontend && npm run build
cd /Users/caike/3.工具/stock-analysis/.worktrees/ai-screening && git diff --check -- jcp/app.go jcp/internal/services/screening_sync_service.go jcp/internal/services/screening_store.go jcp/internal/services/screening_daily_bar_source.go jcp/internal/services/baostock_daily_bar_source.go jcp/frontend/src/components/SettingsDialog.tsx jcp/frontend/src/services/configService.ts jcp/frontend/src/types.ts llmdoc/guides/common-workflows.md llmdoc/architecture/system-map.md llmdoc/overview/project-overview.md llmdoc/index.md
```

Expected:
- `go test ./...` PASS
- `npm run build` PASS
- targeted `git diff --check` no output

**Step 2: Update llmdoc**

Document:
- manual test-limit sync
- progress events
- cancel and checkpoint resume
- source visibility and fallback records

**Step 3: Commit**

```bash
git add llmdoc/guides/common-workflows.md llmdoc/architecture/system-map.md llmdoc/overview/project-overview.md llmdoc/index.md
git commit -m "docs: document screening sync progress and resume"
```
