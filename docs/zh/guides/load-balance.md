# 自适应负载均衡指南

AxonHub 提供智能的自适应负载均衡系统，能够根据多个维度自动选择最优的 AI 通道，确保高可用性和最佳性能。

## 🎯 核心特性

### 智能通道选择
- **会话一致性** - 同一对话的请求优先路由到之前成功的通道
- **健康状态感知** - 自动避开错误率高的通道
- **权重均衡** - 支持管理员设置通道优先级
- **实时负载** - 根据当前连接数动态调整

### 多策略评分系统
每个通道都会被多个策略评分，总分最高的通道优先使用：

| 策略 | 评分范围 | 说明 |
|------|----------|------|
| **会话感知** | 0-1000 分 | 同一会话优先，确保对话连续性 |
| **错误感知** | 0-200 分 | 基于成功率和错误历史 |
| **权重策略** | 0-100 分 | 管理员设置的通道权重 |
| **连接负载** | 0-50 分 | 当前连接使用率 |

## 🚀 快速开始

### 1. 配置多个通道
在管理界面中添加多个相同模型的通道：

```yaml
# 通道 A - 主力通道
name: "openai-primary"
type: "openai"
weight: 100  # 高优先级
base_url: "https://api.openai.com/v1"

# 通道 B - 备用通道  
name: "openai-backup"
type: "openai"
weight: 50   # 中等优先级
base_url: "https://api.openai.com/v1"

# 通道 C - 第三方通道
name: "azure-openai"
type: "azure"
weight: 30   # 低优先级
base_url: "https://your-resource.openai.azure.com"
```

### 2. 启用负载均衡
负载均衡自动启用，无需额外配置。系统会：

- 自动检测通道健康状态
- 根据策略评分排序通道
- 智能选择最优通道
- 失败时自动切换到下一个通道

### 3. 发送请求
使用标准的 OpenAI API 格式：

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

# 系统会自动选择最优通道
response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

## 📊 负载均衡策略详解

### 会话感知策略 (TraceAware)
- **目的**: 保持多轮对话的通道一致性
- **机制**: 如果请求包含 trace ID，优先使用之前成功的通道
- **优势**: 避免通道切换导致的初始化延迟
- **评分**: 匹配通道获得 1000 分，否则 0 分

### 错误感知策略 (ErrorAware)
- **目的**: 避开不健康的通道
- **评分因素**:
  - 连续失败：每次 -50 分
  - 最近失败（5分钟内）：最多 -100 分
  - 成功率 >90%：+30 分
  - 成功率 <50%：-50 分
- **恢复**: 失败通道会随时间自动恢复优先级

### 权重策略 (Weight)
- **目的**: 尊重管理员设置的通道优先级
- **评分**: `通道权重 / 100 * 100`
- **范围**: 0-100 分

### 连接感知策略 (Connection)
- **目的**: 避免单个通道过载
- **评分**: 基于当前连接使用率
- **机制**: 使用率越低，分数越高

## 🔧 高级配置

### 启用调试模式
查看详细的负载均衡决策过程：

```bash
# 设置环境变量
export AXONHUB_LOAD_BALANCER_DEBUG=true

# 或在请求中启用
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "X-Debug-Mode: true" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}'
```

### 查看决策日志
```bash
# 查看负载均衡决策
tail -f axonhub.log | grep "Load balancing decision"

# 查看具体通道评分
tail -f axonhub.log | grep "Channel load balancing details"

# 使用 jq 格式化 JSON 日志
tail -f axonhub.log | jq 'select(.msg | contains("Load balancing"))'
```

## 📈 监控和故障排查

### 关键指标
- **通道切换频率** - 正常情况下应该较低
- **错误率分布** - 某个通道错误率过高可能需要检查配置
- **响应时间** - 负载均衡应该优化整体响应时间

### 常见问题

**Q: 为什么请求总是路由到同一个通道？**
A: 检查是否启用了会话一致性。同一 trace ID 的请求会优先使用相同通道。

**Q: 通道不切换怎么办？**
A: 查看错误感知策略的评分。通道可能仍然健康，或者需要时间恢复。

**Q: 如何验证负载均衡是否工作？**
A: 启用调试模式，查看日志中的通道评分和排序。

## 🎛️ 最佳实践

### 1. 通道配置
- 设置不同的权重值体现优先级
- 配置多个不同提供商的通道提高可用性
- 定期检查通道健康状态

### 2. 监控设置
- 监控各通道的错误率和响应时间
- 设置告警当某个通道持续失败
- 定期分析负载均衡决策日志

### 3. 性能优化
- 地理位置相近的通道设置更高权重
- 根据成本考虑调整通道优先级
- 使用会话一致性提高用户体验

## 🔗 相关文档

- [统一 API 文档](../api-reference/unified-api.md)
- [通道管理指南](../getting-started/quick-start.md)
- [追踪和调试](tracing.md)
