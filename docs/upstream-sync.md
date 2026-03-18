# Upstream Sync Notes

This workspace now uses the top-level `stock-analysis/` directory as the main Git repository.

The original `jcp` repository metadata has been preserved in two forms:

- `.upstreams/jcp-origin.git`: bare mirror for fetching upstream history
- `.upstreams/jcp-worktree.gitdir`: original moved Git directory from `jcp/.git`

## Current Ownership Model

- Main development repository: top-level `stock-analysis/`
- Upstream source of truth for imported code: `https://github.com/run-bigpig/jcp.git`
- Imported project directory inside main repo: `jcp/`

## Useful Commands

Check the preserved upstream mirror remotes:

```bash
git --git-dir=.upstreams/jcp-origin.git remote -v
```

Fetch the latest upstream changes into the preserved mirror:

```bash
git --git-dir=.upstreams/jcp-origin.git fetch --prune --tags origin
```

Inspect the original moved repository metadata against the current `jcp/` worktree:

```bash
git --git-dir=.upstreams/jcp-worktree.gitdir --work-tree=jcp status
```

Fetch upstream refs into the preserved moved Git directory:

```bash
git --git-dir=.upstreams/jcp-worktree.gitdir --work-tree=jcp fetch --prune --tags origin
```

Compare the current `jcp/` tree against upstream `master`:

```bash
git --git-dir=.upstreams/jcp-worktree.gitdir --work-tree=jcp diff origin/master
```

## Notes

- `.upstreams/` is intentionally ignored by the new main repository.
- The patch file `.upstreams/jcp-working-tree.patch` preserves the tracked-but-uncommitted state that existed before migration.
- `.upstreams/jcp-untracked/` preserves the untracked files that existed before migration.
- If you later want structured upstream merges, the next clean step is to adopt a documented `git subtree` workflow from the top-level repository.
