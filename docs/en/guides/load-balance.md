# Adaptive Load Balancing Guide

AxonHub provides an intelligent adaptive load balancing system that automatically selects optimal AI channels based on multiple dimensions, ensuring high availability and optimal performance.

## 🎯 Core Features

### Intelligent Channel Selection
- **Priority Grouping** - Candidates are first grouped by model association priority (Lower value = Higher priority)
- **Session Consistency** - Requests from the same conversation are prioritized to route to previously successful channels
- **Health Awareness** - Automatically avoids channels with high error rates
- **Fair Distribution** - Uses Weighted Round Robin to distribute requests proportionally based on channel weights
- **Latency Awareness** - Uses request-type-specific UX signals: streaming requests prioritize lower first-token latency and higher output throughput, while non-streaming requests prioritize lower end-to-end latency
- **Rate Limit Awareness** - Respects upstream RPM/TPM/concurrency limits, falls back to the default connection tracker when `MaxConcurrent` is not explicitly configured, and automatically cools down channels on 429 Retry-After

### Multi-Strategy Scoring System
Load balancing follows a hierarchical process: first by **Association Priority**, then by **Strategy Scoring** within each priority group.

| Level | Strategy | Score Range | Description |
|-------|----------|-------------|-------------|
| **1** | **Association Priority** | 0-N (Lower is better) | Hard grouping defined in model associations |
| **2** | **Trace Aware** | 0-1000 points | Same session priority, ensures conversation continuity |
| **3** | **Error Aware** | 0-200 points | Based on success rate and error history |
| **4** | **Weight Round Robin** | 10-150 points | Proportional distribution based on weight and history |
| **5** | **Latency Aware** | 0-80 points | Streaming requests use FTTL + TPS, non-streaming requests use end-to-end latency |
| **6** | **Rate Limit Aware** | -10000-100 points | Respects RPM/TPM/concurrency limits and 429 Retry-After |

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
name: "openai-third-party"
type: "openai"
weight: 30   # Low priority
base_url: "https://api.example.com/v1"
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
  - Consecutive failures: -30 points per failure, decaying over the cooldown window
  - Recent failure (within 5 min): up to -40 points (time-decaying)
- **Recovery**: Failed channels automatically recover priority over time as the time-decay penalty decreases.

### Weight Round Robin Strategy
- **Purpose**: Proportional distribution based on weight and historical load.
- **Algorithm**: Normalizes historical request counts by channel weight. Higher weight channels can handle more requests before their score drops.
- **Scoring**: `150 * exp(-normalized_request_count / 150)`
- **Range**: 10-150 points

### Latency Aware Strategy
- **Purpose**: Prioritize channels using latency signals that match actual user experience
- **Mechanism**:
  - **Streaming requests**: score channels by EWMA first-token latency (FTTL/TTFT) and EWMA output throughput (tokens per second)
  - **Non-streaming requests**: score channels by EWMA end-to-end latency
- **Scoring**:
  - **Streaming**: `80 * (0.7 * first_token_component + 0.3 * throughput_component)`
  - **Non-streaming**: `80 * (1 - latency_ewma / 3000)`, clamped to [0, 80]
- **EWMA Alpha**: 0.3 (recent requests have higher influence)
- **Neutral Score**: Channels without latency data receive a neutral score of 40 points
- **Range**: 0-80 points

### Rate Limit Aware Strategy
- **Purpose**: Respect upstream provider rate limits and prevent 429 errors
- **Mechanism**: Tracks per-channel RPM (Requests Per Minute), TPM (Tokens Per Minute), and concurrent request counts using a 1-minute sliding window
- **Scoring**:
  - Full score (100 points) when neither explicit rate limits nor default connection saturation signals apply
  - Score decreases linearly as usage approaches the limit: `100 * (1 - max_usage_ratio)`
  - **-10000 points** (ranked last as fallback) when any limit is exhausted
- **429 Retry-After**: When a channel returns HTTP 429 with a `Retry-After` header, the channel enters a cooldown period and receives -10000 points until the cooldown expires
- **Configuration**: Set RPM, TPM, and Max Concurrent limits per channel in the management interface under **Rate Limit** settings
- **Concurrency Fallback**: If `MaxConcurrent` is not configured but the default connection tracker has a per-channel capacity, adaptive balancing still penalizes channels with many in-flight requests and treats fully saturated channels as exhausted fallback candidates

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
- Adjust channel priorities based on cost considerations
- Use session consistency to improve user experience

## 🔗 Related Documentation

- [Request Processing Guide](request-processing.md)
- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)
- [Channel Management Guide](../getting-started/quick-start.md)
- [Tracing and Debugging](tracing.md)
