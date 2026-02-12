# 渠道配置指南

本指南介绍如何在 AxonHub 中配置 AI 提供商渠道。渠道是您的应用程序与 AI 模型提供商之间的桥梁。

## 概述

每个渠道代表与 AI 提供商（OpenAI、Anthropic、Gemini 等）的连接。通过渠道，您可以：

- 同时连接多个 AI 提供商
- 配置模型映射和请求参数覆盖
- 动态启用/禁用渠道
- 在启用前测试连接
- 配置多个 API Key 实现负载均衡

## 渠道配置

### 基本配置

在管理界面中配置 AI 提供商渠道：

```yaml
# OpenAI 渠道示例
name: "openai"
type: "openai"
base_url: "https://api.openai.com/v1"
credentials:
  api_keys:
    - "sk-your-openai-key-1"
    - "sk-your-openai-key-2"
    - "sk-your-openai-key-3"
supported_models: ["gpt-5", "gpt-4o"]
```

### 配置字段

| 字段 | 类型 | 必需 | 描述 |
|-------|------|------|------|
| `name` | string | 是 | 渠道的唯一标识符 |
| `type` | string | 是 | 提供商类型（openai、anthropic、gemini 等） |
| `base_url` | string | 是 | API 端点 URL |
| `credentials` | object | 是 | 认证凭据（支持多 API Key） |
| `supported_models` | array | 是 | 该渠道支持的模型列表 |
| `settings` | object | 否 | 高级设置（映射、覆盖等） |

## 多 API Key 配置

AxonHub 支持为单个渠道配置多个 API Key，实现自动负载均衡和故障转移。

### 配置方式

```yaml
# 多 API Key 配置示例
credentials:
  api_keys:
    - "sk-your-key-1"
    - "sk-your-key-2"
    - "sk-your-key-3"
```

### 负载均衡策略

当配置多个 API Key 时，AxonHub 使用以下策略：

| 场景 | 策略 | 说明 |
| :--- | :--- | :--- |
| 有 Trace ID | 一致性哈希 | 相同 Trace ID 的请求始终使用相同的 Key |
| 无 Trace ID | 随机选择 | 从可用 Key 中随机选择 |

### API Key 管理

#### 禁用 API Key

当某个 API Key 出现错误（如额度耗尽、被封禁）时，系统会自动或手动将其禁用：

- 被禁用的 Key 将不再被用于新请求
- 系统会自动切换到其他可用 Key
- 禁用信息包括错误代码和原因

#### 启用 API Key

可以手动重新启用之前被禁用的 API Key：

- 从禁用列表中移除该 Key
- 该 Key 将重新参与负载均衡

#### 删除 API Key

可以彻底删除不再使用的 API Key：

- 从禁用列表和凭据中同时删除
- 至少保留一个可用的 API Key

### 向后兼容

AxonHub 仍支持单 API Key 配置（旧格式），系统会自动兼容：

```yaml
# 单 API Key（旧格式，仍支持）
credentials:
  api_key: "sk-your-single-key"

# 等效于
credentials:
  api_keys:
    - "sk-your-single-key"
```

## 测试连接

在启用渠道之前，测试连接以确保凭据正确：

1. 在管理界面中导航到 **渠道管理**
2. 点击渠道旁边的 **测试** 按钮
3. 等待测试结果
4. 如果测试成功，继续启用渠道

## 启用渠道

测试成功后，启用渠道：

1. 点击 **启用** 按钮
2. 渠道状态将变为 **活跃**
3. 该渠道现在可用于路由请求

## 模型映射

当请求中的模型名称与上游提供商支持的名称不一致时，可以通过模型映射在网关侧自动重写模型。

### 使用场景

- 将不支持或旧版本的模型 ID 映射到可用的替代模型
- 为多渠道场景设置回退逻辑（不同渠道对应不同提供商）
- 为应用程序简化模型名称

### 配置

```yaml
# 示例：将产品自定义别名映射到上游模型
settings:
  modelMappings:
    - from: "gpt-4o-mini"
      to: "gpt-4o"
    - from: "claude-3-sonnet"
      to: "claude-3.5-sonnet"
```

### 规则

- AxonHub 仅接受映射到 `supported_models` 中已声明的模型
- 映射按顺序应用，使用第一个匹配的映射
- 如果没有匹配的映射，则使用原始模型名称

## 请求覆盖 (Request Override)

请求覆盖允许您为渠道强制设置默认参数，或使用模板动态修改请求。支持以下操作类型：

| 操作类型 | 描述 |
| :--- | :--- |
| `set` | 设置字段值 |
| `delete` | 删除字段 |
| `rename` | 重命名字段 |
| `copy` | 复制字段 |

### 请求体覆盖示例

```json
[
  {
    "op": "set",
    "path": "temperature",
    "value": "0.7"
  },
  {
    "op": "set",
    "path": "max_tokens",
    "value": "2000"
  },
  {
    "op": "delete",
    "path": "frequency_penalty"
  }
]
```

### 请求头覆盖示例

```json
[
  {
    "op": "set",
    "path": "X-Custom-Header",
    "value": "{{.Model}}"
  }
]
```

有关如何使用模板、条件逻辑和更多高级功能的详细信息，请参阅 [请求覆盖指南](request-override.md)。

## 最佳实践

1. **启用前测试**：在启用渠道之前始终测试连接
2. **使用有意义的名称**：使用描述性的渠道名称以便识别
3. **配置多 API Key**：为生产渠道配置多个 API Key 以提高可用性
4. **监控 Key 状态**：定期检查 API Key 的使用情况和禁用状态
5. **记录映射**：记录模型映射以便维护
6. **监控使用情况**：定期检查渠道使用情况和性能
7. **备份凭据**：安全存储凭据并制定备份计划

## 故障排除

### 连接测试失败

- 验证 API 密钥是否正确且有效
- 检查 API 端点是否可访问
- 确保账户有足够的额度/配额

### 模型未找到

- 验证模型是否在 `supported_models` 中列出
- 检查模型映射是否正确配置
- 确认模型在提供商的目录中可用

### 覆盖参数不生效

- 确保 JSON 有效（使用 JSON 验证器）
- 检查字段名称是否与提供商的 API 规范匹配
- 验证嵌套字段使用正确的点分写法

### API Key 频繁被禁用

- 检查 API Key 的额度是否充足
- 查看禁用原因和错误代码
- 考虑增加 API Key 数量以分散负载

## Base URL 配置

### 概述

`base_url` 是渠道配置中的必需字段，用于指定 AI 提供商的 API 端点地址。AxonHub 支持灵活的 URL 配置方式，以适应不同的部署场景。

### 默认 Base URL

每种渠道类型都有预设的默认 Base URL，当您创建渠道时会自动填充：

| 渠道类型 | 默认 Base URL |
|---------|--------------|
| openai | `https://api.openai.com/v1` |
| anthropic | `https://api.anthropic.com` |
| gemini | `https://generativelanguage.googleapis.com/v1beta` |
| deepseek | `https://api.deepseek.com/v1` |
| moonshot | `https://api.moonshot.cn/v1` |
| ... | 其他类型详见配置界面 |

### 自定义 Base URL

您可以配置自定义 Base URL 以支持：

- **第三方代理服务**：通过兼容 OpenAI/Anthropic 协议的代理服务访问模型
- **私有化部署**：连接企业内部部署的 AI 服务
- **多区域部署**：使用不同区域的 API 端点

### 特殊后缀

AxonHub 支持在 Base URL 末尾添加特殊后缀来控制 URL 标准化行为：

#### `#` 后缀 - 禁用版本自动追加

在 Base URL 末尾添加 `#`，系统将**不会**自动追加 API 版本号：

```yaml
# Anthropic 渠道示例 - 使用原始 URL，不自动追加 /v1
base_url: "https://custom-api.example.com/anthropic#"

# 实际请求 URL: https://custom-api.example.com/anthropic/messages
# 而不是: https://custom-api.example.com/anthropic/v1/messages
```

**适用场景**：
- 使用自定义代理服务，URL 路径已包含版本信息
- 提供商使用非标准的 URL 结构
- 需要完全控制请求路径

#### `##` 后缀 - 完全原始模式（OpenAI 格式）

在 Base URL 末尾添加 `##`，系统将：
1. 禁用版本自动追加
2. 禁用端点自动追加（如 `/chat/completions`）

```yaml
# OpenAI 渠道示例 - 完全原始模式
base_url: "https://custom-gateway.example.com/api/v2##"

# 实际请求 URL: https://custom-gateway.example.com/api/v2
# 而不是: https://custom-gateway.example.com/api/v2/v1/chat/completions
```

**适用场景**：
- 使用完全自定义的 API 网关
- 需要精确控制完整的请求 URL
- 兼容特殊的代理服务或中转服务

### 自动版本追加规则

当不使用 `#` 或 `##` 后缀时，系统会根据渠道类型自动追加 API 版本：

| 渠道类型 | 自动追加的版本 |
|---------|--------------|
| openai, deepseek, moonshot, xai 等 | `/v1` |
| gemini | `/v1beta` |
| doubao | `/v3` |
| zai, zhipu | `/v4` |
| anthropic | `/v1` |
| anthropic_aws (Bedrock) | 不追加 |
| anthropic_gcp (Vertex) | 不追加 |

### 配置示例

```yaml
# 标准配置 - 使用默认行为
name: "openai-standard"
type: "openai"
base_url: "https://api.openai.com"
# 实际请求: https://api.openai.com/v1/chat/completions

# 禁用版本追加 - Anthropic
name: "anthropic-custom"
type: "anthropic"
base_url: "https://api.anthropic.com#"
# 实际请求: https://api.anthropic.com/messages

# 完全原始模式 - OpenAI
name: "openai-raw"
type: "openai"
base_url: "https://gateway.example.com/proxy##"
# 实际请求: https://gateway.example.com/proxy
```

## 相关文档

- [请求重写指南](request-override.md) - 使用模板进行高级请求修改
- [模型管理指南](model-management.md) - 跨渠道管理模型
- [负载均衡指南](load-balance.md) - 在多个渠道间分发请求
- [API 密钥配置指南](api-key-profiles.md) - 组织 API 密钥和权限
