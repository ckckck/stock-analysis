# 同步弹框最小化与筛选确认收敛设计

## 背景

当前 `SyncActionDialog` 同时承载两类场景：

1. `screening` 模式：AI 筛选前的确认弹框
2. `sync-only` 模式：顶部“立即同步”打开的纯同步弹框

其中 `screening` 模式目前仍保留“开始同步”次按钮，和用户新的交互预期不一致；`sync-only` 模式也缺少最小化能力，用户在同步过程中无法把面板收纳到角落继续使用主界面。

## 目标

1. AI 筛选确认弹框只保留“开始筛选”主按钮，不再在该面板内提供“开始同步”入口。
2. 顶部“立即同步”打开的同步弹框新增“最小化”按钮。
3. 最小化后的同步弹框收纳到页面右下角，以悬浮卡片形式持续显示同步进度，并支持恢复。

## 方案

### 1. 收敛 `screening` 模式 CTA

- 更新 `resolveSyncDialogCopy('screening')`：
  - 标题从“确认后执行同步或筛选”收敛为“确认后开始筛选”
  - 描述改为“本次会直接基于当前已同步数据执行 AI 筛选。”
  - 保留 `primaryActionLabel='开始筛选'`
  - 移除 `secondaryActionLabel`
- `SyncActionDialog` 底部按钮区保持现有渲染逻辑，但由于 `secondaryActionLabel` 为空，`screening` 模式不再显示左侧次按钮。

### 2. 为 `sync-only` 模式增加最小化状态

- 在 `App.tsx` 中增加 `syncOnlyDialogMinimized` 状态，仅作用于 `sync-only` 模式。
- 打开同步弹框时强制重置为非最小化。
- 关闭同步弹框时同步清空最小化状态。
- `SyncActionDialog` 增加可选能力：
  - `minimizable?: boolean`
  - `onMinimize?: () => void`
- 仅当 `mode === 'sync-only'` 且传入 `minimizable=true` 时，在右上角关闭按钮左侧显示最小化按钮。

### 3. 右下角悬浮同步卡片

- 当满足以下条件时显示右下角卡片：
  - `syncOnlyDialogVisible === true`
  - `syncOnlyDialogMinimized === true`
- 卡片内容：
  - 标题：根据 `runStatus/loading` 显示“同步进行中 / 同步已完成 / 同步失败 / 同步已取消”
  - 百分比：沿用 `syncStatus.progressPercent`
  - 进度：`completedStocks / totalStocks`
  - 最近状态文案：优先显示 `lastMessage`
  - 恢复按钮：恢复为完整弹框
- 卡片不负责触发同步，只负责状态展示和恢复。

### 4. 同步完成后的行为

- 若同步弹框未最小化，维持当前行为：
  - 成功后自动关闭弹框
- 若同步弹框已最小化：
  - 成功后保留右下角卡片，允许用户恢复查看最终状态
  - 失败/取消同样保留卡片，便于查看结果和错误
- 用户主动关闭同步弹框时，再统一清理 `syncOnlyDialogVisible/syncOnlySyncStarted/syncOnlyDialogMinimized`

## 影响范围

- `jcp/frontend/src/App.tsx`
- `jcp/frontend/src/utils/screeningSync.ts`
- `jcp/frontend/src/utils/screeningSync.test.ts`

## 测试策略

优先为纯前端逻辑补充测试，不引入新的组件测试框架：

1. 校验 `screening` 模式文案已去掉“同步或筛选”和次按钮。
2. 新增最小化卡片状态解析函数测试：
   - 最小化时显示
   - 恢复后隐藏
   - 运行中/完成/失败时文案正确
3. 再落地 `App.tsx` 的 UI 接线。

## 风险

1. `sync-only` 和 `screening` 共用弹框，若条件分支写得过散，容易造成模式串扰。
2. 同步完成后若错误地立即关闭最小化卡片，用户会误以为最小化失效。
3. 若最小化状态未在重新打开时重置，会导致下次打开直接进入缩略态。
