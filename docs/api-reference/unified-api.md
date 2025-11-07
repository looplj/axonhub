# Unified API Reference

## Overview

AxonHub provides a unified API gateway that supports both OpenAI Chat Completions and Anthropic Messages APIs through a single interface. This allows you to use existing OpenAI or Anthropic client SDKs while seamlessly accessing models from multiple providers. The platform automatically handles API format translation, enabling you to use one API format to access models from any supported provider.

## Key Benefits

- **API Interoperability**: Use OpenAI Chat Completions API to call Anthropic models, or use Anthropic Messages API to call OpenAI models
- **Zero Code Changes**: Keep using your existing OpenAI or Anthropic client SDKs without modification
- **Automatic Translation**: AxonHub automatically converts between API formats when needed
- **Provider Flexibility**: Access any supported AI provider regardless of which API format you prefer

## Supported API Formats

### OpenAI Chat Completions API

AxonHub fully supports the OpenAI Chat Completions API specification, allowing you to use any OpenAI-compatible client SDK.

**Endpoints:**
- `POST /v1/chat/completions` - Text generation
- `GET /v1/models` - List available models

**Example Request:**
```python
from openai import OpenAI

client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

# Call Anthropic model using OpenAI API format
response = client.chat.completions.create(
    model="claude-3-5-sonnet",
    messages=[
        {"role": "user", "content": "Hello, Claude!"}
    ]
)
print(response.choices[0].message.content)
```

### Anthropic Messages API

AxonHub also supports the native Anthropic Messages API for applications that prefer Anthropic's specific features and response format.

**Endpoints:**
- `POST /anthropic/v1/messages` - Text generation
- `GET /anthropic/v1/models` - List available models

**Example Request:**
```python
import requests

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
                "content": [
                    {"type": "text", "text": "Hello, GPT!"}
                ]
            }
        ]
    }
)
print(response.json()["content"][0]["text"])
```

## API Translation Capabilities

AxonHub automatically translates between API formats, enabling these powerful scenarios:

### Use OpenAI SDK with Anthropic Models
```python
# OpenAI SDK calling Anthropic model
response = client.chat.completions.create(
    model="claude-3-5-sonnet",  # Anthropic model
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)
# AxonHub automatically translates OpenAI format → Anthropic format
```

### Use Anthropic SDK with OpenAI Models
```python
# Anthropic SDK calling OpenAI model
response = requests.post(
    "http://localhost:8090/anthropic/v1/messages",
    json={
        "model": "gpt-4o",  # OpenAI model
        "messages": [
            {
                "role": "user",
                "content": [{"type": "text", "text": "Hello!"}]
            }
        ]
    }
)
# AxonHub automatically translates Anthropic format → OpenAI format
```

## Supported Providers

| Provider               | Status     | Supported Models             | Compatible APIs |
| ---------------------- | ---------- | ---------------------------- | --------------- |
| **OpenAI**             | ✅ Done    | GPT-4, GPT-4o, GPT-5, etc.   | OpenAI, Anthropic |
| **Anthropic**          | ✅ Done    | Claude 3.5, Claude 3.0, etc. | OpenAI, Anthropic |
| **Zhipu AI**           | ✅ Done    | GLM-4.5, GLM-4.5-air, etc.   | OpenAI, Anthropic |
| **Moonshot AI (Kimi)** | ✅ Done    | kimi-k2, etc.                | OpenAI, Anthropic |
| **DeepSeek**           | ✅ Done    | DeepSeek-V3.1, etc.          | OpenAI, Anthropic |
| **ByteDance Doubao**   | ✅ Done    | doubao-1.6, etc.             | OpenAI, Anthropic |
| **Gemini**             | ✅ Done    | Gemini 2.5, etc.             | OpenAI, Anthropic |
| **AWS Bedrock**        | 🔄 Testing | Claude on AWS                | OpenAI, Anthropic |
| **Google Cloud**       | 🔄 Testing | Claude on GCP                | OpenAI, Anthropic |

## Authentication

Both API formats use the same authentication system:

- **OpenAI API**: Use `Authorization: Bearer <your-api-key>` header
- **Anthropic API**: Use `X-API-Key: <your-api-key>` header

The API keys are managed through AxonHub's API Key management system and provide the same permissions regardless of which API format you use.

## Streaming Support

Both API formats support streaming responses:

### OpenAI Streaming
```python
response = client.chat.completions.create(
    model="claude-3-5-sonnet",
    messages=[{"role": "user", "content": "Tell me a story"}],
    stream=True
)

for chunk in response:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### Anthropic Streaming
```python
response = requests.post(
    "http://localhost:8090/anthropic/v1/messages",
    headers={
        "Content-Type": "application/json",
        "X-API-Key": "your-api-key",
        "Accept": "text/event-stream"
    },
    json={
        "model": "gpt-4o",
        "messages": [
            {
                "role": "user",
                "content": [{"type": "text", "text": "Tell me a story"}]
            }
        ],
        "stream": True
    },
    stream=True
)

for line in response.iter_lines():
    if line:
        print(line.decode('utf-8'))
```

## Error Handling

Both API formats return standardized error responses:

### OpenAI Format Error
```json
{
  "error": {
    "message": "Invalid API key",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

### Anthropic Format Error
```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid API key"
  }
}
```

## Best Practices

1. **Choose Your Preferred API**: Use the API format that best fits your application's needs and existing codebase
2. **Consistent Authentication**: Use the same API key across both API formats
3. **Model Selection**: Specify the target model explicitly in your requests
4. **Error Handling**: Implement proper error handling for both API formats
5. **Streaming**: Use streaming for better user experience with long responses

## Migration Guide

### From OpenAI to AxonHub
```python
# Before: Direct OpenAI
client = OpenAI(api_key="openai-key")

# After: AxonHub with OpenAI API
client = OpenAI(
    api_key="axonhub-api-key",
    base_url="http://localhost:8090/v1"
)
# Your existing code continues to work!
```

### From Anthropic to AxonHub
```python
# Before: Direct Anthropic
response = requests.post(
    "https://api.anthropic.com/v1/messages",
    headers={"X-API-Key": "anthropic-key"}
)

# After: AxonHub with Anthropic API
response = requests.post(
    "http://localhost:8090/anthropic/v1/messages",
    headers={"X-API-Key": "axonhub-api-key"}
)
# Your existing code continues to work!
```

---

# 统一 API 参考

## 概述

AxonHub 提供统一的 API 网关，通过单一接口同时支持 OpenAI Chat Completions 和 Anthropic Messages API。这使您可以在使用现有 OpenAI 或 Anthropic 客户端 SDK 的同时，无缝访问多个提供商的模型。平台自动处理 API 格式转换，让您可以使用一种 API 格式访问任何支持的提供商。

## 核心优势

- **API 互操作性**：使用 OpenAI Chat Completions API 调用 Anthropic 模型，或使用 Anthropic Messages API 调用 OpenAI 模型
- **零代码变更**：继续使用现有的 OpenAI 或 Anthropic 客户端 SDK，无需修改
- **自动转换**：AxonHub 在需要时自动在 API 格式之间进行转换
- **提供商灵活性**：无论您偏好哪种 API 格式，都可以访问任何支持的 AI 提供商

## 支持的 API 格式

### OpenAI Chat Completions API

AxonHub 完全支持 OpenAI Chat Completions API 规范，允许您使用任何 OpenAI 兼容的客户端 SDK。

**端点：**
- `POST /v1/chat/completions` - 文本生成
- `GET /v1/models` - 列出可用模型

**示例请求：**
```python
from openai import OpenAI

client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

# 使用 OpenAI API 格式调用 Anthropic 模型
response = client.chat.completions.create(
    model="claude-3-5-sonnet",
    messages=[
        {"role": "user", "content": "Hello, Claude!"}
    ]
)
print(response.choices[0].message.content)
```

### Anthropic Messages API

AxonHub 还支持原生 Anthropic Messages API，适用于偏好 Anthropic 特定功能和响应格式的应用程序。

**端点：**
- `POST /anthropic/v1/messages` - 文本生成
- `GET /anthropic/v1/models` - 列出可用模型

**示例请求：**
```python
import requests

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
                "content": [
                    {"type": "text", "text": "Hello, GPT!"}
                ]
            }
        ]
    }
)
print(response.json()["content"][0]["text"])
```

## API 转换能力

AxonHub 自动在 API 格式之间进行转换，实现以下强大场景：

### 使用 OpenAI SDK 调用 Anthropic 模型
```python
# OpenAI SDK 调用 Anthropic 模型
response = client.chat.completions.create(
    model="claude-3-5-sonnet",  # Anthropic 模型
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)
# AxonHub 自动转换 OpenAI 格式 → Anthropic 格式
```

### 使用 Anthropic SDK 调用 OpenAI 模型
```python
# Anthropic SDK 调用 OpenAI 模型
response = requests.post(
    "http://localhost:8090/anthropic/v1/messages",
    json={
        "model": "gpt-4o",  # OpenAI 模型
        "messages": [
            {
                "role": "user",
                "content": [{"type": "text", "text": "Hello!"}]
            }
        ]
    }
)
# AxonHub 自动转换 Anthropic 格式 → OpenAI 格式
```

## 支持的提供商

| 提供商                   | 状态       | 支持模型示例                 | 兼容 API |
| ------------------------ | ---------- | ---------------------------- | --------------- |
| **OpenAI**               | ✅ 已完成  | GPT-4、GPT-4o、GPT-5 等      | OpenAI, Anthropic |
| **Anthropic**            | ✅ 已完成  | Claude 3.5、Claude 3.0 等    | OpenAI, Anthropic |
| **智谱 AI（Zhipu）**     | ✅ 已完成  | GLM-4.5、GLM-4.5-air 等      | OpenAI, Anthropic |
| **月之暗面（Moonshot）** | ✅ 已完成  | kimi-k2 等                   | OpenAI, Anthropic |
| **DeepSeek**             | ✅ 已完成  | DeepSeek-V3.1 等             | OpenAI, Anthropic |
| **字节跳动豆包**         | ✅ 已完成  | doubao-1.6 等                | OpenAI, Anthropic |
| **Gemini**               | ✅ 已完成  | Gemini 2.5 等                | OpenAI, Anthropic |
| **AWS Bedrock**          | 🔄 测试中  | Claude on AWS                | OpenAI, Anthropic |
| **Google Cloud**         | 🔄 测试中  | Claude on GCP                | OpenAI, Anthropic |

## 认证

两种 API 格式使用相同的认证系统：

- **OpenAI API**：使用 `Authorization: Bearer <your-api-key>` 头部
- **Anthropic API**：使用 `X-API-Key: <your-api-key>` 头部

API 密钥通过 AxonHub 的 API 密钥管理系统进行管理，无论使用哪种 API 格式，都提供相同的权限。

## 流式支持

两种 API 格式都支持流式响应：

### OpenAI 流式
```python
response = client.chat.completions.create(
    model="claude-3-5-sonnet",
    messages=[{"role": "user", "content": "Tell me a story"}],
    stream=True
)

for chunk in response:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### Anthropic 流式
```python
response = requests.post(
    "http://localhost:8090/anthropic/v1/messages",
    headers={
        "Content-Type": "application/json",
        "X-API-Key": "your-api-key",
        "Accept": "text/event-stream"
    },
    json={
        "model": "gpt-4o",
        "messages": [
            {
                "role": "user",
                "content": [{"type": "text", "text": "Tell me a story"}]
            }
        ],
        "stream": True
    },
    stream=True
)

for line in response.iter_lines():
    if line:
        print(line.decode('utf-8'))
```

## 错误处理

两种 API 格式都返回标准化的错误响应：

### OpenAI 格式错误
```json
{
  "error": {
    "message": "Invalid API key",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

### Anthropic 格式错误
```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid API key"
  }
}
```

## 最佳实践

1. **选择偏好的 API**：使用最适合应用程序需求和现有代码库的 API 格式
2. **一致的认证**：在两种 API 格式中使用相同的 API 密钥
3. **模型选择**：在请求中明确指定目标模型
4. **错误处理**：为两种 API 格式实现适当的错误处理
5. **流式处理**：对于长响应使用流式处理以获得更好的用户体验

## 迁移指南

### 从 OpenAI 迁移到 AxonHub
```python
# 之前：直接 OpenAI
client = OpenAI(api_key="openai-key")

# 之后：使用 OpenAI API 的 AxonHub
client = OpenAI(
    api_key="axonhub-api-key",
    base_url="http://localhost:8090/v1"
)
# 您的现有代码继续工作！
```

### 从 Anthropic 迁移到 AxonHub
```python
# 之前：直接 Anthropic
response = requests.post(
    "https://api.anthropic.com/v1/messages",
    headers={"X-API-Key": "anthropic-key"}
)

# 之后：使用 Anthropic API 的 AxonHub
response = requests.post(
    "http://localhost:8090/anthropic/v1/messages",
    headers={"X-API-Key": "axonhub-api-key"}
)
# 您的现有代码继续工作！
```