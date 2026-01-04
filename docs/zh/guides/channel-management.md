# 渠道配置指南

本指南介绍如何在 AxonHub 中配置 AI 提供商渠道。渠道是您的应用程序与 AI 模型提供商之间的桥梁。

## 概述

每个渠道代表与 AI 提供商（OpenAI、Anthropic、Gemini 等）的连接。通过渠道，您可以：

- 同时连接多个 AI 提供商
- 配置模型映射和请求参数覆盖
- 动态启用/禁用渠道
- 在启用前测试连接

## 渠道配置

### 基本配置

在管理界面中配置 AI 提供商渠道：

```yaml
# OpenAI 渠道示例
name: "openai"
type: "openai"
base_url: "https://api.openai.com/v1"
credentials:
  api_key: "your-openai-key"
supported_models: ["gpt-5", "gpt-4o"]
```

### 配置字段

| 字段 | 类型 | 必需 | 描述 |
|-------|------|------|------|
| `name` | string | 是 | 渠道的唯一标识符 |
| `type` | string | 是 | 提供商类型（openai、anthropic、gemini 等） |
| `base_url` | string | 是 | API 端点 URL |
| `credentials` | object | 是 | 认证凭据 |
| `supported_models` | array | 是 | 该渠道支持的模型列表 |
| `settings` | object | 否 | 高级设置（映射、覆盖等） |

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

## 请求参数覆盖

请求参数覆盖允许为渠道强制设置默认参数，无论上游请求携带了什么内容。配置时提供一个 JSON 对象，系统会在转发请求前自动合并。

### 使用场景

- 强制确定性响应（低温度）
- 限制 token 使用量以控制成本
- 强制特定的响应格式（JSON 等）
- 应用渠道特定的默认值

### 配置

```yaml
# 示例：强制输出确定性的 JSON 结构
settings:
  overrideParameters: |
    {
      "temperature": 0.3,
      "max_tokens": 1024,
      "response_format.type": "json_object"
    }
```

### 支持的字段

- **顶层字段**：`temperature`、`max_tokens`、`top_p`、`frequency_penalty`、`presence_penalty`
- **嵌套字段**：使用点分写法表示嵌套字段（例如 `response_format.type`）
- **无效 JSON**：系统会记录告警日志并保持原始请求不变

## 渠道类型

### OpenAI

```yaml
type: "openai"
base_url: "https://api.openai.com/v1"
credentials:
  api_key: "sk-..."
```

### Anthropic

```yaml
type: "anthropic"
base_url: "https://api.anthropic.com/v1"
credentials:
  api_key: "sk-ant-..."
```

### Gemini

```yaml
type: "gemini"
base_url: "https://generativelanguage.googleapis.com/v1beta"
credentials:
  api_key: "..."
```

### OpenRouter

```yaml
type: "openrouter"
base_url: "https://openrouter.ai/api/v1"
credentials:
  api_key: "sk-or-..."
```

### Zhipu

```yaml
type: "zhipu"
base_url: "https://open.bigmodel.cn/api/paas/v4"
credentials:
  api_key: "..."
```

## 最佳实践

1. **启用前测试**：在启用渠道之前始终测试连接
2. **使用有意义的名称**：使用描述性的渠道名称以便识别
3. **记录映射**：记录模型映射以便维护
4. **监控使用情况**：定期检查渠道使用情况和性能
5. **备份凭据**：安全存储凭据并制定备份计划

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

## 相关文档

- [模型管理指南](model-management.md) - 跨渠道管理模型
- [负载均衡指南](load-balance.md) - 在多个渠道间分发请求
- [API 密钥配置指南](api-key-profiles.md) - 组织 API 密钥和权限
