# Model Management Guide

AxonHub provides a flexible model management system that supports mapping abstract models to specific channels and model implementations through Model Associations, enabling a unified model interface and intelligent channel selection.

## 🎯 Core Concepts

### Model
A Model in AxonHub is an abstract model definition containing:
- **ModelID** - Unique model identifier (e.g., `gpt-4`, `claude-3-opus`)
- **Developer** - Model developer (e.g., `openai`, `anthropic`)
- **Settings** - Model settings, including association rules
- **ModelCard** - Model metadata (capabilities, costs, limits, etc.)

### Channel
A Channel is a specific AI service provider connection containing:
- List of supported models
- API configuration and authentication
- Channel weight and tags

### Model Association
Model Associations define the mapping relationships between abstract models and channels, supporting multiple matching strategies.

## 🔗 Model-Channel Relationship

### Relationship Diagram

```
┌─────────────────┐
│   AxonHub Model │ (Abstract Model)
│   ID: gpt-4     │
└────────┬────────┘
         │
         │ Associations (Association Rules)
         │
         ▼
┌─────────────────────────────────────────────────┐
│          ModelChannelConnection                 │
│  ┌──────────────┐  ┌──────────────┐           │
│  │ Channel A    │  │ Channel B    │           │
│  │ gpt-4-turbo  │  │ gpt-4        │           │
│  │ Priority: 0   │  │ Priority: 1  │           │
│  └──────────────┘  └──────────────┘           │
└─────────────────────────────────────────────────┘
         │
         │ Load Balancer
         │
         ▼
┌─────────────────┐
│   Selected     │ (Selected Candidate)
│   Candidate     │
└─────────────────┘
```

### Request Flow

1. **Request Arrives** - Client requests to use model `gpt-4`
2. **Model Query** - System looks up the Model entity with ModelID `gpt-4`
3. **Association Resolution** - Resolves matching channels and models based on model's Associations
4. **Candidate Generation** - Generates `ChannelModelCandidate` list
5. **Load Balancing** - Applies load balancing strategies to sort candidates
6. **Request Execution** - Executes request using the first candidate
7. **Failure Retry** - Switches to next candidate on failure

## 📋 Model Association Types

### 1. channel_model - Specific Channel, Specific Model

Precisely matches a specific model in a specific channel.

```json
{
  "type": "channel_model",
  "priority": 0,
  "channelModel": {
    "channelId": 1,
    "modelId": "gpt-4-turbo"
  }
}
```

**Use Cases**:
- Need precise control over which channel a model uses
- Primary channel with highest priority

### 2. channel_regex - Specific Channel, Regex Match

Matches all models matching the regex pattern in a specific channel.

```json
{
  "type": "channel_regex",
  "priority": 1,
  "channelRegex": {
    "channelId": 2,
    "pattern": "gpt-4.*"
  }
}
```

**Pattern Syntax**: Patterns are automatically anchored with `^` and `$` to match the entire model string. Use `.*` for flexible matching:
- `gpt-4.*` - Matches `gpt-4`, `gpt-4-turbo`, `gpt-4-vision-preview`
- `.*flash.*` - Matches `gemini-2.5-flash-preview`, `gemini-flash-2.0`
- `claude-3-.*-sonnet` - Matches `claude-3-5-sonnet`, `claude-3-opus-sonnet`

**Use Cases**:
- Channel supports multiple model variants
- Need flexible model name matching

### 3. regex - All Channels, Regex Match

Matches models matching the regex pattern across all enabled channels.

```json
{
  "type": "regex",
  "priority": 2,
  "regex": {
    "pattern": "gpt-4.*",
    "exclude": [
      {
        "channelNamePattern": ".*test.*",
        "channelIds": [5, 6],
        "channelTags": ["beta"]
      }
    ]
  }
}
```

**Exclusion Rules**:
- `channelNamePattern` - Exclude channels with matching names
- `channelIds` - Exclude channels with specific IDs
- `channelTags` - Exclude channels with specific tags

**Use Cases**:
- Broadly match models across multiple channels
- Need to exclude specific channels

### 4. model - All Channels, Specific Model

Matches a specific model across all enabled channels.

```json
{
  "type": "model",
  "priority": 3,
  "modelId": {
    "modelId": "gpt-4",
    "exclude": [
      {
        "channelNamePattern": ".*backup.*",
        "channelIds": [10],
        "channelTags": ["low-priority"]
      }
    ]
  }
}
```

**Use Cases**:
- All channels supporting the model can be candidates
- Need to exclude specific channels

### 5. channel_tags_model - Tagged Channels, Specific Model

Matches a specific model in channels with specified tags (OR logic).

```json
{
  "type": "channel_tags_model",
  "priority": 4,
  "channelTagsModel": {
    "channelTags": ["production", "high-performance"],
    "modelId": "gpt-4"
  }
}
```

**Use Cases**:
- Select candidates based on channel tags
- Environment isolation (production/test)

### 6. channel_tags_regex - Tagged Channels, Regex Match

Matches models matching the regex pattern in channels with specified tags (OR logic).

```json
{
  "type": "channel_tags_regex",
  "priority": 5,
  "channelTagsRegex": {
    "channelTags": ["openai", "azure"],
    "pattern": "gpt-4.*"
  }
}
```

**Use Cases**:
- Select based on provider tags
- Flexible matching of model variants

## 🚀 Quick Start

### 1. Create a Model

Models are created and managed via the Admin UI.

### 2. Configure Channels

Ensure channels are configured and support the corresponding models via the Admin UI.

### 3. Use the Model

Clients use the abstract ModelID for requests:

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

response = client.chat.completions.create(
    model="gpt-4",  # Use abstract ModelID
    messages=[{"role": "user", "content": "Hello!"}]
)
```

The system will automatically:
1. Look up the `gpt-4` model entity
2. Resolve association rules
3. Select the optimal channel
4. Map to the actual model (e.g., `gpt-4-turbo`)

## 🎛️ Advanced Configuration

### Priority Control

Associations are processed in priority order, with smaller priority values being higher priority. When multiple candidates have the same priority, the system uses the channel's **weight** to perform load balancing.

For detailed load balancing logic and weight-based distribution, see the [Weight Round Robin Strategy](load-balance.md#weight-round-robin-strategy) in the Adaptive Load Balancing Guide.

```json
{
  "settings": {
    "associations": [
      {
        "type": "channel_model",
        "priority": 0,  // Highest priority
        "channelModel": {
          "channelId": 1,
          "modelId": "gpt-4-turbo"
        }
      },
      {
        "type": "regex",
        "priority": 10,  // Lower priority
        "regex": {
          "pattern": "gpt-4.*"
        }
      }
    ]
  }
}
```

### Deduplication

The system automatically deduplicates, so the same (channel, model) combination will only appear once.

### Cache Optimization

Association resolution results are cached. Cache invalidation conditions:
- Channel count changes
- Channel update time changes
- Model update time changes
- Cache expires (5 minutes)

### Fallback Strategy

When a model doesn't exist, it can fall back to traditional channel selection:

```json
{
  "queryAllChannelModels": true,
  "fallbackToChannelsOnModelNotFound": true
}
```

## 📊 Monitoring and Debugging

### Debug Logs

```bash
# View model selection logs
tail -f axonhub.log | grep "selected model candidates"

# View association resolution logs
tail -f axonhub.log | grep "association resolution"

# View candidate selection logs
tail -f axonhub.log | grep "Load balanced candidates"
```

## 🔧 Best Practices

### 1. Model Design

- **Use Standardized ModelIDs** - e.g., `gpt-4`, `claude-3-opus`
- **Reasonable Priority Settings** - Primary channels 0-10, backup channels 10-100
- **Flexible Regex Usage** - Avoid overly broad patterns

### 2. Channel Organization

- **Use Tags for Classification** - e.g., `production`, `test`, `backup`
- **Set Reasonable Weights** - Use in conjunction with load balancing
- **Regular Cleanup** - Remove unused channels

### 3. Association Strategies

- **Precision First** - Use `channel_model` for primary channels
- **Flexible Backup** - Use `regex` or `model` for backup
- **Environment Isolation** - Use tag associations to separate production/test environments

### 4. Performance Optimization

- **Leverage Caching** - Association resolution results are cached
- **Avoid Frequent Updates** - Minimize update frequency for models and channels
- **Monitor Candidate Count** - Too many candidates can impact performance

## 🐛 Common Issues

### Q: Why did the request fail?

A: Check the following:
1. Does the model exist and is it enabled?
2. Are association rules correctly configured?
3. Are channels enabled and support the corresponding models?
4. Check logs for specific errors

### Q: How do I verify associations are working?

A:
1. Use the Admin UI to verify association results
2. Enable debug logs to view the candidate selection process
3. Send test requests to verify routing

### Q: How many association rules are supported?

A: Theoretically unlimited, but recommended:
- No more than 10 associations per model
- Total associations not exceeding 100
- Avoid overly complex regex patterns

### Q: How to implement A/B testing?

A: Use multiple associations with the same priority:

```json
{
  "settings": {
    "associations": [
      {
        "type": "channel_model",
        "priority": 0,
        "channelModel": {
          "channelId": 1,
          "modelId": "gpt-4-turbo"
        }
      },
      {
        "type": "channel_model",
        "priority": 0,
        "channelModel": {
          "channelId": 2,
          "modelId": "gpt-4"
        }
      }
    ]
  }
}
```

Load balancing will select among candidates with the same priority.

### Q: How to exclude specific channels?

A: Use the `exclude` field:

```json
{
  "type": "regex",
  "regex": {
    "pattern": "gpt-4.*",
    "exclude": [
      {
        "channelNamePattern": ".*test.*",
        "channelIds": [5, 6],
        "channelTags": ["beta"]
      }
    ]
  }
}
```

## 🔗 Related Documentation

- [Adaptive Load Balancing Guide](load-balance.md)
- [API Key Profiles Guide](api-key-profiles.md)
- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)
- [Tracing and Debugging](tracing.md)
