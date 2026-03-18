# AI Screening Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an `AI 筛选` mode to the main screen, backed by a local SQLite screening database, incremental sync, AI-generated read-only SQL, result history, and reusable stock detail viewing.

**Architecture:** The feature is split into four layers: SQLite market data + history storage, sync services and retention logic in Go, AI-to-SQL generation and guarded execution in Go, and a left-sidebar React mode switch that separates watchlist and AI screening results. The current center detail panel and right `AgentRoom` stay reused as-is to avoid rebuilding stock detail flows.

**Tech Stack:** Go, SQLite, Wails, React 18, TypeScript, Vite

---

### Task 1: Add Screening Config Models

**Files:**
- Modify: `jcp/internal/models/config.go`
- Modify: `jcp/internal/services/config_service.go`
- Modify: `jcp/frontend/src/types.ts`
- Modify: `jcp/frontend/src/services/configService.ts`

**Step 1: Write the failing test**

Document expected config additions:
- screening DB sync settings
- market scope toggles
- sync range days
- retention mode
- auto sync enabled/time

**Step 2: Run test to verify it fails**

Run: project config round-trip check after adding a sample config payload
Expected: current config model drops unknown screening fields.

**Step 3: Write minimal implementation**

Add a new screening config block to app config with:
- enabled market scopes
- initial sync days
- retention mode / retention days
- auto sync enabled
- auto sync time
- result limit default

Also add defaults in `ConfigService.defaultConfig()`.

**Step 4: Run test to verify it passes**

Run: config load/save validation
Expected: screening settings survive save/load and missing values get defaults.

**Step 5: Commit**

```bash
git add jcp/internal/models/config.go jcp/internal/services/config_service.go jcp/frontend/src/types.ts jcp/frontend/src/services/configService.ts
git commit -m "feat: add screening configuration model"
```

### Task 2: Create SQLite Screening Storage Layer

**Files:**
- Create: `jcp/internal/services/screening_store.go`
- Create: `jcp/internal/services/screening_store_test.go`
- Modify: `jcp/internal/pkg/paths/paths.go`

**Step 1: Write the failing test**

Add tests that expect schema creation for:
- `stocks_basic`
- `daily_bars`
- `daily_snapshots`
- `sync_state`
- `screening_runs`
- `screening_run_results`

**Step 2: Run test to verify it fails**

Run: Go test for the new store package
Expected: missing store and schema code.

**Step 3: Write minimal implementation**

Create a storage service that:
- opens SQLite in the app data directory
- creates schema and indexes
- exposes helpers for run history and sync state

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run ScreeningStore -v`
Expected: schema and basic insert/query tests pass.

**Step 5: Commit**

```bash
git add jcp/internal/services/screening_store.go jcp/internal/services/screening_store_test.go jcp/internal/pkg/paths/paths.go
git commit -m "feat: add sqlite screening storage"
```

### Task 3: Implement Manual and Incremental Daily Sync

**Files:**
- Create: `jcp/internal/services/screening_sync_service.go`
- Create: `jcp/internal/services/screening_sync_service_test.go`
- Modify: `jcp/internal/services/market_service.go`
- Modify: `jcp/app.go`

**Step 1: Write the failing test**

Add tests for:
- first sync over last N days
- incremental sync using `sync_state`
- retention cleanup for 30/60 days

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run ScreeningSync -v`
Expected: missing sync implementation.

**Step 3: Write minimal implementation**

Build a sync service that:
- refreshes `stocks_basic` for enabled scopes
- fetches daily bars for tracked scope and dates
- derives `daily_snapshots`
- records last synced trade date
- applies retention cleanup

Expose `App` methods for:
- manual sync
- sync status query

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run ScreeningSync -v`
Expected: first-sync, incremental-sync, and cleanup tests pass.

**Step 5: Commit**

```bash
git add jcp/internal/services/screening_sync_service.go jcp/internal/services/screening_sync_service_test.go jcp/internal/services/market_service.go jcp/app.go
git commit -m "feat: add screening data sync service"
```

### Task 4: Add In-App Auto Sync Scheduler

**Files:**
- Create: `jcp/internal/services/screening_scheduler.go`
- Create: `jcp/internal/services/screening_scheduler_test.go`
- Modify: `jcp/app.go`

**Step 1: Write the failing test**

Add tests for:
- scheduler disabled when config says off
- scheduler fires only while app process is running
- scheduler computes next run from configured local time

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run ScreeningScheduler -v`
Expected: no scheduler exists.

**Step 3: Write minimal implementation**

Create an app-lifetime scheduler that:
- starts on app startup
- reads screening config
- triggers sync at configured time
- stops on app shutdown

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run ScreeningScheduler -v`
Expected: scheduler behavior tests pass.

**Step 5: Commit**

```bash
git add jcp/internal/services/screening_scheduler.go jcp/internal/services/screening_scheduler_test.go jcp/app.go
git commit -m "feat: add in-app screening auto sync"
```

### Task 5: Implement AI-to-SQL Generation and Guarded Execution

**Files:**
- Create: `jcp/internal/services/screening_query_service.go`
- Create: `jcp/internal/services/screening_query_service_test.go`
- Modify: `jcp/internal/adk/model_factory.go`
- Modify: `jcp/app.go`

**Step 1: Write the failing test**

Add tests for:
- SQL generation request payload building
- SQL validation rejecting dangerous statements
- limit mode behavior
- unlimited mode behavior with count + pagination

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run ScreeningQuery -v`
Expected: missing service and validator.

**Step 3: Write minimal implementation**

Create a query service that:
- sends prompt + market scope + result mode + white-list view docs to AI
- receives SQL
- validates read-only rules
- runs count query
- runs paged result query
- stores `screening_runs` and `screening_run_results`

Expose Wails methods for:
- run screening
- list history
- load one history result

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run ScreeningQuery -v`
Expected: dangerous SQL rejected and valid screening query flow passes.

**Step 5: Commit**

```bash
git add jcp/internal/services/screening_query_service.go jcp/internal/services/screening_query_service_test.go jcp/internal/adk/model_factory.go jcp/app.go
git commit -m "feat: add ai-driven screening query service"
```

### Task 6: Add Frontend Mode Switch and Screening Sidebar

**Files:**
- Modify: `jcp/frontend/src/App.tsx`
- Create: `jcp/frontend/src/components/ScreeningWorkspace.tsx`
- Create: `jcp/frontend/src/components/ScreeningResultList.tsx`
- Create: `jcp/frontend/src/services/screeningService.ts`
- Modify: `jcp/frontend/src/types.ts`

**Step 1: Write the failing test**

Define expected UI behaviors:
- top toggle switches between watchlist and screening
- AI screening mode shows result list in upper-left
- workspace and history show in lower-left
- clicking result reuses center detail view
- clicking add-to-watchlist keeps result list intact

**Step 2: Run test to verify it fails**

Run: frontend component tests or a lightweight UI interaction check
Expected: missing components and mode state.

**Step 3: Write minimal implementation**

Add:
- screen mode state in `App.tsx`
- top toggle buttons in the header
- separate state for screening results/history/current selected source
- lower-left workspace with prompt input, result mode selector, run button, history list
- upper-left result list with detail + add-to-watchlist actions

**Step 4: Run test to verify it passes**

Run: frontend test/build command
Expected: mode switching and props wiring pass; TypeScript compiles.

**Step 5: Commit**

```bash
git add jcp/frontend/src/App.tsx jcp/frontend/src/components/ScreeningWorkspace.tsx jcp/frontend/src/components/ScreeningResultList.tsx jcp/frontend/src/services/screeningService.ts jcp/frontend/src/types.ts
git commit -m "feat: add ai screening workspace to sidebar"
```

### Task 7: Extend Settings for Sync and Retention Controls

**Files:**
- Modify: `jcp/frontend/src/components/SettingsDialog.tsx`
- Modify: `jcp/frontend/src/services/configService.ts`
- Modify: `jcp/frontend/src/types.ts`

**Step 1: Write the failing test**

Define expected settings fields:
- enabled market scopes
- initial sync range
- auto sync toggle
- auto sync time
- retention mode

**Step 2: Run test to verify it fails**

Run: frontend build/type-check
Expected: missing settings UI and types.

**Step 3: Write minimal implementation**

Add a screening settings section with:
- checkboxes for market scope
- sync range selector
- retention selector
- auto sync toggle/time
- manual sync trigger + status display

**Step 4: Run test to verify it passes**

Run: frontend build/type-check
Expected: settings compile and state persists through config save/load.

**Step 5: Commit**

```bash
git add jcp/frontend/src/components/SettingsDialog.tsx jcp/frontend/src/services/configService.ts jcp/frontend/src/types.ts
git commit -m "feat: add screening sync settings ui"
```

### Task 8: Document and Verify the End-to-End Flow

**Files:**
- Modify: `llmdoc/overview/project-overview.md`
- Modify: `llmdoc/architecture/system-map.md`
- Modify: `llmdoc/guides/common-workflows.md`
- Modify: `llmdoc/reference/conventions.md`

**Step 1: Write the failing test**

Define expected docs coverage:
- AI screening mode
- SQLite sync and retention
- history replay
- AI SQL safety rules

**Step 2: Run test to verify it fails**

Run: doc review against implemented behavior
Expected: llmdoc missing new screening flow.

**Step 3: Write minimal implementation**

Update llmdoc to match the implemented architecture and workflows.

**Step 4: Run test to verify it passes**

Run:
- `go test ./...`
- `cd jcp/frontend && npm run build`
- any targeted screening service tests

Expected: Go tests pass, frontend build passes, and the app has no uncommitted code changes except intended docs.

**Step 5: Commit**

```bash
git add llmdoc docs/plans
git commit -m "docs: record ai screening architecture and workflows"
```
