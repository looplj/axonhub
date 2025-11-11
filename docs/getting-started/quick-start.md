# Quick Start Guide

## Overview

This guide will help you get started with AxonHub quickly. In just a few minutes, you'll have AxonHub running and be able to make your first API call.

## Prerequisites

- Docker and Docker Compose (recommended)
- Or Go 1.24+ and Node.js 18+ for development setup
- A valid API key from an AI provider (OpenAI, Anthropic, etc.)

## Quick Setup Methods

### Method 1: Docker Compose (Recommended)

1. **Clone the repository**
   ```bash
   git clone https://github.com/looplj/axonhub.git
   cd axonhub
   ```

2. **Configure environment variables**
   ```bash
   cp config.example.yml config.yml
   # Edit config.yml with your preferred settings
   ```

3. **Start services**
   ```bash
   docker-compose up -d
   ```

4. **Access the application**
   - Web interface: http://localhost:8090
   - Default credentials: admin@example.com / admin123

### Method 2: Binary Download

1. **Download the latest release**
   - Visit [GitHub Releases](https://github.com/looplj/axonhub/releases)
   - Download the appropriate binary for your OS

2. **Extract and run**
   ```bash
   unzip axonhub_*.zip
   cd axonhub_*
   chmod +x axonhub
   ./axonhub
   ```

3. **Access the application**
   - Web interface: http://localhost:8090

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

## Configuration Examples

### Basic Configuration

```yaml
# config.yml
server:
  port: 8090
  name: "AxonHub"

db:
  dialect: "sqlite3"
  dsn: "file:axonhub.db?cache=shared&_fk=1"

log:
  level: "info"
  encoding: "json"
```

### Production Configuration

```yaml
server:
  port: 8090
  name: "AxonHub Production"
  debug: false

db:
  dialect: "postgres"
  dsn: "postgres://user:pass@localhost/axonhub?sslmode=disable"

log:
  level: "warn"
  encoding: "json"
  output: "file"
  file:
    path: "/var/log/axonhub/axonhub.log"
```

## Next Steps

### Explore Features
- **Tracing**: Set up request tracing for observability
- **Permissions**: Configure role-based access control
- **Model Profiles**: Create model mapping rules
- **Usage Analytics**: Monitor API usage and costs

### Integration Guides
- [Claude Code Integration](../guides/claude-code-integration.md)
- [API Reference](../api-reference/unified-api.md)
- [Deployment Guide](../deployment/configuration.md)

## Troubleshooting

### Common Issues

**Cannot connect to AxonHub**
- Check if the service is running: `docker-compose ps`
- Verify port 8090 is available
- Check firewall settings

**API key authentication fails**
- Verify the API key is correctly configured
- Check if the channel is enabled
- Ensure the provider API key is valid

**Request timeouts**
- Increase `server.llm_request_timeout` in config
- Check network connectivity to AI providers

### Getting Help

- Check the [GitHub Issues](https://github.com/looplj/axonhub/issues)
- Review the [Architecture Documentation](../architecture/erd.md)
- Join the community discussions

## What's Next?

Now that you have AxonHub running, explore these advanced features:

- Set up multiple channels for failover
- Configure model mappings for cost optimization
- Implement request tracing for debugging
- Set up usage quotas and rate limits
- Integrate with your existing CI/CD pipeline