# Screening Sync Stale Symbols Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让 AI 筛选同步自动排除“连续多次无新增且明显落后于目标交易日”的股票，避免它们反复进入同步队列并永久拖住覆盖率。

**Architecture:** 在 SQLite 中新增一张同步排除状态表，记录单股票的连续无新增次数、最近本地/远端/目标交易日和排除标记。同步队列构建、覆盖率统计和成功写入后的状态恢复都统一走这张表，不再借用 `stocks_basic.is_active` 承担本地同步资格语义。

**Tech Stack:** Go, SQLite, `go test`

---

### Task 1: 为同步排除状态表写 store 级失败测试

**Files:**
- Modify: `jcp/internal/services/screening_store_test.go`
- Modify: `jcp/internal/services/screening_store.go`

**Step 1: Write the failing test**

新增测试，断言 schema 中存在新的同步排除状态表，并能正确写入/读取单股票状态。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run 'Test(NewScreeningStoreCreatesSchemaAndIndexes|ScreeningStoreSyncSymbolStateHelpers)$'`
Expected: FAIL，因为当前 schema 里还没有这张表和对应 helper。

**Step 3: Write minimal implementation**

新增表结构、状态结构体和最小的 upsert/get/list helper。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run 'Test(NewScreeningStoreCreatesSchemaAndIndexes|ScreeningStoreSyncSymbolStateHelpers)$'`
Expected: PASS

### Task 2: 为“连续无新增后排除”写失败测试

**Files:**
- Modify: `jcp/internal/services/screening_sync_service_test.go`
- Modify: `jcp/internal/services/screening_sync_service.go`

**Step 1: Write the failing test**

新增测试，构造一只股票连续 3 次都返回同样旧的 bars，且目标交易日比源最后日期晚 20 个交易日以上；断言第三次后该股票进入排除状态。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run TestScreeningSyncExcludesPersistentlyStaleSymbols`
Expected: FAIL，因为当前逻辑只会反复 `no new bars`，不会排除。

**Step 3: Write minimal implementation**

在 `no new bars` 分支更新单股票状态；达到阈值后标记 `excluded`。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run TestScreeningSyncExcludesPersistentlyStaleSymbols`
Expected: PASS

### Task 3: 为“排除后不再进同步队列”写失败测试

**Files:**
- Modify: `jcp/internal/services/screening_sync_service_test.go`
- Modify: `jcp/internal/services/screening_sync_service.go`

**Step 1: Write the failing test**

新增测试，先预置一只已排除股票，再执行同步，断言不会调用该股票的日线取数。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run TestScreeningSyncSkipsExcludedSymbols`
Expected: FAIL，因为当前队列构建不认识排除状态。

**Step 3: Write minimal implementation**

同步候选构建时读取排除表并剔除已排除股票。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run TestScreeningSyncSkipsExcludedSymbols`
Expected: PASS

### Task 4: 为覆盖率统计修正写失败测试

**Files:**
- Modify: `jcp/internal/services/screening_sync_service_test.go`
- Modify: `jcp/internal/services/screening_sync_service.go`

**Step 1: Write the failing test**

新增测试，断言 `attachCoverageSummary()` 计算 `MarketStockCount / PendingSyncStocks / SyncedToLatestStocks` 时会忽略已排除股票。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run TestScreeningSyncCoverageIgnoresExcludedSymbols`
Expected: FAIL，因为当前覆盖率统计仍把它们算进总数。

**Step 3: Write minimal implementation**

覆盖率统计时复用同一套“活跃且未排除”的候选过滤。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run TestScreeningSyncCoverageIgnoresExcludedSymbols`
Expected: PASS

### Task 5: 为“成功补到新 bars 后自动恢复”写失败测试

**Files:**
- Modify: `jcp/internal/services/screening_sync_service_test.go`
- Modify: `jcp/internal/services/screening_sync_service.go`

**Step 1: Write the failing test**

新增测试，预置一只已排除股票；让后续同步拿到更新 bars，断言排除状态被清空、无新增计数归零。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run TestScreeningSyncClearsExcludedStateAfterFreshBars`
Expected: FAIL，因为当前没有恢复逻辑。

**Step 3: Write minimal implementation**

在成功写入新 bars 后重置该股票的同步状态。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run TestScreeningSyncClearsExcludedStateAfterFreshBars`
Expected: PASS

### Task 6: 文档与全量验证

**Files:**
- Modify: `llmdoc/architecture/system-map.md`
- Modify: `llmdoc/guides/common-workflows.md`
- Modify: `llmdoc/reference/conventions.md`

**Step 1: Update docs**

补充“长期无新增股票会进入本地同步排除名单”的规则，以及覆盖率统计与同步队列共享同一资格判定。

**Step 2: Run full verification**

Run: `go test ./internal/services`
Expected: PASS

**Step 3: Commit**

```bash
git add jcp/internal/services/*.go jcp/internal/services/*_test.go llmdoc docs/plans/2026-03-25-screening-sync-stale-symbols-*.md
git commit -m "fix: exclude persistently stale screening symbols"
```
