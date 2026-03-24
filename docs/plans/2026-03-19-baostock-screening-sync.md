# Baostock Screening Sync Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Baostock-first, Sina-fallback daily bar source used only by AI screening sync.

**Architecture:** Keep the current generic `MarketService.GetKLineData()` unchanged and introduce a screening-specific daily-bar source chain behind `GetScreeningDailyBars()`. Implement Baostock in a dedicated source file and keep fallback orchestration in a separate screening-source file so the sync service remains unaware of provider details.

**Tech Stack:** Go, existing `MarketService`, SQLite-backed screening sync flow, Go testing package, Baostock Go client dependency if available

---

### Task 1: Document and prepare the source seam

**Files:**
- Modify: `jcp/internal/services/market_service.go`
- Test: `jcp/internal/services/market_service_test.go`

**Step 1: Write the failing test**

Add a narrow service-layer test that proves `GetScreeningDailyBars()` no longer has to call the generic daily K-line path directly once a screening source chain exists.

**Step 2: Run test to verify it fails**

Run: `cd jcp && go test ./internal/services -run 'TestFetchKLineDataReturnsHTTPStatusErrorBeforeJSONParsing|TestGetScreeningDailyBars' -v`

Expected: new `GetScreeningDailyBars` test fails because the screening-specific source chain does not exist yet.

**Step 3: Write minimal implementation seam**

Introduce a small internal seam in `MarketService` for screening daily bars, but do not add provider logic yet.

**Step 4: Run test to verify it passes**

Run: `cd jcp && go test ./internal/services -run 'TestFetchKLineDataReturnsHTTPStatusErrorBeforeJSONParsing|TestGetScreeningDailyBars' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add jcp/internal/services/market_service.go jcp/internal/services/market_service_test.go
git commit -m "refactor: add screening daily bar source seam"
```

### Task 2: Add the Baostock daily-bar source

**Files:**
- Create: `jcp/internal/services/baostock_daily_bar_source.go`
- Test: `jcp/internal/services/screening_daily_bar_source_test.go`

**Step 1: Write the failing test**

Add tests for:
- Shanghai/Shenzhen symbol conversion
- Baostock row mapping into `models.KLineData`
- empty-result handling

Use stubbed query responses where possible instead of real network calls.

**Step 2: Run test to verify it fails**

Run: `cd jcp && go test ./internal/services -run 'TestBaostock' -v`

Expected: FAIL because the Baostock source implementation does not exist yet.

**Step 3: Write minimal implementation**

Implement:
- symbol conversion from project symbols to Baostock symbols
- query window calculation from `lookbackDays`
- row parsing into `models.KLineData`
- context-rich errors

Keep Baostock behavior limited to daily bars needed by screening sync.

**Step 4: Run test to verify it passes**

Run: `cd jcp && go test ./internal/services -run 'TestBaostock' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add jcp/internal/services/baostock_daily_bar_source.go jcp/internal/services/screening_daily_bar_source_test.go
git commit -m "feat: add baostock screening daily bar source"
```

### Task 3: Add the screening provider chain with Sina fallback

**Files:**
- Create: `jcp/internal/services/screening_daily_bar_source.go`
- Modify: `jcp/internal/services/market_service.go`
- Test: `jcp/internal/services/screening_daily_bar_source_test.go`

**Step 1: Write the failing test**

Add tests for:
- `Baostock` success short-circuits Sina
- `Baostock` error falls back to Sina
- `Baostock` empty result falls back to Sina
- both sources fail and produce a combined error

**Step 2: Run test to verify it fails**

Run: `cd jcp && go test ./internal/services -run 'TestScreeningDailyBarSource' -v`

Expected: FAIL because fallback orchestration is missing.

**Step 3: Write minimal implementation**

Implement a screening-only source chain that:
- calls Baostock first
- falls back to Sina through existing `fetchKLineData(..., "1d", ...)`
- returns the first usable result
- aggregates failure messages if both fail

**Step 4: Run test to verify it passes**

Run: `cd jcp && go test ./internal/services -run 'TestScreeningDailyBarSource' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add jcp/internal/services/screening_daily_bar_source.go jcp/internal/services/market_service.go jcp/internal/services/screening_daily_bar_source_test.go
git commit -m "feat: add baostock-first screening sync fallback"
```

### Task 4: Verify screening sync integration and regressions

**Files:**
- Modify: `jcp/internal/services/screening_sync_service_test.go` if integration-level assertions are needed

**Step 1: Add or extend integration-facing tests only if required**

If the service seam changed in a way visible to sync flow, add a small regression test that still uses a fake market source and verifies no screening sync behavior regressed.

**Step 2: Run targeted verification**

Run:

```bash
cd /Users/caike/3.工具/stock-analysis/.worktrees/ai-screening/jcp && go test ./internal/services
cd /Users/caike/3.工具/stock-analysis/.worktrees/ai-screening/jcp && go test ./...
cd /Users/caike/3.工具/stock-analysis/.worktrees/ai-screening && git diff --check
```

Expected:
- service tests PASS
- full Go test suite PASS
- `git diff --check` no output

**Step 3: Commit**

```bash
git add jcp/internal/services/screening_sync_service_test.go
git commit -m "test: verify screening sync baostock fallback integration"
```

### Task 5: Sync project docs after implementation

**Files:**
- Modify: `llmdoc/architecture/system-map.md`
- Modify: `llmdoc/overview/project-overview.md`
- Modify: `llmdoc/guides/common-workflows.md`
- Modify: `llmdoc/index.md`

**Step 1: Document the screening sync provider order**

Add the fact that AI screening sync daily bars now use:
- `Baostock` first
- Sina daily K-line as fallback

**Step 2: Document operational caveats**

Add the note that Baostock is intended for sync stability and that fallback remains in place for empty or failed primary-source fetches.

**Step 3: Run doc hygiene checks**

Run:

```bash
cd /Users/caike/3.工具/stock-analysis/.worktrees/ai-screening && git diff --check
```

Expected: no whitespace or patch-format issues

**Step 4: Commit**

```bash
git add llmdoc/architecture/system-map.md llmdoc/overview/project-overview.md llmdoc/guides/common-workflows.md llmdoc/index.md
git commit -m "docs: document baostock screening sync fallback"
```
