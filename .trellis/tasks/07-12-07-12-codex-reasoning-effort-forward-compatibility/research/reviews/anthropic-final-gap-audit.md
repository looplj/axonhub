# Anthropic final protocol-gap audit

- Date: 2026-07-13
- Scope: current A1–A5 and safe D1/D2 work under `llm/transformer/anthropic/`, followed by a source-backed residual-gap pass.
- Public seams audited:
  - Anthropic request JSON → `llm.Request` → Anthropic request JSON;
  - Anthropic non-stream response JSON → `llm.Response` → Anthropic response JSON;
  - Anthropic SSE → canonical response chunks → Anthropic SSE;
  - OpenAI-origin canonical request → Anthropic request plus `LossyDowngrade` diagnostics.
- Write boundary respected: only `llm/transformer/anthropic/` and this report were changed. No shared model/provider-extension, Chat, Responses, strict-matrix, or guideline file was edited. No commit was made.

## Authorities and method

The audit used:

1. `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md`, especially the raw-preserve rule for unknown typed-union variants and the `ANT.MSG.content_blocks`, `ANT.STREAM.events`, and `ANT.RESP.message` backlog;
2. `docs/specs/protocols/anthropic-claude-messages-protocol.md`, especially:
   - line 84: unknown content block types should be raw-preserved on same-protocol replay;
   - lines 147 and 153: preserve native/future stop reasons, raw content blocks, citations, stop details, and usage details;
3. `research/inventory-anthropic.md`, `research/inventory-common.md`, and `research/full-field-gap-inventory.md`;
4. live source reads and symbol discovery from project `Users-asuan-AI-axonhub-llm`, plus direct reads of the already-modified working tree where the graph transport became unavailable;
5. one public-seam RED followed by the smallest adapter-local GREEN for every new production change.

The pre-existing working-tree diff was captured in `/tmp/anthropic-before.diff` before this residual pass. This audit did not revert or rewrite the existing A1–A5/D2 work.

## Current A1–A5 / diagnostic classification

| Area | Classification | Verified disposition |
|---|---|---|
| A1 unknown request blocks and unknown `tool_result.content[]` children | `IMPLEMENTED / PASS` | `a1_unknown_content_blocks_test.go` exercises the public request body seam. Unknown fragments retain their original index and raw JSON through Anthropic-only metadata on canonical content parts. |
| A2 Anthropic-only function-tool children and non-web-search native declarations | `IMPLEMENTED / PASS` | `a2_tool_declaration_preservation_test.go` proves raw declaration replay and order. Ordinary common tools remain typed; exclusive native declarations remain raw Anthropic fragments. No `llm.Tool` widening occurred. |
| A3 response `stop_sequence`, `stop_details`, and future/server-tool usage details | `IMPLEMENTED / PASS` | `a3_response_stop_usage_test.go` proves the non-stream public response seam and omission behavior. Typed usage emission merges with the original Anthropic usage object. |
| A4 unknown SSE event and unknown content-block lifecycle | `IMPLEMENTED / PASS` | `a4_raw_stream_event_test.go` proves raw replay of unknown semantic events and unknown block start/delta/stop without a duplicate stop. `ping` remains an intentional transport-heartbeat filter. |
| A5 stream matched stop sequence | `IMPLEMENTED / PASS` | `a5_stream_stop_sequence_test.go` proves `message_delta.delta.stop_reason` and `stop_sequence` replay. |
| D1/D2 safe field-level diagnostics | `IMPLEMENTED / PASS` | The explicit Anthropic outbound allowlist diagnoses OpenAI `frequency_penalty`, `presence_penalty`, Chat `seed`, `safety_identifier`, `prompt_cache_key`, non-`user_id` metadata, and the existing Chat raw-only fields. It uses `llm.AddLossyDowngradeIfPresent`; there is no reflection-based field matrix and no fake Anthropic mapping. |

## Residual source-proven losses fixed in this pass

### A6 — unknown/future non-stream response content block

**Evidence before fix**

`convertToLlmResponse` iterated `anthropicResp.Content`, but its outer `default` only populated `TransformerMetadataKeyAnthropicResponseContent` for recognized special server-tool use/result blocks. A genuinely unknown block had no canonical assignment and no native-sidecar assignment. The subsequent Anthropic response rebuild therefore emitted only representable blocks.

This contradicted the local baseline's explicit same-family raw-preserve rule and was separate from A1 request-block and A4 stream-event coverage.

**RED**

`TestA6_UnknownResponseContentBlock_SameProtocolRoundTrip` sent:

```json
[
  {"type":"text","text":"before"},
  {"type":"future_result","id":"future_1","payload":{"keep":true,"items":[1,2]}},
  {"type":"text","text":"after"}
]
```

through the public non-stream response seam. It failed because replay contained one block instead of three.

**GREEN**

The unknown branch now clones the original ordered response content into the existing Anthropic response-content native sidecar. It does not add a common field, cross-protocol mapping, or universal raw response bridge.

### A7 — native `pause_turn` and `refusal` stop reasons

**Evidence before fix**

`convertToLlmFinishReason` maps:

- `pause_turn` → canonical `stop`;
- `refusal` → canonical `content_filter`.

The native stop-reason metadata was previously recorded only for `stop_sequence`. On same-family replay, `pause_turn` became `end_turn`, and `refusal` became the literal common value `content_filter`.

Unknown future stop-reason strings already survive via the converter's default return and were not changed.

**RED**

`TestA7_AnthropicNativeStopReasons_SameProtocolRoundTrip` failed with:

- expected `pause_turn`, got `end_turn`;
- expected `refusal`, got `content_filter`.

**GREEN**

The existing Anthropic choice metadata now carries only the lossy native values `stop_sequence`, `pause_turn`, and `refusal`. Common/cross-protocol finish-reason behavior remains unchanged; ordinary `end_turn`, `max_tokens`, and `tool_use` do not gain unnecessary metadata.

### A8 — citation native and future details

**Evidence before fix**

`llm.Annotation` carries citation type/URL/title but not Anthropic `encrypted_index`, `cited_text`, or future citation children. The existing integration test explicitly expected the first two to be absent after replay, proving the same-family loss. `TextCitation` also had no unknown-child raw preservation.

**RED**

`TestA8_AnthropicCitationNativeDetails_SameProtocolRoundTrip` failed because replay reduced:

```json
{
  "type":"char_location",
  "encrypted_index":"enc-index",
  "cited_text":"original quote",
  "future_citation_detail":{"keep":true}
}
```

to only `{"type":"char_location"}`.

**GREEN**

- `TextCitation` now retains raw JSON only when a citation contains an unknown child; normal known citations retain their previous typed representation.
- Responses with citations retain their original ordered Anthropic content through the already-existing response-content native sidecar, while canonical annotations remain available to other targets.
- The existing citation integration test now asserts that `encrypted_index` and `cited_text` survive instead of documenting their loss.

This remains adapter-local and same-family only.

## Files changed by this residual pass

The shared working tree already contained A1–A5/D2 changes. Relative to the captured pre-audit Anthropic diff, this pass changed or added only:

```text
llm/transformer/anthropic/model.go
llm/transformer/anthropic/outbound_convert.go
llm/transformer/anthropic/integration_test.go
llm/transformer/anthropic/a6_unknown_response_content_test.go
llm/transformer/anthropic/a7_stop_reason_test.go
llm/transformer/anthropic/a8_citation_native_details_test.go
.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/reviews/anthropic-final-gap-audit.md
```

No other worker's Chat or Responses files were touched.

## RED / GREEN evidence

| Test | RED observation | GREEN observation |
|---|---|---|
| `TestA6_UnknownResponseContentBlock_SameProtocolRoundTrip` | replay length 1, expected 3 | PASS; unknown nested JSON and block order preserved |
| `TestA7_AnthropicNativeStopReasons_SameProtocolRoundTrip` | `pause_turn → end_turn`; `refusal → content_filter` | PASS for both native values |
| `TestA8_AnthropicCitationNativeDetails_SameProtocolRoundTrip` | native/future citation children absent | PASS; full `content` JSON-equivalent |

After A8, a full module run exposed expected assertions coupled to the former lossy citation behavior and raw fields on ordinary streamed citations. The implementation was narrowed so `TextCitation.Raw` is populated only for unknown children, and the existing non-stream citation integration assertion was updated from loss to preservation. Focused regressions and then the full module passed.

## Final verification

Executed exactly as requested from the repository root:

```bash
cd llm && go test ./transformer/anthropic -count=1 && cd .. && git diff --check
```

Result:

```text
ok  github.com/looplj/axonhub/llm/transformer/anthropic  0.276s
```

`git diff --check` produced no output and exited successfully.

No lint, build, server restart, or commit was performed.

## Explicit non-implementations and unresolved evidence

| Item | Classification | Reason |
|---|---|---|
| OpenAI `temperature > 1` → Anthropic | `BLOCKED_PRODUCT_DECISION` | Error/drop/diagnostic policy is not approved. No clamp or semantics change was made. |
| Cross-protocol `service_tier` values | `BLOCKED_PRODUCT_DECISION` | Protocol vocabularies are not equivalent. No guessed mapping or validation was added. |
| Anthropic `ping` replay | `EXPLICIT_NO_IMPLEMENT` | It remains a transport heartbeat rather than a semantic response event. |
| Unknown Anthropic SSE error-frame replay contract | `BLOCKED_EVIDENCE / POLICY` | Current behavior converts provider error events to stream errors. Exact raw event replay is not an approved contract. |
| Complete official server-tool/version matrix and complete known-block matrix | `TEST_ONLY / DOC_ONLY` | Representative native/raw paths pass, but this audit does not claim every official variant is confirmed. |
| Request `output_config` unknown nested keys and official null/default matrix | `BLOCKED_EVIDENCE` | Current typed storage may not carry every future child, but this pass did not establish an approved exact shape and public RED independent of local extension fields. |
| Dedicated Anthropic response provider-extension owner for all non-stream native fields | `ARCHITECTURE EVIDENCE, OUT OF SCOPE` | Existing non-stream native sidecars use Anthropic-local `TransformerMetadata`; moving them into `llm/provider_extensions.go` would require editing a forbidden shared file. This pass respected the current native sidecar rather than creating a second storage model. |
| Cross-protocol loss diagnostics beyond the explicit verified allowlist | `NO IMPLEMENTATION` | No reflection-based diagnostic matrix was added. Fields without proven source presence and no-equivalent target semantics remain unclaimed. |

## Final conclusion

The current A1–A5 and explicit D1/D2 work passes the Anthropic module. This final audit found and repaired three additional concrete Anthropic-owned same-protocol losses: unknown non-stream response blocks, lossy native stop reasons, and citation-native/future details. Every production change was triggered by a public-seam RED, stayed inside the existing Anthropic sidecars, and passed the required final verification. The result is not a claim that the aggregate Anthropic strict-matrix rows are fully confirmed.
