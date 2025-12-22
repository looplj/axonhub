# AxonHub Model Integration Design

## Overview

This document describes the design for integrating AxonHub Model management into the orchestrator request pipeline.

## Current Architecture

### Request Flow (Before AxonHub Model)

```
Client Request (model="gpt-4")
       │
       ▼
┌──────────────────────────────────────┐
│ 1. Inbound Transformer               │
│    TransformRequest → llm.Request    │
└──────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────┐
│ 2. API Key Profile Model Mapping     │
│    ModelMapper.MapModel()            │
│    (APIKeyProfile.ModelMappings)     │
└──────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────┐
│ 3. Channel Selection                 │
│    ChannelSelector.Select()          │
│    - ChooseChannels (model support)  │
│    - Profile ChannelIDs/Tags filter  │
│    - LoadBalancer sorting            │
└──────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────┐
│ 4. Outbound Transformer              │
│    - channel.ChooseModel()           │
│    - TransformRequest → raw request  │
└──────────────────────────────────────┘
       │
       ▼
     Provider
```

### Key Data Models

| Entity | Key Fields | Purpose |
|--------|-----------|---------|
| `ent.Model` | `developer`, `model_id`, `settings.Associations` | AxonHub Model definition with channel associations |
| `ent.Channel` | `SupportedModels`, `Settings.ModelMappings` | Channel supported models and mappings |
| `ent.APIKey` | `Profiles.ModelMappings`, `ChannelIDs`, `ChannelTags` | API Key level model mapping and channel filtering |

### Current Model Resolution in Channel

The `Channel` struct currently resolves models through multiple mechanisms:

1. **Direct Match**: `SupportedModels` contains the model directly
2. **Prefix Resolution**: `ExtraModelPrefix` allows `prefix/model` → `model`
3. **Auto-Trim Resolution**: `AutoTrimedModelPrefixes` allows `model` → `prefix/model`
4. **Model Mapping**: `ModelMappings` allows `from` → `to`

## Proposed Design

### Goal

Introduce AxonHub Model as a unified abstraction layer that:

1. Provides a unified "virtual model" that maps to multiple channel+model combinations
2. Integrates between API Key mapping and Channel selection
3. Leverages a unified model list from channels for association matching

### New Request Flow

```
Client Request (model="axonhub-gpt4" or "gpt-4")
       │
       ▼
┌──────────────────────────────────────┐
│ 1. Inbound Transformer               │
│    TransformRequest → llm.Request    │
└──────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────┐
│ 2. API Key Profile Model Mapping     │
│    ModelMapper.MapModel()            │
│    (APIKeyProfile.ModelMappings)     │
└──────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────┐
│ 3. AxonHub Model Resolution (NEW)                    │
│    ModelResolver.Resolve(model) →                    │
│      Option A: AxonHub Model found                   │
│        → Generate ChannelModelCandidates from        │
│          Model.Settings.Associations                 │
│      Option B: No AxonHub Model found                │
│        → Fall through to legacy channel selection    │
└──────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────┐
│ 4. Channel Selection                 │
│    - Use ChannelModelCandidates if   │
│      provided by AxonHub Model       │
│    - Otherwise use legacy selection  │
│    - Apply Profile filters           │
│    - LoadBalancer sorting            │
└──────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────┐
│ 5. Outbound Transformer              │
│    - Use pre-resolved model from     │
│      ChannelModelCandidate           │
│    - TransformRequest → raw request  │
└──────────────────────────────────────┘
       │
       ▼
     Provider
```

### Key Components

#### 1. Unified Model List for Channel

Add a new method to `Channel` that provides a unified view of all models it can handle:

```go
// UnifiedModelEntry represents a model that the channel can handle
type UnifiedModelEntry struct {
    // RequestModel is the model name that can be used in requests
    RequestModel string
    // ActualModel is the model that will be sent to the provider
    ActualModel string
    // Source indicates how this model is supported
    Source string // "direct", "prefix", "auto_trim", "mapping"
}

// GetUnifiedModels returns all models this channel can handle
func (c *Channel) GetUnifiedModels() []UnifiedModelEntry
```

This unifies:
- `SupportedModels` (direct models)
- `ExtraModelPrefix` (prefixed models)
- `AutoTrimedModelPrefixes` (auto-trimmed models)
- `ModelMappings` (mapped models)

#### 2. ChannelModelCandidate

A new structure representing a pre-resolved channel+model combination:

```go
// ChannelModelCandidate represents a resolved channel and model pair
type ChannelModelCandidate struct {
    Channel     *Channel
    // RequestModel is the original model from the AxonHub Model association
    RequestModel string
    // ActualModel is the model to send to the provider (after channel mapping)
    ActualModel string
    // Priority from the association order (lower = higher priority)
    Priority int
}
```

#### 3. ModelResolver

A new component that resolves AxonHub Models to channel+model candidates:

```go
type ModelResolver struct {
    ModelService   *biz.ModelService
    ChannelService *biz.ChannelService
}

// Resolve attempts to resolve a model name to AxonHub Model associations
// Returns nil if no AxonHub Model is found (fallback to legacy behavior)
func (r *ModelResolver) Resolve(ctx context.Context, modelName string) ([]*ChannelModelCandidate, error)
```

#### 4. Integration with ChannelSelector

Modify the channel selection to accept pre-resolved candidates:

```go
// AxonHubModelSelector is a ChannelSelector that uses pre-resolved candidates
type AxonHubModelSelector struct {
    candidates []*ChannelModelCandidate
    fallback   ChannelSelector
}

func (s *AxonHubModelSelector) Select(ctx context.Context, req *llm.Request) ([]*Channel, error) {
    if len(s.candidates) > 0 {
        // Use pre-resolved candidates, extract channels
        // Store model mapping info for later use
        return extractChannels(s.candidates), nil
    }
    // Fallback to legacy selection
    return s.fallback.Select(ctx, req)
}
```

### Model Association Types

The `ModelAssociation` in `ent.Model.Settings.Associations` supports:

| Type | Description | Matching Logic |
|------|-------------|----------------|
| `channel_model` | Specific model in specific channel | `channelId` + `modelId` exact match |
| `channel_regex` | Pattern match in specific channel | `channelId` + regex on channel's unified models |
| `regex` | Pattern match across all channels | Regex on all channels' unified models |
| `model` | Specific model across all channels | `modelId` match on all channels' unified models |

### Association Resolution Algorithm

```
For each association in order:
    If type == "channel_model":
        Find channel by channelId
        If channel has modelId in UnifiedModels:
            Add ChannelModelCandidate(channel, modelId, resolved_actual)
    
    If type == "channel_regex":
        Find channel by channelId
        For each entry in channel.GetUnifiedModels():
            If pattern matches entry.RequestModel:
                Add ChannelModelCandidate(channel, entry.RequestModel, entry.ActualModel)
    
    If type == "regex":
        For each enabled channel:
            For each entry in channel.GetUnifiedModels():
                If pattern matches entry.RequestModel:
                    Add ChannelModelCandidate(channel, entry.RequestModel, entry.ActualModel)
    
    If type == "model":
        For each enabled channel:
            If modelId in channel.GetUnifiedModels():
                Add ChannelModelCandidate(channel, modelId, resolved_actual)
```

### State Flow

The `PersistenceState` needs to be extended to carry resolution information:

```go
type PersistenceState struct {
    // ... existing fields ...
    
    // AxonHubModel is the resolved AxonHub Model (if any)
    AxonHubModel *ent.Model
    
    // ChannelModelCandidates are pre-resolved candidates from AxonHub Model
    ChannelModelCandidates []*ChannelModelCandidate
    
    // CurrentCandidate is the currently selected candidate
    CurrentCandidate *ChannelModelCandidate
}
```

### Outbound Model Resolution

In `PersistentOutboundTransformer.TransformRequest`:

```go
func (p *PersistentOutboundTransformer) TransformRequest(ctx context.Context, llmRequest *llm.Request) (*httpclient.Request, error) {
    // If we have a pre-resolved candidate, use its ActualModel
    if p.state.CurrentCandidate != nil {
        llmRequest.Model = p.state.CurrentCandidate.ActualModel
    } else {
        // Legacy: use channel.ChooseModel()
        model, err := p.state.CurrentChannel.ChooseModel(llmRequest.Model)
        if err != nil {
            return nil, err
        }
        llmRequest.Model = model
    }
    
    // Continue with transformation...
}
```

## Implementation Plan

### Phase 1: Channel Unified Model List

1. Add `UnifiedModelEntry` struct
2. Implement `Channel.GetUnifiedModels()` method
3. Update `ModelService.QueryModelChannelConnections()` to use unified models

### Phase 2: Model Resolution

1. Add `ChannelModelCandidate` struct
2. Implement `ModelResolver` component
3. Add resolution middleware in inbound pipeline

### Phase 3: Channel Selection Integration

1. Create `AxonHubModelSelector`
2. Extend `PersistenceState` with candidate fields
3. Update `selectChannels` middleware

### Phase 4: Outbound Integration

1. Update `PersistentOutboundTransformer.TransformRequest`
2. Handle retry with candidates
3. Update `NextChannel` to cycle through candidates

## Backward Compatibility

- If no AxonHub Model matches the request model, the system falls back to legacy channel selection
- Existing API Key Profile mappings continue to work (applied before AxonHub Model resolution)
- Existing Channel model mappings are incorporated into the unified model list

## Design Decisions

### Decision 1: Unified Model List Caching

**Decision**: Cache at Channel load time

- `GetUnifiedModels()` is computed when channels are loaded
- Cache is invalidated when channel is updated
- Avoids per-request computation overhead

### Decision 2: LoadBalancer Granularity

**Decision**: Extend to Channel+Model granularity

The current LoadBalancer operates at Channel level, but with AxonHub Model introducing multiple candidates per channel, we need finer granularity.

#### Current Interface

```go
type LoadBalanceStrategy interface {
    Score(ctx context.Context, channel *biz.Channel) float64
    ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore)
    Name() string
}

func (lb *LoadBalancer) Sort(ctx context.Context, channels []*biz.Channel, model string) []*biz.Channel
```

#### New Interface

```go
// ChannelModelTarget represents a channel+model combination for load balancing
type ChannelModelTarget struct {
    Channel      *biz.Channel
    RequestModel string  // Model name used in request matching
    ActualModel  string  // Model name to send to provider
    Priority     int     // From association order (lower = higher priority)
}

type LoadBalanceStrategy interface {
    // Score calculates a score for a channel+model target
    Score(ctx context.Context, target *ChannelModelTarget) float64
    ScoreWithDebug(ctx context.Context, target *ChannelModelTarget) (float64, StrategyScore)
    Name() string
}

func (lb *LoadBalancer) Sort(ctx context.Context, targets []*ChannelModelTarget) []*ChannelModelTarget
```

#### Strategy Updates

Each strategy needs to be updated:

| Strategy | Current Behavior | New Behavior |
|----------|-----------------|--------------|
| `TraceAwareStrategy` | Match by channel ID | Match by channel ID + model |
| `ErrorAwareStrategy` | Channel error rate | Channel error rate (unchanged) |
| `WeightRoundRobinStrategy` | Channel weight | Channel weight (unchanged) |
| `ConnectionAwareStrategy` | Channel connections | Channel connections (unchanged) |
| `WeightStrategy` | Channel ordering weight | Channel ordering weight (unchanged) |

Most strategies remain channel-based. Only `TraceAwareStrategy` benefits from model-level tracking.

#### Backward Compatibility

For legacy flow (no AxonHub Model), wrap channels as targets:

```go
func ChannelsToTargets(channels []*biz.Channel, model string) []*ChannelModelTarget {
    targets := make([]*ChannelModelTarget, len(channels))
    for i, ch := range channels {
        actualModel, _ := ch.ChooseModel(model)
        targets[i] = &ChannelModelTarget{
            Channel:      ch,
            RequestModel: model,
            ActualModel:  actualModel,
            Priority:     0,
        }
    }
    return targets
}
```

### Decision 3: Priority vs LoadBalancer Score Interaction

**Decision**: Priority grouping (sort by priority first, then by score within group)

When AxonHub Model associations have explicit priority, the sorting works in two stages:

1. **Group by Priority**: Candidates are grouped by their priority value
2. **Sort within Group**: Within each priority group, LoadBalancer scores determine order

#### Priority Field in Association

Add explicit `priority` field to `ModelAssociation`:

```go
type ModelAssociation struct {
    Type         string                   `json:"type"`
    Priority     int                      `json:"priority,omitempty"` // NEW: default 0, lower = higher priority
    ChannelModel *ChannelModelAssociation `json:"channelModel"`
    ChannelRegex *ChannelRegexAssociation `json:"channelRegex"`
    Regex        *RegexAssociation        `json:"regex"`
    ModelID      *ModelIDAssociation      `json:"modelId"`
}
```

#### Sorting Algorithm

```go
func SortCandidates(candidates []*ChannelModelTarget, lb *LoadBalancer) []*ChannelModelTarget {
    // 1. Group by priority
    groups := make(map[int][]*ChannelModelTarget)
    for _, c := range candidates {
        groups[c.Priority] = append(groups[c.Priority], c)
    }
    
    // 2. Get sorted priority keys
    priorities := lo.Keys(groups)
    slices.Sort(priorities) // Lower priority value = higher priority
    
    // 3. Sort each group by LoadBalancer score, then concatenate
    result := make([]*ChannelModelTarget, 0, len(candidates))
    for _, p := range priorities {
        group := groups[p]
        sortedGroup := lb.Sort(ctx, group)
        result = append(result, sortedGroup...)
    }
    
    return result
}
```

#### Example

```yaml
# AxonHub Model associations
associations:
  - type: channel_model
    priority: 0          # Highest priority group
    channelModel:
      channelId: 1       # OpenAI
      modelId: gpt-4-turbo
  - type: channel_model
    priority: 0          # Same priority group
    channelModel:
      channelId: 2       # Azure
      modelId: gpt-4-turbo
  - type: regex
    priority: 1          # Fallback group
    regex:
      pattern: "gpt-4.*"
```

**Sorting result** (assuming LoadBalancer prefers Azure due to lower load):
1. Azure/gpt-4-turbo (priority=0, highest LB score)
2. OpenAI/gpt-4-turbo (priority=0, lower LB score)
3. Other gpt-4.* matches (priority=1)

### Decision 4: API Key Profile Filtering

**Decision**: Apply filtering via decorator pattern

AxonHub Model candidates are filtered by API Key Profile's `ChannelIDs` and `ChannelTags` using the existing decorator pattern from `ChannelSelector`.

#### Implementation

Reuse existing selector decorators for candidate filtering:

```go
// CandidateSelector interface (mirrors ChannelSelector pattern)
type CandidateSelector interface {
    Select(ctx context.Context, candidates []*ChannelModelTarget, req *llm.Request) ([]*ChannelModelTarget, error)
}

// DefaultCandidateSelector returns candidates as-is
type DefaultCandidateSelector struct{}

func (s *DefaultCandidateSelector) Select(ctx context.Context, candidates []*ChannelModelTarget, req *llm.Request) ([]*ChannelModelTarget, error) {
    return candidates, nil
}

// ChannelIDsCandidateSelector filters by allowed channel IDs
type ChannelIDsCandidateSelector struct {
    wrapped    CandidateSelector
    channelIDs []int
}

func (s *ChannelIDsCandidateSelector) Select(ctx context.Context, candidates []*ChannelModelTarget, req *llm.Request) ([]*ChannelModelTarget, error) {
    filtered, err := s.wrapped.Select(ctx, candidates, req)
    if err != nil {
        return nil, err
    }
    
    return lo.Filter(filtered, func(c *ChannelModelTarget, _ int) bool {
        return lo.Contains(s.channelIDs, c.Channel.ID)
    }), nil
}

// TagsCandidateSelector filters by channel tags
type TagsCandidateSelector struct {
    wrapped CandidateSelector
    tags    []string
}

func (s *TagsCandidateSelector) Select(ctx context.Context, candidates []*ChannelModelTarget, req *llm.Request) ([]*ChannelModelTarget, error) {
    filtered, err := s.wrapped.Select(ctx, candidates, req)
    if err != nil {
        return nil, err
    }
    
    return lo.Filter(filtered, func(c *ChannelModelTarget, _ int) bool {
        return lo.Some(c.Channel.Tags, func(tag string) bool {
            return lo.Contains(s.tags, tag)
        })
    }), nil
}
```

#### Usage in Pipeline

```go
func selectChannels(inbound *PersistentInboundTransformer) pipeline.Middleware {
    return pipeline.OnLlmRequest("select-channels", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
        // ... existing logic ...
        
        // If AxonHub Model resolution produced candidates
        if len(inbound.state.ChannelModelCandidates) > 0 {
            candidateSelector := &DefaultCandidateSelector{}
            
            if profile := GetActiveProfile(inbound.state.APIKey); profile != nil {
                if len(profile.ChannelIDs) > 0 {
                    candidateSelector = NewChannelIDsCandidateSelector(candidateSelector, profile.ChannelIDs)
                }
                if len(profile.ChannelTags) > 0 {
                    candidateSelector = NewTagsCandidateSelector(candidateSelector, profile.ChannelTags)
                }
            }
            
            filteredCandidates, err := candidateSelector.Select(ctx, inbound.state.ChannelModelCandidates, llmRequest)
            if err != nil {
                return nil, err
            }
            
            // Apply LoadBalancer with priority grouping
            sortedCandidates := SortCandidates(filteredCandidates, inbound.state.LoadBalancer)
            inbound.state.ChannelModelCandidates = sortedCandidates
            
            // Extract channels for compatibility
            inbound.state.Channels = ExtractChannels(sortedCandidates)
            
            return llmRequest, nil
        }
        
        // Fall through to legacy channel selection...
    })
}
```

## Summary of Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Unified Model Caching** | Cache at Channel load | Avoid per-request overhead |
| **LoadBalancer Granularity** | Channel+Model level | Support multiple models per channel |
| **Priority vs Score** | Priority grouping | Respect user intent while allowing LB optimization |
| **API Key Profile Filtering** | Apply via decorator | Consistent with existing architecture |
