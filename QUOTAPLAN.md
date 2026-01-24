# Provider Quota Tracking System - Implementation Plan

## Overview
Build an extensible, active-polling quota tracking system supporting both Claude Code and Codex providers. The system will poll every 20 minutes, store current status only (no historical data), and focus on capture/storage without UI integration or alerting.

## Design Decisions

- **Polling Interval:** Fixed 20 minutes for all providers
- **Error Handling:** Log errors and wait for next cycle (no retries)
- **Cost:** Acceptable (~1 token per Claude Code check every 20 minutes)
- **Strategy:** Active polling only (no passive header capture from real requests)
- **UI Integration:** None (just capture and store)
- **Alerting:** None (just capture and store)

---

## 1. Database Schema

### New Table: `provider_quota_status`

**File:** `internal/ent/schema/provider_quota_status.go`

```go
package schema

import (
    "entgo.io/ent"
    "entgo.io/ent/schema/edge"
    "entgo.io/ent/schema/field"
    "entgo.io/ent/schema/index"
)

type ProviderQuotaStatus struct {
    ent.Schema
}

func (ProviderQuotaStatus) Mixin() []ent.Mixin {
    return []ent.Mixin{
        TimeMixin{},  // Provides created_at, updated_at
    }
}

func (ProviderQuotaStatus) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("channel_id").Unique(),
        index.Fields("next_check_at"),
    }
}

func (ProviderQuotaStatus) Fields() []ent.Field {
    return []ent.Field{
        field.Int("channel_id").Immutable(),
        field.Enum("provider_type").
            Values("claudecode", "codex").
            Immutable(),
        field.String("status").
            Comment("Overall status: ok, throttled, overage, error"),
        field.JSON("quota_data", map[string]interface{}{}).
            Comment("Provider-specific quota data"),
        field.Time("next_check_at").
            Comment("Timestamp for next scheduled quota check"),
    }
}

func (ProviderQuotaStatus) Edges() []ent.Edge {
    return []ent.Edge{
        edge.From("channel", Channel.Type).
            Ref("provider_quota_status").
            Field("channel_id").
            Required().
            Immutable().
            Unique(),
    }
}
```

**Add edge to Channel schema:**
```go
// In internal/ent/schema/channel.go, add to Edges():
edge.To("provider_quota_status", ProviderQuotaStatus.Type).
    Unique().
    Annotations(
        entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
    ),
```

### Quota Data Structures

**Claude Code (`quota_data`):**
```json
{
  "unified_status": "ok",
  "windows": {
    "5h": {
      "status": "ok",
      "reset": 1737654000,
      "utilization": 0.45
    },
    "7d": {
      "status": "ok",
      "reset": 1738258800,
      "utilization": 0.23
    },
    "overage": {
      "status": "ok",
      "reset": 0,
      "utilization": 0.0
    }
  },
  "representative_claim": "5h",
  "fallback": "overage",
  "fallback_percentage": 10.0,
  "reset": 1737654000
}
```

**Codex (`quota_data`):**
```json
{
  "plan_type": "plus",
  "rate_limit": {
    "allowed": true,
    "limit_reached": false,
    "primary_window": {
      "used_percent": 35.5,
      "reset_at": 1737654000,
      "reset_after_seconds": 3600,
      "limit_window_seconds": 86400
    },
    "secondary_window": {
      "used_percent": 12.0,
      "reset_at": 1737740400,
      "reset_after_seconds": 90000,
      "limit_window_seconds": 604800
    }
  },
  "code_review_rate_limit": {
    "allowed": true,
    "limit_reached": false,
    "primary_window": {
      "used_percent": 5.0,
      "reset_at": 1737654000,
      "reset_after_seconds": 3600,
      "limit_window_seconds": 86400
    }
  }
}
```

---

## 2. Quota Infrastructure Package

### Directory Structure
```
internal/server/biz/provider_quota/
├── types.go                    # Interfaces and common types
├── claudecode_parser.go        # Claude Code quota parser
├── claudecode_checker.go       # Claude Code quota checker
├── codex_parser.go             # Codex quota parser
├── codex_checker.go            # Codex quota checker
└── utils.go                    # Shared utilities
```

### Core Interfaces

**File:** `internal/server/biz/provider_quota/types.go`

```go
package provider_quota

import (
    "context"
    "net/http"
    
    "github.com/looplj/axonhub/internal/ent"
)

// QuotaParser extracts quota data from provider responses
type QuotaParser interface {
    // ParseResponse extracts quota data from HTTP response
    ParseResponse(headers http.Header, body []byte) (QuotaData, error)
    
    // GetProviderType returns the provider type this parser handles
    GetProviderType() string
}

// QuotaChecker makes API requests to check quota status
type QuotaChecker interface {
    // CheckQuota makes a minimal API request to get quota information
    CheckQuota(ctx context.Context, channel *ent.Channel) (http.Header, []byte, error)
    
    // SupportsChannel returns true if this checker supports the channel
    SupportsChannel(channel *ent.Channel) bool
}

// QuotaData is the unified quota data structure
type QuotaData struct {
    Status       string                 `json:"status"`
    ProviderType string                 `json:"provider_type"`
    RawData      map[string]interface{} `json:"raw_data"`
}
```

---

## 3. Claude Code Implementation

### Parser

**File:** `internal/server/biz/provider_quota/claudecode_parser.go`

```go
package provider_quota

import (
    "net/http"
    "strconv"
)

type ClaudeCodeQuotaParser struct{}

func (p *ClaudeCodeQuotaParser) ParseResponse(headers http.Header, body []byte) (QuotaData, error) {
    // Guard clause - early return if no quota headers
    if headers.Get("anthropic-ratelimit-unified-status") == "" {
        return QuotaData{}, nil
    }
    
    status := headers.Get("anthropic-ratelimit-unified-status")
    
    rawData := map[string]interface{}{
        "unified_status": status,
        "windows": map[string]interface{}{
            "5h": map[string]interface{}{
                "status":      headers.Get("anthropic-ratelimit-unified-5h-status"),
                "reset":       parseUnixTimestamp(headers.Get("anthropic-ratelimit-unified-5h-reset")),
                "utilization": parseFloat(headers.Get("anthropic-ratelimit-unified-5h-utilization")),
            },
            "7d": map[string]interface{}{
                "status":      headers.Get("anthropic-ratelimit-unified-7d-status"),
                "reset":       parseUnixTimestamp(headers.Get("anthropic-ratelimit-unified-7d-reset")),
                "utilization": parseFloat(headers.Get("anthropic-ratelimit-unified-7d-utilization")),
            },
            "overage": map[string]interface{}{
                "status":      headers.Get("anthropic-ratelimit-unified-overage-status"),
                "reset":       parseUnixTimestamp(headers.Get("anthropic-ratelimit-unified-overage-reset")),
                "utilization": parseFloat(headers.Get("anthropic-ratelimit-unified-overage-utilization")),
            },
        },
        "representative_claim":  headers.Get("anthropic-ratelimit-unified-representative-claim"),
        "fallback":              headers.Get("anthropic-ratelimit-unified-fallback"),
        "fallback_percentage":   parseFloat(headers.Get("anthropic-ratelimit-unified-fallback-percentage")),
        "reset":                 parseUnixTimestamp(headers.Get("anthropic-ratelimit-unified-reset")),
    }
    
    return QuotaData{
        Status:       status,
        ProviderType: "claudecode",
        RawData:      rawData,
    }, nil
}

func (p *ClaudeCodeQuotaParser) GetProviderType() string {
    return "claudecode"
}

// Defensive parsing - silently default to zero on error
func parseUnixTimestamp(s string) int64 {
    v, _ := strconv.ParseInt(s, 10, 64)
    return v
}

func parseFloat(s string) float64 {
    v, _ := strconv.ParseFloat(s, 64)
    return v
}
```

### Checker

**File:** `internal/server/biz/provider_quota/claudecode_checker.go`

```go
package provider_quota

import (
    "context"
    "fmt"
    "io"
    "net/http"
    
    "github.com/looplj/axonhub/internal/ent"
    "github.com/looplj/axonhub/internal/ent/channel"
    "github.com/looplj/axonhub/llm"
)

type ClaudeCodeQuotaChecker struct {
    channelService ChannelLLMService
}

// ChannelLLMService is the interface for creating LLM clients
type ChannelLLMService interface {
    CreateLLM(ctx context.Context, channel *ent.Channel) (llm.LLM, error)
}

func NewClaudeCodeQuotaChecker(channelService ChannelLLMService) *ClaudeCodeQuotaChecker {
    return &ClaudeCodeQuotaChecker{
        channelService: channelService,
    }
}

func (c *ClaudeCodeQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (http.Header, []byte, error) {
    // Create minimal request using cheapest model
    request := &llm.Request{
        Model: "claude-haiku-4-5",  // Cheapest model
        Messages: []llm.Message{
            {Role: "user", Content: llm.NewTextContent("limit")},
        },
        MaxTokens:   ptr(1),  // Minimal output
        Temperature: ptr(0.0),
    }
    
    // Create LLM client for this channel
    client, err := c.channelService.CreateLLM(ctx, ch)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to create LLM client: %w", err)
    }
    
    // Make request
    response, err := client.CreateChatCompletion(ctx, request)
    if err != nil {
        return nil, nil, fmt.Errorf("quota check request failed: %w", err)
    }
    
    // Extract headers - need to capture from actual HTTP response
    // The LLM response should expose headers
    var headers http.Header
    var body []byte
    
    // If response has RawResponse or Headers field, use it
    // Otherwise return empty (will need to be implemented based on actual LLM interface)
    if httpResp, ok := response.(interface{ GetHTTPResponse() *http.Response }); ok {
        resp := httpResp.GetHTTPResponse()
        headers = resp.Header
        if resp.Body != nil {
            body, _ = io.ReadAll(resp.Body)
        }
    }
    
    return headers, body, nil
}

func (c *ClaudeCodeQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
    return ch.Type == channel.TypeClaudecode
}

func ptr[T any](v T) *T {
    return &v
}
```

---

## 4. Codex Implementation

### Parser

**File:** `internal/server/biz/provider_quota/codex_parser.go`

```go
package provider_quota

import (
    "encoding/json"
    "fmt"
    "net/http"
)

type CodexQuotaParser struct{}

// CodexUsageResponse matches ChatGPT backend API response
type CodexUsageResponse struct {
    PlanType             string               `json:"plan_type,omitempty"`
    RateLimit            *CodeRateLimitInfo   `json:"rate_limit,omitempty"`
    CodeReviewRateLimit  *CodeRateLimitInfo   `json:"code_review_rate_limit,omitempty"`
}

type CodeRateLimitInfo struct {
    Allowed          *bool            `json:"allowed,omitempty"`
    LimitReached     *bool            `json:"limit_reached,omitempty"`
    PrimaryWindow    *CodeUsageWindow `json:"primary_window,omitempty"`
    SecondaryWindow  *CodeUsageWindow `json:"secondary_window,omitempty"`
}

type CodeUsageWindow struct {
    UsedPercent        *float64 `json:"used_percent,omitempty"`
    ResetAt            *int64   `json:"reset_at,omitempty"`
    ResetAfterSeconds  *int     `json:"reset_after_seconds,omitempty"`
    LimitWindowSeconds *int     `json:"limit_window_seconds,omitempty"`
}

func (p *CodexQuotaParser) ParseResponse(headers http.Header, body []byte) (QuotaData, error) {
    var response CodexUsageResponse
    
    if err := json.Unmarshal(body, &response); err != nil {
        return QuotaData{}, fmt.Errorf("failed to parse codex usage response: %w", err)
    }
    
    // Determine overall status
    status := "ok"
    if response.RateLimit != nil && response.RateLimit.LimitReached != nil && *response.RateLimit.LimitReached {
        status = "limit_reached"
    } else if response.RateLimit != nil && response.RateLimit.Allowed != nil && !*response.RateLimit.Allowed {
        status = "not_allowed"
    }
    
    // Convert to raw data map
    rawData := map[string]interface{}{
        "plan_type": response.PlanType,
    }
    
    if response.RateLimit != nil {
        rawData["rate_limit"] = convertRateLimitToMap(response.RateLimit)
    }
    
    if response.CodeReviewRateLimit != nil {
        rawData["code_review_rate_limit"] = convertRateLimitToMap(response.CodeReviewRateLimit)
    }
    
    return QuotaData{
        Status:       status,
        ProviderType: "codex",
        RawData:      rawData,
    }, nil
}

func (p *CodexQuotaParser) GetProviderType() string {
    return "codex"
}

func convertRateLimitToMap(rateLimit *CodeRateLimitInfo) map[string]interface{} {
    result := make(map[string]interface{})
    
    if rateLimit.Allowed != nil {
        result["allowed"] = *rateLimit.Allowed
    }
    if rateLimit.LimitReached != nil {
        result["limit_reached"] = *rateLimit.LimitReached
    }
    if rateLimit.PrimaryWindow != nil {
        result["primary_window"] = convertWindowToMap(rateLimit.PrimaryWindow)
    }
    if rateLimit.SecondaryWindow != nil {
        result["secondary_window"] = convertWindowToMap(rateLimit.SecondaryWindow)
    }
    
    return result
}

func convertWindowToMap(window *CodeUsageWindow) map[string]interface{} {
    result := make(map[string]interface{})
    
    if window.UsedPercent != nil {
        result["used_percent"] = *window.UsedPercent
    }
    if window.ResetAt != nil {
        result["reset_at"] = *window.ResetAt
    }
    if window.ResetAfterSeconds != nil {
        result["reset_after_seconds"] = *window.ResetAfterSeconds
    }
    if window.LimitWindowSeconds != nil {
        result["limit_window_seconds"] = *window.LimitWindowSeconds
    }
    
    return result
}
```

### Checker

**File:** `internal/server/biz/provider_quota/codex_checker.go`

```go
package provider_quota

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    
    "github.com/looplj/axonhub/internal/ent"
    "github.com/looplj/axonhub/internal/ent/channel"
)

type CodexQuotaChecker struct {
    httpClient *http.Client
}

func NewCodexQuotaChecker() *CodexQuotaChecker {
    return &CodexQuotaChecker{
        httpClient: &http.Client{},
    }
}

func (c *CodexQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (http.Header, []byte, error) {
    // Extract OAuth credentials
    if ch.Credentials == nil || ch.Credentials.OAuth == nil {
        return nil, nil, fmt.Errorf("codex channel missing OAuth credentials")
    }
    
    oauth := ch.Credentials.OAuth
    
    // Extract chatgpt_account_id from id_token JWT
    accountID, err := extractAccountIDFromToken(oauth.IDToken)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to extract account ID: %w", err)
    }
    
    // Build request
    req, err := http.NewRequestWithContext(ctx, "GET", "https://chatgpt.com/backend-api/wham/usage", nil)
    if err != nil {
        return nil, nil, err
    }
    
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oauth.AccessToken))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("User-Agent", "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal")
    req.Header.Set("Chatgpt-Account-Id", accountID)
    
    // Execute request
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, nil, fmt.Errorf("quota request failed: %w", err)
    }
    defer resp.Body.Close()
    
    // Read body
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to read response body: %w", err)
    }
    
    if resp.StatusCode != http.StatusOK {
        return nil, nil, fmt.Errorf("quota request failed with status %d: %s", resp.StatusCode, string(body))
    }
    
    return resp.Header, body, nil
}

func (c *CodexQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
    return ch.Type == channel.TypeCodex
}

// extractAccountIDFromToken parses JWT id_token and extracts chatgpt_account_id claim
func extractAccountIDFromToken(idToken string) (string, error) {
    // JWT format: header.payload.signature
    parts := strings.Split(idToken, ".")
    if len(parts) != 3 {
        return "", fmt.Errorf("invalid JWT format")
    }
    
    // Decode payload (base64url)
    payload, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        return "", fmt.Errorf("failed to decode JWT payload: %w", err)
    }
    
    // Parse JSON
    var claims map[string]interface{}
    if err := json.Unmarshal(payload, &claims); err != nil {
        return "", fmt.Errorf("failed to parse JWT claims: %w", err)
    }
    
    // Extract chatgpt_account_id
    // Expected path: claims["https://api.openai.com/auth"]["account_id"]
    authClaim, ok := claims["https://api.openai.com/auth"].(map[string]interface{})
    if !ok {
        return "", fmt.Errorf("auth claim not found in token")
    }
    
    accountID, ok := authClaim["account_id"].(string)
    if !ok {
        return "", fmt.Errorf("account_id not found in auth claim")
    }
    
    return accountID, nil
}
```

---

## 5. Provider Quota Service

**File:** `internal/server/biz/provider_quota_service.go`

```go
package biz

import (
    "context"
    "sync"
    "time"
    
    "entgo.io/ent/dialect/sql"
    "github.com/zhenzou/executors"
    "go.uber.org/fx"
    
    "github.com/looplj/axonhub/internal/ent"
    "github.com/looplj/axonhub/internal/ent/channel"
    "github.com/looplj/axonhub/internal/ent/privacy"
    "github.com/looplj/axonhub/internal/ent/providerquotastatus"
    "github.com/looplj/axonhub/internal/log"
    "github.com/looplj/axonhub/internal/server/biz/provider_quota"
)

type ProviderQuotaServiceParams struct {
    fx.In
    
    Ent            *ent.Client
    SystemService  *SystemService
    ChannelService *ChannelService
}

type ProviderQuotaService struct {
    *AbstractService
    
    SystemService  *SystemService
    ChannelService *ChannelService
    Executor       executors.ScheduledExecutor
    
    // Registry
    parsers  map[string]provider_quota.QuotaParser
    checkers map[string]provider_quota.QuotaChecker
    
    mu sync.Mutex
}

func NewProviderQuotaService(params ProviderQuotaServiceParams) *ProviderQuotaService {
    svc := &ProviderQuotaService{
        AbstractService: &AbstractService{db: params.Ent},
        SystemService:   params.SystemService,
        ChannelService:  params.ChannelService,
        Executor:        executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1)),
        parsers:         make(map[string]provider_quota.QuotaParser),
        checkers:        make(map[string]provider_quota.QuotaChecker),
    }
    
    // Register providers
    svc.registerClaudeCodeSupport()
    svc.registerCodexSupport()
    
    // Start polling
    if err := svc.Start(context.Background()); err != nil {
        panic(err)
    }
    
    return svc
}

func (svc *ProviderQuotaService) registerClaudeCodeSupport() {
    svc.parsers["claudecode"] = &provider_quota.ClaudeCodeQuotaParser{}
    svc.checkers["claudecode"] = provider_quota.NewClaudeCodeQuotaChecker(svc.ChannelService)
}

func (svc *ProviderQuotaService) registerCodexSupport() {
    svc.parsers["codex"] = &provider_quota.CodexQuotaParser{}
    svc.checkers["codex"] = provider_quota.NewCodexQuotaChecker()
}

func (svc *ProviderQuotaService) Start(ctx context.Context) error {
    // Run every minute
    _, err := svc.Executor.ScheduleFuncAtCronRate(
        svc.runQuotaCheck,
        executors.CRONRule{Expr: "* * * * *"},
    )
    return err
}

func (svc *ProviderQuotaService) Stop(ctx context.Context) error {
    return svc.Executor.Shutdown(ctx)
}

func (svc *ProviderQuotaService) runQuotaCheck(ctx context.Context) {
    ctx = ent.NewContext(ctx, svc.db)
    ctx = privacy.DecisionContext(ctx, privacy.Allow)
    
    now := time.Now()
    
    // Find channels needing quota check:
    // 1. Enabled channels with supported types
    // 2. No quota status OR next_check_at <= now
    channelsToCheck, err := svc.db.Channel.Query().
        Where(
            channel.StatusEQ(channel.StatusEnabled),
            channel.TypeIn(channel.TypeClaudecode, channel.TypeCodex),
            channel.Or(
                channel.Not(channel.HasProviderQuotaStatus()),
                channel.HasProviderQuotaStatusWith(
                    providerquotastatus.NextCheckAtLTE(now),
                ),
            ),
        ).
        All(ctx)
    
    if err != nil {
        log.Error(ctx, "Failed to query channels for quota check", log.Cause(err))
        return
    }
    
    if len(channelsToCheck) == 0 {
        return
    }
    
    log.Debug(ctx, "Running quota check", log.Int("channels", len(channelsToCheck)))
    
    for _, ch := range channelsToCheck {
        svc.checkChannelQuota(ctx, ch, now)
    }
}

func (svc *ProviderQuotaService) checkChannelQuota(ctx context.Context, ch *ent.Channel, now time.Time) {
    providerType := svc.getProviderType(ch)
    if providerType == "" {
        return
    }
    
    checker, ok := svc.checkers[providerType]
    if !ok {
        log.Error(ctx, "No checker for provider", 
            log.String("provider", providerType),
            log.Int("channel_id", ch.ID))
        return
    }
    
    parser, ok := svc.parsers[providerType]
    if !ok {
        log.Error(ctx, "No parser for provider",
            log.String("provider", providerType),
            log.Int("channel_id", ch.ID))
        return
    }
    
    // Make quota check request
    headers, body, err := checker.CheckQuota(ctx, ch)
    if err != nil {
        log.Error(ctx, "Quota check failed",
            log.Int("channel_id", ch.ID),
            log.String("provider", providerType),
            log.Cause(err))
        
        // Store error status
        svc.saveQuotaStatus(ctx, ch.ID, providerType, "error", nil, now)
        return
    }
    
    // Parse quota data
    quotaData, err := parser.ParseResponse(headers, body)
    if err != nil {
        log.Error(ctx, "Failed to parse quota response",
            log.Int("channel_id", ch.ID),
            log.String("provider", providerType),
            log.Cause(err))
        
        svc.saveQuotaStatus(ctx, ch.ID, providerType, "parse_error", nil, now)
        return
    }
    
    // Save quota status
    svc.saveQuotaStatus(ctx, ch.ID, providerType, quotaData.Status, quotaData.RawData, now)
    
    log.Debug(ctx, "Updated quota status",
        log.Int("channel_id", ch.ID),
        log.String("provider", providerType),
        log.String("status", quotaData.Status))
}

func (svc *ProviderQuotaService) saveQuotaStatus(
    ctx context.Context,
    channelID int,
    providerType string,
    status string,
    quotaData map[string]interface{},
    now time.Time,
) {
    nextCheck := now.Add(20 * time.Minute)
    
    err := svc.db.ProviderQuotaStatus.Create().
        SetChannelID(channelID).
        SetProviderType(providerType).
        SetStatus(status).
        SetQuotaData(quotaData).
        SetNextCheckAt(nextCheck).
        OnConflict(
            sql.ConflictColumns("channel_id"),
        ).
        UpdateNewValues().
        Exec(ctx)
    
    if err != nil {
        log.Error(ctx, "Failed to save quota status",
            log.Int("channel_id", channelID),
            log.Cause(err))
    }
}

func (svc *ProviderQuotaService) getProviderType(ch *ent.Channel) string {
    switch ch.Type {
    case channel.TypeClaudecode:
        return "claudecode"
    case channel.TypeCodex:
        return "codex"
    default:
        return ""
    }
}
```

---

## 6. Integration & Wiring

### Dependency Injection

**File:** `cmd/axonhub/main.go` or appropriate module file

Add to fx.Provide:
```go
biz.NewProviderQuotaService,
```

### Lifecycle Hook (Optional)

If you need graceful shutdown:
```go
fx.Invoke(func(lc fx.Lifecycle, svc *biz.ProviderQuotaService) {
    lc.Append(fx.Hook{
        OnStop: func(ctx context.Context) error {
            return svc.Stop(ctx)
        },
    })
})
```

---

## 7. Testing Strategy

### Unit Tests

**File:** `internal/server/biz/provider_quota/claudecode_parser_test.go`
- Test header parsing with various quota states
- Test missing headers (guard clause)
- Test invalid values (defensive parsing)

**File:** `internal/server/biz/provider_quota/codex_parser_test.go`
- Test JSON response parsing
- Test all window variations
- Test account ID extraction from JWT

### Integration Tests

**File:** `internal/server/biz/provider_quota_service_test.go`
- Mock channel creation
- Mock API responses
- Verify DB updates
- Test 20-minute interval logic
- Test error handling (log and wait)

---

## 8. Implementation Checklist

### Phase 1: Schema & Code Generation
- [ ] Create `internal/ent/schema/provider_quota_status.go`
- [ ] Add edge to `channel.go`
- [ ] Run `make generate` to generate Ent models
- [ ] Verify generated files compile

### Phase 2: Infrastructure Package
- [ ] Create `internal/server/biz/provider_quota/` directory
- [ ] Implement `types.go` with interfaces
- [ ] Implement `claudecode_parser.go`
- [ ] Implement `claudecode_checker.go`
- [ ] Implement `codex_parser.go`
- [ ] Implement `codex_checker.go`

### Phase 3: Service Implementation
- [ ] Implement `provider_quota_service.go`
- [ ] Wire up service in fx dependency injection
- [ ] Add lifecycle hooks if needed

### Phase 4: Testing
- [ ] Write unit tests for parsers
- [ ] Write unit tests for checkers
- [ ] Write integration tests for service
- [ ] Manual testing with real channels

### Phase 5: Deployment
- [ ] Database migration (auto-handled by Ent)
- [ ] Deploy and monitor logs
- [ ] Verify quota checks running every 20 minutes
- [ ] Verify data stored correctly in DB

---

## 9. Future Extensibility

To add a new provider (e.g., "gemini"):

1. **Create parser:** `gemini_parser.go` implementing `QuotaParser`
2. **Create checker:** `gemini_checker.go` implementing `QuotaChecker`
3. **Register in service:**
   ```go
   func (svc *ProviderQuotaService) registerGeminiSupport() {
       svc.parsers["gemini"] = &provider_quota.GeminiQuotaParser{}
       svc.checkers["gemini"] = provider_quota.NewGeminiQuotaChecker()
   }
   ```
4. **Update provider type mapping:**
   ```go
   case channel.TypeGemini:
       return "gemini"
   ```
5. **Add to schema enum:** Update `provider_type` enum in `provider_quota_status.go`

**No database migration needed** - the JSON `quota_data` field handles any structure!

---

## Technical Implementation Notes

### Claude Code Quota Check Technique

Based on your reference implementation, the Claude Code checker uses:

- **Cost minimization:** Uses `claude-haiku-4-5` (cheapest model)
- **Minimal request:** `max_tokens: 1`, single message with content "limit"
- **Header extraction:** All `anthropic-ratelimit-unified-*` headers are captured
- **Defensive parsing:** Invalid/missing values silently default to zero

### Codex Quota Check Technique

Based on your reference implementation:

- **Endpoint:** `https://chatgpt.com/backend-api/wham/usage`
- **Method:** GET request
- **Authentication:** Bearer token from OAuth credentials
- **Special header:** `Chatgpt-Account-Id` extracted from JWT `id_token` payload
- **JWT parsing:** Extracts `claims["https://api.openai.com/auth"]["account_id"]`
- **Response:** JSON structure with plan type and rate limit windows

### Error Handling Strategy

Following your requirements:
- **Log errors** to standard logging system
- **Wait for next cycle** (20 minutes)
- **No retry logic** or exponential backoff
- **Store error status** in database for visibility

### Polling Logic

- **Cron schedule:** Every 1 minute (checks if channels need polling)
- **Next check calculation:** 20 minutes from last successful check
- **Query optimization:** Only fetches channels where `next_check_at <= now`
- **Channel filtering:** Only `enabled` channels with types `claudecode` or `codex`

---

## Open Questions & Notes

1. **Codex JWT structure:** Assumes the account ID path in JWT is stable. If OpenAI changes this, the parser will need updating.

2. **Claude Code LLM interface:** The checker assumes the `CreateLLM` method returns an object that can make HTTP requests and expose response headers. This may need adjustment based on actual implementation.

3. **OAuth token refresh:** For Codex, we assume OAuth tokens are managed elsewhere and kept fresh. If tokens expire, quota checks will fail and be logged.

4. **Channel credentials:** Both checkers assume credentials are properly stored and accessible through the channel entity.

5. **Database migrations:** Ent will auto-generate migration on next run. No manual SQL needed.
