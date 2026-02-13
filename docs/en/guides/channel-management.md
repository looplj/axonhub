# Channel Configuration Guide

This guide explains how to configure AI provider channels in AxonHub. Channels are the bridge between your applications and AI model providers.

## Overview

Each channel represents a connection to an AI provider (OpenAI, Anthropic, Gemini, etc.). Through channels, you can:

- Connect to multiple AI providers simultaneously
- Configure model mappings and request parameter overrides
- Enable/disable channels dynamically
- Test connections before enabling
- Configure multiple API Keys for load balancing

## Channel Configuration

### Basic Configuration

Configure AI provider channels in the management interface:

```yaml
# OpenAI channel example
name: "openai"
type: "openai"
base_url: "https://api.openai.com/v1"
credentials:
  api_keys:
    - "sk-your-openai-key-1"
    - "sk-your-openai-key-2"
    - "sk-your-openai-key-3"
supported_models: ["gpt-5", "gpt-4o"]
```

### Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique channel identifier |
| `type` | string | Yes | Provider type (openai, anthropic, gemini, etc.) |
| `base_url` | string | Yes | API endpoint URL |
| `credentials` | object | Yes | Authentication credentials (supports multiple API Keys) |
| `supported_models` | array | Yes | List of models this channel supports |
| `settings` | object | No | Advanced settings (mappings, overrides, etc.) |

## Multiple API Key Configuration

AxonHub supports configuring multiple API Keys for a single channel to achieve automatic load balancing and failover.

### Configuration Method

```yaml
# Multiple API Key configuration example
credentials:
  api_keys:
    - "sk-your-key-1"
    - "sk-your-key-2"
    - "sk-your-key-3"
```

### Load Balancing Strategy

When multiple API Keys are configured, AxonHub uses the following strategies:

| Scenario | Strategy | Description |
| :--- | :--- | :--- |
| With Trace ID | Consistent Hashing | Requests with the same Trace ID always use the same Key |
| Without Trace ID | Random Selection | Randomly select from available Keys |

### API Key Management

#### Disabling API Key

When an API Key encounters an error (such as quota exhausted, banned), the system will automatically or manually disable it:

- Disabled Keys will no longer be used for new requests
- The system will automatically switch to other available Keys
- Disable information includes error code and reason

#### Enabling API Key

You can manually re-enable a previously disabled API Key:

- Remove the Key from the disabled list
- The Key will rejoin load balancing

#### Deleting API Key

You can completely delete an API Key that is no longer in use:

- Delete from both the disabled list and credentials
- At least one available API Key must be retained

### Backward Compatibility

AxonHub still supports single API Key configuration (legacy format), and the system will automatically handle compatibility:

```yaml
# Single API Key (legacy format, still supported)
credentials:
  api_key: "sk-your-single-key"

# Equivalent to
credentials:
  api_keys:
    - "sk-your-single-key"
```

## Testing Connection

Before enabling a channel, test the connection to ensure credentials are correct:

1. Navigate to **Channel Management** in the management interface
2. Click the **Test** button next to your channel
3. Wait for the test result
4. If successful, proceed to enable the channel

## Enabling a Channel

After successful testing, enable the channel:

1. Click the **Enable** button
2. The channel status will change to **Active**
3. The channel is now available for routing requests

## Model Mappings

When the requested model name differs from the upstream provider's supported names, you can use model mapping to automatically rewrite the model at the gateway side.

### Use Cases

- Map unsupported or legacy model IDs to available alternative models
- Set fallback logic for multi-channel scenarios (different channels for different providers)
- Simplify model names for your applications

### Configuration

```yaml
# Example: map product-specific aliases to upstream models
settings:
  modelMappings:
    - from: "gpt-4o-mini"
      to: "gpt-4o"
    - from: "claude-3-sonnet"
      to: "claude-3.5-sonnet"
```

### Rules

- AxonHub only accepts mappings to models already declared in `supported_models`
- Mappings are applied in order; the first matching mapping is used
- If no mapping matches, the original model name is used

## Request Override

Request Override allows you to enforce channel-specific default parameters or dynamically modify requests using templates. The following operation types are supported:

| Operation Type | Description |
| :--- | :--- |
| `set` | Set field value |
| `delete` | Delete field |
| `rename` | Rename field |
| `copy` | Copy field |

### Request Body Override Example

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

### Request Header Override Example

```json
[
  {
    "op": "set",
    "path": "X-Custom-Header",
    "value": "{{.Model}}"
  }
]
```

For detailed information on how to use templates, conditional logic, and more advanced features, see the [Request Override Guide](request-override.md).

## Best Practices

1. **Test before enabling**: Always test connections before enabling channels
2. **Use meaningful names**: Use descriptive channel names for easy identification
3. **Configure multiple API Keys**: Configure multiple API Keys for production channels to improve availability
4. **Monitor Key status**: Regularly check API Key usage and disabled status
5. **Document mappings**: Keep track of model mappings for maintenance
6. **Monitor usage**: Regularly review channel usage and performance
7. **Backup credentials**: Store credentials securely and have backup plans

## Troubleshooting

### Connection Test Fails

- Verify API key is correct and active
- Check if the API endpoint is accessible
- Ensure the account has sufficient credits/quota

### Model Not Found

- Verify the model is listed in `supported_models`
- Check if model mappings are correctly configured
- Confirm the model is available in the provider's catalog

### Override Parameters Not Working

- Ensure JSON is valid (use a JSON validator)
- Check that field names match the provider's API specification
- Verify nested fields use correct dot notation

### API Key Frequently Disabled

- Check if the API Key has sufficient quota
- View the disable reason and error code
- Consider increasing the number of API Keys to distribute load

## Base URL Configuration

### Overview

`base_url` is a required field in channel configuration, used to specify the API endpoint address of the AI provider. AxonHub supports flexible URL configuration options to accommodate different deployment scenarios.

### Default Base URLs

Each channel type has a preset default Base URL that is automatically populated when you create a channel:

| Channel Type | Default Base URL |
|-------------|------------------|
| openai | `https://api.openai.com/v1` |
| anthropic | `https://api.anthropic.com` |
| gemini | `https://generativelanguage.googleapis.com/v1beta` |
| deepseek | `https://api.deepseek.com/v1` |
| moonshot | `https://api.moonshot.cn/v1` |
| ... | See configuration interface for other types |

### Custom Base URL

You can configure a custom Base URL to support:

- **Third-party proxy services**: Access models through OpenAI/Anthropic-compatible proxy services
- **Private deployments**: Connect to internally deployed AI services within your organization
- **Multi-region deployments**: Use API endpoints from different regions

### Special Suffixes

AxonHub supports adding special suffixes to the end of the Base URL to control URL normalization behavior:

#### `#` Suffix - Disable Automatic Version Appending

Adding `#` at the end of the Base URL tells the system **not** to automatically append the API version:

```yaml
# Anthropic channel example - use raw URL without auto-appending /v1
base_url: "https://custom-api.example.com/anthropic#"

# Actual request URL: https://custom-api.example.com/anthropic/messages
# Instead of: https://custom-api.example.com/anthropic/v1/messages
```

**Use Cases**:
- Using custom proxy services where the URL path already includes version information
- Providers using non-standard URL structures
- Need full control over the request path

#### `##` Suffix - Fully Raw Mode (OpenAI Format)

Adding `##` at the end of the Base URL tells the system to:
1. Disable automatic version appending
2. Disable automatic endpoint appending (e.g., `/chat/completions`)

```yaml
# OpenAI channel example - fully raw mode
base_url: "https://custom-gateway.example.com/api/v2##"

# Actual request URL: https://custom-gateway.example.com/api/v2
# Instead of: https://custom-gateway.example.com/api/v2/v1/chat/completions
```

**Use Cases**:
- Using fully custom API gateways
- Need precise control over the complete request URL
- Compatible with special proxy or relay services

### Automatic Version Appending Rules

When not using `#` or `##` suffixes, the system automatically appends API versions based on channel type:

| Channel Type | Automatically Appended Version |
|-------------|-------------------------------|
| openai, deepseek, moonshot, xai, etc. | `/v1` |
| gemini | `/v1beta` |
| doubao | `/v3` |
| zai, zhipu | `/v4` |
| anthropic | `/v1` |
| anthropic_aws (Bedrock) | Not appended |
| anthropic_gcp (Vertex) | Not appended |

### Configuration Examples

```yaml
# Standard configuration - use default behavior
name: "openai-standard"
type: "openai"
base_url: "https://api.openai.com"
# Actual request: https://api.openai.com/v1/chat/completions

# Disable version appending - Anthropic
name: "anthropic-custom"
type: "anthropic"
base_url: "https://api.anthropic.com#"
# Actual request: https://api.anthropic.com/messages

# Fully raw mode - OpenAI
name: "openai-raw"
type: "openai"
base_url: "https://gateway.example.com/proxy##"
# Actual request: https://gateway.example.com/proxy
```

## Related Documentation

- [Request Override Guide](request-override.md) - Advanced request modification with templates
- [Model Management Guide](model-management.md) - Managing models across channels
- [Load Balancing Guide](load-balance.md) - Distributing requests across channels
- [API Key Profiles Guide](api-key-profiles.md) - Organizing API keys and permissions
