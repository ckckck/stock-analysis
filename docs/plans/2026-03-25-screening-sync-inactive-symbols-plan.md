# Screening Sync Inactive Symbols Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让 AI 筛选同步排除非活跃股票，并在日线取数异常时具备单源和单票级容错，避免整轮同步被单只股票打断。

**Architecture:** 同步入口先过滤 `IsActive == false` 的候选股票；日线取数仍保持 `Baostock -> Sina` 两级链路，但补上 Baostock 空字段容错和新的 Sina 回退 URL；同步主循环把单票失败降级为跳过并记录事件，而不是立即返回失败。这样可以同时解决退市票、旧接口 456 和单票异常拖垮整轮的问题。

**Tech Stack:** Go, Wails, SQLite, `go test`

---

### Task 1: 为非活跃股票过滤写失败测试

**Files:**
- Modify: `jcp/internal/services/screening_sync_service_test.go`

**Step 1: Write the failing test**

新增一个测试，构造 1 只 `IsActive=false` 和 1 只 `IsActive=true` 的股票源，断言同步只会对活跃股票调用日线取数。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run TestScreeningSyncSkipsInactiveStocks`
Expected: FAIL，因为当前同步仍会尝试处理非活跃股票。

**Step 3: Write minimal implementation**

在 `SyncWithOptions()` 中构建同步队列时过滤 `!stock.IsActive` 的个股。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run TestScreeningSyncSkipsInactiveStocks`
Expected: PASS

### Task 2: 为 Baostock 空字段容错写失败测试

**Files:**
- Modify: `jcp/internal/services/market_service_test.go`
- Modify: `jcp/internal/services/baostock_daily_bar_source.go`

**Step 1: Write the failing test**

为 `parseBaoStockKLines()` 增加测试：`volume`/`amount` 为空字符串时应解析为 `0`，而不是报错。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run TestParseBaoStockKLinesAllowsEmptyVolumeAndAmount`
Expected: FAIL，因为当前会返回 `parse volume` 错误。

**Step 3: Write minimal implementation**

只对 `volume`、`amount` 空字符串做零值兼容。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run TestParseBaoStockKLinesAllowsEmptyVolumeAndAmount`
Expected: PASS

### Task 3: 为单票失败不中断整轮写失败测试

**Files:**
- Modify: `jcp/internal/services/screening_sync_service_test.go`
- Modify: `jcp/internal/services/screening_sync_service.go`

**Step 1: Write the failing test**

新增测试：第一只股票取数失败、第二只成功时，同步最终状态仍为完成，且成功股票被写入。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run TestScreeningSyncContinuesAfterSingleSymbolFailure`
Expected: FAIL，因为当前逻辑会直接 `return status, err`。

**Step 3: Write minimal implementation**

把单票 `getScreeningDailyBars()` 失败从“整轮失败”改为“记录事件并继续”。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run TestScreeningSyncContinuesAfterSingleSymbolFailure`
Expected: PASS

### Task 4: 更新 Sina 日线回退源

**Files:**
- Modify: `jcp/internal/services/market_service.go`
- Test: `jcp/internal/services/market_service_test.go`

**Step 1: Write the failing test**

新增或调整测试，断言日线回退 URL 使用新的 `money.finance.sina.com.cn` 路径，并且 HTTP 非 200 时仍返回带状态码的错误。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run 'TestFetchKLineData|TestGetScreeningDailyBars'`
Expected: 至少有一项 FAIL，说明旧 URL 尚未切换。

**Step 3: Write minimal implementation**

更新 1d K 线回退 URL，不改其它新浪实时接口。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run 'TestFetchKLineData|TestGetScreeningDailyBars'`
Expected: PASS

### Task 5: 文档与全量验证

**Files:**
- Modify: `llmdoc/architecture/system-map.md`
- Modify: `llmdoc/guides/common-workflows.md`
- Modify: `llmdoc/reference/conventions.md`

**Step 1: Update docs**

补充：非活跃股票同步过滤、Baostock 空量容错、单票失败跳过继续。

**Step 2: Run full verification**

Run: `go test ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add jcp/internal/services/*.go jcp/internal/services/*_test.go llmdoc docs/plans/2026-03-25-screening-sync-inactive-symbols-*.md
git commit -m "fix: harden screening sync for inactive symbols"
```
