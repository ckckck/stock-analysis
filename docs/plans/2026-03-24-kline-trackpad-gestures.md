# Kline Trackpad Gestures Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 恢复 K 线图区域内触摸板双指左右平移和捏合缩放，同时保留现有左右方向键的自定义步进行为。

**Architecture:** 继续让方向键走 `App.tsx -> StockChartLW ref` 这条自定义逻辑，不改现有平移步进。触摸板手势则恢复 lightweight-charts 默认交互，四个周期统一开启 `handleScroll` 与 `handleScale`，避免分时 `1m` 被禁用。用一个最小测试锁定“图表交互选项不再因为 intraday 被关闭”。

**Tech Stack:** React, TypeScript, Vitest, lightweight-charts, Wails

---

### Task 1: 锁定图表交互策略

**Files:**
- Modify: `jcp/frontend/src/utils/klineKeyboard.ts`
- Modify: `jcp/frontend/src/utils/klineKeyboard.test.ts`

**Step 1: Write the failing test**

给 `klineKeyboard` 增加一个小的纯函数，用来决定图表是否启用默认手势；测试要求四个周期都返回开启。

**Step 2: Run test to verify it fails**

Run: `npm --prefix jcp/frontend run test -- src/utils/klineKeyboard.test.ts`
Expected: FAIL，提示新函数不存在或断言失败。

**Step 3: Write minimal implementation**

实现一个最小纯函数，固定返回：
- `handleScroll: true`
- `handleScale: true`

**Step 4: Run test to verify it passes**

Run: `npm --prefix jcp/frontend run test -- src/utils/klineKeyboard.test.ts`
Expected: PASS

### Task 2: 接回图表默认手势

**Files:**
- Modify: `jcp/frontend/src/components/StockChartLW.tsx`

**Step 1: Replace the intraday interaction toggle**

把周期切换时的 `handleScroll: !isIntraday` / `handleScale: !isIntraday` 改成统一使用纯函数返回值，让 `1m` 分时也保留图表库默认平移和缩放。

**Step 2: Keep keyboard behavior unchanged**

不要改：
- `panLeft/panRight`
- `getNextEffectiveZoomPercent`
- `App.tsx` 里的方向键监听

**Step 3: Run frontend verification**

Run: `npm --prefix jcp/frontend run test -- src/utils/klineKeyboard.test.ts`
Expected: PASS

Run: `npm --prefix jcp/frontend run build`
Expected: PASS

### Task 3: 手动验收与打包

**Files:**
- Modify: none

**Step 1: Verify expected behavior**

手动验收目标：
- 触摸板双指左右移动只在 K 线图区域生效
- 触摸板捏合缩放只在 K 线图区域生效
- 左右方向键仍按当前步进规则移动

**Step 2: Package app when requested**

Run: `cd jcp && ~/go/bin/wails build`
Expected: PASS，生成新的 `散牛盘.app`
