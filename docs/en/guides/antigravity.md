# Antigravity Integration Guide

---

## Overview

AxonHub supports Google's Antigravity API as a channel provider, offering access to Claude, Gemini, and GPT-OSS models through Google's internal infrastructure. This guide explains how to configure Antigravity channels and leverage its advanced features like intelligent endpoint fallback and dual quota pools.

### Key Points
- Antigravity provides access to multiple model families (Claude, Gemini, GPT-OSS) through a unified Google infrastructure
- AxonHub automatically handles endpoint failover across Daily, Autopush, and Production environments
- Per-model cooldown tracking prevents wasted requests to rate-limited endpoints
- Support for dual quota pools allows maximizing available capacity

### Prerequisites
- AxonHub instance with channel management access
- Valid Google account with Antigravity access
- OAuth credentials obtained through the Antigravity OAuth flow

---

## Configuring Antigravity Channel

### Getting OAuth Credentials

1. Navigate to the **Channels** section in the AxonHub management interface

2. Click **Create Channel** and select **Antigravity** as the channel type

3. Click **Start OAuth** to initiate the authentication flow

4. A Google OAuth URL will be generated. Click **Open OAuth Link** to authenticate in your browser

5. After successful authentication, you'll be redirected to a callback URL. Copy the entire callback URL

6. Paste the callback URL into the AxonHub form and click **Exchange & Fill API Key**

7. The credentials will be automatically filled. Complete the channel configuration:
   - **Name**: A descriptive name (e.g., "Antigravity - Daily")
   - **Base URL**: Defaults to `https://daily-cloudcode-pa.sandbox.googleapis.com` (recommended)
   - **Supported Models**: Add the models you want to expose (see Model List below)

8. Click **Test** to verify the connection

9. Enable the channel once the test succeeds

### Available Models

Antigravity provides access to multiple model families:

**Claude Models (Anthropic via Google):**
- `claude-sonnet-4-5`
- `claude-sonnet-4-5-thinking`
- `claude-opus-4-5-thinking`

**Gemini Models (Google):**
- `gemini-2.5-pro`
- `gemini-2.5-flash`
- `gemini-2.5-flash-lite`
- `gemini-3-pro-low`
- `gemini-3-pro-high`
- `gemini-3-pro-medium`
- `gemini-3-flash`
- `gemini-3-pro-image`

**GPT-OSS Models:**
- `gpt-oss-120b-medium`

---

## Endpoint Fallback & Health Tracking

AxonHub implements intelligent endpoint management for Antigravity to maximize availability and quota utilization.

### Available Endpoints

Antigravity operates across three endpoints:

1. **Daily** (`https://daily-cloudcode-pa.sandbox.googleapis.com`)
   - Latest features and models
   - Primary endpoint for Antigravity quota models

2. **Autopush** (`https://autopush-cloudcode-pa.sandbox.googleapis.com`)
   - Staging environment
   - Fallback endpoint

3. **Production** (`https://cloudcode-pa.googleapis.com`)
   - Most stable
   - Primary endpoint for Gemini CLI quota models

### Automatic Failover

When a request fails with a retryable error (429 Rate Limit, 403 Forbidden, 404 Not Found, 5xx Server Error), AxonHub automatically:

1. Records the failure and puts the endpoint into a **60-second cooldown**
2. Retries the request on the next available endpoint
3. Returns the first successful response

**Example:**
```text
Request for claude-sonnet-4-5:
  Daily → 429 Rate Limit → [60s cooldown]
  Autopush → 200 OK ✓

Next request (within 60 seconds):
  [Skip Daily - in cooldown]
  Autopush → 200 OK ✓  (Faster, no wasted attempt)
```

### Per-Model Cooldown Tracking

Cooldowns are tracked separately for each model+endpoint combination:

- If `claude-sonnet-4-5` hits rate limits on Daily, only that specific combination goes into cooldown
- `gemini-2.5-pro` can still use Daily (different model)
- `claude-sonnet-4-5` can still use Autopush/Prod (different endpoints)

This per-model isolation maximizes quota utilization across your entire fleet.

### Fail-Fast Behavior

If **all endpoints are in cooldown** for a specific model, AxonHub returns an error immediately:

```text
Error: all antigravity endpoints in cooldown for model claude-sonnet-4-5
```

This allows upstream retry logic (channel failover, other providers) to handle the request instead of blocking.

---

## Dual Quota Pools

Antigravity supports two separate quota pools: **Antigravity** and **Gemini CLI**. You can access both pools for the same model using special naming conventions.

### Default Quota Assignment

**Antigravity Quota** (uses Daily endpoint first):
- All Claude models (`claude-*`)
- All GPT-OSS models (`gpt-*`)
- Image generation models (`*-image`, `*-imagen`)
- Legacy Gemini 3 models (`gemini-3-pro-low`, `gemini-3-flash`, etc.)

**Gemini CLI Quota** (uses Production endpoint first):
- Standard Gemini models (`gemini-2.5-pro`, `gemini-2.5-flash`, `gemini-1.5-pro`)
- Preview models (`gemini-3-pro-preview`, `gemini-3-flash-preview`)

### Explicit Quota Override

You can override the default quota pool using suffix notation:

**Suffix Override:**
- `:antigravity` - Force Antigravity quota
- `:gemini-cli` - Force Gemini CLI quota

**Example:**
```text
gemini-2.5-pro              → Gemini CLI quota (default)
gemini-2.5-pro:antigravity  → Antigravity quota (override)
```

### Dual Pool Access

Add the `antigravity-` prefix to access both quota pools for the same model:

**Prefix Override:**
```text
gemini-2.5-pro                  → Gemini CLI quota only
antigravity-gemini-2.5-pro      → Antigravity quota (separate pool)
```

**Use Case:** Configure both model names in your supported models list to maximize available quota:
```yaml
supported_models:
  - gemini-2.5-pro               # Uses Gemini CLI quota
  - antigravity-gemini-2.5-pro   # Uses Antigravity quota
```

When one quota pool is exhausted, AxonHub can automatically fail over to the other pool via channel retry logic.

---

## Model Routing Examples

| Model Name | Quota Pool | Initial Endpoint | Fallback Sequence |
|------------|-----------|------------------|-------------------|
| `claude-sonnet-4-5` | Antigravity | Daily | Daily → Autopush → Prod |
| `gemini-2.5-pro` | Gemini CLI | Prod | Prod → Daily → Autopush |
| `gemini-2.5-pro:antigravity` | Antigravity | Daily | Daily → Autopush → Prod |
| `antigravity-gemini-2.5-pro` | Antigravity | Daily | Daily → Autopush → Prod |
| `gemini-3-flash` | Antigravity | Daily | Daily → Autopush → Prod |
| `gpt-oss-120b-medium` | Antigravity | Daily | Daily → Autopush → Prod |

---

## Best Practices

### Quota Maximization

1. **Use Dual Pools**: Configure both standard and `antigravity-` prefixed models for maximum capacity
   ```yaml
   supported_models:
     - gemini-2.5-pro
     - antigravity-gemini-2.5-pro
   ```

2. **Model Profiles**: Use AxonHub model profiles to route between quota pools automatically
   - Create profile mapping `gemini-2.5-pro` → `antigravity-gemini-2.5-pro` as fallback
   - When Gemini CLI quota exhausts, profile routing switches to Antigravity quota

3. **Multiple Channels**: Create separate channels for each endpoint to control priority manually
   - Channel 1: Antigravity (Daily endpoint) - Priority 10
   - Channel 2: Antigravity (Production endpoint) - Priority 5

### Performance Optimization

1. **Cooldown Awareness**: Configure health checks to detect when cooldowns are active

2. **Channel Priority**: Set higher priority for Production endpoint if stability is more important than latest features

3. **Load Balancing**: Use AxonHub's adaptive load balancing to distribute load across healthy endpoints

### Monitoring

Track these metrics in AxonHub traces:

- **Endpoint failures**: Frequency of 429/5xx errors per endpoint
- **Cooldown events**: How often endpoints enter cooldown
- **Fallback success rate**: Percentage of requests that succeed on fallback endpoints
- **Model-specific quota exhaustion**: Which models/quotas hit limits most often

---

## Troubleshooting

### Channel Test Fails

**Symptoms**: OAuth exchange succeeds but channel test fails

**Solutions**:
- Verify the project ID in credentials is correct
- Check that the base URL is reachable from your AxonHub instance
- Ensure the model name is valid for the selected endpoint
- Try a different endpoint (Daily, Autopush, or Prod)

### All Endpoints in Cooldown

**Symptoms**: Requests fail with "all antigravity endpoints in cooldown"

**Solutions**:
- Configure additional Antigravity channels as fallback
- Use model profiles to route to alternative providers
- Implement request queuing/rate limiting on the client side
- Consider upgrading quota limits with Google

### Per-Model Quota Exhaustion

**Symptoms**: One model consistently hits rate limits while others work fine

**Solutions**:
- Add `antigravity-` prefixed version to access dual quota pool
- Use model profiles to route to alternative models (e.g., `gemini-2.5-pro` → `gemini-2.5-flash`)
- Distribute load across multiple model variants
- Check if requests can be batched or cached

### OAuth Token Expired

**Symptoms**: Requests fail with 401 Unauthorized after channel was working

**Solutions**:
- Re-run OAuth flow to get fresh credentials
- Update channel with new OAuth credentials
- Check token expiration time in channel settings
- Enable automatic token refresh if available

### Model Not Available on Endpoint

**Symptoms**: Requests fail with 404 Not Found

**Solutions**:
- Verify the model name is correct (check supported models list above)
- Try a different endpoint (some models are endpoint-specific)
- Check Google's Antigravity documentation for model availability
- Use `antigravity-` prefix or `:antigravity` suffix to try Antigravity quota pool

---

## Advanced Configuration

### Custom Cooldown Duration

Cooldown duration is fixed at 60 seconds by default. This is hard-coded in the AxonHub implementation but can be modified by:

1. Forking the repository
2. Modifying `DefaultCooldownDuration` in `llm/transformer/antigravity/health_tracker.go`
3. Rebuilding AxonHub

### Health Tracker Statistics

The health tracker maintains statistics about endpoint health. To access these programmatically:

```go
// Example: Access health tracker stats (for custom integrations)
stats := healthTracker.Stats()
fmt.Printf("Total entries: %d\n", stats.TotalEntries)
fmt.Printf("In cooldown: %d\n", stats.InCooldown)
fmt.Printf("Expired: %d\n", stats.Expired)
```

### Memory Management

The health tracker uses lazy cleanup with TTL-based expiration:
- Entries expire after **10 minutes** of inactivity (no failures or successes)
- Memory usage: ~200 bytes per model+endpoint combination
- Typical deployment: 20 models × 3 endpoints = ~12 KB total memory

No background cleanup is performed; entries are removed when accessed after TTL expiration.

---

## Related Documentation

- [Claude Code Integration Guide](claude-code-integration.md)
- [Channel Management Guide](channel-management.md)
- [Model Profiles](../../../README.md#model-profiles)
- [Tracing Guide](tracing.md)
- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)

---

## FAQ

**Q: How long do endpoints stay in cooldown?**  
A: 60 seconds after the last failure. Successful requests clear the cooldown immediately.

**Q: Can I use multiple Antigravity channels?**  
A: Yes! Create separate channels for each endpoint (Daily, Autopush, Prod) and use priority/load balancing to control routing.

**Q: Does the health tracker persist across restarts?**  
A: No, the health tracker is in-memory only. Cooldown state is lost on restart, but this is intentional for clean slate recovery.

**Q: What happens if I use an invalid model name?**  
A: The request will fail with 404 Not Found, triggering endpoint fallback. If all endpoints return 404, AxonHub returns the error to the client.

**Q: Can I disable endpoint fallback?**  
A: Not currently, but you can configure only one endpoint per channel to effectively disable fallback for that channel.

**Q: How do I know which endpoint served my request?**  
A: Check the AxonHub trace logs. Successful fallback attempts log: "antigravity request succeeded with fallback endpoint".
