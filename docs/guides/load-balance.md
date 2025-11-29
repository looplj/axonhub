# Adaptive Load Balancing Guide

AxonHub provides an intelligent adaptive load balancing system that automatically selects optimal AI channels based on multiple dimensions, ensuring high availability and optimal performance.

## 🎯 Core Features

### Intelligent Channel Selection
- **Session Consistency** - Requests from the same conversation are prioritized to route to previously successful channels
- **Health Awareness** - Automatically avoids channels with high error rates
- **Weight Balancing** - Supports admin-configured channel priorities
- **Real-time Load** - Dynamically adjusts based on current connection count

### Multi-Strategy Scoring System
Each channel is scored by multiple strategies, with the highest-scoring channel getting priority:

| Strategy | Score Range | Description |
|----------|-------------|-------------|
| **Trace Aware** | 0-1000 points | Same session priority, ensures conversation continuity |
| **Error Aware** | 0-200 points | Based on success rate and error history |
| **Weight Strategy** | 0-100 points | Admin-configured channel weights |
| **Connection Load** | 0-50 points | Current connection utilization |

## 🚀 Quick Start

### 1. Configure Multiple Channels
Add multiple channels for the same model in the management interface:

```yaml
# Channel A - Primary channel
name: "openai-primary"
type: "openai"
weight: 100  # High priority
base_url: "https://api.openai.com/v1"

# Channel B - Backup channel  
name: "openai-backup"
type: "openai"
weight: 50   # Medium priority
base_url: "https://api.openai.com/v1"

# Channel C - Third-party channel
name: "azure-openai"
type: "azure"
weight: 30   # Low priority
base_url: "https://your-resource.openai.azure.com"
```

### 2. Enable Load Balancing
Load balancing is automatically enabled, no additional configuration needed. The system will:

- Automatically detect channel health status
- Sort channels based on strategy scores
- Intelligently select the optimal channel
- Automatically switch to the next channel on failure

### 3. Send Requests
Use standard OpenAI API format:

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

# System will automatically select the optimal channel
response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

## 📊 Load Balancing Strategy Details

### Trace Aware Strategy
- **Purpose**: Maintain channel consistency for multi-turn conversations
- **Mechanism**: If request contains trace ID, prioritize previously successful channel
- **Advantage**: Avoids initialization delays from channel switching
- **Scoring**: Matching channel gets 1000 points, otherwise 0 points

### Error Aware Strategy
- **Purpose**: Avoid unhealthy channels
- **Scoring Factors**:
  - Consecutive failures: -50 points per failure
  - Recent failure (within 5 min): up to -100 points
  - Success rate >90%: +30 points
  - Success rate <50%: -50 points
- **Recovery**: Failed channels automatically recover priority over time

### Weight Strategy
- **Purpose**: Respect admin-configured channel priorities
- **Scoring**: `channel_weight / 100 * 100`
- **Range**: 0-100 points

### Connection Strategy
- **Purpose**: Prevent individual channel overload
- **Scoring**: Based on current connection utilization
- **Mechanism**: Lower utilization = higher score

## 🔧 Advanced Configuration

### Enable Debug Mode
View detailed load balancing decision process:

```bash
# Set environment variable
export AXONHUB_LOAD_BALANCER_DEBUG=true

# Or enable in request
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "X-Debug-Mode: true" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}'
```

### View Decision Logs
```bash
# View load balancing decisions
tail -f axonhub.log | grep "Load balancing decision"

# View specific channel scoring
tail -f axonhub.log | grep "Channel load balancing details"

# Use jq to format JSON logs
tail -f axonhub.log | jq 'select(.msg | contains("Load balancing"))'
```

## 📈 Monitoring and Troubleshooting

### Key Metrics
- **Channel switching frequency** - Should be relatively low under normal conditions
- **Error rate distribution** - High error rate on a channel may indicate configuration issues
- **Response time** - Load balancing should optimize overall response time

### Common Issues

**Q: Why do requests always route to the same channel?**
A: Check if session consistency is enabled. Requests with the same trace ID will prioritize the same channel.

**Q: What to do if channels don't switch?**
A: Check Error Aware strategy scoring. The channel may still be healthy or needs time to recover.

**Q: How to verify load balancing is working?**
A: Enable debug mode and view channel scoring and sorting in logs.

## 🎛️ Best Practices

### 1. Channel Configuration
- Set different weight values to reflect priorities
- Configure multiple different provider channels for higher availability
- Regularly check channel health status

### 2. Monitoring Setup
- Monitor error rates and response times for each channel
- Set alerts when a channel continuously fails
- Regularly analyze load balancing decision logs

### 3. Performance Optimization
- Set higher weights for geographically closer channels
- Adjust channel priorities based on cost considerations
- Use session consistency to improve user experience

## 🔗 Related Documentation

- [Unified API Documentation](../api-reference/unified-api.md)
- [Channel Management Guide](../getting-started/quick-start.md)
- [Tracing and Debugging](tracing.md)

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

---

