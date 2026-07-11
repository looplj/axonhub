# Implement — Responses Reasoning / Stream

For each micro-slice:

1. Read Responses baseline, reasoning/stream draft, and current native request/response/stream code.
2. Add red fixture for only that variant.
3. Implement smallest native sidecar/raw merge or stream-fidelity change.
4. Assert same-protocol replay and relevant explicit lossy behavior separately.
5. Run only scoped Responses transformer tests plus diff check.
6. Write micro-slice report before moving to next micro-slice.

After 8A–8E, run module reviewers for protocol split, native storage architecture, and stream lifecycle correctness.

