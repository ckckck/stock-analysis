# AI 筛选确认与进度设计

## 背景

当前 AI 筛选存在两个问题：

1. 欢迎页或筛选工作区提交后，若本地已有同步数据，会直接开始筛选，不会先让用户确认。
2. AI 筛选请求是一次性阻塞调用，前端只有 `loading` 布尔值，没有阶段、日志或进度条。

## 目标

1. 任意 AI 筛选提交都必须先进入确认流程，再开始执行。
2. AI 筛选执行中要显示当前阶段、百分比进度和最近日志。
3. 已有“测试同步子集”限制要继续生效，确认后执行的筛选仍只能跑在该测试 universe 上。

## 方案

### 入口统一确认

- 不再由欢迎页或工作区直接调用 `executeScreening()`。
- 两个入口都改成先写入待执行请求，再显示统一确认弹框。
- 确认弹框内容：
  - 当前筛选条件
  - 当前同步策略
  - 当前数据状态
  - 若未同步则提示“会先同步再继续”
  - 若开启测试范围，则明确显示“本次筛选仅作用于测试子集”

### 查询进度事件

- 后端新增 `ScreeningQueryProgress` / `ScreeningQueryLog`。
- `ScreeningQueryService` 新增带回调的运行接口，按阶段发事件：
  - `prepare`
  - `generate_sql`
  - `validate_sql`
  - `execute_query`
  - `store_results`
  - `completed` / `failed`
- `App.RunScreeningQuery` 把回调事件转成 `screening:query:progress` Wails 事件。

### 前端进度展示

- `App.tsx` 订阅 `screening:query:progress`，保存当前进度状态。
- `ScreeningWorkspace` 顶部新增进度面板，显示：
  - 当前阶段
  - 进度条
  - 最近日志
  - 是否为测试 universe
- `ScreeningResultList` 在加载时同步显示当前阶段文案，避免只看到泛化的“分析中”。

## 范围约束

- 不引入异步任务表或后台队列。
- 不为历史回放增加进度；只覆盖主动发起的 AI 筛选。
- 不修改历史结果回放和筛选结果持久化语义。

## 验证

- Go 测试覆盖查询进度事件与测试 universe 约束。
- `go test ./...`
- `cd jcp/frontend && npm run build`
- `~/go/bin/wails build`
