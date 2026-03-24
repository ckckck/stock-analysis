# AI 筛选历史结果重跑与查询超时设计

## 背景

当前 AI 筛选还存在三类问题：

1. SQL 生成阶段使用固定 45 秒超时，未复用 AI 配置中的 `timeout`，慢模型或不稳定网关会直接报 `context deadline exceeded`。
2. 左侧侧栏目前把“当前结果”和“历史记录入口”混在一个工作区里，无法按“当前筛选结果 / 历史筛选结果”分栏切换。
3. 从首页确认弹框打开设置时，设置弹窗层级低于确认弹框，会被遮挡。

## 目标

1. AI 筛选生成 SQL 的超时应优先读取当前 AI 配置中的 `timeout`，并给出更明确的超时错误。
2. 左侧结果侧栏顶部新增双页签：
   - 当前筛选结果
   - 历史筛选结果
3. 在历史筛选结果模式中，左侧整栏展示历史记录列表；点击一条历史记录后，把其结果恢复到当前结果页，并允许“根据当前条件重新筛选”。
4. 历史重跑不再调用 AI 生成 SQL，而是直接使用该历史记录保存的 `generatedSql`，重新执行并生成一条新的历史记录。
5. 设置弹窗层级必须高于 AI 筛选确认弹框。

## 方案

### 查询超时修复

- `screeningSQLGeneratorAdapter.GenerateSQL()` 先解析当前 `AIConfig.Timeout`。
- `ScreeningQueryService.RunWithProgress()` 不再写死 45 秒，而是为 SQL 生成链传入可配置超时。
- 若发生 `context deadline exceeded`，后端统一包装为“AI 生成 SQL 超时”，并带上当前秒数，便于用户判断是模型慢还是网络慢。

### 历史 SQL 重跑

- 后端新增基于历史记录重跑的接口，例如 `RerunScreeningHistoryRun(runID, page, pageSize)`。
- 实现方式：
  - 读取指定 `screening_run`
  - 取出其 `generated_sql`
  - 按当前数据库重新执行
  - 重新写入一条新的 `screening_run`
  - 重新写入新的 `screening_run_results`
- 这条链不再调用模型，因此不会经过 `generate_sql` 阶段。
- 仍保留 SQL 白名单校验，避免旧数据或脏数据绕过约束。

### 前端双页签

- `ScreeningResultList` 顶部新增页签：
  - `当前筛选结果`
  - `历史筛选结果`
- 当前结果模式：
  - 展示当前结果股票列表
  - 若当前结果来自历史回放，则顶部按钮显示“根据当前条件重新筛选”
- 历史结果模式：
  - 左侧整栏展示历史筛选记录
  - 点击记录后：
    - 读取历史结果
    - 切回“当前筛选结果”页签
    - 同步 `prompt`
    - 标记当前结果来源为 `history`
- 左下工作区仍保留 prompt 文本框，但在历史来源状态下点击按钮走“历史 SQL 重跑”。

### 弹窗层级

- 设置弹窗挂到比 `ScreeningConfirmDialog` 更高的 z-index。
- 或在打开设置时主动关闭确认弹框；推荐保留确认状态但让设置覆盖其上，避免用户返回后丢失输入。

## 范围约束

- 不新增复杂查询会话表。
- 不支持编辑历史 SQL。
- 不做多版本结果对比视图。

## 验证

- Go 测试覆盖：
  - 读取 AI 配置超时
  - 历史 SQL 重跑会生成新的 run 和结果
- 前端构建通过
- `go test ./...`
- `cd jcp/frontend && npm run build`
- `~/go/bin/wails build`
