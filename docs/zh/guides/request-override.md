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

## 重写操作类型

AxonHub 支持以下四种重写操作：

| 操作类型 | 描述 | 适用场景 |
| :--- | :--- | :--- |
| `set` | 设置字段值，如果字段不存在则创建 | 修改或添加参数 |
| `delete` | 删除指定字段 | 移除不需要的参数 |
| `rename` | 重命名字段（从 `from` 移动到 `to`） | 字段名映射转换 |
| `copy` | 复制字段值（从 `from` 复制到 `to`） | 参数复用 |

## 重写参数 (Override Parameters)

重写参数定义为一个操作数组，每个操作包含以下字段：

| 字段 | 类型 | 必需 | 描述 |
| :--- | :--- | :--- | :--- |
| `op` | string | 是 | 操作类型：`set`、`delete`、`rename`、`copy` |
| `path` | string | 条件 | 目标字段路径（`set` 和 `delete` 必需） |
| `from` | string | 条件 | 源字段路径（`rename` 和 `copy` 必需） |
| `to` | string | 条件 | 目标字段路径（`rename` 和 `copy` 必需） |
| `value` | string | 条件 | 字段值（`set` 操作必需），支持模板 |
| `condition` | string | 否 | 条件表达式，结果为 `"true"` 时执行 |

### 基础示例

```json
[
  {
    "op": "set",
    "path": "temperature",
    "value": "0.7"
  },
  {
    "op": "set",
    "path": "max_tokens",
    "value": "2000"
  },
  {
    "op": "delete",
    "path": "frequency_penalty"
  }
]
```

### 使用模板

你可以使用模板使参数根据输入请求动态变化：

```json
[
  {
    "op": "set",
    "path": "custom_field",
    "value": "model-{{.Model}}"
  },
  {
    "op": "set",
    "path": "effort_level",
    "value": "effort-{{.ReasoningEffort}}"
  },
  {
    "op": "set",
    "path": "user_context",
    "value": "user-{{index .Metadata \"user_id\"}}"
  }
]
```

### 条件执行

使用 `condition` 字段实现条件逻辑：

```json
[
  {
    "op": "set",
    "path": "top_k",
    "value": "40",
    "condition": "{{eq .Model \"claude-3-opus-20240229\"}}"
  },
  {
    "op": "set",
    "path": "logic_field",
    "value": "premium-mode",
    "condition": "{{eq .Model \"gpt-4o\"}}"
  },
  {
    "op": "set",
    "path": "logic_field",
    "value": "standard-mode",
    "condition": "{{ne .Model \"gpt-4o\"}}"
  }
]
```

### 字段重命名与复制

```json
[
  {
    "op": "rename",
    "from": "old_field_name",
    "to": "new_field_name"
  },
  {
    "op": "copy",
    "from": "model",
    "to": "custom_model_header"
  }
]
```

### 动态 JSON 对象

如果渲染后的模板字符串是一个有效的 JSON 对象或数组，AxonHub 会自动解析它，并将其作为结构化的 JSON 对象插入，而不是作为字符串：

```json
[
  {
    "op": "set",
    "path": "settings",
    "value": "{\"id\": \"{{.Model}}\", \"enabled\": true}"
  }
]
```

*结果 Body:* `{"settings": {"id": "gpt-4o", "enabled": true}}`

### 删除字段

使用 `delete` 操作删除指定字段：

```json
[
  {
    "op": "delete",
    "path": "frequency_penalty"
  }
]
```

## 重写请求头 (Override Headers)

重写请求头使用与重写参数相同的操作格式：

```json
[
  {
    "op": "set",
    "path": "X-Custom-Model",
    "value": "{{.Model}}"
  },
  {
    "op": "set",
    "path": "X-User-ID",
    "value": "{{index .Metadata \"user_id\"}}"
  },
  {
    "op": "delete",
    "path": "X-Internal-Header"
  },
  {
    "op": "rename",
    "from": "Old-Header",
    "to": "New-Header"
  }
]
```

## 常见用例

### 1. 映射推理强度 (Reasoning Effort)

如果提供商使用不同的字段名或值来表示推理强度：

```json
[
  {
    "op": "set",
    "path": "provider_specific_effort",
    "value": "max",
    "condition": "{{eq .ReasoningEffort \"high\"}}"
  },
  {
    "op": "set",
    "path": "provider_specific_effort",
    "value": "normal",
    "condition": "{{ne .ReasoningEffort \"high\"}}"
  }
]
```

### 2. 特定模型参数

某些模型可能需要 OpenAI/Anthropic 标准 API 之外的特定参数：

```json
[
  {
    "op": "set",
    "path": "top_k",
    "value": "40",
    "condition": "{{eq .Model \"claude-3-opus-20240229\"}}"
  }
]
```

### 3. 在请求头中注入元数据

将内部追踪 ID 传递给提供商以便调试：

```json
[
  {
    "op": "set",
    "path": "X-Request-Source",
    "value": "axonhub-gateway"
  },
  {
    "op": "set",
    "path": "X-Internal-User",
    "value": "{{index .Metadata \"internal_id\"}}"
  }
]
```

## 向后兼容

AxonHub 仍然支持旧版的重写参数格式（JSON 对象），系统会自动将其转换为新的操作格式：

**旧版格式（仍支持）：**
```json
{
  "temperature": 0.7,
  "max_tokens": 2000
}
```

这会等效转换为：
```json
[
  {"op": "set", "path": "temperature", "value": "0.7"},
  {"op": "set", "path": "max_tokens", "value": "2000"}
]
```

## 注意事项与限制

- **Stream 参数**: 请求体中的 `stream` 参数无法被重写，因为它由 AxonHub 的流水线统一管理。
- **请求头安全**: 在重写 `Authorization` 等安全敏感的请求头时请务必小心。
- **无效模板**: 如果模板解析或执行失败，将使用原始值，并记录警告日志。
- **执行顺序**: 操作按数组顺序执行，后续操作可以覆盖前面的操作结果。
