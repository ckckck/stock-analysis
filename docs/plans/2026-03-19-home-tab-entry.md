# Home Tab Entry Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为顶部模式切换增加独立“首页”入口，让用户在已有自选股或筛选历史时也能返回欢迎页搜索入口。

**Architecture:** 在前端新增独立 `home` 屏幕模式，把 `WelcomePage` 从“仅空状态显示”改为显式路由页面。顶部模式切换改成 `首页 / 自选 / AI 筛选` 三态，保持现有 AI 筛选确认和同步链路不变。

**Tech Stack:** React 18, TypeScript, Wails, Tailwind 风格类名

---

### Task 1: 增加首页模式

**Files:**
- Modify: `jcp/frontend/src/types.ts`
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: 更新 `AppScreenMode`，加入 `home`。**

**Step 2: 调整 `App.tsx` 的首页显示条件，优先按 `screenMode === 'home'` 渲染欢迎页。**

**Step 3: 保持欢迎页发起筛选时仍走原有确认弹框与同步流程。**

### Task 2: 调整顶部模式切换

**Files:**
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: 在顶部按钮组左侧增加“首页”。**

**Step 2: `首页 / 自选 / AI 筛选` 分别切换到 `home / watchlist / screening`。**

**Step 3: 保持现有选中态与样式一致。**

### Task 3: 验证与文档

**Files:**
- Modify: `llmdoc/guides/common-workflows.md`
- Modify: `llmdoc/architecture/system-map.md`

**Step 1: 运行 `npm --prefix frontend run build`。**

**Step 2: 运行 `~/go/bin/wails build`。**

**Step 3: 同步 llmdoc，记录首页入口与三态切换。**
