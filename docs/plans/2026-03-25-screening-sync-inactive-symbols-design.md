# AI 筛选同步非活跃股票与数据源容错设计

## 背景

当前 AI 筛选同步会把 `list_status == "D"` 的股票写入 `stocks_basic`，但不会在同步队列阶段排除它们。像 `sz002231` 这类已不再活跃的股票会继续参与日线补齐，随后在 `Baostock -> Sina` 回退链上同时失败，最终把整轮同步打断。

## 目标

1. 非活跃股票不再进入后续同步与筛选候选。
2. `Baostock` 对空成交量/空成交额具备容错，不因单行空值直接报错。
3. `Sina` 回退切换到当前可用的新接口。
4. 单只股票取数失败只记录并跳过，不中断整轮同步。

## 方案比较

### 方案 A：只过滤 `is_active=false`

- 优点：改动最小，能直接解决已退市股票反复卡同步。
- 缺点：仍然会被“活跃但单票接口异常”的股票打断整轮。

### 方案 B：过滤非活跃 + 数据源容错

- 优点：同时解决已退市股票和空量/旧接口问题。
- 缺点：若仍有单票异常，整轮同步依旧会被中断。

### 方案 C：过滤非活跃 + 数据源容错 + 单票失败跳过

- 优点：把问题拆成“候选过滤、单源容错、任务容错”三层，整轮同步稳定性最高。
- 缺点：需要同步更新测试和诊断文案。

推荐方案：C。

## 设计

### 1. 同步候选过滤

- `ListScreeningStocks()` 仍保留 `IsActive` 字段写入 `stocks_basic`。
- `ScreeningSyncService.SyncWithOptions()` 在构建同步队列前，过滤掉 `IsActive == false` 的个股。
- 指数不受影响。

### 2. Baostock 解析容错

- `parseBaoStockKLines()` 对 `volume`、`amount` 的空字符串按 `0` 处理，而不是直接返回错误。
- 保持 OHLC 字段仍为强解析，因为这些字段为空意味着整行数据不可用。

### 3. Sina 回退源更新

- 将日线回退接口从旧的 `quotes.sina.cn` 切换到可正常返回 JSON 的 `money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData`。
- 仅影响 `GetScreeningDailyBars()` 的 1d 回退链，不改其它实时行情接口。

### 4. 单票失败处理

- 同步过程中，若单只股票 `getScreeningDailyBars()` 失败：
  - 记录诊断事件和失败计数；
  - 跳过该股票继续后续队列；
  - 整轮同步最终可完成，并保留失败股票列表用于状态展示。

## 测试

- 新增/更新单元测试覆盖：
  - `is_active=false` 股票不会进入同步取数队列；
  - `Baostock` 空 `volume`/`amount` 可正常解析；
  - 单票失败时同步继续完成，不再整体失败；
  - `Sina` 回退源切换后仍保留原有回退链行为。
