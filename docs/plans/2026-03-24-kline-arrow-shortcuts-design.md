# K 线方向键快捷操作设计

## 背景

当前页面已经支持 `cmd/ctrl +/-` 调整整体文字大小，但用户还需要一组只作用于 K 线图本身的键盘操作：

- `ArrowUp`：放大当前 K 线图
- `ArrowDown`：缩小当前 K 线图
- `ArrowLeft`：当前 K 线图向左平移
- `ArrowRight`：当前 K 线图向右平移

这里的“左右”明确指同一只股票 K 线内容的时间窗口平移，不是切换股票。

## 方案选择

### 方案 1：方向键驱动图表 API

- `App.tsx` 监听全局方向键
- 过滤输入框、文本域和可编辑区域
- 将图表操作通过 ref 下发给 `StockChartLW`
- `StockChartLW` 内部用 lightweight-charts API 操作时间轴

这是本次采用的方案。实现边界清晰，行为稳定，不影响列表选中态和页面其它区域。

### 方案 2：左右切股、上下改容器高度

- 这会把“图表操作”混成“列表导航”或“布局修改”
- 不符合用户想要的“只操作 K 线内容”

不采用。

### 方案 3：模拟鼠标滚轮/拖拽

- 对 WebView 和图表库实现细节依赖太重
- 稳定性差

不采用。

## 设计

### 快捷键范围

- 全局监听 `ArrowUp/ArrowDown/ArrowLeft/ArrowRight`
- 在以下场景下不生效：
  - `input`
  - `textarea`
  - `contenteditable`
  - 任何带文本输入焦点的弹窗或工作区

### 图表行为

- `ArrowUp/ArrowDown`
  - 改变当前 K 线图的时间轴可视逻辑区间宽度
  - 宽度变小表示放大，宽度变大表示缩小
  - 只持久化这个“缩放宽度因子”
- `ArrowLeft/ArrowRight`
  - 在当前逻辑区间不变的前提下，整体平移时间窗口
  - 这是临时视图操作，不持久化

### 持久化

- 在现有 `config.layout` 中新增 `klineZoomPercent`
- 默认值为 `100`
- 旧配置文件加载时自动补默认值
- 切股票、切周期、刷新图表后，仍按当前持久化缩放因子重新应用

### 实现层

- `App.tsx`
  - 增加方向键监听
  - 保留现有 `cmd/ctrl +/-` 页面文字缩放
  - 通过 `ref` 调用 `StockChartLW` 暴露的方法
- `StockChartLW.tsx`
  - 改为 `forwardRef`
  - 暴露 `zoomIn / zoomOut / panLeft / panRight`
  - 在 `fitContent()` 后按当前缩放因子重算可视逻辑范围
- 工具函数
  - 抽一个纯函数模块处理：
    - 方向键动作识别
    - 输入区过滤
    - 缩放步进
    - 平移步进

### 测试

- 前端纯函数测试：
  - 方向键识别
  - 输入区排除
  - 缩放步进与边界
  - 平移步进
- 后端配置测试：
  - 老配置缺少 `klineZoomPercent` 时补成 `100`

