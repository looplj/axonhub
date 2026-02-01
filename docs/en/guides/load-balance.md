# Adaptive Load Balancing Guide

AxonHub provides an intelligent adaptive load balancing system that automatically selects optimal AI channels based on multiple dimensions, ensuring high availability and optimal performance.

## 🎯 Core Features

### Intelligent Channel Selection
- **Priority Grouping** - Candidates are first grouped by model association priority (Lower value = Higher priority)
- **Session Consistency** - Requests from the same conversation are prioritized to route to previously successful channels
- **Health Awareness** - Automatically avoids channels with high error rates
- **Fair Distribution** - Uses Weighted Round Robin to distribute requests proportionally based on channel weights
- **Real-time Load** - Dynamically adjusts based on current connection count

### Multi-Strategy Scoring System
Load balancing follows a hierarchical process: first by **Association Priority**, then by **Strategy Scoring** within each priority group.

| Level | Strategy | Score Range | Description |
|-------|----------|-------------|-------------|
| **1** | **Association Priority** | 0-N (Lower is better) | Hard grouping defined in model associations |
| **2** | **Trace Aware** | 0-1000 points | Same session priority, ensures conversation continuity |
| **3** | **Error Aware** | 0-200 points | Based on success rate and error history |
| **4** | **Weight Round Robin** | 10-150 points | Proportional distribution based on weight and history |
| **5** | **Connection Load** | 0-50 points | Current connection utilization |

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

### Model Association Priority
- **Purpose**: High-level traffic control and hard grouping.
- **Mechanism**: Candidates are first sorted by the `priority` field in the Model Association.
- **Rule**: Lower values have higher priority. All candidates in priority group `N` will be exhausted before any candidate in group `N+1` is considered.
- **Use Case**: Primary/Secondary channel separation, A/B testing (by setting same priority).

### Trace Aware Strategy
- **Purpose**: Maintain channel consistency for multi-turn conversations
- **Mechanism**: If request contains trace ID, prioritize previously successful channel
- **Advantage**: Avoids initialization delays from channel switching
- **Scoring**: Matching channel gets 1000 points, otherwise 0 points

### Error Aware Strategy
- **Purpose**: Avoid unhealthy channels
- **Base Score**: 200 points for healthy channels
- **Scoring Factors**:
  - Consecutive failures: -50 points per failure
  - Recent failure (within 5 min): up to -100 points (time-decaying)
  - Success rate >90%: +30 points
  - Success rate <50%: -50 points
- **Recovery**: Failed channels automatically recover priority over time as the time-decay penalty decreases.

### Weight Round Robin Strategy
- **Purpose**: Proportional distribution based on weight and historical load.
- **Algorithm**: Normalizes historical request counts by channel weight. Higher weight channels can handle more requests before their score drops.
- **Scoring**: `150 * exp(-normalized_request_count / 150)`
- **Range**: 10-150 points

### Connection Strategy
- **Purpose**: Prevent individual channel overload (saturation protection)
- **Scoring**: Based on current connection utilization
- **Mechanism**: `50 * (1 - active_connections / max_connections)`

## 🔧 Advanced Configuration

### Enable Debug Mode
View detailed load balancing decision process by setting the environment variable.

```bash
# Set environment variable
export AXONHUB_DEBUG_LOAD_BALANCER_ENABLED=true
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

- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)
- [Channel Management Guide](../getting-started/quick-start.md)
- [Tracing and Debugging](tracing.md)
