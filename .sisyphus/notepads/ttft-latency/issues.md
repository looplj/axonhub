# Issues

*Record problems, blockers, and gotchas encountered during the TTFT latency regression.*

## Initial State
- No issues identified yet; investigation phase pending

---

## ROOT CAUSE IDENTIFIED: TTFT Visibility Regression (2025-02-04)

### Problem Summary
TTFT (Time To First Token) no longer appears in the `/project/requests` latency column after token/s changes.

### Data Flow Analysis

**1. TTFT Capture (internal/server/orchestrator/performance.go:134-151)**
```go
func (s *recordPerformanceStream) Current() *llm.Response {
    event := s.stream.Current()
    if event == nil {
        return event
    }

    if !s.firstTokenSet && s.state.Perf != nil {
        s.state.Perf.MarkFirstToken()  // <-- First token timestamp recorded here
        s.firstTokenSet = true
    }
    // ...
}
```

**2. MarkFirstToken (internal/server/biz/channel_metrics.go:493-498)**
```go
func (m *PerformanceRecord) MarkFirstToken() {
    if m.FirstTokenTime == nil {
        now := time.Now()
        m.FirstTokenTime = &now
    }
}
```

**3. Persistence Gating (MULTIPLE LOCATIONS)**

All persistence points use the same gating logic:
- `inbound.go:135-137`: `if ts.perf.Stream && ts.perf.FirstTokenTime != nil`
- `outbound.go:152-154`: `if ts.perf.Stream && ts.perf.FirstTokenTime != nil`
- `request.go:98-100`: `if state.Perf.Stream && state.Perf.FirstTokenTime != nil`
- `request_execution.go:122-124`: `if state.Perf.Stream && state.Perf.FirstTokenTime != nil`

**4. UI Display Gating (frontend/src/features/requests/components/requests-columns.tsx:425-427)**
```typescript
if (request.stream && request.metricsFirstTokenLatencyMs != null) {
  latencyParts.push(`TTFT: ${formatDuration(request.metricsFirstTokenLatencyMs)}`);
}
```

### Root Cause: Discrepancy in Validation Logic

**CRITICAL FINDING**: `request_execution.go` has ADDITIONAL validation that other persistence paths LACK:

**request_execution.go:96-125** (HAS extra validation):
```go
if state.Perf != nil && !state.Perf.StartTime.IsZero() {  // <-- EXTRA CHECK
    var (
        firstTokenLatencyMs int64
        requestLatencyMs    int64
    )

    if state.Perf.RequestCompleted && !state.Perf.EndTime.IsZero() {
        firstTokenLatencyMs, requestLatencyMs, _ = state.Perf.Calculate()
    } else {
        requestLatencyMs = time.Since(state.Perf.StartTime).Milliseconds()
        if state.Perf.Stream && state.Perf.FirstTokenTime != nil {
            firstTokenLatencyMs = state.Perf.FirstTokenTime.Sub(state.Perf.StartTime).Milliseconds()
        }
    }

    // Negative value protection
    if requestLatencyMs < 0 {
        requestLatencyMs = 0
    }
    if firstTokenLatencyMs < 0 {
        firstTokenLatencyMs = 0
    }
    // ...
}
```

**inbound.go:127-138, outbound.go:143-155, request.go:89-101** (MISSING validation):
```go
if ts.perf != nil {  // <-- NO !ts.perf.StartTime.IsZero() check!
    firstTokenLatencyMs, requestLatencyMs, _ := ts.perf.Calculate()

    metrics = &biz.LatencyMetrics{
        LatencyMs: &requestLatencyMs,
    }
    if ts.perf.Stream && ts.perf.FirstTokenTime != nil {
        metrics.FirstTokenLatencyMs = &firstTokenLatencyMs
    }
}
```

### The Breaking Condition

TTFT will NOT be displayed when ANY of the following is true:

1. **Missing Capture**: `MarkFirstToken()` was never called (stream had no events, or events arrived before performance record was initialized)
2. **Stream Flag Mismatch**: `ts.perf.Stream` is false (request was non-streaming or flag was not propagated correctly)
3. **FirstTokenTime Not Set**: `ts.perf.FirstTokenTime` is nil (first token arrived but wasn't recorded)
4. **Race Condition**: Stream events processed before `OnOutboundRawRequest` initializes `Perf.StartTime`

### Most Likely Failing Condition

**The gating condition `ts.perf.Stream && ts.perf.FirstTokenTime != nil` fails when:**

1. **Stream events arrive before performance record is fully initialized** - The `recordPerformanceStream.Current()` method checks `!s.firstTokenSet && s.state.Perf != nil`, but if the stream starts processing before `OnOutboundRawRequest` initializes `Perf` with `StartTime`, the first token may be recorded incorrectly.

2. **Inconsistent Stream flag propagation** - The `Stream` flag is set in `OnInboundLlmRequest` (performance.go:41-45) but the performance record is created in `OnOutboundRawRequest` (performance.go:58-69). If the request is modified between these two points, the Stream flag may become inconsistent.

### Evidence Files and Line Ranges

| File | Line Range | Issue |
|------|------------|-------|
| `internal/server/orchestrator/performance.go` | 36-48 | Stream flag set BEFORE performance record initialized |
| `internal/server/orchestrator/performance.go` | 50-77 | Performance record created AFTER stream flag set |
| `internal/server/orchestrator/inbound.go` | 127-138 | Missing `StartTime.IsZero()` check |
| `internal/server/orchestrator/outbound.go` | 143-155 | Missing `StartTime.IsZero()` check |
| `internal/server/orchestrator/request.go` | 89-101 | Missing `StartTime.IsZero()` check |
| `internal/server/orchestrator/request_execution.go` | 96-125 | HAS proper validation (inconsistent!) |
| `frontend/src/features/requests/components/requests-columns.tsx` | 425-427 | UI gating depends on persisted value |

### Impact
- TTFT column shows `-` instead of actual values
- Channel probe calculations may be affected (uses `metrics_first_token_latency_ms`)
- Users cannot see first token latency metrics for streaming requests

### Status
**CONFIRMED** - Root cause identified as inconsistent validation logic between `request_execution.go` (which has extra checks) and other persistence paths (`inbound.go`, `outbound.go`, `request.go` which lack the checks).

