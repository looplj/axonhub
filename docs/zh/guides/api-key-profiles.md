# API Key Profile 指南

## 概述

API Key Profile 是一个强大的配置系统，允许为每个 API Key 定义多个配置文件。每个配置文件包含模型映射、渠道限制和模型访问控制。通过切换激活配置，可以改变请求的处理方式，而无需修改 API Key 本身。

## 什么是 API Key Profile？

API Key Profile 使您能够：

- **映射模型**：使用精确匹配或正则表达式将用户请求的模型转换为实际可用的模型
- **限制渠道**：通过渠道 ID 或标签限制配置文件可以使用的渠道
- **过滤模型**：控制特定配置文件可访问的模型
- **切换配置**：通过激活不同的配置文件即时更改行为

### 核心概念

| 组件 | 描述 |
|------|------|
| **配置文件 (Profile)** | 包含模型映射和访问规则的命名配置 |
| **激活配置 (Active Profile)** | 当前处理请求的已启用配置 |
| **模型映射 (Model Mapping)** | 将源模型转换为目标模型的规则 |
| **渠道限制 (Channel Restrictions)** | 配置文件可使用渠道的限制 |

## 为什么使用 API Key Profile？

### 1. 模型转换

将面向用户的模型名称转换为内部模型标识符或替代提供商：

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

### 2. 环境隔离

为开发、测试和生产环境维护独立的配置：

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

### 3. 访问控制

限制 API Key 只能使用特定渠道或模型：

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

### 4. A/B 测试

通过切换配置文件测试不同的模型或提供商：

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

## 如何使用 API Key Profile

### 步骤 1：访问配置文件管理

1. 导航到管理界面中的 **API Keys** 页面
2. 找到要配置的 API Key
3. 点击右侧的 **操作** 菜单（三个点）
4. 从下拉菜单中选择 **Profiles** 或 **配置**

### 步骤 2：创建配置文件

配置文件对话框允许您：

- **添加配置**：点击"新增配置"创建新的配置
- **设置配置名称**：为每个配置文件提供描述性名称
- **配置模型映射**：添加转换模型的规则
- **设置渠道限制**：通过 ID 或标签选择允许的渠道
- **设置模型限制**：指定可访问的模型

### 步骤 3：配置模型映射

每个模型映射包含：

- **源模型 (From)**：用户请求中的模型名称
  - 支持精确匹配：`"gpt-4"`
  - 支持正则表达式：`"gpt-.*"`、`".*"`（通配符）
- **目标模型 (To)**：实际使用的模型

**映射示例：**

```json
{
  "modelMappings": [
    {"from": "gpt-4", "to": "claude-3-opus"},
    {"from": "gpt-3.5-turbo", "to": "claude-3-haiku"},
    {"from": "gpt-.*", "to": "claude-3-sonnet"}
  ]
}
```

### 步骤 4：设置激活配置

选择应该激活的配置文件：

1. 从"生效配置"下拉菜单中选择配置文件名称
2. 选中的配置文件将处理所有传入请求
3. 您可以随时更改激活配置

### 步骤 5：保存配置

点击 **保存** 应用更改。API Key 将立即开始使用新配置。

## 模型映射详解

### 精确匹配

映射特定的模型名称：

```json
{"from": "gpt-4", "to": "claude-3-opus"}
```

这只会将 `gpt-4` 的请求转换为 `claude-3-opus`。

### 正则表达式模式

使用正则表达式匹配多个模型：

```json
{"from": "gpt-.*", "to": "claude-3-sonnet"}
```

这将把任何以 `gpt-` 开头的模型转换为 `claude-3-sonnet`。

### 通配符

匹配所有模型：

```json
{"from": ".*", "to": "gpt-4"}
```

这将把所有请求转换为使用 `gpt-4`。

### 评估顺序

模型映射按顺序评估，第一个匹配的规则将被应用：

```json
{
  "modelMappings": [
    {"from": "gpt-4", "to": "claude-3-opus"},
    {"from": "gpt-.*", "to": "claude-3-sonnet"}
  ]
}
```

在此示例中：
- `gpt-4` → `claude-3-opus`（精确匹配优先）
- `gpt-3.5-turbo` → `claude-3-sonnet`（正则匹配）

## 渠道和模型限制

### 渠道限制

控制配置文件可以使用的渠道：

**按渠道 ID：**

```json
{
  "channelIDs": [1, 2, 3]
}
```

**按渠道标签：**

```json
{
  "channelTags": ["production", "high-priority"]
}
```

**组合使用：**

```json
{
  "channelIDs": [1, 2],
  "channelTags": ["production"]
}
```

如果同时指定两者，渠道必须匹配任一条件。

### 模型限制

限制可访问的模型：

```json
{
  "modelIDs": ["gpt-4", "claude-3-opus", "claude-3-sonnet"]
}
```

只有此列表中的模型才能通过此配置文件访问。

## 验证规则

### 配置名称

- 在 API Key 内必须唯一（不区分大小写）
- 不能为空或仅包含空格
- 示例：`"production"`、`"staging"`、`"development"`

### 激活配置

- 必须引用现有的配置文件名称
- 不能设置为不存在的配置文件

### 模型映射

- 源模型不能为空
- 目标模型不能为空
- 模式将作为有效的正则表达式进行验证

## API 使用示例

### 使用 OpenAI SDK 的配置文件

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

# 请求使用 gpt-4，但将根据激活配置进行映射
response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### 使用 cURL 的配置文件

```bash
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-axonhub-api-key" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

实际使用的模型取决于激活配置的模型映射。

## 最佳实践

### 1. 使用描述性配置名称

选择清楚表明配置文件用途的名称：

- ✅ `"production-gpt4"`、`"staging-claude"`、`"dev-low-cost"`
- ❌ `"profile1"`、`"test"`、`"config"`

### 2. 策略性地排序模型映射

将更具体的模式放在通用模式之前：

```json
{
  "modelMappings": [
    {"from": "gpt-4-turbo", "to": "claude-3-opus"},
    {"from": "gpt-4", "to": "claude-3-sonnet"},
    {"from": "gpt-.*", "to": "claude-3-haiku"}
  ]
}
```

### 3. 在生产前测试配置

1. 创建测试 API Key
2. 使用相同设置配置配置文件
3. 使用示例请求进行测试
4. 验证模型映射按预期工作
5. 应用于生产 API Key

### 4. 记录配置更改

维护配置文件配置的文档：

```markdown
## API Key: production-service

### 配置文件: production
- 用途：生产流量
- 激活：是
- 模型映射：
  - gpt-4 → claude-3-opus
  - gpt-3.5-turbo → claude-3-haiku
- 渠道标签：["production"]
```

### 5. 使用渠道标签提高灵活性

不要硬编码渠道 ID，而是使用标签：

```json
{
  "channelTags": ["production", "high-availability"]
}
```

这允许您添加/删除渠道而无需更新配置文件。

## 故障排查

### 模型未映射

**问题**：请求使用原始模型而不是映射的模型。

**解决方案**：
- 验证激活配置设置正确
- 检查模型映射是否包含请求的模型
- 确保正则表达式模式匹配模型名称
- 检查日志中的映射应用消息

### 配置文件未找到

**问题**：错误消息"激活配置 'xxx' 不存在于配置文件列表中"

**解决方案**：
- 确保激活配置名称与现有配置匹配
- 检查配置名称是否有拼写错误
- 验证配置已成功保存

### 渠道访问被拒绝

**问题**：请求因渠道访问错误而失败。

**解决方案**：
- 验证渠道 ID 正确
- 检查渠道标签是否匹配现有渠道
- 确保渠道已启用且健康
- 查看渠道权限

### 模型不可用

**问题**：请求失败，因为映射的模型不存在。

**解决方案**：
- 验证目标模型存在于系统中
- 检查模型在渠道中是否已启用
- 确保模型 ID 完全匹配（区分大小写）
- 查看模型映射是否有拼写错误

## 高级用例

### 多环境设置

为每个环境配置不同的配置文件：

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

### 成本优化

将不同模型路由到具有成本效益的替代方案：

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

### 提供商迁移

逐步从一个提供商迁移到另一个提供商：

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

准备好完成迁移时，切换到 `"anthropic"` 配置文件。

## 相关文档

- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)
- [负载均衡指南](load-balance.md)
- [细粒度权限](permissions.md)
- [快速入门指南](../getting-started/quick-start.md)
