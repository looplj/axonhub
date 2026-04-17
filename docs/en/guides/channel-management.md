# Channel Configuration Guide

This guide explains how to configure AI provider channels (like OpenAI, Anthropic, DeepSeek) in AxonHub.

## What is a Channel?

A **channel** is AxonHub's connection to an AI provider. Think of it as a "provider connection line" — each channel connects to one AI service (like OpenAI, Claude, or DeepSeek).

Through channels, you can:
- Connect to multiple AI providers simultaneously
- Set up model name conversion rules
- Enable or disable providers
- Configure multiple API Keys for load balancing

## Where Channel Model Mapping Fits in the Request Flow

Channel model mapping is the **last** step in a three-layer pipeline. For the full picture, see [Request Processing Guide](../getting-started/request-processing.md#core-concept-three-layers-of-model-settings).

In short: **API Key Profile renames → Model Association selects channel → Channel renames → Send upstream**

## Creating a Channel

### Basic Steps

1. Go to AxonHub management interface → **Channel Management**
2. Click **New Channel**
3. Fill in basic information:
   - **Name**: Give your channel a name (e.g., "OpenAI Main", "DeepSeek Backup")
   - **Type**: Select provider type (OpenAI, Anthropic, DeepSeek, etc.)
   - **Base URL**: API address (usually use the default)
   - **API Key**: The key from your provider

### Configuration Examples

**OpenAI Channel:**

| Field | Value |
|-------|-------|
| Name | OpenAI Main |
| Type | openai |
| Base URL | https://api.openai.com/v1 |
| API Key | sk-your-openai-key |
| Supported Models | gpt-4o, gpt-4o-mini, gpt-5 |

**DeepSeek Channel:**

| Field | Value |
|-------|-------|
| Name | DeepSeek China |
| Type | deepseek |
| Base URL | https://api.deepseek.com/v1 |
| API Key | sk-your-deepseek-key |
| Supported Models | deepseek-chat, deepseek-reasoner |

## Multiple API Keys

When an account has multiple API Keys, you can configure them all in the same channel. AxonHub will automatically rotate between them for better stability.

Simply add all keys in the API Keys field, one per line:
```
sk-key-1
sk-key-2
sk-key-3
```

### Load Balancing

- Same Trace ID always uses the same Key (session consistency)
- Different requests randomly select from available Keys
- If one Key fails, the system automatically switches to another

## Model Mapping

**When do you need model mapping?**

When you want the client to use one name, but send a different name to the upstream provider.

**Common Scenarios:**

1. **Client uses simplified names**: Client requests `gpt-4`, but OpenAI receives `gpt-4o`
2. **Unify model names across channels**: Both `claude-sonnet` and `gpt-4` point to the same actual model
3. **Legacy compatibility**: Old model names automatically map to newer versions

### How to Configure

In the channel's **Settings** → **Model Mappings**:

| From (Client Requests) | To (Sent to Provider) |
|------------------------|----------------------|
| gpt-4o-mini | gpt-4o |
| claude-3-sonnet | claude-3.5-sonnet |

**Note**: The target model (To) must be in the Supported Models list.

## Testing and Enabling Channels

### Test Connection

Before enabling a channel, test the connection:

1. Find your channel in the channel list
2. Click the **Test** button
3. Wait for the result
4. If successful, proceed to enable

### Enable Channel

After testing passes, click **Enable**. The channel status changes to **Active** and can now receive requests.

## Real-World Scenarios

### Scenario 1: Claude Code with OpenRouter

You want to use OpenRouter models in Claude Code:

1. **Create OpenRouter Channel**:
   - Type: `openai` (OpenRouter is OpenAI-compatible)
   - Base URL: `https://openrouter.ai/api/v1`
   - API Key: Your OpenRouter key
   - Supported Models: `anthropic/claude-3.5-sonnet`, `anthropic/claude-3-opus`

2. **Configure API Key Model Mapping** (in API Key management):
   - From: `claude-sonnet-4-5` → To: `anthropic/claude-3.5-sonnet`
   - From: `claude-opus-4-5` → To: `anthropic/claude-3-opus`

3. **Claude Code Configuration**:
   ```bash
   export ANTHROPIC_AUTH_TOKEN="your-axonhub-api-key"
   export ANTHROPIC_BASE_URL="http://localhost:8090/anthropic"
   ```

### Scenario 2: Multi-Provider Backup

Configure OpenAI as primary, DeepSeek as backup:

1. **Create OpenAI Channel** (Weight: 10, Priority: 0)
2. **Create DeepSeek Channel** (Weight: 5, Priority: 10)
3. **Configure Model Association**:
   - Set OpenAI as Priority 0 (primary)
   - Set DeepSeek as Priority 10 (backup)

### Scenario 3: Cost Optimization

Route expensive model requests to cheaper alternatives:

In API Key configuration:
- From: `gpt-4` → To: `claude-3-sonnet`
- From: `gpt-4-turbo` → To: `deepseek-reasoner`

## Base URL Special Configuration

### Default URLs

| Provider | Default Base URL |
|----------|-----------------|
| OpenAI | `https://api.openai.com/v1` |
| Anthropic | `https://api.anthropic.com` |
| DeepSeek | `https://api.deepseek.com/v1` |
| Gemini | `https://generativelanguage.googleapis.com/v1beta` |

### Custom URLs

For proxies or private deployments, you can modify the Base URL.

**Disable automatic version appending**: Add `#` at the end
```
https://custom-proxy.example.com/api#
# Actual request: /api/messages (no /v1 added)
```

**Fully raw mode**: Add `##` at the end
```
https://custom-gateway.example.com/api##
# Actual request: /api (no version or endpoint added)
```

## FAQ

### Q: Connection test failed?

- Check if the API Key is correct (no extra spaces when copying)
- Verify the Base URL is accessible
- Check if your provider account has sufficient credits

### Q: "Model not found" error?

- Confirm the model is in the Supported Models list
- Check if model mapping is configured correctly
- Verify the channel is enabled

### Q: How to set up multiple API Keys?

Enter all keys in the API Keys field, one per line. The system will automatically rotate them.

### Q: How to restore a disabled API Key?

Go to channel details, find the key in the **Disabled List**, and click **Restore**.

## Related Documentation

- [Model Management Guide](model-management.md) - Configure model-channel associations
- [API Key Profiles Guide](api-key-profiles.md) - Configure model mappings and permissions
- [Request Processing Guide](../getting-started/request-processing.md) - Understand the full request flow
