# Repository Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Convert the top-level workspace into the new main repository without losing the original `jcp` Git history or local modifications.

**Architecture:** Preserve the existing `jcp` repository state first, then detach `jcp/` from its embedded Git metadata and initialize a new repository at the workspace root. Keep upstream history in an ignored mirror directory so future synchronization remains possible without nested repository conflicts.

**Tech Stack:** Git, shell tooling, markdown documentation

---

### Task 1: Capture Current Repository State

**Files:**
- Create: `.upstreams/`
- Create: `.upstreams/jcp-working-tree.patch`
- Create: `.upstreams/jcp-untracked/`
- Create: `.upstreams/jcp-origin.git/`

**Step 1: Inspect the embedded repository**

Run: `git -C jcp status --short --branch`
Expected: embedded `jcp` repository with local modifications.

**Step 2: Preserve local modifications**

Run: `git -C jcp diff --binary > .upstreams/jcp-working-tree.patch`
Expected: a patch file containing tracked but uncommitted changes.

**Step 3: Preserve untracked files**

Run: copy untracked files from `jcp/` into `.upstreams/jcp-untracked/`
Expected: untracked content retained outside the embedded repository.

**Step 4: Preserve upstream Git history**

Run: `git clone --mirror jcp .upstreams/jcp-origin.git`
Expected: a reusable mirror that retains remotes, refs, and object history.

### Task 2: Detach `jcp` From Embedded Git

**Files:**
- Delete logically from worktree role: `jcp/.git`
- Create: `.upstreams/README.md`

**Step 1: Remove embedded repository metadata from the working tree**

Run: move or remove `jcp/.git` after the mirror is confirmed valid.
Expected: `jcp/` becomes an ordinary directory.

**Step 2: Document upstream access**

Write a short README describing how to fetch upstream changes from `.upstreams/jcp-origin.git`.

### Task 3: Initialize the New Main Repository

**Files:**
- Create: `.git`
- Create: `.gitignore`

**Step 1: Initialize the top-level repository**

Run: `git init`
Expected: top-level repository created at `stock-analysis/`.

**Step 2: Ignore backup artifacts**

Write `.gitignore` entries for `.upstreams/` and any other local-only metadata that should not ship to GitHub.

**Step 3: Stage the intended project tree**

Run: `git add AGENTS.md llmdoc docs jcp .gitignore`
Expected: main repository tracks the actual project contents, not the backup mirror.

### Task 4: Verify and Record the New Layout

**Files:**
- Modify: `llmdoc/index.md` or related docs if needed

**Step 1: Verify top-level repository state**

Run: `git status --short`
Expected: staged files under the top-level repository and no embedded Git warning for `jcp/`.

**Step 2: Verify backup usability**

Run: `git --git-dir=.upstreams/jcp-origin.git remote -v`
Expected: upstream `origin` still present in the preserved mirror.

**Step 3: Summarize follow-up actions**

Record how to create the GitHub repository, add the user’s remote, and push the new main repository.
