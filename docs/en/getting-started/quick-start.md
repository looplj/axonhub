# Quick Start Guide

## Overview

This guide explains how to configure a channel, create an API key, and make a first request in an AxonHub instance that is already running and initialized.

Prepare a valid API key from an AI provider such as OpenAI or Anthropic.

## First Steps

### 1. Configure Your First Channel

1. Log in to the web interface
2. Navigate to **Channels**
3. Click **Add Channel**
4. Select your provider (e.g., OpenAI)
5. Enter your API key and configuration
6. Test the connection
7. Enable the channel

### 2. Create an API Key

1. Navigate to **API Keys**
2. Click **Create API Key**
3. Give it a descriptive name
4. Select the appropriate scopes
5. Copy the generated API key

### 3. Make Your First API Call

AxonHub supports both OpenAI Chat Completions and Anthropic Messages APIs, allowing you to use your preferred API format to access any supported model.

#### Using OpenAI API Format

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

# Call OpenAI model
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[
        {"role": "user", "content": "Hello, AxonHub!"}
    ]
)
print(response.choices[0].message.content)

# Call Anthropic model using OpenAI API
response = client.chat.completions.create(
    model="claude-3-5-sonnet",
    messages=[
        {"role": "user", "content": "Hello, Claude!"}
    ]
)
print(response.choices[0].message.content)
```

#### Using Anthropic API Format

```python
import requests

# Call Anthropic model
response = requests.post(
    "http://localhost:8090/anthropic/v1/messages",
    headers={
        "Content-Type": "application/json",
        "X-API-Key": "your-axonhub-api-key"
    },
    json={
        "model": "claude-3-5-sonnet",
        "max_tokens": 512,
        "messages": [
            {
                "role": "user",
                "content": [{"type": "text", "text": "Hello, Claude!"}]
            }
        ]
    }
)
print(response.json()["content"][0]["text"])

# Call OpenAI model using Anthropic API
response = requests.post(
    "http://localhost:8090/anthropic/v1/messages",
    headers={
        "Content-Type": "application/json",
        "X-API-Key": "your-axonhub-api-key"
    },
    json={
        "model": "gpt-4o",
        "max_tokens": 512,
        "messages": [
            {
                "role": "user",
                "content": [{"type": "text", "text": "Hello, GPT!"}]
            }
        ]
    }
)
print(response.json()["content"][0]["text"])
```

#### Key Benefits of Unified API

- **API Interoperability**: Use OpenAI API to call Anthropic models, or Anthropic API to call OpenAI models
- **Zero Code Changes**: Keep using your existing OpenAI or Anthropic client SDKs
- **Automatic Translation**: AxonHub handles API format conversion automatically
- **Provider Flexibility**: Access any supported AI provider with your preferred API format

### 4. Advanced Channel Configuration

#### Model Mapping

Model mapping allows you to redirect requests for specific models to different upstream models. This is useful for:

- **Cost optimization**: Map expensive models to cheaper alternatives
- **Legacy support**: Map deprecated model names to current models
- **Provider switching**: Map models to different providers
- **Failover**: Configure multiple channels with different providers

**Example Model Mapping Configuration:**

```yaml
# In channel settings
settings:
  modelMappings:
    # Map product-specific aliases to upstream models
    - from: "gpt-4o-mini"
      to: "gpt-4o"

    # Map legacy model names to current models
    - from: "claude-3-sonnet"
      to: "claude-3.5-sonnet"

    # Map to different providers
    - from: "my-company-model"
      to: "gpt-4o"

    # Cost optimization
    - from: "expensive-model"
      to: "cost-effective-model"
```

**Usage Example:**

```python
# Client requests "gpt-4o-mini" but gets "gpt-4o"
response = client.chat.completions.create(
    model="gpt-4o-mini",  # Will be mapped to "gpt-4o"
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)
```

#### Override Parameters

Override parameters let you enforce channel-specific defaults regardless of incoming request payloads. This is useful for:

- **Security**: Enforce safe parameter values
- **Consistency**: Ensure consistent behavior across applications
- **Compliance**: Meet organizational requirements
- **Optimization**: Set optimal parameters for specific use cases

**Example Override Parameters Configuration:**

```yaml
# In channel settings
settings:
  overrideParameters: |
    {
      # Basic parameters
      "temperature": 0.3,
      "max_tokens": 1024,
      "top_p": 0.9,

      # JSON response enforcement
      "response_format": {
        "type": "json_object"
      },

      # Safety parameters
      "presence_penalty": 0.1,
      "frequency_penalty": 0.1,

      # Provider-specific parameters
      "stop_sequences": ["\nHuman:", "\nAssistant:"]
    }
```

**Advanced Override Examples:**

```yaml
# Enforce deterministic responses for production
overrideParameters: |
  {
    "temperature": 0.1,
    "max_tokens": 500,
    "top_p": 0.95
  }

# Creative writing channel
overrideParameters: |
  {
    "temperature": 0.8,
    "max_tokens": 2000,
    "frequency_penalty": 0.5
  }

# Code generation channel
overrideParameters: |
  {
    "temperature": 0.2,
    "max_tokens": 4096,
    "stop": ["```", "\n\n"]
  }
```

#### Combined Example: Model Mapping + Override Parameters

```yaml
# Complete channel configuration
name: "openai-production"
type: "openai"
base_url: "https://api.openai.com/v1"
credentials:
  api_key: "your-openai-key"
supported_models: ["gpt-4o", "gpt-4", "gpt-3.5-turbo"]
settings:
  modelMappings:
    - from: "chat-model"
      to: "gpt-4o"
    - from: "fast-model"
      to: "gpt-3.5-turbo"
  overrideParameters: |
    {
      "temperature": 0.3,
      "max_tokens": 1024,
      "response_format": {
        "type": "json_object"
      }
    }
```

#### Best Practices

1. **Model Mapping**
   - Only map to models that are declared in `supported_models`
   - Use descriptive mapping names for clarity
   - Test mappings thoroughly before production use
   - Document your mapping strategy for team members

2. **Override Parameters**
   - Start with conservative values and adjust based on use case
   - Consider the impact on cost and performance
   - Test overrides with different types of requests
   - Monitor usage patterns to optimize parameters

3. **Security Considerations**
   - Avoid overriding sensitive parameters in development channels
   - Use separate channels for different security requirements
   - Regularly review and update override configurations

## Next Steps

### Understand the Request Flow
- [Request Processing Guide](request-processing.md): Understand the full request lifecycle from entry to upstream execution, and the differences between model mapping, model association, and channel selection

### Explore Features
- **Tracing**: Set up request tracing for observability
- **Permissions**: Configure role-based access control
- **Model Profiles**: Create model mapping rules
- **Usage Analytics**: Monitor API usage and costs

### Integration Guides
- [Claude Code Integration](../guides/claude-code-integration.md)
- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)

## Troubleshooting

### Common Issues

**API key authentication fails**
- Verify the API key is correctly configured
- Check if the channel is enabled
- Ensure the provider API key is valid

### Getting Help

- Check the [GitHub Issues](https://github.com/looplj/axonhub/issues)
- Review the [Architecture Documentation](../development/erd.md)
- Join the community discussions

## What's Next?

Now that you have AxonHub running, explore these advanced features:

- Set up multiple channels for failover
- Configure model mappings for cost optimization
- Implement request tracing for debugging
- Set up usage quotas and rate limits
