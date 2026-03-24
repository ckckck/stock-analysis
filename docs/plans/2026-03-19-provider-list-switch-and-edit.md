# Provider List Switch And Edit Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让设置页 AI 模型列表支持点击卡片直接切换当前默认模型，并新增独立“编辑”按钮进入编辑页。

**Architecture:** 复用现有 `isDefault` 状态作为“当前使用中的模型”展示。列表卡片点击改为触发 `set default`，右侧按钮区新增独立编辑入口，进入当前已有编辑视图。

**Tech Stack:** React 18, TypeScript, Wails, Tailwind 风格类名

---

### Task 1: 调整 Provider 列表交互

**Files:**
- Modify: `jcp/frontend/src/components/SettingsDialog.tsx`

**Step 1: 将列表卡片点击行为从“进入编辑”改为“设为默认模型”。**

**Step 2: 为列表项新增独立 `编辑` 按钮，并复用现有编辑视图。**

**Step 3: 保持复制、设为默认、删除按钮逻辑不变，避免按钮点击冒泡触发卡片切换。**

### Task 2: 验证与文档

**Files:**
- Modify: `llmdoc/guides/common-workflows.md`
- Modify: `llmdoc/architecture/system-map.md`

**Step 1: 运行 `npm --prefix frontend run build`。**

**Step 2: 运行 `~/go/bin/wails build`。**

**Step 3: 更新 llmdoc，说明模型列表卡片现在用于切换当前模型，编辑改为独立按钮。**
