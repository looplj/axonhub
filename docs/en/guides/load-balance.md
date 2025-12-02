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
