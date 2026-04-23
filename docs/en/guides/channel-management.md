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

## Model Renaming

AxonHub provides multiple mechanisms to rename or alias models at the channel level. When a request arrives, the channel resolves the request model to the actual upstream model through the following priority chain:

1. **Direct match** — the request model is directly in the Supported Models list
2. **Extra Model Prefix** — adds a prefix alias for all supported models
3. **Auto-Trimmed Model Prefixes** — strips known prefixes from supported models to create trimmed aliases
4. **Model Mappings** — explicit `from → to` alias pairs

> **Note**: If multiple mechanisms produce the same request model name, the first match wins (based on the order above).

### Model Mappings

**When do you need model mappings?**

When you want the client to use one name, but send a different name to the upstream provider.

**Common Scenarios:**

1. **Client uses simplified names**: Client requests `gpt-4`, but OpenAI receives `gpt-4o`
2. **Unify model names across channels**: Both `claude-sonnet` and `gpt-4` point to the same actual model
3. **Legacy compatibility**: Old model names automatically map to newer versions

**How to Configure:**

In the channel's **Settings** → **Model Mappings**, add `from → to` pairs:

| From (Client Requests) | To (Sent to Provider) |
|------------------------|----------------------|
| gpt-4o-mini | gpt-4o |
| claude-3-sonnet | claude-3.5-sonnet |

**Note**: The target model (`to`) must be in the Supported Models list. If the target model is not supported, the mapping is silently ignored.

### Extra Model Prefix

Adds a **prefixed alias** for every model in the Supported Models list, allowing clients to request models with or without the prefix.

**Use case**: You want to namespace all models in a channel under a common prefix (e.g., `deepseek/`).

**Example:**
- Supported Models: `deepseek-chat`, `deepseek-reasoner`
- Extra Model Prefix: `deepseek`

The channel now accepts **both** of the following request formats:
- `deepseek-chat` → sends `deepseek-chat` upstream
- `deepseek/deepseek-chat` → sends `deepseek-chat` upstream

This is useful when you want to differentiate models from different channels that share the same name — clients can prefix the model with the channel's namespace.

### Auto-Trimmed Model Prefixes

Automatically **strips specified prefixes** from model names in the Supported Models list, creating trimmed aliases. This is the inverse of Extra Model Prefix.

**Use case**: Providers like OpenRouter or SiliconFlow add vendor prefixes to model names (e.g., `openai/gpt-5.4`). You want clients to request using the short name `gpt-5.4` without manually creating mappings for every model.

**Example:**
- Supported Models: `openai/gpt-5.4`, `anthropic/claude-sonnet-4`, `deepseek-ai/deepseek-chat`
- Auto-Trimmed Model Prefixes: `openai`, `anthropic`, `deepseek-ai`

The channel now accepts both the original and trimmed names:
- `gpt-5.4` → sends `openai/gpt-5.4` upstream
- `claude-sonnet-4` → sends `anthropic/claude-sonnet-4` upstream
- `deepseek-chat` → sends `deepseek-ai/deepseek-chat` upstream
- `openai/gpt-5.4` → still works as a direct match

> **Tip**: This is the recommended approach for providers that use vendor-prefixed model IDs. It enables batch model ID rewriting without having to create individual model mappings.

### Visibility Controls

Two options control which model names are exposed in the model list (e.g., when clients call the `/v1/models` endpoint):

| Option | Effect |
|--------|--------|
| **Hide Original Models** | Hides the original (direct) model names. Only transformed names (from prefix, auto-trim, or mapping) are shown. |
| **Hide Mapped Models** | Hides the `from` names of model mappings. Only the original model names are shown. |

**Example — Hide Original Models:**
- Supported Models: `openai/gpt-5.4`
- Auto-Trimmed Prefixes: `openai`
- Hide Original Models: enabled

The `/v1/models` response only shows `gpt-5.4`, not `openai/gpt-5.4`. Both names still work for requests.

**Example — Hide Mapped Models:**
- Supported Models: `gpt-4o`
- Model Mapping: `gpt-4` → `gpt-4o`
- Hide Mapped Models: enabled

The `/v1/models` response shows `gpt-4o` but hides `gpt-4`. Both names still work for requests.

## Testing and Enabling Channels

### Test Connection

Before enabling a channel, test the connection:

1. Find your channel in the channel list
2. Click the **Test** button
3. Wait for the result
4. If successful, proceed to enable

### Enable Channel

After testing passes, click **Enable**. The channel status changes to **Active** and can now receive requests.

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
