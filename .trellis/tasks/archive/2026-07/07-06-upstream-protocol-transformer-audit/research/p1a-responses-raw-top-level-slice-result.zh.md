# P1a result — OpenAI Responses raw top-level preservation

Date: 2026-07-07
Implementation worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
Branch: `codex/responses-top-level-preservation-clean`

## Scope

P1a only covers OpenAI Responses -> OpenAI Responses same-protocol request preservation for top-level fields not currently represented by `responses.Request`.

In scope:

- Preserve same-protocol raw top-level fields such as `context_management` and future/profile fields.
- Do not let raw top-level replay override structured fields such as `model`.
- Keep raw data in `ProviderExtensions.OpenAIResponses.Request`, not in common `llm.Request`.

Out of scope:

- Chat emission policy.
- Anthropic native preservation.
- Stream event fidelity.
- Cross-protocol lossy downgrade diagnostics.
- Typed `prompt` / `conversation` modeling; those remain for P1b.

## Red test evidence

Command:

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'TestOutboundTransformer_TransformRequest_(ReplaysRawTopLevelFields|RawTopLevelDoesNotOverrideStructuredFields|ReplaysProviderRawToolsAndToolChoice|ReplaysProviderRawInputItems|DoesNotReplayRawToolWhenToolsChanged)$' -count=1
```

Initial failure:

- `TestOutboundTransformer_TransformRequest_ReplaysRawTopLevelFields` failed because `context_management` was absent from outbound body.
- `TestOutboundTransformer_TransformRequest_RawTopLevelDoesNotOverrideStructuredFields` failed for the same missing raw top-level field.

## Implementation

Changed files:

- `llm/provider_extensions.go`
- `llm/transformer/openai/responses/request_extensions.go`
- `llm/transformer/openai/responses/outbound_test.go`

Implementation summary:

- Added `RawTopLevelFields map[string]json.RawMessage` to `llm.OpenAIResponsesRequestExtensions`.
- Deep-cloned that map in `CloneProviderExtensions` using the same raw-message clone style as existing raw fragments.
- Extended `parseRawRequestFragments` to collect raw top-level fields after deleting fields already represented by `responses.Request`.
- Replayed raw top-level fields in `marshalRequestPayload` only when the outbound payload does not already contain that key.
- Added tests for raw top-level replay, structured field precedence, and non-serialization of raw top-level provider extension data.

## Green test evidence

Commands:

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'TestOutboundTransformer_TransformRequest_(ReplaysRawTopLevelFields|RawTopLevelDoesNotOverrideStructuredFields|ReplaysProviderRawToolsAndToolChoice|ReplaysProviderRawInputItems|DoesNotReplayRawToolWhenToolsChanged)$|TestProviderExtensions_NotSerializedWithLLMRequest$' -count=1

go test ./transformer/openai/responses -count=1
```

Result:

- Both commands passed.

## Self-review

Pass:

- Field owner is the existing OpenAI Responses provider extension sidecar.
- No OpenAI Responses-specific field was added to common `llm.Request`.
- Same-protocol raw fallback is scoped to `llm/transformer/openai/responses`.
- Existing raw tools, raw input items, and raw tool choice behavior stayed green.
- Raw top-level replay does not overwrite structured outbound fields.
- Provider extension raw values remain excluded from JSON serialization of `llm.Request`.
- No Chat, Anthropic, Gemini, OpenRouter, or stream code was changed.

Known follow-up:

- P1b should decide typed ownership for official `prompt` and `conversation` instead of leaving them as generic raw top-level preservation.
- P1c should revisit the structured top-level field exclusion list against the final typed `responses.Request` model to avoid drift.
