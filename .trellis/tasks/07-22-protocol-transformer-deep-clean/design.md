# Design: Protocol transformer deep clean S1–S7

## Architecture constraints (do not re-litigate)

- ADR 0001 / 0002 / CONTEXT / protocol-transformer-guidelines (refreshed 2026-07-22).
- Deepen existing seams; no second conversion framework.
- Vocabulary: module, interface, depth, seam, adapter, leverage, locality.

## Target module map (after S1–S7)

```text
llm.Request / Response          → CrossProtocolCanonical only
ProviderExtensions
  .OpenAIResponses.Request|Response  → Responses natives + raw
  .OpenAIChat (new or equiv)         → Chat same-protocol natives (S4)
  .Anthropic.Request|Response        → Anthropic natives + raw (S5 expand)
  .Diagnostics                       → Lossy (+ optional preservation section S2)
llm/transformer/shared
  CustomToolLifecycle (S3)           → preserve | bridge | drop + rehydrate
  StripOpaqueReasoningState (S6)     → field mutation only
orchestrator
  candidate eligibility, when-to-strip, pass-through mode select (S7)
  NO protocol field shape ownership for tools/reasoning body
```

## Slice design notes

### S1 Responses PE cutover

- Delete writes of Responses body fields into `TransformerMetadata`.
- Outbound/inbound read only PE getters.
- Deprecate/remove `MetadataKeyInclude` usage as write target; tests assert PE.
- Files: `responses/request_extensions.go`, `inbound.go`, `outbound*.go`, `model.go` comments already fixed, tests g13a / inbound_test conflicts.

### S2 Diagnostics convergence

- Prefer `AddLossyDowngrade` / PE.Diagnostics.
- Collapse duplicate custom-tool / field counters where same event is recorded thrice.
- Metadata diagnostic blobs only if HTTP round-trip forces it; prefer delete.

### S3 CustomToolLifecycle

- Interface (conceptual): `Apply(req, targetFormat) (req, state, err)` + `Rehydrate(resp|stream, state)`.
- Orchestrator freeform bridge becomes thin adapter.
- Chat native custom preserve and Anthropic drop both call same policy table.

### S4 Chat PE

- `ProviderExtensions.OpenAIChat` (name final during implement if collision).
- Migrate `chat_n` raw preserve fields off full-body reparse for ownership; outbound merge helper.
- Cross-protocol readers stop unmarshalling entire Chat body for one field.

### S5 Anthropic PE

- Move container / inference_geo / mcp_servers primary storage to Anthropic PE request extensions.
- Metadata keys become migration delete targets after tests green.

### S6 Opaque strip helper

- `llm` or `transformer/shared`: strip signatures, ResponseReasoningItemID rules, PE RawInputItems opaque items.
- Orchestrator: CanRecover / NextChannel / model switch call helper + flags only.

### S7 Pass-through adapter

- RouteDecision or explicit mode: Convert vs PassThrough.
- Pass-through: constrained patches (model, stream align); not “transform then dump” without mode.
- Recovery disables pass-through remains.

## Compatibility / migration

- External API wire unchanged for clients.
- Internal PE shape may grow; clone helpers updated.
- No DB migration.

## Risks

| Risk | Mitigation |
|---|---|
| Test suite encodes metadata body | Rewrite tests same slice (locked) |
| Orch + llm dual merge conflicts vs unstable | Package-scoped PRs |
| Pass-through product behavior change | Explicit adapter, same enable flags |
| Scope creep to S8 bridges | Out of scope checklist |

## Rollback

- Per-slice git revert.
- Do not merge PRs until human review.
