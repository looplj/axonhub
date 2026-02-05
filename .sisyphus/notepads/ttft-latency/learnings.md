# Learnings

*Record patterns, conventions, and successful approaches discovered during the TTFT latency regression work.*

## Initial Setup
- TTFT (Time To First Token) measures the time from request initiation to first response token
- Latency column on /project/requests should display TTFT values
- Backend tracks request timing through middleware; frontend displays via React components

---

## TTFT Data Flow Investigation (2025-02-04)

### Backend Capture & Persistence

**Entry Point**: `internal/server/orchestrator/performance.go`
- `MarkFirstToken()`: Records first token timestamp when streaming response begins
- `PerformanceRecord`: Struct capturing timing data including `FirstTokenAt`

**Request Flow**:
- `internal/server/orchestrator/request.go` - Main request orchestration
- `internal/server/orchestrator/request_execution.go` - Execution tracking
- `internal/server/orchestrator/inbound.go` - Inbound request handling
- `internal/server/orchestrator/outbound.go` - Outbound response handling

**Business Logic**: `internal/server/biz/request.go`
- `LatencyMetrics`: Contains `FirstTokenLatencyMs` field
- `UpdateRequestCompleted()` / `UpdateRequestExecutionCompleted()`: Persist metrics to database

**Database Schema**:
- Field: `metrics_first_token_latency_ms` (bigint)
- Stored via Ent ORM in `Request` and `RequestExecution` entities

### Frontend API & Display

**GraphQL Query**: `frontend/src/features/requests/data/requests.ts`
- Selection includes `metricsFirstTokenLatencyMs` field
- Maps DB field to camelCase for TypeScript

**UI Component**: `frontend/src/features/requests/components/requests-columns.tsx`
- Latency column renders TTFT values
- **Display Gating**: Only shows when `request.stream && metricsFirstTokenLatencyMs`
- Non-streaming requests don't display TTFT (expected behavior)

### Key Field Names
- **API/TS**: `metricsFirstTokenLatencyMs`
- **Database**: `metrics_first_token_latency_ms`
- **Display Condition**: `request.stream && metricsFirstTokenLatencyMs`

### Data Flow Summary
1. `MarkFirstToken()` captures timestamp during streaming response
2. `LatencyMetrics` computed and passed to `UpdateRequestCompleted()`
3. Persisted to `metrics_first_token_latency_ms` via Ent
4. GraphQL query fetches field as `metricsFirstTokenLatencyMs`
5. Frontend column displays value only for streaming requests with non-null latency
---

## Git History Analysis (2025-02-04)

### Stream Flag Overwrite Root Cause

**Commit that introduced the bug**: `8afd95c3`  
**Author**: Loop <bababaa261@gmail.com>  
**Date**: Wed Feb 4 16:31:49 2026 +0800  
**Message**: `feat: trace stikcy api key for multiple api keys channel (#740)`

**What changed in `internal/server/orchestrator/performance.go`**:

**Before** (commit `58c0b439^`):
```go
if m.outbound.state.Perf == nil {
    m.outbound.state.Perf = &biz.PerformanceRecord{}
}
perf := m.outbound.state.Perf
perf.StartTime = time.Now()
// ... set other fields
```

**After** (commit `8afd95c3`):
```go
// Create a new PerformanceRecord instance for each request.
perf := biz.PerformanceRecord{}
perf.StartTime = time.Now()
// ... set other fields
m.outbound.state.Perf = &perf
```

**Why this breaks TTFT**:
- The `Stream` flag is set in `OnInboundLlmRequest()` earlier in the pipeline
- `OnOutboundRawRequest()` now creates a **brand new** `PerformanceRecord` on every call
- This overwrites the entire struct, including the `Stream` field that was previously set
- Without `Stream = true`, the frontend UI gating (`request.stream && metricsFirstTokenLatencyMs`) fails
- Result: TTFT values exist in the database but are not displayed

**Relation to tokens/s change**:  
Commit `919c81d7` (fix: use completion tokens for TPS calculation) modified only `internal/server/biz/channel_probe.go` and its test. It **did not touch** `performance.go`. The two changes are **unrelated** - the TTFT bug was introduced by the sticky API key refactor, not the TPS calculation fix.

**Evidence**:
```
$ git log --oneline -- internal/server/orchestrator/performance.go
8afd95c3 feat: trace stikcy api key for multiple api keys channel (#740)
58c0b439 refactor: cleanup channel performance (#726)
...

$ git show 919c81d7 --stat
 internal/server/biz/channel_probe.go      | 20 ++++++++++----------
 internal/server/biz/channel_probe_test.go |  4 +++-
 (no performance.go changes)
```

## Git History Analysis - Stream Flag Overwrite (2025-02-04)

### Bug Introduction

**Commit**: `8afd95c3`
**Author**: Loop <bababaa261@gmail.com>
**Date**: Wed Feb 4 16:31:49 2026 +0800
**Message**: feat: trace stikcy api key for multiple api keys channel (#740)

**Change in `internal/server/orchestrator/performance.go`**:

*Before* (reusing existing PerformanceRecord):
```go
if m.outbound.state.Perf == nil {
    m.outbound.state.Perf = &biz.PerformanceRecord{}
}
perf := m.outbound.state.Perf
perf.StartTime = time.Now()
// ... other field updates
```

*After* (creating new instance):
```go
// Create a new PerformanceRecord instance for each request.
perf := biz.PerformanceRecord{}
perf.StartTime = time.Now()
// ... other field updates
m.outbound.state.Perf = &perf
```

**Impact**: The new instance overwrites the `Stream` flag that was set in `OnInboundLlmRequest()`. This causes TTFT values to be hidden in the UI (gating condition: `request.stream && metricsFirstTokenLatencyMs`).

### Relation to Tokens/S Change

**Tokens/S commit**: `919c81d7` (fix: use completion tokens for TPS calculation)
- Modified only: `internal/server/biz/channel_probe.go` and `channel_probe_test.go`
- **No changes** to `performance.go`
- **Conclusion**: The two changes are **unrelated**. The TTFT bug was introduced by the sticky API key refactor, not the TPS calculation fix.

### Evidence

```bash
$ git show 919c81d7 --stat
 internal/server/biz/channel_probe.go      | 20 ++++++++++----------
 internal/server/biz/channel_probe_test.go |  4 +++-
 (no performance.go changes)
```

$ git log --oneline -- internal/server/orchestrator/performance.go
8afd95c3 feat: trace stikcy api key for multiple api keys channel (#740)
58c0b439 refactor: cleanup channel performance (#726)
...
