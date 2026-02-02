# API Key Profile Guide

## Overview

API Key Profile is a powerful configuration system that allows you to define multiple profiles for each API Key. Each profile contains model mappings, channel restrictions, and model access controls. By switching the active profile, you can change how requests are processed without modifying the API Key itself.

## What is API Key Profile?

API Key Profile enables you to:

- **Map Models**: Transform user-requested models to actual available models using exact match or regex patterns
- **Restrict Channels**: Limit which channels a profile can use by channel IDs or tags
- **Filter Models**: Control which models are accessible through a specific profile
- **Switch Profiles**: Change behavior on-the-fly by activating different profiles

### Core Concepts

| Component | Description |
|-----------|-------------|
| **Profile** | A named configuration containing model mappings and access rules |
| **Active Profile** | The currently enabled profile that processes requests |
| **Model Mapping** | Rules that transform source models to target models |
| **Channel Restrictions** | Limits on which channels can be used by a profile |

## Why Use API Key Profiles?

### 1. Model Transformation

Convert user-facing model names to internal model identifiers or alternative providers:

```json
{
  "profiles": [
    {
      "name": "production",
      "modelMappings": [
        {
          "from": "gpt-4",
          "to": "claude-3-opus"
        },
        {
          "from": "gpt-3.5-turbo",
          "to": "claude-3-haiku"
        }
      ]
    }
  ]
}
```

### 2. Environment Isolation

Maintain separate configurations for development, staging, and production:

```json
{
  "profiles": [
    {
      "name": "development",
      "channelTags": ["dev"],
      "modelMappings": [
        {"from": ".*", "to": "gpt-3.5-turbo"}
      ]
    },
    {
      "name": "production",
      "channelTags": ["prod"],
      "modelMappings": [
        {"from": ".*", "to": "gpt-4"}
      ]
    }
  ],
  "activeProfile": "development"
}
```

### 3. Access Control

Restrict API Keys to specific channels or models:

```json
{
  "profiles": [
    {
      "name": "restricted",
      "channelIDs": [1, 2, 3],
      "modelIDs": ["gpt-4", "claude-3-opus"]
    }
  ]
}
```

### 4. A/B Testing

Test different models or providers by switching profiles:

```json
{
  "profiles": [
    {
      "name": "group-a",
      "modelMappings": [
        {"from": ".*", "to": "gpt-4"}
      ]
    },
    {
      "name": "group-b",
      "modelMappings": [
        {"from": ".*", "to": "claude-3-opus"}
      ]
    }
  ]
}
```

## How to Use API Key Profiles

### Step 1: Access Profile Configuration

1. Navigate to the **API Keys** page in the management interface
2. Locate the API Key you want to configure
3. Click the **Actions** menu (three dots) on the right side
4. Select **Profiles** or **配置** from the dropdown

### Step 2: Create Profiles

The profiles dialog allows you to:

- **Add Profile**: Click "Add Profile" to create a new configuration
- **Set Profile Name**: Give each profile a descriptive name
- **Configure Model Mappings**: Add rules to transform models
- **Set Channel Restrictions**: Choose allowed channels by ID or tag
- **Set Model Restrictions**: Specify which models are accessible

### Step 3: Configure Model Mappings

Each model mapping consists of:

- **Source Model (From)**: The model name in the user's request
  - Supports exact match: `"gpt-4"`
  - Supports regex: `"gpt-.*"`, `".*"` (wildcard)
- **Target Model (To)**: The actual model to use

**Example Mappings:**

```json
{
  "modelMappings": [
    {"from": "gpt-4", "to": "claude-3-opus"},
    {"from": "gpt-3.5-turbo", "to": "claude-3-haiku"},
    {"from": "gpt-.*", "to": "claude-3-sonnet"}
  ]
}
```

### Step 4: Set Active Profile

Choose which profile should be active:

1. Select the profile name from the "Active Profile" dropdown
2. The selected profile will process all incoming requests
3. You can change the active profile at any time

### Step 5: Save Configuration

Click **Save** to apply your changes. The API Key will immediately start using the new configuration.

## Model Mapping Details

### Exact Match

Maps a specific model name:

```json
{"from": "gpt-4", "to": "claude-3-opus"}
```

This will only transform requests for `gpt-4` to `claude-3-opus`.

### Regex Pattern

Use regex to match multiple models:

```json
{"from": "gpt-.*", "to": "claude-3-sonnet"}
```

This will transform any model starting with `gpt-` to `claude-3-sonnet`.

### Wildcard

Match all models:

```json
{"from": ".*", "to": "gpt-4"}
```

This will transform all requests to use `gpt-4`.

### Evaluation Order

Model mappings are evaluated in order. The first matching rule is applied:

```json
{
  "modelMappings": [
    {"from": "gpt-4", "to": "claude-3-opus"},
    {"from": "gpt-.*", "to": "claude-3-sonnet"}
  ]
}
```

In this example:
- `gpt-4` → `claude-3-opus` (exact match takes precedence)
- `gpt-3.5-turbo` → `claude-3-sonnet` (regex match)

## Channel and Model Restrictions

### Channel Restrictions

Control which channels a profile can use:

**By Channel IDs:**

```json
{
  "channelIDs": [1, 2, 3]
}
```

**By Channel Tags:**

```json
{
  "channelTags": ["production", "high-priority"]
}
```

**Combined:**

```json
{
  "channelIDs": [1, 2],
  "channelTags": ["production"]
}
```

If both are specified, channels must match either criteria.

### Model Restrictions

Limit which models can be accessed:

```json
{
  "modelIDs": ["gpt-4", "claude-3-opus", "claude-3-sonnet"]
}
```

Only models in this list will be accessible through this profile.

## Validation Rules

### Profile Names

- Must be unique within the API Key (case-insensitive)
- Cannot be empty or contain only whitespace
- Example: `"production"`, `"staging"`, `"development"`

### Active Profile

- Must reference an existing profile name
- Cannot be set to a non-existent profile

### Model Mappings

- Source model cannot be empty
- Target model cannot be empty
- Patterns are validated as valid regex

## API Examples

### Using Profiles with OpenAI SDK

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

# Request uses gpt-4, but will be mapped according to active profile
response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### Using Profiles with cURL

```bash
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-axonhub-api-key" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

The actual model used depends on the active profile's model mappings.

## Best Practices

### 1. Use Descriptive Profile Names

Choose names that clearly indicate the profile's purpose:

- ✅ `"production-gpt4"`, `"staging-claude"`, `"dev-low-cost"`
- ❌ `"profile1"`, `"test"`, `"config"`

### 2. Order Model Mappings Strategically

Place more specific patterns before general ones:

```json
{
  "modelMappings": [
    {"from": "gpt-4-turbo", "to": "claude-3-opus"},
    {"from": "gpt-4", "to": "claude-3-sonnet"},
    {"from": "gpt-.*", "to": "claude-3-haiku"}
  ]
}
```

### 3. Test Profiles Before Production

1. Create a test API Key
2. Configure profiles with the same settings
3. Test with sample requests
4. Verify model mappings work as expected
5. Apply to production API Key

### 4. Document Profile Changes

Maintain documentation for profile configurations:

```markdown
## API Key: production-service

### Profile: production
- Purpose: Production traffic
- Active: Yes
- Model Mappings:
  - gpt-4 → claude-3-opus
  - gpt-3.5-turbo → claude-3-haiku
- Channel Tags: ["production"]
```

### 5. Use Channel Tags for Flexibility

Instead of hardcoding channel IDs, use tags:

```json
{
  "channelTags": ["production", "high-availability"]
}
```

This allows you to add/remove channels without updating profiles.

## Troubleshooting

### Model Not Mapped

**Problem**: Requests use the original model instead of the mapped model.

**Solutions**:
- Verify the active profile is set correctly
- Check that model mappings include the requested model
- Ensure regex patterns match the model name
- Check logs for mapping application messages

### Profile Not Found

**Problem**: Error message "Active profile 'xxx' does not exist in the profiles list"

**Solutions**:
- Ensure the active profile name matches an existing profile
- Check for typos in profile names
- Verify the profile was saved successfully

### Channel Access Denied

**Problem**: Requests fail with channel access errors.

**Solutions**:
- Verify channel IDs are correct
- Check that channel tags match existing channels
- Ensure channels are enabled and healthy
- Review channel permissions

### Model Not Available

**Problem**: Requests fail because the mapped model doesn't exist.

**Solutions**:
- Verify the target model exists in the system
- Check that the model is enabled in the channel
- Ensure the model ID matches exactly (case-sensitive)
- Review model mappings for typos

## Advanced Use Cases

### Multi-Environment Setup

Configure different profiles for each environment:

```json
{
  "profiles": [
    {
      "name": "development",
      "channelTags": ["dev"],
      "modelMappings": [
        {"from": ".*", "to": "gpt-3.5-turbo"}
      ]
    },
    {
      "name": "staging",
      "channelTags": ["staging"],
      "modelMappings": [
        {"from": ".*", "to": "gpt-4"}
      ]
    },
    {
      "name": "production",
      "channelTags": ["production"],
      "modelMappings": [
        {"from": ".*", "to": "claude-3-opus"}
      ]
    }
  ],
  "activeProfile": "development"
}
```

### Cost Optimization

Route different models to cost-effective alternatives:

```json
{
  "profiles": [
    {
      "name": "cost-optimized",
      "modelMappings": [
        {"from": "gpt-4-turbo", "to": "claude-3-sonnet"},
        {"from": "gpt-4", "to": "claude-3-opus"},
        {"from": "gpt-3.5-turbo", "to": "claude-3-haiku"}
      ]
    }
  ]
}
```

### Provider Migration

Gradually migrate from one provider to another:

```json
{
  "profiles": [
    {
      "name": "openai",
      "modelMappings": [
        {"from": ".*", "to": "gpt-4"}
      ],
      "channelTags": ["openai"]
    },
    {
      "name": "anthropic",
      "modelMappings": [
        {"from": ".*", "to": "claude-3-opus"}
      ],
      "channelTags": ["anthropic"]
    }
  ],
  "activeProfile": "openai"
}
```

Switch to `"anthropic"` profile when ready to complete migration.

## Related Documentation

- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)
- [Load Balancing Guide](load-balance.md)
- [Fine-grained Permissions](permissions.md)
- [Quick Start Guide](../getting-started/quick-start.md)
