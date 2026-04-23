# 渠道配置指南

本指南介绍如何在 AxonHub 中配置 AI 服务提供商（如 OpenAI、Anthropic、DeepSeek 等）。

## 什么是渠道？

**渠道**是 AxonHub 连接 AI 提供商的通道。你可以把渠道理解为"供应商连接线"——每个渠道对应一个 AI 服务商（如 OpenAI、Claude、DeepSeek）。

通过渠道，你可以：
- 同时连接多个 AI 服务商
- 设置模型名称转换规则
- 启用或暂停某个服务商
- 配置多个 API Key 实现负载均衡

## 渠道模型映射在请求流程中的位置

渠道模型映射是三层流水线中的**最后一步**。完整说明请参阅 [请求处理流程](../getting-started/request-processing.md#核心概念三层模型设置)。

简单来说：**API Key Profile 改模型名 → 模型关联选渠道 → 渠道改模型名 → 发给上游**

## 创建渠道

### 基本步骤

1. 进入 AxonHub 管理界面 → **渠道管理**
2. 点击 **新建渠道**
3. 填写基本信息：
   - **名称**：给渠道起个名字（如"OpenAI 主账号"、"DeepSeek 国内"）
   - **类型**：选择服务商类型（OpenAI、Anthropic、DeepSeek 等）
   - **Base URL**：API 地址（一般使用默认值即可）
   - **API Key**：服务商提供的密钥

### 配置示例

**OpenAI 渠道：**

| 字段 | 值 |
|------|-----|
| 名称 | OpenAI 主账号 |
| 类型 | openai |
| Base URL | https://api.openai.com/v1 |
| API Key | sk-your-openai-key |
| 支持模型 | gpt-4o, gpt-4o-mini, gpt-5 |

**DeepSeek 渠道：**

| 字段 | 值 |
|------|-----|
| 名称 | DeepSeek 国内 |
| 类型 | deepseek |
| Base URL | https://api.deepseek.com/v1 |
| API Key | sk-your-deepseek-key |
| 支持模型 | deepseek-chat, deepseek-reasoner |

## 配置多个 API Key

当一个账号有多个 API Key 时，可以都配置到同一个渠道中，AxonHub 会自动轮流使用，提高稳定性。

在渠道编辑界面的 **API Key** 区域，逐行添加多个 Key 即可，例如：
- `sk-key-1`
- `sk-key-2`
- `sk-key-3`

### 负载均衡说明

- 相同的 Trace ID 会始终使用同一个 Key（保证会话一致性）
- 不同请求会随机选择可用的 Key
- 某个 Key 出错时，系统会自动切换到其他 Key

## 模型重命名

AxonHub 在渠道层面提供多种模型重命名和别名机制。当请求到达时，渠道按以下优先级链解析请求模型到实际上游模型：

1. **直接匹配** — 请求模型直接在支持模型列表中
2. **额外模型前缀** — 为所有支持模型添加前缀别名
3. **自动裁剪模型前缀** — 从支持模型中去除已知前缀，创建精简别名
4. **模型映射** — 显式 `from → to` 别名配对

> **注意**：如果多个机制产生了相同的请求模型名，则第一个匹配生效（按上述顺序）。

### 模型映射

**什么时候需要模型映射？**

当你想让客户端用一个名称请求，但实际发给上游的是另一个名称时。

**常见场景：**

1. **客户端用简化的名称**：客户端请求 `gpt-4`，实际发给 OpenAI 的是 `gpt-4o`
2. **统一不同渠道的模型名**：让 `claude-sonnet` 和 `gpt-4` 都指向同一个实际模型
3. **旧版兼容**：客户端请求旧版模型名，自动映射到新版

**配置方法：**

在渠道的 **Settings** → **模型映射** 中添加 `from → to` 配对：

| 客户端请求的模型名 (from) | 实际发给上游的模型名 (to) |
|--------------------------|--------------------------|
| gpt-4o-mini | gpt-4o |
| claude-3-sonnet | claude-3.5-sonnet |

**注意**：目标模型（`to`）必须在支持模型列表中。如果目标模型不在列表中，该映射将被静默忽略。

### 额外模型前缀（Extra Model Prefix）

为支持模型列表中的每个模型添加**带前缀的别名**，允许客户端使用带前缀或不带前缀的格式请求模型。

**使用场景**：你想将渠道中的所有模型归入统一前缀命名空间（如 `deepseek/`）。

**示例：**
- 支持模型：`deepseek-chat`、`deepseek-reasoner`
- 额外模型前缀：`deepseek`

渠道现在**同时**接受以下两种请求格式：
- `deepseek-chat` → 发送 `deepseek-chat` 给上游
- `deepseek/deepseek-chat` → 发送 `deepseek-chat` 给上游

当不同渠道存在同名模型时，客户端可以通过前缀来区分来源渠道。

### 自动裁剪模型前缀（Auto-Trimmed Model Prefixes）

自动**去除支持模型名中的指定前缀**，创建精简别名。这是额外模型前缀的反向操作。

**使用场景**：像 OpenRouter、SiliconFlow 等供应商会在模型名前加上供应商前缀（如 `openai/gpt-5.4`）。你想让客户端直接使用短名 `gpt-5.4` 请求，而不必为每个模型手动创建映射。

**示例：**
- 支持模型：`openai/gpt-5.4`、`anthropic/claude-sonnet-4`、`deepseek-ai/deepseek-chat`
- 自动裁剪模型前缀：`openai`、`anthropic`、`deepseek-ai`

渠道同时接受原始名和精简名：
- `gpt-5.4` → 发送 `openai/gpt-5.4` 给上游
- `claude-sonnet-4` → 发送 `anthropic/claude-sonnet-4` 给上游
- `deepseek-chat` → 发送 `deepseek-ai/deepseek-chat` 给上游
- `openai/gpt-5.4` → 作为直接匹配仍然有效

> **提示**：对于使用供应商前缀模型 ID 的供应商，推荐使用此功能。它可以批量重写模型 ID，无需逐个创建模型映射。

### 可见性控制

两个选项控制哪些模型名在模型列表中可见（例如客户端调用 `/v1/models` 接口时）：

| 选项 | 效果 |
|------|------|
| **隐藏原始模型** | 隐藏原始（直接匹配）的模型名。仅显示经过转换的名称（来自前缀、自动裁剪或映射）。 |
| **隐藏映射模型** | 隐藏模型映射的 `from` 名称。仅显示原始模型名。 |

**示例 — 隐藏原始模型：**
- 支持模型：`openai/gpt-5.4`
- 自动裁剪前缀：`openai`
- 隐藏原始模型：启用

`/v1/models` 响应只显示 `gpt-5.4`，不显示 `openai/gpt-5.4`。两个名称都可用于请求。

**示例 — 隐藏映射模型：**
- 支持模型：`gpt-4o`
- 模型映射：`gpt-4` → `gpt-4o`
- 隐藏映射模型：启用

`/v1/models` 响应显示 `gpt-4o` 但隐藏 `gpt-4`。两个名称都可用于请求。

## 测试和启用渠道

### 测试连接

在启用渠道前，建议先测试连接：

1. 在渠道列表中找到刚创建的渠道
2. 点击 **测试** 按钮
3. 等待测试结果
4. 如果显示成功，说明配置正确

### 启用渠道

测试通过后，点击 **启用** 按钮，渠道状态变为 **活跃**，即可开始接收请求。

## Base URL 特殊配置

### 默认地址

| 服务商 | 默认 Base URL |
|-------|--------------|
| OpenAI | `https://api.openai.com/v1` |
| Anthropic | `https://api.anthropic.com` |
| DeepSeek | `https://api.deepseek.com/v1` |
| Gemini | `https://generativelanguage.googleapis.com/v1beta` |

### 自定义地址

如果使用代理或私有化部署，可以修改 Base URL。

**禁用版本号自动追加**：在 URL 末尾加 `#`
```
https://custom-proxy.example.com/api#
# 实际请求: /api/messages（不会自动加 /v1）
```

**完全原始模式**：在 URL 末尾加 `##`
```
https://custom-gateway.example.com/api##
# 实际请求: /api（不会加版本号和端点路径）
```

## 常见问题

### Q: 测试连接失败怎么办？

- 检查 API Key 是否正确（复制时是否有多余空格）
- 确认 Base URL 是否可访问
- 检查服务商账户是否有余额/额度

### Q: 请求时提示"模型未找到"？

- 确认模型已在渠道的 `supported_models` 中
- 检查模型映射配置是否正确
- 确认渠道已启用

### Q: 如何设置多个 API Key？

在 `credentials.api_keys` 中列出所有 Key，系统会自动轮询使用。

### Q: API Key 被禁用了怎么恢复？

进入渠道详情，在 **禁用列表** 中找到该 Key，点击 **恢复**。

## 相关文档

- [模型管理指南](model-management.md) - 配置模型与渠道的关联关系
- [API Key Profile 指南](api-key-profiles.md) - 配置模型映射和访问权限
- [请求处理流程](../getting-started/request-processing.md) - 了解完整请求链路
