# AI 筛选连续思考摘要 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让 AI 筛选在生成 SQL 之前先实时显示连续自然语言思考摘要，同时调整结果区头部布局，并在空结果时明确反馈“没有符合条件的结果”。

**Architecture:** 在 `ScreeningQueryService` 中保留 `reasoning` 进度阶段，但把提示词改为连续自然语言思考摘要，再进入现有 SQL 生成链路。前端复用现有进度卡显示思考摘要或 SQL 草稿，同时调整 `ScreeningResultList` 的头部顺序与空结果文案，`App.tsx` 继续在有结果时默认选中第一只股票。

**Tech Stack:** Go, Wails v2, React, TypeScript

---

### Task 1: 后端连续思考摘要提示词

**Files:**
- Modify: `jcp/internal/services/screening_query_service.go`
- Modify: `jcp/app.go`
- Modify: `jcp/internal/adk/model_factory.go`
- Test: `jcp/internal/services/screening_query_service_test.go`

**Step 1: Write the failing test**

- 修改 reasoning prompt 测试，断言 prompt 要求连续自然语言摘要，不再要求“3 到 5 条 / 每行一句”。
- 保留 `reasoning` 阶段先于 `generate_sql`、并支持流式增长的测试。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run 'TestScreeningReasoningPromptIncludesConstraints|TestScreeningQueryRunReportsReasoningBeforeGenerateSQL|TestScreeningQueryRunStreamsReasoningText' -count=1`

Expected: FAIL，提示 reasoning prompt 约束与新设计不一致。

**Step 3: Write minimal implementation**

- 更新 reasoning prompt，要求连续自然语言说明、不要列表、不要 SQL。
- 保持现有 `reasoning` 流式接口和事件链不变。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run 'TestScreeningReasoningPromptIncludesConstraints|TestScreeningQueryRunReportsReasoningBeforeGenerateSQL|TestScreeningQueryRunStreamsReasoningText|TestScreeningQueryRunReportsStreamingGenerateSQLProgressEvents' -count=1`

### Task 2: 结果区头部与空结果状态

**Files:**
- Modify: `jcp/frontend/src/components/ScreeningResultList.tsx`
- Modify: `jcp/frontend/src/App.tsx`

**Step 1: Write the failing test**

- 以 TypeScript 构建为约束，先让 `ScreeningResultList` 头部结构和空结果文案变更暴露编译缺口。

**Step 2: Run test to verify it fails**

Run: `npm --prefix jcp/frontend run build`

Expected: FAIL，直到结果区头部结构和空结果状态接线完成。

**Step 3: Write minimal implementation**

- 把页签切换按钮移动到标题上方。
- 空结果时左侧显示“没有符合条件的结果”。
- 有结果时继续默认选中第一只；无结果时不保留旧的筛选结果提示文案。

**Step 4: Run test to verify it passes**

Run: `npm --prefix jcp/frontend run build`

### Task 3: 全量验证与文档同步

**Files:**
- Modify: `llmdoc/guides/common-workflows.md`
- Modify: `llmdoc/architecture/system-map.md`
- Modify: `llmdoc/index.md`

**Step 1: Run backend verification**

Run: `go test ./... -count=1`

**Step 2: Run frontend verification**

Run: `npm --prefix jcp/frontend run build`

**Step 3: Update llmdoc**

- 补充 AI 筛选的 `reasoning -> generate_sql -> validate_sql -> execute_query` 事件链路。
- 说明 `reasoning` 现在是连续自然语言摘要，不是列表式结构化步骤，也不是原始模型推理链。

**Step 4: Package verification**

Run: `~/go/bin/wails build -platform darwin/universal -clean -ldflags "-X main.Version=1.0.0"`
