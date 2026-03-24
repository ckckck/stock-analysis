# Topbar And Screening Workspace Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the topbar's first two icon buttons with text buttons and tighten the AI screening workspace layout for better readability and alignment.

**Architecture:** Keep all interaction behavior unchanged and limit the work to front-end presentation. Update the topbar button markup in `App.tsx` and restack the workspace controls in `ScreeningWorkspace.tsx` so the helper text, textarea, result preset selector, primary action, and SQL button follow a cleaner visual rhythm.

**Tech Stack:** React, TypeScript, Tailwind CSS, Wails

---

### Task 1: Topbar button labels

**Files:**
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: Introduce the failing UI change**

- Replace the first two topbar icon buttons with text-button markup for `龙虎榜` and `全网热点`.

**Step 2: Run build to verify the file still compiles**

Run: `npm --prefix frontend run build`

**Step 3: Adjust spacing and hover states**

- Use horizontal padding and keep the existing panel/border styling.
- Leave theme and settings buttons untouched.

**Step 4: Run build again**

Run: `npm --prefix frontend run build`

### Task 2: Screening workspace alignment

**Files:**
- Modify: `jcp/frontend/src/components/ScreeningWorkspace.tsx`

**Step 1: Introduce the new layout**

- Make the helper text gray.
- Keep the textarea full-width.
- Put preset select and main action on one aligned row.
- Move `查看 SQL` into a dedicated aligned row below, only when SQL exists.

**Step 2: Run build to verify the file still compiles**

Run: `npm --prefix frontend run build`

**Step 3: Refine spacing**

- Tighten vertical spacing between helper text, textarea, control row, and SQL button row.
- Ensure the main button and selector align cleanly.

**Step 4: Run build again**

Run: `npm --prefix frontend run build`

### Task 3: Final verification

**Files:**
- Modify: `jcp/frontend/src/App.tsx`
- Modify: `jcp/frontend/src/components/ScreeningWorkspace.tsx`

**Step 1: Run full frontend build**

Run: `npm --prefix frontend run build`

**Step 2: Spot-check requirements**

- Topbar first two buttons show text labels.
- Theme and settings controls remain unchanged.
- Workspace helper text is gray.
- Workspace controls look aligned and compact.
