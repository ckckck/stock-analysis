# 同步弹框最小化与筛选确认收敛 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让 AI 筛选确认弹框只保留“开始筛选”，并为顶部“立即同步”的同步弹框增加最小化到右下角的能力。

**Architecture:** 保持 `SyncActionDialog` 作为统一弹框组件，只在 `sync-only` 模式下增加最小化入口；最小化后的右下角悬浮卡片由 `App.tsx` 承担，状态判断抽到 `screeningSync.ts` 做纯函数测试。

**Tech Stack:** React 18、TypeScript、Vitest、Tailwind、Wails

---

### Task 1: 先锁定文案变化

**Files:**
- Modify: `jcp/frontend/src/utils/screeningSync.test.ts`
- Modify: `jcp/frontend/src/utils/screeningSync.ts`

**Step 1: 写失败测试**

- 修改 `resolveSyncDialogCopy('screening')` 相关断言：
  - 标题改为“确认后开始筛选”
  - 描述改为“本次会直接基于当前已同步数据执行 AI 筛选。”
  - `secondaryActionLabel` 改为 `undefined`

**Step 2: 运行测试确认失败**

Run: `npm test`

**Step 3: 最小实现**

- 更新 `resolveSyncDialogCopy('screening')`

**Step 4: 运行测试确认通过**

Run: `npm test`

### Task 2: 为同步最小化状态写纯函数测试

**Files:**
- Modify: `jcp/frontend/src/utils/screeningSync.test.ts`
- Modify: `jcp/frontend/src/utils/screeningSync.ts`

**Step 1: 写失败测试**

- 新增一个“同步最小化卡片状态”纯函数测试，覆盖：
  - 只有 `visible && minimized` 时显示
  - `running/completed/failed/canceled` 文案映射
  - 百分比和 `completed/total` 正常透传

**Step 2: 运行测试确认失败**

Run: `npm test`

**Step 3: 最小实现**

- 在 `screeningSync.ts` 中新增状态解析函数和类型

**Step 4: 运行测试确认通过**

Run: `npm test`

### Task 3: 在 App 中接线最小化状态

**Files:**
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: 写代码前对照设计检查状态位**

- 增加：
  - `syncOnlyDialogMinimized`
  - 打开时重置
  - 关闭时清理
  - 最小化/恢复 handler

**Step 2: 修改 `SyncActionDialog` 头部**

- 仅 `sync-only` 模式显示最小化按钮
- 保留关闭按钮

**Step 3: 修改 `screening` 模式底部按钮**

- 由于 copy 中 `secondaryActionLabel` 已为空，确认不再渲染左侧“开始同步”

**Step 4: 增加右下角悬浮卡片**

- 条件渲染右下角卡片
- 展示：
  - 标题
  - 百分比
  - 完成数/总数
  - 最近状态
  - 恢复按钮

### Task 4: 处理同步完成后的最小化收尾

**Files:**
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: 调整 `handleStartSyncOnly` 成功分支**

- 未最小化：沿用自动关闭
- 已最小化：保留悬浮卡片，不自动关闭

**Step 2: 校验取消/失败分支**

- 保持卡片可恢复查看

**Step 3: 统一关闭时清理状态**

- `closeSyncOnlyDialog` 一次性清理 `visible/start/minimized/error`

### Task 5: 最终验证

**Files:**
- Modify: `jcp/frontend/src/App.tsx`
- Modify: `jcp/frontend/src/utils/screeningSync.ts`
- Modify: `jcp/frontend/src/utils/screeningSync.test.ts`

**Step 1: 跑前端测试**

Run: `npm test`

Expected:
- 相关测试全部通过

**Step 2: 跑前端构建**

Run: `npm run build`

Expected:
- `vite build` 成功

**Step 3: 自查交互**

- `screening` 弹框只剩“开始筛选”
- `sync-only` 弹框右上角有最小化按钮
- 最小化后右下角显示同步进度卡片
- 点击恢复后回到完整弹框
