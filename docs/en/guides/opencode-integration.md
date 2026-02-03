# OpenCode Integration Guide

---

## Overview
AxonHub can act as a drop-in replacement for Anthropic endpoints, letting OpenCode connect through your own infrastructure. This guide explains how to configure OpenCode and how to combine it with AxonHub model profiles for flexible routing.

### Key Points
- AxonHub performs AI protocol/format transformation. You can configure multiple upstream channels (providers) and expose a single Anthropic-compatible interface for OpenCode.
- You can aggregate OpenCode requests from the same session into one trace (see "Configure OpenCode").

### Prerequisites
- AxonHub instance reachable from your development machine.
- Valid AxonHub API key with project access.
- Access to OpenCode CLI tool.
- Optional: one or more model profiles configured in the AxonHub console.

---

## Configure OpenCode

### 1. Create OpenCode Configuration File

Create or edit your OpenCode configuration file at `~/.config/opencode/opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": [
    "opencode-axonhub-tracing"
  ],
  "provider": {
    "axonhub": {
      "npm": "@ai-sdk/anthropic",
      "name": "AxonHub",
      "options": {
        "baseURL": "http://127.0.0.1:8090/anthropic/v1",
        "apiKey": "AXONHUB_API_KEY"
      },
      "models": {
        "claude-sonnet-4-5": {
          "name": "AxonHub - Claude Sonnet 4.5",
          "modalities": {
            "input": [
              "text",
              "image"
            ],
            "output": [
              "text"
            ]
          }
        }
      }
    }
  }
}
```

### 2. Configuration Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `npm` | The npm package to use for the provider | `@ai-sdk/anthropic` |
| `name` | Display name for the provider | `AxonHub` |
| `baseURL` | AxonHub Anthropic API endpoint | `http://127.0.0.1:8090/anthropic/v1` |
| `apiKey` | Your AxonHub API key | Replace `AXONHUB_API_KEY` with your actual key |

### 3. Add Multiple Models

You can configure multiple models in the same provider:

```json
{
  "provider": {
    "axonhub": {
      "npm": "@ai-sdk/anthropic",
      "name": "AxonHub",
      "options": {
        "baseURL": "http://127.0.0.1:8090/anthropic/v1",
        "apiKey": "your-axonhub-api-key"
      },
      "models": {
        "claude-sonnet-4-5": {
          "name": "AxonHub - Claude Sonnet 4.5",
          "modalities": {
            "input": ["text", "image"],
            "output": ["text"]
          }
        },
        "claude-haiku-4-5": {
          "name": "AxonHub - Claude Haiku 4.5",
          "modalities": {
            "input": ["text", "image"],
            "output": ["text"]
          }
        },
        "claude-opus-4-5": {
          "name": "AxonHub - Claude Opus 4.5",
          "modalities": {
            "input": ["text", "image"],
            "output": ["text"]
          }
        }
      }
    }
  }
}
```

### 4. Using Remote AxonHub Instance

If your AxonHub instance is deployed remotely, update the `baseURL`:

```json
{
  "options": {
    "baseURL": "https://your-axonhub-domain.com/anthropic/v1",
    "apiKey": "your-axonhub-api-key"
  }
}
```

## Working with Model Profiles

AxonHub model profiles remap incoming model names to provider-specific equivalents:
- Create a profile in the AxonHub console and add mapping rules (exact name or regex).
- Assign the profile to your API key.
- Switch active profiles to alter OpenCode behavior without changing tool settings.

<table>
  <tr align="center">
    <td align="center">
      <a href="../../screenshots/axonhub-profiles.png">
        <img src="../../screenshots/axonhub-profiles.png" alt="Model Profiles" width="250"/>
      </a>
      <br/>
      Model Profiles
    </td>
  </tr>
</table>

### Example Use Cases

#### Cost Optimization
Map expensive models to cheaper alternatives:
- Request `claude-sonnet-4-5` → mapped to `deepseek-chat` for reducing costs
- Request `claude-haiku-4-5` → mapped to `gpt-4o-mini` for simple tasks

#### Performance Optimization
Route to faster models for specific tasks:
- Request `claude-opus-4-5` → mapped to `claude-sonnet-4-5` for faster responses
- Request `claude-sonnet-4-5` → mapped to `gpt-4o` for better availability

#### Advanced Reasoning
Route to specialized models:
- Request `claude-sonnet-4-5` → mapped to `deepseek-reasoner` for complex reasoning tasks
- Request `claude-opus-4-5` → mapped to `o1-preview` for mathematical problems

---

## OpenCode Tracing Plugin

The `opencode-axonhub-tracing` plugin enhances your development experience by providing:
- Request/response logging
- Performance metrics
- Error tracking
- Session correlation

### Installation

The plugin is specified in the configuration file:

```json
{
  "plugin": [
    "opencode-axonhub-tracing"
  ]
}
```

### Benefits
- **Debugging**: Easily trace issues in your AI interactions
- **Performance**: Monitor response times and identify bottlenecks
- **Cost Tracking**: See token usage and costs per request
- **Session Management**: Group related requests for better analysis

---

## Troubleshooting

### OpenCode Cannot Connect

**Symptoms**: Connection errors, timeout errors

**Solutions**:
1. Verify `baseURL` points to the correct AxonHub endpoint
2. Check that AxonHub is running: `curl http://localhost:8090/health`
3. Verify firewall allows outbound connections
4. For HTTPS endpoints with self-signed certificates, configure trust settings

### Authentication Errors

**Symptoms**: 401 Unauthorized, 403 Forbidden

**Solutions**:
1. Verify your API key is correct in the configuration
2. Check that the API key has not expired in AxonHub console
3. Ensure the API key has access to the requested project
4. Verify the API key has permissions for the requested models

### Unexpected Model Responses

**Symptoms**: Wrong model responding, unexpected behavior

**Solutions**:
1. Review active profile mappings in the AxonHub console
2. Check channel configuration and model associations
3. Verify the requested model name matches your configuration
4. Disable or adjust profile rules if necessary

### Configuration Not Loading

**Symptoms**: OpenCode uses default settings, ignores config file

**Solutions**:
1. Verify config file location: `~/.config/opencode/opencode.json`
2. Check JSON syntax is valid (use a JSON validator)
3. Ensure file permissions allow reading
4. Restart OpenCode after configuration changes

---

## Advanced Configuration

### Multiple AxonHub Providers

You can configure multiple AxonHub instances as different providers:

```json
{
  "provider": {
    "axonhub-prod": {
      "npm": "@ai-sdk/anthropic",
      "name": "AxonHub Production",
      "options": {
        "baseURL": "https://prod.axonhub.com/anthropic/v1",
        "apiKey": "prod-api-key"
      },
      "models": {
        "claude-sonnet-4-5": {
          "name": "Production - Claude Sonnet 4.5",
          "modalities": {
            "input": ["text", "image"],
            "output": ["text"]
          }
        }
      }
    },
    "axonhub-dev": {
      "npm": "@ai-sdk/anthropic",
      "name": "AxonHub Development",
      "options": {
        "baseURL": "http://localhost:8090/anthropic/v1",
        "apiKey": "dev-api-key"
      },
      "models": {
        "claude-sonnet-4-5": {
          "name": "Development - Claude Sonnet 4.5",
          "modalities": {
            "input": ["text", "image"],
            "output": ["text"]
          }
        }
      }
    }
  }
}
```

### Using OpenAI-Compatible Endpoint

OpenCode can also use AxonHub's OpenAI-compatible endpoint:

```json
{
  "provider": {
    "axonhub-openai": {
      "npm": "@ai-sdk/openai",
      "name": "AxonHub OpenAI",
      "options": {
        "baseURL": "http://127.0.0.1:8090/v1",
        "apiKey": "your-axonhub-api-key"
      },
      "models": {
        "gpt-4": {
          "name": "AxonHub - GPT-4",
          "modalities": {
            "input": ["text"],
            "output": ["text"]
          }
        }
      }
    }
  }
}
```

---

## Best Practices

### Security
- **Never commit API keys**: Use environment variables or secure vaults
- **Rotate keys regularly**: Update API keys periodically
- **Use HTTPS in production**: Always use encrypted connections for remote instances
- **Restrict API key permissions**: Grant only necessary permissions

### Performance
- **Enable trace aggregation**: Improves cache hit rates
- **Use appropriate models**: Match model capabilities to task complexity
- **Monitor usage**: Track costs and performance in AxonHub console
- **Configure timeouts**: Set reasonable timeout values for your use case

---

## Related Documentation
- [Tracing Guide](tracing.md)
- [API Key Profiles Guide](api-key-profiles.md)
- [Model Management Guide](model-management.md)
- [Channel Management Guide](channel-management.md)
- [Anthropic API Reference](../api-reference/anthropic-api.md)
- README sections on [Usage Guide](../../../README.md#usage-guide)
