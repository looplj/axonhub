# Decisions

*Record architectural choices and their rationales during the TTFT latency work.*

## Initial State
- No decisions made yet; investigation phase pending

---
*Append new decisions below this line with date and context.*
## 2025-02-04: Preserve Stream Flag in OnOutboundRawRequest

**Problem**: 
The Stream flag set in `OnInboundLlmRequest` was being overwritten in `OnOutboundRawRequest` because a new `PerformanceRecord` was created without preserving the existing flag. This caused TTFT (Time To First Token) to not be persisted for streaming requests, as the UI relies on `request.stream && metricsFirstTokenLatencyMs`.

**Solution**:
Modified `OnOutboundRawRequest` in `internal/server/orchestrator/performance.go` to:
1. Capture the existing `Stream` flag from `m.outbound.state.Perf` before creating a new record
2. Apply the preserved flag to the new `PerformanceRecord`
3. Continue resetting timing fields (`StartTime`, `Success`, `RequestCompleted`) for the new request attempt

**Implementation**:
```go
// Preserve Stream flag from existing PerformanceRecord (set in OnInboundLlmRequest)
var streamFlag bool
if m.outbound.state.Perf != nil {
    streamFlag = m.outbound.state.Perf.Stream
}

perf.Stream = streamFlag
```

This ensures streaming requests maintain their Stream flag across the outbound request initialization, allowing TTFT metrics to be properly recorded and displayed.

**Impact**:
- TTFT display restored for streaming requests
- No behavior change for non-streaming requests
- Minimal performance overhead (single boolean check)

## 2025-02-05: Live Verification Status

**Current Status**: PR #753 is created but **NOT YET MERGED** (state: OPEN, mergedAt: null)

**Verification Blocker**:
- Live site verification cannot proceed until PR #753 is merged and deployed
- The fix is in branch `fix/ttft-stream-flag` with commits:
  - `c435c072`: fix(orchestrator): preserve stream flag for TTFT
  - `5d13c481`: test(orchestrator): add tests for Stream flag preservation

**Next Steps for Verification**:
1. Merge PR #753 to `unstable` branch
2. Deploy to production/staging environment
3. Make a streaming request via the UI or API
4. Navigate to `/project/requests`
5. Verify TTFT appears in the latency column (format: "TTFT: XXXms, Latency: YYYms")

**Local Testing Alternative**:
Run `pnpm test:e2e` from the frontend directory to verify end-to-end flow in test environment.

**Test Coverage**:
Comprehensive tests added in `internal/server/orchestrator/performance_test.go` (7 tests) all pass, confirming the fix works correctly.
