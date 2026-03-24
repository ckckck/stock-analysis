# Screening Streaming Output Follow Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 AI 筛选进度中的流式输出框增加固定可视高度、内部滚动和“默认追底 / 用户滚动后停止追底”的交互。

**Architecture:** 在 `ScreeningResultList` 内新增一个小型流式输出组件，统一用于“思考中”和“生成 SQL”步骤。组件通过 `ref + scroll` 判断用户是否离开底部，并只在自动追底状态下随着新内容滚到最新位置。

**Tech Stack:** React, TypeScript, Tailwind CSS

---

### Task 1: 实现流式输出框

**Files:**
- Modify: `jcp/frontend/src/components/ScreeningResultList.tsx`

**Step 1: 写最小实现**

- 抽一个局部 `StreamingOutputBox`
- 固定最大高度并启用内部滚动
- 新增自动追底状态和滚动监听
- 两个流式步骤统一复用

**Step 2: 构建验证**

Run: `npm --prefix frontend run build`

Expected: PASS
