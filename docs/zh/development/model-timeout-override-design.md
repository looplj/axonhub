# 模型单次请求超时覆写设计

## 背景与问题

当前系统仅通过 `server.llm_request_timeout` 统一控制 LLM 请求的总超时。在多渠道负载均衡场景下，如果单个渠道阻塞过久，容易耗尽整体超时预算，导致无法进行后续重试或切换渠道。

需求：在网页端为特定模型配置“单次渠道请求超时”，当单次超时发生时触发重试，但整体仍严格受 `server.llm_request_timeout` 约束。例如：全局 600s，单次 100s，则在 600s 总时长内可以进行多次候选尝试。

## 目标

- 支持在模型级别配置单次请求超时覆盖值。
- 单次超时仅影响一次候选执行；整体超时仍由 `server.llm_request_timeout` 统一限制。
- 与现有数据库和旧版本数据完全兼容，默认行为不变。
- 前端可配置、可清空，后台可验证与可观测。

## 非目标

- 不引入渠道级别或请求级别的超时覆盖（后续可扩展）。
- 不改变现有负载均衡策略或重试策略的排序逻辑。
- 不调整网关层的全局请求超时机制。

## 总体方案

在请求生命周期中引入“双层超时”概念：

1. **全局超时（总时长）**：仍由 `server.llm_request_timeout` 控制，作用于整个请求链路。
2. **单次超时（候选级）**：由模型配置提供，限制每个候选渠道的单次执行时长。

执行策略：
- 每次执行候选时创建 `attemptCtx`，其超时为 `min(remaining, attemptTimeout)`。
- 若单次超时触发，则视为可重试错误，进入下一候选。
- 若总时长耗尽，直接返回超时错误，不再重试。

## 数据与配置设计

### 新增配置项（默认值）

在服务配置中新增可选项，用于提供“单次超时”的全局默认值，作为模型未配置时的回退：\n

```yaml
server:
  llm_request_timeout: "600s"
  llm_request_attempt_timeout: "0s" # 单次候选请求超时默认值；0s 表示不启用，等同于仅使用总时长
```

优先级：`模型配置 > 配置文件默认值 > 总时长（llm_request_timeout）`。

### ModelSettings 扩展

在 `ModelSettings` 中新增可选字段 `timeouts`，用于承载模型级别超时配置，结构如下：

```json
{
  "associations": [
    {
      "type": "model",
      "priority": 0,
      "modelId": { "modelId": "gpt-4" }
    }
  ],
  "timeouts": {
    "attempt": "100s"
  }
}
```

- `timeouts.attempt`：单次候选请求超时，使用 `time.Duration` 字符串格式（如 `"100s"`、`"2m"`）。
- 未设置或为空时，行为与当前一致（使用全局超时进行一次或多次尝试）。

### GraphQL 结构

`ModelSettings` 与 `ModelSettingsInput` 新增字段：

- `timeouts: ModelTimeouts`
- `ModelTimeouts.attempt: String`

这是一项**向后兼容**的字段扩展，旧客户端可继续使用旧字段，不受影响。

### 前端配置

模型编辑界面新增“单次请求超时”输入项：

- 输入形式：秒数或持续时间字符串（建议 UI 统一为秒数并保存为 `"{n}s"`）。
- 为空时代表继承全局配置。
- 提示文案：说明该值仅限制单次渠道执行，总时长仍受 `server.llm_request_timeout` 限制。

## 执行逻辑设计

### 关键流程

- 在请求进入 LLM 处理链时记录全局 deadline。
- 每次候选执行前计算剩余时长。
- 按以下规则创建单次超时上下文：

```go
// 伪代码：单次超时计算
remaining := deadline.Sub(time.Now())
if remaining <= 0 {
    return ErrRequestTimeout
}

attemptTimeout := remaining
if modelTimeoutAttempt != nil {
    attemptTimeout = min(remaining, *modelTimeoutAttempt)
}

attemptCtx, cancel := context.WithTimeout(parentCtx, attemptTimeout)
// 使用 attemptCtx 发起请求
```

### 错误与重试语义

- `attemptCtx` 超时：映射为“单次超时”，可触发重试。
- `parentCtx` 超时：表示总时长耗尽，直接返回超时错误。
- 其他错误仍沿用现有重试判定规则。

### 可观测性

- 在请求日志/追踪中记录：
  - `model_attempt_timeout`（如 `100s`）
  - `request_deadline` / `remaining_before_attempt`
- 便于定位因单次超时导致的重试行为。

## 兼容性与迁移

### 数据库兼容性

- `ModelSettings` 存储在 JSON 字段中，新增 `timeouts` 为**可选字段**，无需数据库迁移。
- 旧数据没有该字段时按默认值处理，行为不变。

### 版本兼容性

- **新版本读取旧数据**：正常工作，`timeouts` 为 `nil`。
- **旧版本读取新数据**：未知字段会被忽略，不影响运行。
- **旧版本写回数据**：可能会丢弃 `timeouts` 字段（JSON 结构重新序列化）。

为避免回滚时丢失配置：
- 建议在发布说明中提示“降级可能丢失模型超时配置”。
- 若需强一致，可在后续版本考虑单独的持久化字段或版本化设置结构。

## 测试计划

- **单元测试**：
  - 当 `timeouts.attempt` 未配置时，单次超时等于剩余时间。
  - 当 `timeouts.attempt` 小于剩余时间时，单次超时生效。
  - 当 `timeouts.attempt` 大于剩余时间时，单次超时被剩余时间夹断。

- **集成测试**：
  - 设置 `server.llm_request_timeout = 600s`，模型 `attempt = 100s`，模拟首渠道超时后切换成功。
  - 总时长耗尽时返回超时且不再重试。

## 影响范围

- **后端**：
  - `objects.ModelSettings` 增加 `timeouts` 字段。
  - GraphQL schema 与生成代码更新。
  - orchestrator 内部执行逻辑引入单次超时。

- **前端**：
  - 模型编辑页面新增超时配置项。
  - 保存/读取与 `ModelSettings` 对齐。

- **文档**：
  - 补充模型管理或配置指南中的超时说明。

## 里程碑

1. 数据结构与 GraphQL 扩展
2. 后端超时逻辑实现与测试
3. 前端配置与校验
4. 文档更新与发布说明
