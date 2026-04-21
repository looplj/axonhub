# Model Management Guide

This guide explains how to manage AI models in AxonHub and use "Model Associations" for intelligent routing.

## Core Concepts: Models and Channels

### Simple Analogy

Imagine you're sending a package:
- **Model** = The type of item you want to send (e.g., "document", "package")
- **Channel** = Different courier companies (e.g., "SF Express", "YTO")
- **Model Association** = Your rule: "Documents go via SF Express, packages via YTO; if SF fails, use YTO"

In AxonHub:
- **Model**: An abstract name you expose, like `gpt-4` or `claude-sonnet`
- **Channel**: An actual AI provider connection
- **Model Association**: Determines which channel and actual model to use when a client requests a model

### Request Flow

```
Client Request: "Please answer using gpt-4"
        ↓
System Lookup: What associations does gpt-4 have?
        ↓
Association Resolution: Priority 0 → OpenAI channel's gpt-4-turbo
                       Priority 1 → DeepSeek channel's deepseek-chat
        ↓
Load Balancing: Select best channel to execute request
```

## Where Model Association Fits in the Request Flow

Model Association is the **middle** step in a three-layer pipeline. For the full picture, see [Request Processing Guide](../getting-started/request-processing.md#core-concept-three-layers-of-model-settings).

In short: **API Key Profile renames → Model Association selects channel → Channel renames → Send upstream**

## Model Association Types

Model associations are "routing rules." AxonHub supports 6 rule types:

### 1. Specific Channel, Specific Model (Most Precise)

**Purpose**: Precise control over which model version goes through which channel

**Configuration in Admin UI:**
- Association Type: "Specific Channel Model"
- Priority: 0
- Channel: OpenAI (ID: 1)
- Model: gpt-4-turbo

**Scenario**: "When user wants gpt-4, prioritize using OpenAI channel's gpt-4-turbo"

### 2. Specific Channel, Regex Match (More Flexible)

**Purpose**: Match a batch of models in a specific channel

**Configuration in Admin UI:**
- Association Type: "Channel Regex Match"
- Priority: 1
- Channel: DeepSeek (ID: 2)
- Pattern: `gpt-4.*`

**Scenario**: "All models starting with gpt-4 in the DeepSeek channel are acceptable"

**Common Patterns:**
- `gpt-4.*` — Matches `gpt-4`, `gpt-4-turbo`, `gpt-4-vision`
- `claude-3-.*-sonnet` — Matches `claude-3-5-sonnet`, `claude-3-opus-sonnet`
- `.*` — Matches all models

### 3. All Channels, Regex Match (Most Flexible)

**Purpose**: Find matching models across all enabled channels

**Configuration in Admin UI:**
- Association Type: "Global Regex Match"
- Priority: 2
- Pattern: `gpt-4.*`
- Exclude Channels with Tags: `test`

**Scenario**: "Any gpt-4 series model from any channel, but exclude test channels"

### 4. All Channels, Specific Model

**Purpose**: Don't specify channel; any channel supporting this model can be used

**Configuration in Admin UI:**
- Association Type: "Global Model Match"
- Priority: 3
- Model: gpt-4

**Scenario**: "Any channel supporting gpt-4 can be a backup"

### 5. Tagged Channels, Specific Model

**Purpose**: Select based on channel tags (e.g., only production environment channels)

**Configuration in Admin UI:**
- Association Type: "Tagged Channel Model"
- Priority: 4
- Channel Tags: production, high-performance
- Model: gpt-4

**Scenario**: "Only look for gpt-4 in channels tagged as production or high-performance"

### 6. Tagged Channels, Regex Match

**Purpose**: Tag + regex combination

**Configuration in Admin UI:**
- Association Type: "Tagged Channel Regex"
- Priority: 5
- Channel Tags: openai, azure
- Pattern: `gpt-4.*`

**Scenario**: "Find gpt-4 series models in OpenAI or Azure channels"

## Priority Settings

**Smaller priority value = Higher priority**

Recommended settings:
- **Primary channels**: Priority 0-10
- **Backup channels**: Priority 10-50
- **Emergency channels**: Priority 50-100

Example configuration in Admin UI:
- Association 1: Priority 0 (Highest priority: Primary)
  - Type: Specific Channel Model
  - Channel: OpenAI
  - Model: gpt-4o
- Association 2: Priority 10 (Lower priority: Backup)
  - Type: Global Model Match
  - Model: gpt-4

## Real-World Scenarios

### Scenario 1: Primary-Backup Channel Setup

**Need**: Prioritize OpenAI, automatically switch to DeepSeek on failure

**Configuration:**
1. Create OpenAI channel and DeepSeek channel
2. In Model Management, for model "gpt-4", add associations:
   - Priority 0: Specific Channel Model → OpenAI channel → gpt-4o
   - Priority 10: Specific Channel Model → DeepSeek channel → deepseek-chat

### Scenario 2: Multi-Version Compatibility

**Need**: Client requests `gpt-4`, but any gpt-4 variant can be used

**Configuration:**
- Priority 0: Channel Regex Match → OpenAI channel → Pattern: `gpt-4.*`

### Scenario 3: Environment Isolation

**Need**: Production environment only uses production channels

**Setup:**
- Tag OpenAI channel: `production`
- Tag test channel: `test`

**Configuration:**
- Priority 0: Tagged Channel Model → Tag: production → Model: gpt-4

### Scenario 4: Exclude Specific Channels

**Need**: Use gpt-4 from all channels, but exclude test and backup channels

**Configuration:**
- Priority 0: Global Regex Match
  - Pattern: `gpt-4.*`
  - Exclude Channel Tags: test, backup

## System Settings

In **System Settings > Model Settings**, there are two important options:

| Setting | Default | Description |
|---------|---------|-------------|
| Query All Channel Models | Enabled | When enabled, `/v1/models` API returns all models from enabled channels + configured models |
| Fallback to Channels on Model Not Found | Enabled | When enabled, if requested model has no associations, system automatically finds channels supporting it |

**Recommendations:**
- For beginners: Keep both enabled, system handles most cases automatically
- For strict control: Disable both to only allow explicitly configured models

## FAQ

### Q: Why does the request say "Model not found"?

Check in order:
1. Is the model created?
2. Are model associations configured with correct channels?
3. Are channels enabled?
4. Do channels support the models specified in associations?

### Q: How to verify associations are working?

1. Send a test request
2. Check the Trace in the console to see which channel the request actually went through
3. Check logs for candidate selection records

### Q: Will too many associations affect performance?

Generally no significant impact, but recommended:
- No more than 10 association rules per model
- Avoid overly complex regex patterns

### Q: Will the same (channel, model) combination be duplicated?

No, the system automatically deduplicates.

## Best Practices

1. **Naming Convention**: Use standardized model names like `gpt-4`, `claude-3-opus`
2. **Priority Planning**: Primary 0-10, backup 10-50, emergency 50-100
3. **Use Tags**: Tag channels (e.g., production, test) for batch management
4. **Test Before Enabling**: Verify request routing in the console after configuration
5. **Regular Review**: Clean up unused models and association rules

## Related Documentation

- [Channel Management Guide](channel-management.md) - Configure AI provider channels
- [API Key Profiles Guide](api-key-profiles.md) - Configure model mappings
- [Load Balancing Guide](load-balance.md) - Learn about channel selection and failover
- [Request Processing Guide](../getting-started/request-processing.md) - Complete request flow explanation
