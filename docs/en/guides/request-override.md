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

## Override Operation Types

AxonHub supports the following four override operations:

| Operation Type | Description | Use Case |
| :--- | :--- | :--- |
| `set` | Set field value, create if field doesn't exist | Modify or add parameters |
| `delete` | Delete specified field | Remove unwanted parameters |
| `rename` | Rename field (move from `from` to `to`) | Field name mapping conversion |
| `copy` | Copy field value (copy from `from` to `to`) | Parameter reuse |

## Override Parameters

Override parameters are defined as an array of operations, each containing the following fields:

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `op` | string | Yes | Operation type: `set`, `delete`, `rename`, `copy` |
| `path` | string | Conditional | Target field path (required for `set` and `delete`) |
| `from` | string | Conditional | Source field path (required for `rename` and `copy`) |
| `to` | string | Conditional | Target field path (required for `rename` and `copy`) |
| `value` | string | Conditional | Field value (required for `set` operation), supports templates |
| `condition` | string | No | Condition expression, executes when result is `"true"` |

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
