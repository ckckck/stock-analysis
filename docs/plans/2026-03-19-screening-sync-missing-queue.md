# Screening Sync Missing Queue Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让 AI 筛选同步只处理当日仍缺失日线数据的股票，并按单股票本地最新日期做增量补齐，避免每次从头逐只尝试。

**Architecture:** 同步开始前先从 `stocks_basic` / `daily_bars` 反查候选股票的本地最新交易日，构建真正需要同步的缺失队列。手动测试模式的 `LimitStocks` 先作用于候选股票范围，再基于该范围计算缺失队列；单股票写入时改为按该股票自己的本地最新日期过滤新增 bar。

**Tech Stack:** Go, SQLite, Wails backend tests

---

### Task 1: 为缺失队列补测试

**Files:**
- Modify: `jcp/internal/services/screening_sync_service_test.go`

**Step 1: Write the failing test**

增加用例覆盖：
- 已同步的股票再次手动同步时，应直接跳过，不再调用远端日线源。
- 部分股票缺失当日数据时，只同步缺失股票。
- 单股票落后于全局 `lastTradeDate` 时，仍能补齐缺失日期。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run 'TestScreeningSync(SkipsAlreadySyncedStocksForTargetTradeDate|OnlyProcessesSymbolsMissingTargetTradeDate|BackfillsLaggingSymbolDespiteGlobalSyncState)' -count=1`

Expected: FAIL，证明当前实现仍会遍历已同步股票，且存在按全局日期过滤的遗漏风险。

### Task 2: 为缺失队列补查询能力

**Files:**
- Modify: `jcp/internal/services/screening_store.go`
- Test: `jcp/internal/services/screening_store_test.go`

**Step 1: Write the failing test**

增加针对 store 的测试，验证：
- 给定候选 symbol 和目标交易日，只返回本地缺失该交易日数据的 symbol。
- 能返回候选 symbol 各自的本地最新交易日。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run 'TestScreeningStore(ListSymbolsMissingDailyBarsOnTradeDate|GetLatestBarTradeDatesForSymbols)' -count=1`

Expected: FAIL，因为方法尚不存在。

### Task 3: 实现同步队列优化

**Files:**
- Modify: `jcp/internal/services/screening_sync_service.go`
- Modify: `jcp/internal/services/screening_store.go`

**Step 1: Write minimal implementation**

实现：
- 候选股票列表先应用 scope 与手动 `LimitStocks`
- 通过 store 反查缺失目标交易日的股票，生成真实同步队列
- checkpoint 只针对该队列生效
- `filterBarsForSync` 改为基于每只股票自己的本地最新交易日过滤

**Step 2: Run focused tests**

Run: `go test ./internal/services -run 'TestScreeningSync|TestScreeningStore' -count=1`

Expected: PASS

### Task 4: 完整验证

**Files:**
- Modify: `jcp/internal/services/screening_sync_service.go`
- Modify: `jcp/internal/services/screening_store.go`
- Modify: `jcp/internal/services/screening_sync_service_test.go`
- Modify: `jcp/internal/services/screening_store_test.go`

**Step 1: Run verification**

Run: `go test ./internal/services/... -count=1`

Expected: PASS

**Step 2: Commit**

```bash
git add docs/plans/2026-03-19-screening-sync-missing-queue.md jcp/internal/services/screening_store.go jcp/internal/services/screening_store_test.go jcp/internal/services/screening_sync_service.go jcp/internal/services/screening_sync_service_test.go
git commit -m "feat: optimize screening sync missing queue"
```
