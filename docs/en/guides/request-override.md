# Request Override Guide

Request Override is a powerful feature in AxonHub that allows you to dynamically modify request bodies (Body) and headers (Headers) before they are sent to the AI provider. This is particularly useful for model-specific parameter adjustments, feature mapping (like `reasoning_effort`), or injecting custom metadata.

## Core Concepts

Overrides are configured at the **Channel** level. There are two types of overrides:
1. **Override Parameters**: Modifies the JSON request body.
2. **Override Headers**: Modifies the HTTP request headers.

### Template Rendering

AxonHub uses Go templates for dynamic value rendering. You can access the following variables in your templates:

| Variable | Description | Example |
| :--- | :--- | :--- |
| `.RequestModel` | The original model name from the client's request. | `{{.RequestModel}}` |
| `.Model` | The model name currently set in the request (after model mapping). | `{{.Model}}` |
| `.ReasoningEffort` | The `reasoning_effort` value (none, low, medium, high). | `{{.ReasoningEffort}}` |
| `.Metadata` | Custom metadata map passed in the request. | `{{index .Metadata "user_id"}}` |
| `.RequestHeader` | Filtered inbound client headers. Supports canonical/lowercase lookup and returns the first value. | `{{index .RequestHeader "X-Trace-Id"}}` |

## Override Operation Types

AxonHub supports the following override operations:

| Operation Type | Description | Use Case |
| :--- | :--- | :--- |
| `set` | Set field value, create if field doesn't exist | Modify or add parameters |
| `delete` | Delete specified field | Remove unwanted parameters |
| `rename` | Rename field (move from `from` to `to`) | Field name mapping conversion |
| `copy` | Copy field value (copy from `from` to `to`) | Parameter reuse |
| `array_append` | Append value(s) to the end of the array at `path` | Inject items after existing array content |
| `array_prepend` | Prepend value(s) to the start of the array at `path` | Inject items before existing array content |
| `array_insert` | Insert value(s) at a specific position in the array at `path` | Insert items at an arbitrary position |
| `array_remove` | Remove matching items from the array at `path` | Filter tools or messages by a field inside each array item |

> Array operations only apply to the body. Headers only support `set`, `delete`, `rename`, and `copy`.

## Override Parameters

Override parameters are defined as an array of operations, each containing the following fields:

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `op` | string | Yes | Operation type: `set`, `delete`, `rename`, `copy`, `array_append`, `array_prepend`, `array_insert`, `array_remove` |
| `path` | string | Conditional | Target field path (required for `set`, `delete`, and all array ops) |
| `from` | string | Conditional | Source field path (required for `rename` and `copy`) |
| `to` | string | Conditional | Target field path (required for `rename` and `copy`) |
| `value` | string | Conditional | Field value (required for `set`, `array_append`, `array_prepend`, and `array_insert`), supports templates |
| `condition` | string | No | Condition expression, executes when result is `"true"` |
| `match` | object | Conditional | Match rule (required for `array_remove`), formatted as `{"path":"function.name","eq":"web_search"}` |
| `index` | number | Conditional | Insertion position (required for `array_insert`); negative values count from the end, out-of-range values are clamped to `[0, len]` |
| `splat` | bool | No | When the rendered value is a JSON array, controls whether elements are spread into the target array. Defaults to `true`. Set to `false` to insert the array itself as a single nested element. Only meaningful for array ops. |

### Basic Example

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

### Using Templates

You can use templates to make parameters dynamic based on the input request:

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
  },
  {
    "op": "set",
    "path": "trace_id",
    "value": "{{index .RequestHeader \"x-trace-id\"}}"
  }
]
```

### Conditional Execution

Use the `condition` field to implement conditional logic:

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

### Field Renaming and Copying

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

### Array Operations

Array operations let you inject or remove items in an existing array (e.g. `system`, `messages`, `tools`) without replacing it. Use them when you need to keep the user's original content and add proxy-side content around it, or when you need to filter specific array items.

**Behavior:**
- For `array_append`, `array_prepend`, and `array_insert`, if `path` does not exist, a new array is created with the value(s).
- If `path` exists but is not an array, the operation is skipped and a warning is logged.
- For inserting array operations, if the rendered `value` is a JSON array and `splat` is `true` (default), its elements are spread into the target array. Set `splat: false` to insert the array as a single nested element.
- For `array_insert`, `index` may be negative (counted from the end). `index = -1` inserts before the last element. Out-of-range values are clamped to `[0, len]`.
- For `array_remove`, `match.path` is resolved relative to each array item. When that path's string value equals `match.eq`, the item is removed. If the target array does not exist, the operation leaves the body unchanged; if the target path is not an array, the operation is skipped and a warning is logged.

**Append a single object:**

```json
[
  {
    "op": "array_append",
    "path": "messages",
    "value": "{\"role\":\"system\",\"content\":\"appended note\"}"
  }
]
```

**Prepend multiple system items (preserving the user's original content):**

```json
[
  {
    "op": "array_prepend",
    "path": "system",
    "value": "[{\"type\":\"text\",\"text\":\"x-anthropic-billing-header: ...\"},{\"type\":\"text\",\"text\":\"You are Claude Code...\",\"cache_control\":{\"type\":\"ephemeral\"}}]"
  }
]
```

Result (assuming the request originally has `system: [{"type":"text","text":"<user>"}]`):

```json
{
  "system": [
    {"type": "text", "text": "x-anthropic-billing-header: ..."},
    {"type": "text", "text": "You are Claude Code...", "cache_control": {"type": "ephemeral"}},
    {"type": "text", "text": "<user>"}
  ]
}
```

**Insert at a specific position:**

```json
[
  {
    "op": "array_insert",
    "path": "messages",
    "index": 1,
    "value": "{\"role\":\"system\",\"content\":\"inserted between message 0 and 1\"}"
  }
]
```

**Insert an array as a single nested element (disable splat):**

```json
[
  {
    "op": "array_prepend",
    "path": "tags",
    "value": "[\"a\",\"b\"]",
    "splat": false
  }
]
```

Result on `{"tags": ["x"]}`: `{"tags": [["a","b"], "x"]}`.

**Remove a tool by name:**

```json
[
  {
    "op": "array_remove",
    "path": "tools",
    "match": {
      "path": "function.name",
      "eq": "web_search"
    }
  }
]
```

For `{"tools":[{"function":{"name":"get_weather"}},{"function":{"name":"web_search"}}]}`, the `web_search` tool is removed from the `tools` array.

### Dynamic JSON Objects

If a rendered template string is a valid JSON object or array, AxonHub will automatically parse it and insert it as a structured JSON object rather than a string:

```json
[
  {
    "op": "set",
    "path": "settings",
    "value": "{\"id\": \"{{.Model}}\", \"enabled\": true}"
  }
]
```

*Resulting Body:* `{"settings": {"id": "gpt-4o", "enabled": true}}`

### Deleting Fields

Use the `delete` operation to remove specified fields:

```json
[
  {
    "op": "delete",
    "path": "frequency_penalty"
  }
]
```

## Override Headers

Override headers use the same operation format as override parameters:

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
    "op": "set",
    "path": "X-Trace-Id",
    "value": "{{index .RequestHeader \"x-trace-id\"}}"
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

## Common Use Cases

### 1. Mapping Reasoning Effort

If a provider uses a different field name or value for reasoning effort:

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

### 2. Model-Specific Parameters

Some models might require specific parameters that aren't part of the standard OpenAI/Anthropic API:

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

### 3. Injecting Metadata into Headers

Pass internal tracking IDs to the provider for debugging:

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

## Backward Compatibility

AxonHub still supports the legacy override parameters format (JSON object), and the system will automatically convert it to the new operation format:

**Legacy Format (Still Supported):**
```json
{
  "temperature": 0.7,
  "max_tokens": 2000
}
```

This will be equivalently converted to:
```json
[
  {"op": "set", "path": "temperature", "value": "0.7"},
  {"op": "set", "path": "max_tokens", "value": "2000"}
]
```

## Notes & Limitations

- **Stream Parameter**: The `stream` parameter in the request body cannot be overridden as it is managed by the AxonHub pipeline.
- **Header Security**: Be careful when overriding security-sensitive headers like `Authorization`.
- **Invalid Templates**: If a template fails to parse or execute, the original raw value will be used, and a warning will be logged.
- **Execution Order**: Operations are executed in array order, and subsequent operations can override the results of previous operations.
