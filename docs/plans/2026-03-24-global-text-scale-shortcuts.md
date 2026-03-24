# Global Text Scale Shortcuts Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 支持 `cmd/ctrl +/-` 调整页面整体文字与图标大小，并把缩放值持久化到现有应用配置中。

**Architecture:** 后端在 `LayoutConfig` 中新增 `textScalePercent` 并补齐旧配置默认值；前端把缩放逻辑抽成纯工具函数，在 `App.tsx` 中注册全局快捷键、应用根字号，并复用现有布局配置保存链路持久化。

**Tech Stack:** Go, Wails, React 18, TypeScript, Vitest, Tailwind CSS

---

### Task 1: 补齐前端缩放工具测试

**Files:**
- Create: `jcp/frontend/src/utils/textScale.ts`
- Create: `jcp/frontend/src/utils/textScale.test.ts`

**Step 1: Write the failing test**

- 覆盖快捷键识别：
  - `meta + =`
  - `ctrl + -`
  - `meta + NumpadAdd`
  - 非法按键返回空
- 覆盖缩放步进：
  - `100 -> 110`
  - `100 -> 90`
- 覆盖边界：
  - 上限封顶
  - 下限封底

**Step 2: Run test to verify it fails**

Run: `npm --prefix jcp/frontend run test -- src/utils/textScale.test.ts`

**Step 3: Write minimal implementation**

- 实现快捷键方向判断
- 实现缩放百分比步进和 clamp

**Step 4: Run test to verify it passes**

Run: `npm --prefix jcp/frontend run test -- src/utils/textScale.test.ts`

### Task 2: 补齐后端布局默认值测试

**Files:**
- Modify: `jcp/internal/services/config_service_test.go`
- Modify: `jcp/internal/models/config.go`
- Modify: `jcp/internal/services/config_service.go`

**Step 1: Write the failing test**

- 给 legacy config 不写 `layout.textScalePercent`
- 断言重新加载后默认值为 `100`

**Step 2: Run test to verify it fails**

Run: `go test ./internal/services -run TestConfigServiceAddsLayoutTextScaleDefaultForLegacyConfig -count=1`

**Step 3: Write minimal implementation**

- 为 `LayoutConfig` 新增 `TextScalePercent`
- 在默认配置里给 `100`
- 在 `loadConfig()` 里补 layout 缺省值

**Step 4: Run test to verify it passes**

Run: `go test ./internal/services -run TestConfigServiceAddsLayoutTextScaleDefaultForLegacyConfig -count=1`

### Task 3: 接入前端全局缩放

**Files:**
- Modify: `jcp/frontend/src/App.tsx`
- Modify: `jcp/frontend/src/style.css`
- Modify: `jcp/frontend/wailsjs/go/models.ts`

**Step 1: Write the failing integration expectation**

- 依赖 Task 1 的纯函数测试先锁定行为
- 在实现前先确认 `App.tsx` 当前未持久化该值

**Step 2: Implement minimal wiring**

- 增加 `textScalePercent` state
- 启动时从 `config.layout.textScalePercent` 恢复
- 通过 `document.documentElement.style.fontSize` 应用
- 在 `saveLayoutConfig()` 中连同布局一起保存
- 注册 `keydown` 监听并拦截 `cmd/ctrl +/-`

**Step 3: Run focused verification**

Run:
- `npm --prefix jcp/frontend run test -- src/utils/textScale.test.ts`
- `npm --prefix jcp/frontend run build`

### Task 4: 全量验证

**Files:**
- Verify only

**Step 1: Run Go tests**

Run: `go test ./...`

**Step 2: Run frontend tests**

Run: `npm --prefix jcp/frontend run test`

**Step 3: Run frontend build**

Run: `npm --prefix jcp/frontend run build`

