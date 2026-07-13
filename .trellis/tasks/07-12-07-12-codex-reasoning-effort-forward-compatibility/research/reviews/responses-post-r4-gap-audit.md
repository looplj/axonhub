# Responses post-R4 gap audit

- Date: 2026-07-13
- Scope: `llm/transformer/openai/responses/**` only; focused on `RSP.RESP`, `RSP.STREAM`, `output[]`, `response.compaction`, aggregation and R3/R4 raw sidecars.
- Evidence method: code graph CLI against `Users-asuan-AI-axonhub`, then the local Responses protocol baseline, strict matrix and full-field inventory. No cross-protocol mapping or common `llm` model expansion is proposed here.

## Result

One real, same-protocol aggregation gap was found and fixed. The remaining reviewed items are already fixed, test/documentation work, or intentionally outside the terminal `Response` schema.

## Confirmed and fixed: aggregated compaction output item

### Evidence

1. The local Responses reference documents `Compaction` as an `output[]` item with `id`, `type`, `encrypted_content`, and optional `created_by` (`docs/specs/vendor/protocol-canonical-2026-07-06/openai-responses-reference.exa.md`, compaction item reference).
2. `Item` already owns these fields in `llm/transformer/openai/responses/model.go`; this is not a request field or a cross-protocol semantic.
3. `streamAggregator.processEvent` accepted `response.output_item.added` / `.done` for a `type:"compaction"` item, but `buildResponse` had no compaction branch. The aggregate therefore emitted no native compaction item, or emitted one without the native compaction fields depending on the partial event sequence.

### RED → GREEN

- RED test: `TestR2_AggregateStreamChunks_CompactionOutputItem` in `r2_stream_terminal_error_test.go`.
- RED failure before the change: `encrypted_content` was empty after `AggregateStreamChunks`.
- Minimal implementation in `aggregator.go`:
  - retain `CreatedBy` in the internal aggregated item during `added` and `done` snapshots;
  - emit `compaction` / `compaction_summary` using the existing native `Item` fields: `id`, `type`, `status`, `encrypted_content`, `created_by`.
- GREEN verification:

  ```bash
  cd llm && go test ./transformer/openai/responses -count=1
  git diff --check
  ```

  Both passed.

## Already fixed by R1–R4; no further change

| Area | Evidence / owner | Audit result |
|---|---|---|
| non-stream status `queued` / `in_progress` | R1 status carrier and Responses inbound reconstruction | Already fixed; no duplicate change. |
| `incomplete_details` | `ProviderExtensions.OpenAIResponses.Response.RawTopLevelFields` | Already fixed with named Responses response owner; not `TransformerMetadata`. |
| non-stream raw `output[]` families | R3 `RawOutputItems` ordered merge | Already fixed for output item types without a canonical representation. |
| raw/unknown Responses SSE | R4 `RawStreamEvents`, replayed only by Responses inbound stream | Already fixed. It is deliberately not a generic chunk or an aggregation carrier. |
| standalone top-level `error` aggregation | R2 `streamAggregator.responseError`, failed status, existing R2 contract fixture | Already fixed for response `status`, error `type/code/message`. |

## `error.param`: reviewed, deliberately not added to final response

`StreamEvent.Param` is a field of the standalone stream `error` event. The local Responses reference describes the terminal response's error object with response error `code` and `message` (the local model also retains `type`); it does not give the final `Response.Error` object a `param` member.

Therefore:

- outbound stream conversion may carry `param` in `llm.ResponseError.Detail.Param` while handling the live event;
- `AggregateStreamChunks` preserves the terminal response's official `status` and `error` object but must **not** invent `Response.Error.param`;
- the R2 test comment now states that boundary explicitly.

This is `EXPLICIT_NO_IMPLEMENT`, not an unimplemented field.

## `response.compaction` event versus compaction output item

The phrase "response.compaction" appears in the local corpus as the compact API response object (`object:"response.compaction"`) and as an output-item family. This audit found no checked-in official streaming-event shape that supplies an independent `response.compaction` SSE payload with a defined terminal aggregation mapping.

- An unhandled syntactically valid Responses SSE event is already handled by R4's same-protocol raw stream sidecar and replayed only on the Responses outbound path.
- The fixed issue is narrower and proven: a **known `response.output_item.*` event whose `item.type` is `compaction`** was accepted by the aggregator but not reconstructed.
- No public event constant, common AST field, or fabricated `response.compaction` stream mapping was added.

## Raw sidecar / aggregation boundary

| Path | Owner | Correct behavior |
|---|---|---|
| Responses non-stream `output[]` raw-only item | R3 `RawOutputItems` | Ordered same-Responses response replay. |
| Responses stream unknown semantic event | R4 `RawStreamEvents` | Exact same-Responses stream replay; it bypasses aggregation by design. |
| Responses stream known `output_item` lifecycle | aggregator + native `Item` | Reconstruct a final native `Response.Output` item when a concrete native item owner exists. Compaction now follows this path. |

The sidecars must not be injected into `AggregateStreamChunks`: aggregation produces a terminal Responses `Response`, while raw events preserve the live event transcript. Combining them would conflate two separate contracts.

## Remaining items (not implementation changes in this audit)

| Classification | Item | Reason |
|---|---|---|
| TEST_ONLY | mixed supported output ordering, terminal usage precedence, annotation subtype coverage | Existing typed owners exist; current evidence identifies narrow confidence fixtures, not an observed loss. |
| TEST_ONLY | output-item `compaction_summary` with a dedicated aggregation fixture | Implementation branch supports it, but this audit's official fixture proved `compaction`; add a separate baseline-shaped fixture before declaring full family coverage. |
| IMPLEMENTATION candidate, separately fixture-split | raw output item families such as file search, computer, code interpreter, MCP/tool-search/local-shell/apply-patch | R3 protects non-stream raw replay, but each stream lifecycle / terminal aggregation behavior needs an exact local official shape and its own slice. Do not bulk-model or fake bridge. |
| DIAGNOSTIC / DOC_ONLY | malformed JSON and unknown future aggregation event | Existing aggregator intentionally skips malformed JSON; R4 preserves valid unknown events in the live same-protocol stream. No concrete official terminal item mapping was proven here. |

## Changed files in this scope

- `llm/transformer/openai/responses/aggregator.go`
- `llm/transformer/openai/responses/r2_stream_terminal_error_test.go`
- `.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/reviews/responses-post-r4-gap-audit.md`

## R1 follow-up — completed response envelope sidecar

### Scope and owner

- Verified gap: non-stream `Responses HTTP response -> llm.Response -> Responses HTTP response` dropped supplied top-level `completed_at` and `output_text`.
- Owner: `ProviderExtensions.OpenAIResponses.Response.RawTopLevelFields`, same as the existing Responses-native response sidecar policy. These fields are not stored in `TransformerMetadata` and are replayed only by the Responses response adapter.
- Limited implementation: capture only `completed_at` and `output_text` when present in the source envelope, then restore only those allowlisted fields. Existing serialized/typed fields take precedence over sidecar values.
- Omission control: a completed response without either field remains without either field; the Hub does not synthesize them.

### TDD evidence

RED before production change:

```text
cd llm && go test ./transformer/openai/responses -run '^TestR1_NonStreamCompletedEnvelopeFields_RoundTrip$' -count=1
--- FAIL: TestR1_NonStreamCompletedEnvelopeFields_RoundTrip/provided
Error: Expected value not to be nil.
FAIL
```

The failure occurred because the supplied completed-response envelope produced no `OpenAIResponses.Response` sidecar.

GREEN after the minimal capture/restore change:

```text
cd llm && go test ./transformer/openai/responses -run '^TestR1_(NonStreamCompletedEnvelopeFields_RoundTrip|RestoreResponseTopLevelFields_TypedEnvelopeWins)$' -count=1
ok github.com/looplj/axonhub/llm/transformer/openai/responses
```

### Explicit non-scope / remaining items

- No change to `llm.Response`, `TransformerMetadata`, common/provider-extension definitions, Anthropic, Chat, stream aggregation, output-item modeling, or the strict protocol matrix.
- `incomplete_details` remains under its existing `RawTopLevelFields` response-sidecar owner and was not reworked.
- Other response envelope/output/stream residuals listed above remain separate fixture-split work; this follow-up makes no claim about them.
