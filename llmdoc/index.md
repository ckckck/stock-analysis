# llmdoc 索引

- `overview/`：项目目标、范围、关键上下文
- `architecture/`：系统结构、模块关系、核心流程
- `guides/`：常见任务操作步骤
- `reference/`：约定、接口、数据模型索引

## 当前文档

- `overview/project-overview.md`：项目定位、能力面、核心模块边界
- `architecture/system-map.md`：桌面壳、前端、服务编排、会议系统和 OpenClaw 的执行链
- `guides/common-workflows.md`：开发运行、发起分析、调整策略、使用 OpenClaw 的常见流程
- `reference/conventions.md`：仓库根约定、数据落盘位置、上游同步入口

## 当前范围说明

- 当前 `llmdoc` 以顶层 `stock-analysis/` 为主仓库视角。
- 业务代码主体位于 `jcp/`，相关实现说明统一引用 `jcp/` 下源码。
- 上游 Git 历史已迁移到 `.upstreams/`，同步入口见 `docs/upstream-sync.md`。

## 阅读顺序

1. `index.md`
2. `overview/project-overview.md`
3. `architecture/system-map.md`
4. `guides/common-workflows.md`
5. `reference/conventions.md`
