# Responses final source-proven gap audit

- Date: 2026-07-13
- Scope: `llm/transformer/openai/responses/**` only.
- Public seam: Responses SSE chunks -> `AggregateStreamChunks` -> terminal Responses JSON.
- Boundary: same-Responses native preservation. No Chat/Anthropic bridge, common-model widening, universal raw item architecture, model capability table, or guessed future event/item shape.
- Shared-tree rule: R1-R4 and another worker's in-progress compaction aggregation change were audited in place and not reverted or replaced.

## Evidence used

1. `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md`
   - `RSP.STREAM.events` and `RSP.RESP.output` remain broad `PARTIAL` rows; this audit does not promote either whole family to confirmed.
2. `docs/specs/protocols/openai-responses-protocol.md`
   - Requires Responses status/error/incomplete details, output item type/order, and semantic SSE fidelity.
3. Checked-in OpenAI canonical extracts:
   - `docs/specs/vendor/protocol-canonical-2026-07-06/openai-responses-create.platform-snapshot.md`
   - `docs/specs/vendor/openai/official-2026-07-06/_try_responses-create.md`
   - These concretely define message refusal content (`{type:"refusal", refusal}`), web-search actions (`search`, `open_page`, `find_in_page`), required function-call arguments, and compaction (`id`, `type`, `encrypted_content`, optional `created_by`).
4. Existing concrete stream fixture:
   - `llm/transformer/openai/responses/image_stream_test.go` contains an `image_generation_call` final item with `action`, `background`, `output_format`, `quality`, `result`, `revised_prompt`, and `size`.
   - The local official image guide extract also documents response `revised_prompt`.
5. Current R1-R4 code/tests and `responses-post-r4-gap-audit.md` were reviewed before adding slices.

## Pre-existing R1-R4 / compaction audit

| Area | Classification | Result |
|---|---|---|
| non-stream queued/in-progress status, incomplete details, refusal | already repaired by R1 | No duplicate implementation. |
| terminal completion without usage and top-level error aggregation | already repaired by R2 | No duplicate implementation. |
| raw non-stream output item replay | already repaired by R3 | Remains same-Responses only. |
| valid unknown live SSE replay | already repaired by R4 | Remains live-stream replay, not terminal aggregation. |
| compaction `output_item.added/done` aggregation | another worker's verified in-progress repair | Audited without overwriting: aggregator retains `id`, `type`, `encrypted_content`, `created_by` (and its current status handling); its targeted test is present in `r2_stream_terminal_error_test.go`. |
| `compaction_summary` | DOC_ONLY / TEST_ONLY | No concrete official baseline shape was found in the checked-in extracts. The existing branch accepts the type, but this audit does not claim family completeness or add guessed fields. |

## Source-proven gaps fixed

### 1. Refusal content in terminal stream aggregation

**Before:** `StreamEventContentPart.Refusal` and final `Item.Refusal` were parsed, but `aggregatedContentPart` retained only text and annotations. The terminal item became `type:"refusal"` with an empty text pointer and no `refusal` payload.

**Fix:** retain `Refusal` from both content-part and authoritative output-item-done snapshots; emit refusal without rewriting it as `output_text`.

**Test:** `TestResponsesFinalGap_AggregateStreamChunks_PreservesRefusalContent`.

- RED: failed because `part.Refusal` was nil.
- GREEN: exact refusal string survives; `Text` remains nil.

### 2. Fully modeled web-search action aggregation

**Before:** `Item.Action` parsed the official web-search action union, but the aggregator discarded it and rebuilt a generic `{id,type,status,role}` item.

**Fix:** retain `ItemAction` in aggregation and emit a native `web_search_call`; complete `WebSearchAction`'s already-official union with `url` and `pattern` for `open_page` / `find_in_page`.

**Tests:**

- `TestResponsesFinalGap_AggregateStreamChunks_PreservesWebSearchAction`
  - RED: aggregated `Action` was nil.
  - GREEN: `search` type, ordered queries, and URL source survive.
- `TestResponsesFinalGap_AggregateStreamChunks_PreservesFindInPageAction`
  - RED: the typed model had no `URL` / `Pattern` fields (compile failure).
  - GREEN: official `find_in_page` URL and pattern survive.

### 3. Modeled image-generation final fields

**Before:** the final snapshot was parsed into `Item`, but aggregation rebuilt only id/type/status/call_id/result. It lost action and all modeled option/result details. `revised_prompt` was present in a checked-in concrete stream fixture but absent from `Item` entirely.

**Fix:** add the native `revised_prompt` field to the Responses `Item`; retain and rebuild action, background, output format, quality, revised prompt, size, and result.

**Test:** `TestResponsesFinalGap_AggregateStreamChunks_PreservesModeledImageFields`.

- RED 1: `Action` was nil and modeled final fields were absent.
- RED 2: adding the concrete `revised_prompt` assertion exposed the missing typed field at compile time.
- GREEN: all concrete final snapshot fields survive.

### 4. Authoritative final tool-call snapshots

**Before:** `response.output_item.done` updated status and non-empty arguments, but did not copy final `call_id`, `name`, `namespace`, custom-tool `input`, role/type/id, or an authoritative empty function-call arguments string. A skeletal added event followed by a complete done event therefore emitted incomplete tool calls.

**Fix:** merge the fully modeled final item identity/type/role/tool fields into the existing aggregate. For official `function_call`, final `arguments` replaces accumulated data even when it is the required empty string.

**Tests:**

- `TestResponsesFinalGap_AggregateStreamChunks_UsesFinalToolSnapshots`
  - RED: final function `call_id` remained empty (and subsequent fields were likewise unavailable).
  - GREEN: final function/custom call fields survive independently.
- `TestResponsesFinalGap_AggregateStreamChunks_FinalEmptyArgumentsWin`
  - RED: stale added-event arguments survived instead of the final empty value.
  - GREEN: final empty arguments wins.

## Files changed by this final audit

- `llm/transformer/openai/responses/aggregator.go`
  - native aggregation carriers and final snapshot merge for already modeled fields.
- `llm/transformer/openai/responses/model.go`
  - official web-search action `url`/`pattern`; image `revised_prompt`.
- `llm/transformer/openai/responses/responses_final_gap_test.go`
  - six public aggregation regression tests.
- This report.

The same files also contain pre-existing uncommitted R1-R4/compaction work from other workers; this audit did not revert or reformat those changes.

## Remaining classifications and rationale

| Classification | Remaining area | Rationale |
|---|---|---|
| TEST_ONLY | mixed supported-family output ordering, usage snapshot precedence, all annotation subtypes | Current code has typed owners and existing broad tests; no new observable loss was proved during this audit. |
| TEST_ONLY / DOC_ONLY | `compaction_summary` | Existing code mentions the type, but no checked-in official concrete shape was found. Do not infer it from `compaction`. |
| IMPLEMENTATION candidates requiring separate exact fixtures | file search, computer, code interpreter, MCP call/list/approval, tool-search, shell/local-shell/apply-patch stream item aggregation | R3 protects non-stream raw replay, but terminal stream aggregation still narrows unsupported item families. Each union needs its exact official fixture and owner; this audit deliberately did not bulk-model them. |
| DIAGNOSTIC / DOC_ONLY | unknown future output item/event types | A syntactically valid unknown live event is handled by R4 same-stream replay. There is no justified terminal aggregation mapping without a concrete official shape. |
| EXPLICIT_NO_IMPLEMENT | Chat/Anthropic mapping, client approval/sandbox/OAuth/multi-agent/telemetry | These are different protocol/control-plane domains and were outside the authorized Responses same-protocol seam. |
| EXPLICIT_NO_IMPLEMENT | standalone stream error `param` on terminal `Response.Error` | The checked-in terminal response error shape has no `param`; do not invent one. |

## Verification

Each test above was observed RED before its minimal fix and GREEN afterward. Final commands:

```text
cd llm && GOCACHE=/tmp/axonhub-go-cache go test ./transformer/openai/responses -count=1
ok github.com/looplj/axonhub/llm/transformer/openai/responses

git diff --check
(no output; exit 0)
```

`GOCACHE` was redirected to `/tmp` because the sandbox denied access to the default user cache; this does not change test semantics. No lint, build, server restart, or commit was performed.

## Self-review

- Ownership: all new state is Responses-native and package-local.
- Same-protocol first: tests exercise only Responses SSE -> terminal Responses JSON.
- Presence/value semantics: refusal stays refusal; final empty function arguments is retained as a real value; no IDs/defaults are synthesized.
- Final-snapshot precedence: only fields concretely owned by `Item` are merged; no reflection or raw universal carrier was introduced.
- Cross-protocol boundary: no shared `llm` model, Chat adapter, Anthropic adapter, strict matrix, or shared documentation was modified.
- Shared tree: no other worker's files were reverted, overwritten, or committed.
