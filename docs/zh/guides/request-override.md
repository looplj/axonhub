# 请求重写 (Request Override) 指南

请求重写是 AxonHub 的一项强大功能，允许你在请求发送到 AI 提供商之前，动态地修改请求体 (Body) 和请求头 (Headers)。这在处理特定模型的参数调整、功能映射（如 `reasoning_effort`）或注入自定义元数据时非常有用。

## 核心概念

重写是在 **渠道 (Channel)** 级别配置的。主要分为两种类型：
1. **重写参数 (Override Parameters)**：修改 JSON 请求体。
2. **重写请求头 (Override Headers)**：修改 HTTP 请求头。

### 模板渲染

AxonHub 使用 Go 模板 (Go templates) 进行动态值渲染。你可以在模板中使用以下变量：

| 变量 | 描述 | 示例 |
| :--- | :--- | :--- |
| `.RequestModel` | 来自客户端原始请求的模型名称。 | `{{.RequestModel}}` |
| `.Model` | 当前请求中的模型名称（可能经过了模型映射）。 | `{{.Model}}` |
| `.ReasoningEffort` | `reasoning_effort` 的值 (none, low, medium, high)。 | `{{.ReasoningEffort}}` |
| `.Metadata` | 请求中传递的自定义元数据 Map。 | `{{index .Metadata "user_id"}}` |

## 重写参数 (Override Parameters)

重写参数定义为一个 JSON 对象，其中 Key 是你想要修改的字段路径，Value 是新的值（或模板）。

### 基础重写

```json
{
  "temperature": 0.7,
  "max_tokens": 2000,
  "response_format.type": "json_object"
}
```

### 使用模板

你可以使用模板使参数根据输入请求动态变化。

```json
{
  "custom_field": "model-{{.Model}}",
  "effort_level": "effort-{{.ReasoningEffort}}",
  "user_context": "user-{{index .Metadata \"user_id\"}}"
}
```

### 复杂逻辑

你可以使用标准的 Go 模板逻辑，例如 `if/else`。

```json
{
  "logic_field": "{{if eq .Model \"gpt-4o\"}}premium-mode{{else}}standard-mode{{end}}"
}
```

### 动态 JSON 对象

如果渲染后的模板字符串是一个有效的 JSON 对象或数组，AxonHub 会自动解析它，并将其作为结构化的 JSON 对象插入，而不是作为字符串。

```json
{
  "settings": "{\"id\": \"{{.Model}}\", \"enabled\": true}"
}
```
*结果 Body:* `{"settings": {"id": "gpt-4o", "enabled": true}}`

### 删除字段

使用特殊值 `__AXONHUB_CLEAR__` 从请求体中删除某个字段。

```json
{
  "frequency_penalty": "__AXONHUB_CLEAR__"
}
```

## 重写请求头 (Override Headers)

重写请求头允许你注入或修改发送给提供商的 HTTP 头部。

| Key | Value |
| :--- | :--- |
| `X-Custom-Model` | `{{.Model}}` |
| `X-User-ID` | `{{index .Metadata "user_id"}}` |
| `Authorization` | `__AXONHUB_CLEAR__` (删除该请求头) |

## 常见用例

### 1. 映射推理强度 (Reasoning Effort)

如果提供商使用不同的字段名或值来表示推理强度，你可以轻松进行映射：

**重写参数:**
```json
{
  "provider_specific_effort": "{{if eq .ReasoningEffort \"high\"}}max{{else}}normal{{end}}"
}
```

### 2. 特定模型参数

某些模型可能需要 OpenAI/Anthropic 标准 API 之外的特定参数：

**重写参数:**
```json
{
  "top_k": "{{if eq .Model \"claude-3-opus-20240229\"}}40{{else}}__AXONHUB_CLEAR__{{end}}"
}
```

### 3. 在请求头中注入元数据

将内部追踪 ID 传递给提供商以便调试：

**重写请求头:**
| Key | Value |
| :--- | :--- |
| `X-Request-Source` | `axonhub-gateway` |
| `X-Internal-User` | `{{index .Metadata "internal_id"}}` |

## 注意事项与限制

- **Stream 参数**: 请求体中的 `stream` 参数无法被重写，因为它由 AxonHub 的流水线统一管理。
- **请求头安全**: 在重写 `Authorization` 等安全敏感的请求头时请务必小心。
- **无效模板**: 如果模板解析或执行失败，将使用原始值，并记录警告日志。
