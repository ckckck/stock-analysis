# Screening Query Cancel And Progress Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make AI screening wait indefinitely by default, support explicit query cancellation, and replace the current top streaming-output box with an ordered step progress display that embeds the live AI output into the active step.

**Architecture:** Extend the backend query flow so `sqlTimeoutSeconds=0` means no deadline and add an app-level cancelable query context for both fresh screening and history reruns. On the frontend, restack the screening workspace controls responsively and rebuild the loading card into a forward-ordered stepper with per-step states and wrapped larger streaming text shown only inside the active step.

**Tech Stack:** Go, React, TypeScript, Wails, Tailwind CSS

---

### Task 1: Query timeout and cancel regression tests

**Files:**
- Modify: `jcp/internal/services/screening_query_service_test.go`
- Create: `jcp/app_test.go`

**Step 1: Write the failing service test**

- Add a test proving `Screening.SQLTimeoutSeconds = 0` produces a context without deadline for SQL generation.

**Step 2: Run the targeted test and verify it fails**

Run: `go test ./internal/services -run TestScreeningQueryRunAllowsUnlimitedSQLGenerationTimeout -count=1`

**Step 3: Write the failing app cancel test**

- Add a test that starts a long-running screening query, calls app cancellation, and expects the query to stop without waiting for completion.

**Step 4: Run the targeted app test and verify it fails**

Run: `go test . -run TestAppCancelScreeningQueryStopsInFlightRequest -count=1`

### Task 2: Implement unlimited timeout and query cancellation

**Files:**
- Modify: `jcp/internal/services/config_service.go`
- Modify: `jcp/internal/services/config_service_test.go`
- Modify: `jcp/internal/services/screening_query_service.go`
- Modify: `jcp/app.go`

**Step 1: Implement unlimited timeout**

- Change default screening config timeout to `0`.
- Make query generation use `context.WithCancel` instead of `WithTimeout` when timeout is disabled.

**Step 2: Implement app-level screening query cancel**

- Add app fields to track one in-flight screening query cancel func.
- Use a dedicated cancelable context in `RunScreeningQuery` and `RerunScreeningHistoryRun`.
- Add `CancelScreeningQuery` Wails method.

**Step 3: Re-run targeted backend tests**

Run:
- `go test ./internal/services -run TestScreeningQueryRunAllowsUnlimitedSQLGenerationTimeout -count=1`
- `go test . -run TestAppCancelScreeningQueryStopsInFlightRequest -count=1`

### Task 3: Frontend state and workspace controls

**Files:**
- Modify: `jcp/frontend/src/components/ScreeningWorkspace.tsx`
- Modify: `jcp/frontend/src/services/screeningService.ts`
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: Introduce the failing build change**

- Wire a cancel callback into the screening workspace and use unlimited-timeout labels in the UI.

**Step 2: Run build and verify it fails**

Run: `npm --prefix frontend run build`

**Step 3: Implement responsive workspace controls**

- Prevent the main button from breaking awkwardly in narrow sidebars.
- Allow controls to wrap into stacked rows when needed.
- Add an explicit `取消筛选` action while loading.

**Step 4: Re-run build**

Run: `npm --prefix frontend run build`

### Task 4: Rebuild screening progress card as ordered stepper

**Files:**
- Modify: `jcp/frontend/src/components/ScreeningResultList.tsx`

**Step 1: Introduce the failing build change**

- Replace the old top streaming-output box with a stepper model in the component.

**Step 2: Run build and verify it fails**

Run: `npm --prefix frontend run build`

**Step 3: Implement the stepper**

- Ordered steps:
  - `准备请求`
  - `思考中`
  - `生成 SQL`
- Show completed steps first with green checks.
- Show the current step with emphasized styling.
- Show upcoming steps in muted styling.
- Render the live AI output only inside the active step.
- Increase font size and allow wrapping for the live output content.

**Step 4: Re-run build**

Run: `npm --prefix frontend run build`

### Task 5: Final verification

**Files:**
- Modify: `jcp/app.go`
- Modify: `jcp/internal/services/screening_query_service.go`
- Modify: `jcp/frontend/src/components/ScreeningWorkspace.tsx`
- Modify: `jcp/frontend/src/components/ScreeningResultList.tsx`

**Step 1: Run backend tests**

Run: `go test ./... -count=1`

**Step 2: Run frontend build**

Run: `npm --prefix frontend run build`

**Step 3: Spot-check requirements**

- Default SQL timeout label shows unlimited semantics.
- Manual cancel stops the in-flight AI screening wait.
- Workspace buttons stay stable in a narrow sidebar.
- Step progress runs in forward order and embeds the active live output.
