# Windows Artifact Rename Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 Windows 构建产物名从 `jcp.exe` 改为 `散牛盘.exe`，并同步更新和谐 Windows 部署技能与部署真值。

**Architecture:** 保持应用展示名、内部模块名和数据目录不变，只修改 Wails 的 `outputfilename` 与共享部署文档里的产物路径。这样不会影响 `%APPDATA%/jcp`、Go module 或历史数据目录，只调整 Windows 对外可见的 exe 文件名。

**Tech Stack:** Wails v2, JSON 配置, Markdown 技能文档

---

### Task 1: 更新 Wails Windows 产物名

**Files:**
- Modify: `jcp/wails.json`

**Step 1: 写一个最小验证目标**

确认 `outputfilename` 当前为 `jcp`，修改后应为 `散牛盘`。

**Step 2: 修改配置**

将 `jcp/wails.json` 中的 `outputfilename` 从 `jcp` 改为 `散牛盘`。

**Step 3: 运行验证**

Run: `rg -n '"outputfilename"' /Users/caike/3.工具/stock-analysis/jcp/wails.json`
Expected: 输出包含 `"outputfilename": "散牛盘"`

### Task 2: 更新共享部署技能

**Files:**
- Modify: `/Users/caike/.config/shared-skills/deploying-stock-tool-to-hexie-windows/SKILL.md`

**Step 1: 修改产物与部署路径**

把技能中的：
- `~/jcp-build/jcp/build/bin/jcp.exe`
- `/mnt/d/股票工具/jcp.exe`
- `/mnt/d/股票工具/jcp_new.exe`

分别改为：
- `~/jcp-build/jcp/build/bin/散牛盘.exe`
- `/mnt/d/股票工具/散牛盘.exe`
- `/mnt/d/股票工具/散牛盘_new.exe`

**Step 2: 运行验证**

Run: `rg -n 'jcp\\.exe|jcp_new\\.exe|散牛盘\\.exe|散牛盘_new\\.exe' /Users/caike/.config/shared-skills/deploying-stock-tool-to-hexie-windows/SKILL.md`
Expected: 只保留 `散牛盘.exe` 和 `散牛盘_new.exe`

### Task 3: 更新部署真值文档

**Files:**
- Modify: `/Users/caike/.config/shared-skills/deploying-stock-tool-to-hexie-windows/references/deploy-config.md`

**Step 1: 修改 Windows 侧 exe 文件名**

把 `exe 文件名` 从 `jcp.exe` 改为 `散牛盘.exe`。

**Step 2: 运行验证**

Run: `rg -n 'exe 文件名|jcp\\.exe|散牛盘\\.exe' /Users/caike/.config/shared-skills/deploying-stock-tool-to-hexie-windows/references/deploy-config.md`
Expected: `exe 文件名` 对应 `散牛盘.exe`

### Task 4: 整体验证

**Files:**
- Verify: `jcp/wails.json`
- Verify: `/Users/caike/.config/shared-skills/deploying-stock-tool-to-hexie-windows/SKILL.md`
- Verify: `/Users/caike/.config/shared-skills/deploying-stock-tool-to-hexie-windows/references/deploy-config.md`

**Step 1: 运行全局验证**

Run: `rg -n 'outputfilename|jcp\\.exe|jcp_new\\.exe|散牛盘\\.exe|散牛盘_new\\.exe' /Users/caike/3.工具/stock-analysis/jcp/wails.json /Users/caike/.config/shared-skills/deploying-stock-tool-to-hexie-windows/SKILL.md /Users/caike/.config/shared-skills/deploying-stock-tool-to-hexie-windows/references/deploy-config.md`
Expected: `wails.json` 输出 `散牛盘`，共享技能与真值不再引用 `jcp.exe` / `jcp_new.exe`

**Step 2: 提交**

```bash
git -C /Users/caike/3.工具/stock-analysis add /Users/caike/3.工具/stock-analysis/jcp/wails.json /Users/caike/3.工具/stock-analysis/docs/plans/2026-03-24-windows-artifact-rename-design.md
git -C /Users/caike/3.工具/stock-analysis commit -m "chore: rename windows artifact to 散牛盘"
```
