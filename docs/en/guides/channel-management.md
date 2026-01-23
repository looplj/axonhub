# Channel Configuration Guide

This guide explains how to configure AI provider channels in AxonHub. Channels are the bridge between your applications and AI model providers.

## Overview

Each channel represents a connection to an AI provider (OpenAI, Anthropic, Gemini, etc.). Through channels, you can:

- Connect to multiple AI providers simultaneously
- Configure model mappings and request overrides
- Enable/disable channels dynamically
- Test connections before enabling

## Channel Configuration

### Basic Configuration

Configure AI provider channels in the management interface:

```yaml
# OpenAI channel example
name: "openai"
type: "openai"
base_url: "https://api.openai.com/v1"
credentials:
  api_key: "your-openai-key"
supported_models: ["gpt-5", "gpt-4o"]
```

### Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique channel identifier |
| `type` | string | Yes | Provider type (openai, anthropic, gemini, etc.) |
| `base_url` | string | Yes | API endpoint URL |
| `credentials` | object | Yes | Authentication credentials |
| `supported_models` | array | Yes | List of models this channel supports |
| `settings` | object | No | Advanced settings (mappings, overrides) |

## Testing Connection

Before enabling a channel, test the connection to ensure credentials are correct:

1. Navigate to **Channels** in the management interface
2. Click the **Test** button next to your channel
3. Wait for the test result
4. If successful, proceed to enable the channel

## Enabling a Channel

After successful testing, enable the channel:

1. Click the **Enable** button
2. The channel status will change to **Active**
3. The channel is now available for routing requests

## Model Mappings

Use model mappings when the requested model name differs from the upstream provider's supported names. AxonHub transparently rewrites the request model before it leaves the gateway.

### Use Cases

- Map unsupported or legacy model IDs to the closest available alternative
- Implement failover by configuring multiple channels with different providers
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

- AxonHub only accepts mappings where the `to` model is already declared in `supported_models`
- Mappings are applied in order; the first matching mapping is used
- If no mapping matches, the original model name is used

## Request Override

Request Override lets you enforce channel-specific defaults or dynamically modify requests using templates. You can provide a JSON object for body parameters and configure custom HTTP headers.

For detailed information on how to use templates, dynamic JSON, and field removal, see the [Request Override Guide](request-override.md).

## Channel Types

### OpenAI

```yaml
type: "openai"
base_url: "https://api.openai.com/v1"
credentials:
  api_key: "sk-..."
```

### Anthropic

```yaml
type: "anthropic"
base_url: "https://api.anthropic.com/v1"
credentials:
  api_key: "sk-ant-..."
```

### Gemini

```yaml
type: "gemini"
base_url: "https://generativelanguage.googleapis.com/v1beta"
credentials:
  api_key: "..."
```

### OpenRouter

```yaml
type: "openrouter"
base_url: "https://openrouter.ai/api/v1"
credentials:
  api_key: "sk-or-..."
```

### Zhipu

```yaml
type: "zhipu"
base_url: "https://open.bigmodel.cn/api/paas/v4"
credentials:
  api_key: "..."
```

## Best Practices

1. **Test before enabling**: Always test connections before enabling channels
2. **Use meaningful names**: Use descriptive channel names for easy identification
3. **Document mappings**: Keep track of model mappings for maintenance
4. **Monitor usage**: Regularly review channel usage and performance
5. **Backup credentials**: Store credentials securely and have backup plans

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

## Related Documentation

- [Request Override Guide](request-override.md) - Advanced request modification with templates
- [Model Management Guide](model-management.md) - Managing models across channels
- [Load Balancing Guide](load-balance.md) - Distributing requests across channels
- [API Key Profiles Guide](api-key-profiles.md) - Organizing API keys and permissions
