# Request Override Guide

Request Override is a powerful feature in AxonHub that allows you to dynamically modify request bodies and headers before they are sent to the AI provider. This is particularly useful for model-specific parameter adjustments, feature mapping (like `reasoning_effort`), or injecting custom metadata.

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

## Override Parameters

Override parameters are defined as a JSON object where keys are the paths to the fields you want to modify, and values are the new values (or templates).

### Basic Overrides

```json
{
  "temperature": 0.7,
  "max_tokens": 2000,
  "response_format.type": "json_object"
}
```

### Using Templates

You can use templates to make parameters dynamic based on the input request.

```json
{
  "custom_field": "model-{{.Model}}",
  "effort_level": "effort-{{.ReasoningEffort}}",
  "user_context": "user-{{index .Metadata \"user_id\"}}"
}
```

### Complex Logic

You can use standard Go template logic like `if/else`.

```json
{
  "logic_field": "{{if eq .Model \"gpt-4o\"}}premium-mode{{else}}standard-mode{{end}}"
}
```

### Dynamic JSON Objects

If a rendered template string is a valid JSON object or array, AxonHub will automatically parse it and insert it as a structured JSON object rather than a string.

```json
{
  "settings": "{\"id\": \"{{.Model}}\", \"enabled\": true}"
}
```
*Resulting Body:* `{"settings": {"id": "gpt-4o", "enabled": true}}`

### Removing Fields

Use the special value `__AXONHUB_CLEAR__` to remove a field from the request body.

```json
{
  "frequency_penalty": "__AXONHUB_CLEAR__"
}
```

## Override Headers

Override headers allow you to inject or modify HTTP headers sent to the provider.

| Key | Value |
| :--- | :--- |
| `X-Custom-Model` | `{{.Model}}` |
| `X-User-ID` | `{{index .Metadata "user_id"}}` |
| `Authorization` | `__AXONHUB_CLEAR__` (Removes the header) |

## Common Use Cases

### 1. Mapping Reasoning Effort

If a provider uses a different field name or value for reasoning effort, you can map it easily:

**Override Parameters:**
```json
{
  "provider_specific_effort": "{{if eq .ReasoningEffort \"high\"}}max{{else}}normal{{end}}"
}
```

### 2. Model-Specific Parameters

Some models might require specific parameters that aren't part of the standard OpenAI/Anthropic API:

**Override Parameters:**
```json
{
  "top_k": "{{if eq .Model \"claude-3-opus-20240229\"}}40{{else}}__AXONHUB_CLEAR__{{end}}"
}
```

### 3. Injecting Metadata into Headers

Pass internal tracking IDs to the provider for debugging:

**Override Headers:**
| Key | Value |
| :--- | :--- |
| `X-Request-Source` | `axonhub-gateway` |
| `X-Internal-User` | `{{index .Metadata "internal_id"}}` |

## Notes & Limitations

- **Stream Parameter**: The `stream` parameter in the request body cannot be overridden as it is managed by the AxonHub pipeline.
- **Header Security**: Be careful when overriding security-sensitive headers like `Authorization`.
- **Invalid Templates**: If a template fails to parse or execute, the original raw value will be used, and a warning will be logged.
