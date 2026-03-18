# Repository Migration Design

**Date:** 2026-03-18

**Goal:** Make `/Users/caike/3.工具/stock-analysis` the new primary repository root, while preserving the original `jcp` Git history for future upstream synchronization.

## Confirmed Facts

- The top-level `stock-analysis/` directory is not yet a Git repository.
- `jcp/` is currently an embedded standalone Git repository with its own `.git/`.
- `jcp/` contains uncommitted local changes and one untracked file.
- The embedded `jcp` repository is currently shallow and points to upstream `https://github.com/run-bigpig/jcp.git`.

## Chosen Approach

Use the top-level directory as the new primary Git repository and convert `jcp/` into a normal tracked subdirectory.

Preserve the original `jcp` Git history in a separate hidden mirror under the top-level workspace, rather than keeping a nested `.git/` inside `jcp/`.

## Why This Approach

- It matches the intended ownership model: one main repository for the whole project.
- It keeps top-level docs, workspace metadata, and future tooling under the same repository.
- It avoids Git’s embedded-repository behavior, which would otherwise make `jcp/` awkward to track from the top level.
- It preserves an auditable path back to upstream and a clean mechanism for future fetches.

## Migration Rules

- Do not discard the current `jcp` working tree changes.
- Preserve upstream remote metadata before removing `jcp/.git` from the working tree.
- Keep the preserved upstream history outside the main tracked tree and ignore it in the new main repository.
- Document the upstream sync path so the preserved history remains usable later.

## Expected End State

- `stock-analysis/` is a normal Git repository owned by the user.
- `jcp/` is a regular tracked directory inside that repository.
- Original upstream history is preserved separately under a hidden backup/mirror location.
- The main repository ignores backup/mirror artifacts.
