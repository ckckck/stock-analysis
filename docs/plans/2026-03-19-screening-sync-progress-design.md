# Screening Sync Progress And Resume Design

**Date:** 2026-03-19

**Goal:** Add manual-sync test limiting, real-time progress with source-switch visibility, cancellation, and checkpoint resume for both manual and automatic AI screening sync.

## Context

- Current manual sync is a single blocking call from the settings page to `RunScreeningSync`, with only a boolean `screeningSyncing` state in the frontend.
- The backend `ScreeningSyncService.Sync()` has no context cancellation, no progress callback, and no persisted in-flight checkpoint state.
- Users currently only see a spinner and a final success/failure toast, so a slow sync appears stuck.
- AI screening sync now uses `Baostock -> Sina` source fallback, and users want to see which source is active and when fallback happens.

## Approved Scope

- Add a manual-only test option that limits sync to the first N stocks.
- Add a progress bar with percent and structured progress details.
- Add a cancel action for manual sync.
- Persist checkpoint state so both manual and automatic sync can resume from the last unfinished stock.
- Show active source and source-switch history in sync progress.

## Recommended Architecture

### 1. Split sync into run options + persisted task state

Keep existing sync business logic but introduce explicit runtime options:

- `ScreeningSyncRunOptions`
  - `Mode`: `manual` or `auto`
  - `LimitStocks`: optional, used only for manual sync
- `ScreeningSyncProgress`
  - total/completed stock counts
  - current symbol/name
  - percent
  - stage
  - active source
  - recent event messages
  - canceled/completed/failed flags

Persist resumable sync state in SQLite so restart or re-entry can continue from the last successful stock boundary.

### 2. Add a dedicated sync job state store

Store at least:

- job id
- mode
- market scope
- total stocks
- completed stocks
- current stock index
- current symbol
- limit stocks
- status (`running`, `canceled`, `failed`, `completed`)
- last completed symbol
- last source
- updated timestamp

The checkpoint is shared by manual and automatic sync. Manual-only testing limit is stored only on that job record and does not become global config.

### 3. Emit progress events instead of polling only final status

Use Wails runtime events for sync progress, similar to existing meeting/update progress flows.

Suggested event:

- `screening:sync:progress`

Each event should include:

- percent
- total stocks
- completed stocks
- current symbol/name
- current stage
- active source
- last message
- recent fallback/source-switch history
- status

### 4. Source visibility belongs in the screening sync progress layer

The sync service should receive source-attempt events from the screening daily-bar source chain:

- `baostock started`
- `baostock failed`
- `switching to sina`
- `sina succeeded`

This should not stay as logs only. It must be surfaced into the progress event payload.

### 5. Cancellation model

- Manual sync starts with a cancelable context stored on `App`.
- Cancel only affects manual sync UI entry.
- The service should stop at stock boundaries or after the current in-flight stock fetch returns.
- Already written bars and snapshots remain valid.
- Checkpoint is persisted as `canceled`, so next manual or auto sync resumes from the next unfinished stock.

## UI Changes

### Settings page

Add under manual sync:

- checkbox or toggle: enable test mode
- numeric input/select: sync only first N stocks
- progress bar
- current stock/source summary
- recent sync events list
- cancel button during manual sync

The test limit applies only when clicking manual sync.

### Status display

Show:

- `37%`
- `74 / 200`
- current symbol + name
- active source
- recent source-switch records

If canceled:

- `已取消，断点已保存，下次将继续`

## Risks

- If checkpoint granularity is too fine and written before data commits, resume can skip partially written work. Checkpoint must advance only after a stock has been fully persisted.
- Auto sync resuming a previously canceled manual test-limited run must ignore the manual limit and continue with the normal stock universe.
- Progress events can become noisy if every source attempt is kept forever. The frontend should show only a bounded recent list.

## Acceptance Criteria

- Manual sync can run in test mode with a stock limit.
- Progress bar updates during sync with percent and stock counts.
- Current active source is visible.
- Source failure and fallback messages are visible in recent progress history.
- Manual sync can be canceled.
- Both manual and automatic sync resume from the saved checkpoint on the next run.
