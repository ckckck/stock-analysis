# Welcome AI Screening Entry Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a welcome-page AI screening entry that checks first-run sync status, asks for sync confirmation inline, then syncs and continues the screening query automatically.

**Architecture:** Keep the backend unchanged and add a frontend orchestration layer in `App.tsx`. `WelcomePage.tsx` becomes a dual-entry screen: AI screening is the primary flow, single-stock search is a secondary expandable tool. First-run sync uses existing `GetScreeningSyncStatus` and `RunScreeningSync`, then hands the pending prompt back into the existing `handleRunScreening` path.

**Tech Stack:** React 18, TypeScript, Wails bindings, existing frontend service layer

---

### Task 1: Add welcome-page AI screening contract

**Files:**
- Modify: `jcp/frontend/src/components/WelcomePage.tsx`
- Modify: `jcp/frontend/src/types.ts`

**Step 1: Write the failing UI contract mentally against current props**

Expected missing behaviors today:
- `WelcomePage` cannot submit an AI screening prompt.
- `WelcomePage` cannot show sync-confirm state.
- `WelcomePage` cannot open the single-stock search as a secondary path.

**Step 2: Extend `WelcomePageProps`**

Add props for:
- current AI prompt
- prompt change callback
- AI submit callback
- sync summary/status fields
- confirm-sync callback
- open-settings callback

Keep `onAddStock` for the single-stock search flow.

**Step 3: Add minimal front-end-only types if needed**

Prefer small local prop types in `WelcomePage.tsx`.
Only add to `types.ts` if multiple files need the same summary shape.

**Step 4: Verify TypeScript compile errors appear only where `WelcomePage` is called**

Run: `cd jcp/frontend && npm run build`
Expected: fail in `App.tsx` because new props are not yet wired.

**Step 5: Commit**

```bash
git add jcp/frontend/src/components/WelcomePage.tsx jcp/frontend/src/types.ts
git commit -m "feat: define welcome ai screening entry contract"
```

### Task 2: Rebuild `WelcomePage` into dual-entry layout

**Files:**
- Modify: `jcp/frontend/src/components/WelcomePage.tsx`

**Step 1: Write the failing interaction checklist**

Current failures:
- main input still searches stock code/name
- no `AI 筛选` primary button
- no `搜索单只股票` secondary button
- no inline sync confirmation card

**Step 2: Implement the welcome-page layout**

Required UI changes:
- main input becomes AI screening natural-language input
- primary button becomes `AI 筛选`
- add a secondary `搜索单只股票` button
- clicking the secondary button reveals the smaller stock search box and current dropdown behavior

**Step 3: Implement inline sync confirmation states**

Render an inline card below the main AI input with:
- summary text
- `去设置`
- `开始同步并继续`
- syncing state
- error state

Do not show this card by default. Show it only when parent state says first-run sync confirmation is needed.

**Step 4: Re-run frontend build**

Run: `cd jcp/frontend && npm run build`
Expected: still fail or remain incomplete until `App.tsx` wiring exists, but `WelcomePage.tsx` itself should be type-correct.

**Step 5: Commit**

```bash
git add jcp/frontend/src/components/WelcomePage.tsx
git commit -m "feat: redesign welcome page for ai screening first run"
```

### Task 3: Add first-run sync orchestration in `App.tsx`

**Files:**
- Modify: `jcp/frontend/src/App.tsx`
- Modify: `jcp/frontend/src/services/configService.ts`

**Step 1: Add app-level state for welcome flow**

Add state for:
- welcome AI prompt
- whether single-stock search is expanded
- whether sync confirmation card is visible
- whether welcome sync is running
- welcome sync error
- cached sync status/config summary if needed

**Step 2: Load screening sync status for welcome flow**

Reuse existing `getConfig()` and `getScreeningSyncStatus()` during startup.
Do not add new backend APIs.

**Step 3: Add a dedicated `handleWelcomeScreeningSubmit`**

Behavior:
- trim prompt
- if no previous sync (`lastTradeDate` empty), show confirm card and do not run query
- if already synced, set `screeningPrompt`, switch to screening mode, and reuse the existing screening execution path

**Step 4: Add `handleWelcomeSyncAndContinue`**

Behavior:
- run `RunScreeningSync`
- refresh sync status
- on success: set `screeningPrompt`, switch mode to `screening`, and execute the pending query
- on failure: keep user on welcome page and show the error

**Step 5: Route settings open from the welcome confirm card**

Reuse existing `setShowSettings(true)`.
No separate modal stack needed.

**Step 6: Wire `WelcomePage` with the new props**

Update the no-watchlist/no-history/no-results branch in `App.tsx` to pass:
- AI prompt state
- submit handler
- sync confirmation visibility/state
- summary/config display data
- single-stock search handlers

**Step 7: Run frontend build**

Run: `cd jcp/frontend && npm run build`
Expected: PASS

**Step 8: Commit**

```bash
git add jcp/frontend/src/App.tsx jcp/frontend/src/services/configService.ts
git commit -m "feat: add welcome ai screening sync flow"
```

### Task 4: Verify screening execution handoff and regressions

**Files:**
- Modify: `jcp/frontend/src/App.tsx` if minor fixes are needed

**Step 1: Check the execution handoff logic**

Confirm:
- first run shows confirmation, not direct screening
- after sync success, prompt is preserved
- screening history refresh still works
- switching to watchlist/screening modes still behaves as before

**Step 2: Check single-stock search regression**

Confirm the secondary search still:
- searches by stock name/code
- shows dropdown
- adds the selected stock to watchlist

**Step 3: Run full verification**

Run:

```bash
cd jcp && go test ./...
cd /Users/caike/3.工具/stock-analysis/.worktrees/ai-screening/jcp && go build ./...
cd /Users/caike/3.工具/stock-analysis/.worktrees/ai-screening/jcp/frontend && npm run build
cd /Users/caike/3.工具/stock-analysis/.worktrees/ai-screening && git diff --check
```

Expected:
- `go test ./...` PASS
- `go build ./...` PASS
- `npm run build` PASS
- `git diff --check` no output

**Step 4: Commit**

```bash
git add jcp/frontend/src/App.tsx
git commit -m "fix: stabilize welcome ai screening handoff"
```

### Task 5: Sync docs after implementation

**Files:**
- Modify: `llmdoc/overview/project-overview.md`
- Modify: `llmdoc/guides/common-workflows.md`
- Modify: `llmdoc/architecture/system-map.md`
- Modify: `llmdoc/index.md`

**Step 1: Document welcome-page AI screening entry**

Add the new first-run AI screening entry point and inline sync confirmation behavior.

**Step 2: Document workflow changes**

Update common workflows to include:
- first-run screening from welcome page
- confirm sync then continue screening
- secondary single-stock search path

**Step 3: Run diff check**

Run: `git diff --check`
Expected: no output

**Step 4: Commit**

```bash
git add llmdoc/overview/project-overview.md llmdoc/guides/common-workflows.md llmdoc/architecture/system-map.md llmdoc/index.md
git commit -m "docs: add welcome ai screening workflow"
```
