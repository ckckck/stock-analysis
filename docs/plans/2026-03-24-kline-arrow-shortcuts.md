# Kline Arrow Shortcuts Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 支持方向键直接控制当前 K 线图：上下缩放、左右平移，并持久化缩放级别。

**Architecture:** 后端在 `config.layout` 中新增 `klineZoomPercent` 默认值；前端把方向键识别和缩放/平移步进抽成纯函数，在 `App.tsx` 监听全局方向键并通过 `ref` 调用 `StockChartLW` 的图表操作方法。`StockChartLW` 使用 lightweight-charts 的 `timeScale().getVisibleLogicalRange()/setVisibleLogicalRange()` 在 K 线层做缩放和平移。

**Tech Stack:** Go, Wails, React 18, TypeScript, lightweight-charts, Vitest

---

### Task 1: 写前端方向键纯函数测试

**Files:**
- Create: `jcp/frontend/src/utils/klineKeyboard.ts`
- Create: `jcp/frontend/src/utils/klineKeyboard.test.ts`

**Step 1: Write the failing test**

- 识别 `ArrowUp/ArrowDown/ArrowLeft/ArrowRight`
- 排除输入区目标元素
- 验证缩放百分比步进和边界
- 验证平移逻辑范围步进

**Step 2: Run test to verify it fails**

Run: `npm --prefix jcp/frontend run test -- src/utils/klineKeyboard.test.ts`

**Step 3: Write minimal implementation**

- 实现动作识别
- 实现输入区过滤
- 实现缩放与平移辅助函数

**Step 4: Run test to verify it passes**

Run: `npm --prefix jcp/frontend run test -- src/utils/klineKeyboard.test.ts`

### Task 2: 写后端布局默认值测试

**Files:**
- Modify: `jcp/internal/models/config.go`
- Modify: `jcp/internal/services/config_service.go`
- Modify: `jcp/internal/services/config_service_test.go`
- Modify: `jcp/frontend/wailsjs/go/models.ts`

**Step 1: Write the failing test**

- 旧配置只带已有 layout 字段，不写 `klineZoomPercent`
- 断言重新加载后默认值是 `100`

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run TestConfigServiceAddsLayoutKlineZoomDefaultForLegacyConfig -count=1`

**Step 3: Write minimal implementation**

- 在 `LayoutConfig` 增加 `klineZoomPercent`
- 默认值设为 `100`
- legacy 配置补默认值
- 同步前端 Wails model

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run TestConfigServiceAddsLayoutKlineZoomDefaultForLegacyConfig -count=1`

### Task 3: 接入图表 ref 和方向键分发

**Files:**
- Modify: `jcp/frontend/src/components/StockChartLW.tsx`
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: Implement chart imperative API**

- `StockChartLW` 改为 `forwardRef`
- 暴露 `zoomIn / zoomOut / panLeft / panRight`
- 用当前可视逻辑范围操作时间轴

**Step 2: Persist zoom level**

- `App.tsx` 增加 `klineZoomPercent` state
- 启动恢复配置
- 保存布局时一起写入配置

**Step 3: Add global keyboard handling**

- 过滤输入区
- 保留原有 `cmd/ctrl +/-`
- 方向键调用图表 ref

**Step 4: Run focused verification**

Run:
- `npm --prefix jcp/frontend run test -- src/utils/klineKeyboard.test.ts`
- `npm --prefix jcp/frontend run build`

### Task 4: 全量验证

**Files:**
- Verify only

**Step 1: Run Go tests**

Run: `go test ./...`

**Step 2: Run frontend tests**

Run: `npm --prefix jcp/frontend run test`

**Step 3: Run frontend build**

Run: `npm --prefix jcp/frontend run build`

