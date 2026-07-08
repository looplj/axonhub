# G9 Slice Review: stream_options raw nested preservation

- Date: 2026-07-08
- Reviewer role: independent Trellis check sub-agent
- Reviewed worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
- Branch: `codex/responses-top-level-preservation-clean`
- Scope: G9 slice fix for OpenAI Responses `stream_options` raw nested preservation
- Review mode: code review + validation commands; no business-code edits

## Verdict

PASS

The slice fixes the reported M1 bug: OpenAI Responses same-protocol round-trip now preserves `stream_options` unknown nested extension fields together with typed `include_obfuscation`.

## Reviewed diff

Changed files:

- `llm/transformer/openai/responses/request_extensions.go`
- `llm/transformer/openai/responses/outbound_test.go`

Diff summary observed during review:

```text
llm/transformer/openai/responses/outbound_test.go  | 86 ++++++++++++++++++++++
llm/transformer/openai/responses/request_extensions.go | 37 +++++++++-
2 files changed, 122 insertions(+), 1 deletion(-)
```

## Key evidence

### 1. M1 behavior is fixed

File: `llm/transformer/openai/responses/request_extensions.go`

Evidence:

- `marshalRequestPayload` no longer uses `mergeRawNativeTopLevelField` for `stream_options`.
- It now calls `mergeOpenAIResponsesStreamOptions(obj, requestExt.RawStreamOptions)`.
- The new helper unmarshals raw `stream_options`, reads any existing typed outbound `stream_options`, then overlays raw keys into the emitted object.

Reason:

- Before the fix, typed `convertStreamOptions` could emit `stream_options` containing only `include_obfuscation`; `mergeRawNativeTopLevelField` then skipped raw replay because the top-level key already existed.
- That dropped unknown nested fields such as `future_nested`.
- The new merge path preserves nested raw fields instead of treating the whole `stream_options` object as an all-or-nothing top-level field.

### 2. Raw precedence matches the comment

File: `llm/transformer/openai/responses/request_extensions.go`

Evidence:

```go
for name, value := range rawOptions {
    currentOptions[name] = cloneRaw(value)
}
```

Reason:

- `currentOptions` starts from typed outbound JSON if present.
- Raw keys are assigned afterward.
- Therefore raw values win on key collisions, matching the helper comment: raw values take precedence over typed values.

### 3. Chat-side pattern alignment is acceptable

Compared with: `llm/transformer/openai/chat_extensions.go` / `mergeOpenAIChatStreamOptions`

Evidence:

- Both helpers:
  - parse raw nested `stream_options` as `map[string]json.RawMessage`;
  - parse current typed `stream_options` if present;
  - merge raw nested fields into the current object;
  - marshal and write back `stream_options`.

Difference reviewed:

- Chat deletes `include_usage` from raw before merging because Chat has typed `include_usage` ownership.
- Responses does not delete `include_obfuscation`; this is intentional for the requested raw-priority semantics and same-protocol preservation.

Conclusion:

- The implementation follows the Chat merge pattern without copying Chat-specific ownership rules into Responses.

### 4. Regression coverage exists

File: `llm/transformer/openai/responses/outbound_test.go`

New tests:

- `TestOutboundTransformer_TransformRequest_PreservesStreamOptionsRawNestedFields`
  - Covers `include_obfuscation` plus unknown nested fields.
  - Verifies `future_nested` and `another_unknown` survive inbound -> outbound.

- `TestOutboundTransformer_TransformRequest_PreservesStreamOptionsWithoutTypedField`
  - Covers raw-only nested `stream_options` where typed `include_obfuscation` is absent.
  - Verifies outbound still includes `stream_options`.

Existing regression checked separately:

- `TestConvertStreamOptions`
- `TestOutboundTransformer_TransformRequest_RawTopLevelDoesNotOverrideStructuredFields`

### 5. Architecture boundary is preserved

Evidence:

- No changes to `llm.Request`.
- No changes to Chat, Anthropic, stream fidelity, shared lossy downgrade, or unrelated provider adapters.
- Storage remains in existing Responses provider extension/raw preservation seam.
- The fix is localized to `llm/transformer/openai/responses`.

Reason:

- This aligns with `.trellis/spec/backend/protocol-transformer-guidelines.md`: same-protocol preservation should deepen protocol native/raw seams, not widen common request models or metadata buckets.

## Edge cases reviewed

| Edge case | Result | Evidence |
|---|---|---|
| typed + raw coexist | Covered and passing | `TestOutboundTransformer_TransformRequest_PreservesStreamOptionsRawNestedFields` |
| typed nil + raw only | Covered and passing | `TestOutboundTransformer_TransformRequest_PreservesStreamOptionsWithoutTypedField` |
| raw empty | Code safely returns early | `if len(raw) == 0 { return }` |
| raw parse failure | Code safely returns without mutation | `if err := json.Unmarshal(raw, &rawOptions); err != nil { return }` |
| raw/typed same-key conflict | Code raw-wins | raw assignment happens after current typed parse |
| current typed parse failure | Non-fatal | ignored unmarshal error leaves empty `currentOptions`; raw can still be emitted |

## Validation commands run

From:

```text
/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
```

### Targeted StreamOptions tests

```bash
go test ./transformer/openai/responses/ -count=1 -v -run StreamOptions
```

Result: PASS

Observed passing tests:

- `TestConvertStreamOptions`
- `TestOutboundTransformer_TransformRequest_PreservesStreamOptionsRawNestedFields`
- `TestOutboundTransformer_TransformRequest_PreservesStreamOptionsWithoutTypedField`

### Raw top-level regression

```bash
go test ./transformer/openai/responses/ -count=1 -v -run 'TestOutboundTransformer_TransformRequest_RawTopLevelDoesNotOverrideStructuredFields'
```

Result: PASS

### Full llm module tests

```bash
go test ./... -count=1
```

Result: PASS

All `llm` packages completed successfully.

## Must-fix issues

None.

## Suggested follow-ups

Not required for this slice, but useful to lock helper behavior more directly:

1. Add helper-level/table coverage for empty raw input.
2. Add helper-level/table coverage for invalid raw JSON.
3. Add explicit collision test proving raw `include_obfuscation` overrides typed emitted `include_obfuscation`.

These are suggestions only; current integration-level tests plus passing regressions are sufficient for G9 acceptance.

## Final assessment

The G9 slice is small, localized, architecture-aligned, and verified. It fixes the M1 same-protocol preservation bug without introducing observed regressions or widening the protocol transformer architecture.
