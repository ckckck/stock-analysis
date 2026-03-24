# Screening Status Display Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve history timestamps, home-page helper text, and screening sync status readability so users can quickly judge whether the market scope is up to date.

**Architecture:** Add a backend sync-status summary for full-market latest coverage, then render it with clearer formatting and color states in the confirm dialog. Keep homepage and history-list changes front-end only, sharing common date-formatting helpers where practical.

**Tech Stack:** Go, SQLite, React, TypeScript, Wails, Tailwind CSS

---

### Task 1: Add sync-status summary regression tests

**Files:**
- Modify: `jcp/internal/services/screening_sync_service_test.go`

**Step 1: Write the failing tests**

- Add a test covering full-market summary counts for:
- latest trade date
- synced-to-latest stock count
- pending stock count
- total stock count
- latest synced trade date and formatted last synced timestamp source data

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run TestScreeningSyncGetStatusIncludesMarketCoverageSummary -count=1`

**Step 3: Write minimal implementation**

- Extend sync status payload with latest-coverage summary fields.
- Compute counts from current market scope, independent of manual test-limit settings.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run TestScreeningSyncGetStatusIncludesMarketCoverageSummary -count=1`

### Task 2: Expose status summary to the frontend

**Files:**
- Modify: `jcp/frontend/src/types.ts`
- Modify: `jcp/frontend/src/services/configService.ts`
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: Write the failing build expectation**

- Use the new backend fields in the confirm-dialog summary shape so TypeScript fails until types are updated.

**Step 2: Run build to verify it fails**

Run: `npm --prefix frontend run build`

**Step 3: Write minimal implementation**

- Map the new status fields into the frontend type.
- Build a status-card summary with:
- readable datetime
- synced-to-latest count
- pending count
- total stock count
- color state

**Step 4: Run build to verify it passes**

Run: `npm --prefix frontend run build`

### Task 3: Update history list and homepage helper copy

**Files:**
- Modify: `jcp/frontend/src/components/ScreeningResultList.tsx`
- Modify: `jcp/frontend/src/components/WelcomePage.tsx`

**Step 1: Write the failing build expectation**

- Change the list/home components to use the shared formatted copy and status layout.

**Step 2: Run build to verify it fails**

Run: `npm --prefix frontend run build`

**Step 3: Write minimal implementation**

- Format history timestamps as `YYYY-MM-DD HH:mm:ss`.
- Change result count label to `X 条`.
- Replace homepage helper row with inline icon + short text inside the existing upper slot.
- Remove the lower duplicated helper text.

**Step 4: Run build to verify it passes**

Run: `npm --prefix frontend run build`

### Task 4: Verify end-to-end integrity

**Files:**
- Modify: `jcp/internal/services/screening_sync_service.go`
- Modify: `jcp/frontend/src/App.tsx`
- Modify: `jcp/frontend/src/components/ScreeningResultList.tsx`
- Modify: `jcp/frontend/src/components/WelcomePage.tsx`

**Step 1: Run backend tests**

Run: `go test ./... -count=1`

**Step 2: Run frontend build**

Run: `npm --prefix frontend run build`

**Step 3: Spot-check requirements**

- History time is readable
- History count uses `X 条`
- Homepage helper text is moved and deduplicated
- Confirm dialog shows full-market sync health with color state
