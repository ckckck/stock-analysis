# Sync Entry And Shared Dialog Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a topbar sync-status action, refactor the existing screening sync confirmation into a shared dialog with `sync-only` and `screening` modes, restore correct history-SQL rerun behavior, and keep partial sync progress visible after cancel/resume.

**Architecture:** Keep sync data and event subscriptions in the existing front-end flow, then extract pure helper functions for sync-button state and screening action copy so they can be tested independently. Refactor `App.tsx` to track whether the current screening workspace still matches a historical prompt baseline, reuse history SQL only in that exact case, and refresh sync status after cancel so the topbar action reflects checkpoint progress before the next resume run.

**Tech Stack:** React, TypeScript, Tailwind CSS, Wails, Vitest

---

### Task 1: Add a minimal frontend test harness

**Files:**
- Modify: `jcp/frontend/package.json`
- Create: `jcp/frontend/vitest.config.ts`
- Create: `jcp/frontend/src/utils/screeningSync.test.ts`

**Step 1: Write the failing test**

- Add tests for a new helper that resolves the topbar sync button state:
  - returns `completed` with disabled `true` and label `已同步` when synced count reaches total
  - returns `partial` with label `立即同步` for partial sync
  - returns `empty` with label `立即同步` for zero sync
- Add a test that the screening workspace primary label resolver always returns `开始筛选` when not loading.

**Step 2: Run test to verify it fails**

Run: `npm --prefix jcp/frontend run test -- screeningSync`
Expected: FAIL because the helper functions and test script do not exist yet.

**Step 3: Write minimal implementation**

- Add the test script and Vitest config.
- Extract small pure helpers into `jcp/frontend/src/utils/screeningSync.ts` or a nearby utility file so the tests can target stable logic instead of JSX.

**Step 4: Run test to verify it passes**

Run: `npm --prefix jcp/frontend run test -- screeningSync`
Expected: PASS

### Task 2: Refactor the shared sync dialog

**Files:**
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: Write the failing test**

- Extend the helper tests to cover dialog copy mode helpers if they are extracted:
  - `sync-only` title/description/button label
  - `screening` title/description/button label

**Step 2: Run test to verify it fails**

Run: `npm --prefix jcp/frontend run test -- screeningSync`
Expected: FAIL because the new mode helpers are not implemented.

**Step 3: Write minimal implementation**

- Extract the current `ScreeningConfirmDialog` into a shared sync dialog structure in `App.tsx`.
- Add mode-specific title, description, content blocks, and primary action labels.
- Ensure the header text block uses explicit left-aligned classes.
- Keep sync progress, cancel, and settings actions shared.

**Step 4: Run test to verify it passes**

Run: `npm --prefix jcp/frontend run test -- screeningSync`
Expected: PASS

### Task 3: Add the topbar sync button and remove obsolete settings entry

**Files:**
- Modify: `jcp/frontend/src/App.tsx`
- Modify: `jcp/frontend/src/components/SettingsDialog.tsx`

**Step 1: Write the failing test**

- Extend the helper tests to cover sync button secondary text formatting and partial-progress state:
  - `(5003/5003)` for completed
  - `(1/5003)` for partial
  - `(--/--)` when total is unknown
  - `立即同步` remains the main label for partial progress
  - loading/disabled handling differs between completed and in-progress states

**Step 2: Run test to verify it fails**

Run: `npm --prefix jcp/frontend run test -- screeningSync`
Expected: FAIL because the formatted display helper does not support all cases yet.

**Step 3: Write minimal implementation**

- Add the new topbar sync button before `龙虎榜`.
- Keep the button single-line and aligned with neighboring controls.
- Disable it and switch label to `已同步` only when the market is fully synced.
- Keep the main label as `立即同步` for partial progress, while the right-side count reflects live synced stock totals.
- Open the shared dialog in `sync-only` mode when clicked.
- Remove the `软件更新` tab from `SettingsDialog` navigation.
- After canceling sync, immediately refresh status from `GetScreeningSyncStatus()` so checkpoint progress is reflected before the next resume.

**Step 4: Run test to verify it passes**

Run: `npm --prefix jcp/frontend run test -- screeningSync`
Expected: PASS

### Task 4: Restore history rerun semantics in screening workspace

**Files:**
- Modify: `jcp/frontend/src/components/ScreeningWorkspace.tsx`
- Modify: `jcp/frontend/src/App.tsx`
- Modify: `jcp/frontend/src/utils/screeningSync.ts`

**Step 1: Write the failing test**

- Add tests for the screening action label resolver:
  - returns `根据历史筛选方式重新筛选` when the current prompt still matches the historical prompt baseline
  - returns `开始筛选` after the user edits the prompt
  - keeps the loading label while a run is active

**Step 2: Run test to verify it fails**

Run: `npm --prefix jcp/frontend run test -- screeningSync`
Expected: FAIL because the resolver does not yet distinguish untouched historical reruns from edited prompts.

**Step 3: Write minimal implementation**

- Track a historical prompt baseline in `App.tsx` when loading a history run.
- Reuse historical SQL only when the current prompt still matches that baseline.
- Switch the workspace button copy back to `根据历史筛选方式重新筛选` in that case; otherwise show `开始筛选`.
- When the user edits the prompt, immediately clear the history-rerun baseline so the next run regenerates SQL.

**Step 4: Run test to verify it passes**

Run: `npm --prefix jcp/frontend run test -- screeningSync`
Expected: PASS

### Task 5: Full verification

**Files:**
- Modify: `jcp/frontend/src/App.tsx`
- Modify: `jcp/frontend/src/components/SettingsDialog.tsx`
- Modify: `jcp/frontend/src/components/ScreeningWorkspace.tsx`
- Modify: `jcp/frontend/src/utils/screeningSync.ts`

**Step 1: Run full targeted tests**

Run: `npm --prefix jcp/frontend run test -- screeningSync`

**Step 2: Run frontend build**

Run: `npm --prefix jcp/frontend run build`

**Step 3: Spot-check requirements**

- 顶部同步按钮在完成、部分完成、未同步状态下展示正确文案、颜色和禁用态。
- “已同步”状态不可点击；部分完成状态保持 `立即同步` 主文案但数字实时更新。
- 同步取消后按钮会回刷到最新 checkpoint 进度，再次点击走断点续传。
- 顶部按钮点击只打开同步弹窗，不触发筛选。
- AI 筛选流程仍能进入“同步后筛选”模式。
- 历史筛选结果仅在输入未改动时复用历史 SQL，并显示 `根据历史筛选方式重新筛选`。
- 两个弹窗头部说明区左对齐。
- 设置面板不再显示“软件更新”。
- 普通筛选主按钮为“开始筛选”。
