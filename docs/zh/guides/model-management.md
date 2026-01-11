# 模型管理指南

AxonHub 提供灵活的模型管理系统，支持通过模型关联（Model Associations）将抽象模型映射到具体的渠道和模型实现，实现统一的模型接口和智能的渠道选择。

## 🎯 核心概念

### 模型（Model）
AxonHub 中的模型是抽象的模型定义，包含：
- **ModelID** - 模型的唯一标识符（如 `gpt-4`、`claude-3-opus`）
- **Developer** - 模型开发者（如 `openai`、`anthropic`）
- **Settings** - 模型设置，包含关联规则
- **ModelCard** - 模型元数据（能力、成本、限制等）

### 渠道（Channel）
渠道是具体的 AI 服务提供商连接，包含：
- 支持的模型列表
- API 配置和认证信息
- 渠道权重和标签

### 模型关联（Model Association）
模型关联定义了抽象模型与渠道之间的映射关系，支持多种匹配策略。

## 🔗 模型与渠道的关系

### 关系图

```
┌─────────────────┐
│   AxonHub Model │ (抽象模型)
│   ID: gpt-4     │
└────────┬────────┘
         │
         │ Associations (关联规则)
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
         │ Load Balancer (负载均衡)
         │
         ▼
┌─────────────────┐
│   Selected     │ (选中的候选)
│   Candidate     │
└─────────────────┘
```

### 请求流程

1. **请求到达** - 客户端请求使用模型 `gpt-4`
2. **模型查询** - 系统查找 ModelID 为 `gpt-4` 的模型实体
3. **关联解析** - 根据模型的 Associations 解析匹配的渠道和模型
4. **候选生成** - 生成 `ChannelModelCandidate` 列表
5. **负载均衡** - 应用负载均衡策略排序候选
6. **请求执行** - 使用第一个候选执行请求
7. **失败重试** - 失败时切换到下一个候选

## 📋 模型关联类型

### 1. channel_model - 指定渠道的指定模型

精确匹配指定渠道中的指定模型。

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

**使用场景**：
- 需要精确控制某个模型使用特定渠道
- 优先级最高的主力渠道

### 2. channel_regex - 指定渠道的正则匹配

匹配指定渠道中符合正则表达式的所有模型。

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

**模式语法**：模式会自动添加 `^` 和 `$` 锚点来匹配完整模型字符串。使用 `.*` 进行灵活匹配：
- `gpt-4.*` - 匹配 `gpt-4`、`gpt-4-turbo`、`gpt-4-vision-preview`
- `.*flash.*` - 匹配 `gemini-2.5-flash-preview`、`gemini-flash-2.0`
- `claude-3-.*-sonnet` - 匹配 `claude-3-5-sonnet`、`claude-3-opus-sonnet`

**使用场景**：
- 渠道支持多个模型变体
- 需要灵活匹配模型名称

### 3. regex - 所有渠道的正则匹配

匹配所有启用渠道中符合正则表达式的模型。

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

**排除规则**：
- `channelNamePattern` - 排除渠道名称匹配的渠道
- `channelIds` - 排除指定 ID 的渠道
- `channelTags` - 排除包含指定标签的渠道

**使用场景**：
- 广泛匹配多个渠道中的模型
- 需要排除特定渠道

### 4. model - 所有渠道的指定模型

匹配所有启用渠道中的指定模型。

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

**使用场景**：
- 所有支持该模型的渠道都可作为候选
- 需要排除特定渠道

### 5. channel_tags_model - 标签渠道的指定模型

匹配具有指定标签的渠道中的指定模型（OR 逻辑）。

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

**使用场景**：
- 根据渠道标签选择候选
- 环境隔离（生产/测试）

### 6. channel_tags_regex - 标签渠道的正则匹配

匹配具有指定标签的渠道中符合正则表达式的模型（OR 逻辑）。

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

**使用场景**：
- 根据提供商标签选择
- 灵活匹配模型变体

## 🚀 快速开始

### 1. 创建模型

模型通过管理界面进行创建和管理。

### 2. 配置渠道

确保通过管理界面配置渠道并支持相应的模型。

### 3. 使用模型

客户端使用抽象的 ModelID 请求：

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

response = client.chat.completions.create(
    model="gpt-4",  # 使用抽象的 ModelID
    messages=[{"role": "user", "content": "Hello!"}]
)
```

系统会自动：
1. 查找 `gpt-4` 模型实体
2. 解析关联规则
3. 选择最优渠道
4. 映射到实际模型（如 `gpt-4-turbo`）

## 🎛️ 高级配置

### 优先级控制

关联按优先级顺序处理，优先级值越小越优先。当多个候选渠道具有相同的优先级时，系统将根据渠道的 **权重（weight）** 进行负载均衡。

有关详细的负载均衡逻辑和基于权重的分配，请参阅自适应负载均衡指南中的 [加权轮询策略](load-balance.md#加权轮询策略-weight-round-robin)。

```json
{
  "settings": {
    "associations": [
      {
        "type": "channel_model",
        "priority": 0,  // 最高优先级
        "channelModel": {
          "channelId": 1,
          "modelId": "gpt-4-turbo"
        }
      },
      {
        "type": "regex",
        "priority": 10,  // 较低优先级
        "regex": {
          "pattern": "gpt-4.*"
        }
      }
    ]
  }
}
```

### 去重机制

系统自动去重，同一个（渠道，模型）组合只会出现一次。

### 缓存优化

关联解析结果会被缓存，缓存失效条件：
- 渠道数量变化
- 渠道更新时间变化
- 模型更新时间变化
- 缓存过期（5 分钟）

### 回退策略

当模型不存在时，可以回退到传统的渠道选择：

```json
{
  "queryAllChannelModels": true,
  "fallbackToChannelsOnModelNotFound": true
}
```

## 📊 监控和调试

### 调试日志

```bash
# 查看模型选择日志
tail -f axonhub.log | grep "selected model candidates"

# 查看关联解析日志
tail -f axonhub.log | grep "association resolution"

# 查看候选选择日志
tail -f axonhub.log | grep "Load balanced candidates"
```

## 🔧 最佳实践

### 1. 模型设计

- **使用标准化的 ModelID** - 如 `gpt-4`、`claude-3-opus`
- **合理的优先级设置** - 主力渠道优先级 0-10，备用渠道 10-100
- **灵活使用正则** - 避免过于宽泛的模式

### 2. 渠道组织

- **使用标签分类** - 如 `production`、`test`、`backup`
- **设置合理的权重** - 配合负载均衡使用
- **定期清理** - 移除不再使用的渠道

### 3. 关联策略

- **精确优先** - 使用 `channel_model` 作为主力渠道
- **灵活备用** - 使用 `regex` 或 `model` 作为备用
- **环境隔离** - 使用标签关联区分生产/测试环境

### 4. 性能优化

- **利用缓存** - 关联解析结果会被缓存
- **避免频繁更新** - 减少模型和渠道的更新频率
- **监控候选数量** - 过多的候选会影响性能

## 🐛 常见问题

### Q: 为什么请求失败了？

A: 检查以下几点：
1. 模型是否存在且已启用
2. 关联规则是否正确配置
3. 渠道是否启用且支持相应模型
4. 查看日志了解具体错误

### Q: 如何验证关联是否生效？

A:
1. 使用管理界面查询关联结果
2. 启用调试日志查看候选选择过程
3. 发送测试请求验证路由

### Q: 支持多少个关联规则？

A: 理论上无限制，但建议：
- 单个模型不超过 10 条关联
- 总关联数量不超过 100 条
- 避免过于复杂的正则表达式

### Q: 如何实现 A/B 测试？

A: 使用相同优先级的多个关联：

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

负载均衡会在相同优先级的候选中选择。

### Q: 如何排除特定渠道？

A: 使用 `exclude` 字段：

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

## 🔗 相关文档

- [自适应负载均衡指南](load-balance.md)
- [API Key Profiles 指南](api-key-profiles.md)
- [统一 API 文档](../api-reference/unified-api.md)
- [追踪和调试](tracing.md)
