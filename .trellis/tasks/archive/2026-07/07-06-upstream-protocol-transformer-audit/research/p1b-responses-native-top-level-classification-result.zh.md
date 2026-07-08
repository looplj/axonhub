# P1b result — OpenAI Responses native top-level field classification

Date: 2026-07-07
Implementation worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
Branch: `codex/responses-top-level-preservation-clean`

## Scope

P1b classifies official OpenAI Responses request top-level fields separately from same-protocol unknown raw fallback.

In scope:

- `prompt`
- `conversation`
- `context_management`

Out of scope:

- Chat emission policy.
- Anthropic native preservation.
- Stream event fidelity.
- Cross-protocol lossy downgrade diagnostics.

## Protocol evidence

Local OpenAPI extraction shows these OpenAI Responses request fields:

- `prompt`: `Prompt`.
- `conversation`: `ConversationParam | null`; `ConversationParam` can be string ID or object.
- `context_management`: `array[ContextManagementParam] | null`.
- `ResponsePromptVariables` can contain strings, input text, input image, or input file content.

Therefore the request struct uses `json.RawMessage` for these fields instead of a narrow Go struct that would lose union/object variants.

## Red test evidence

Command:

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'TestOutboundTransformer_TransformRequest_ClassifiesNativeTopLevelFieldsSeparatelyFromUnknownRaw$' -count=1
```

Initial failure:

- Build failed because `llm.OpenAIResponsesRequestExtensions` had no `RawPrompt`, `RawConversation`, or `RawContextManagement` fields.
- This proved the current code had no explicit native bucket for these official Responses fields; they were only preserved by generic raw top-level fallback from P1a.

## Implementation

Changed files:

- `llm/provider_extensions.go`
- `llm/transformer/openai/responses/model.go`
- `llm/transformer/openai/responses/request_extensions.go`
- `llm/transformer/openai/responses/outbound_test.go`

Implementation summary:

- Added `Prompt`, `Conversation`, and `ContextManagement` as `json.RawMessage` fields on `responses.Request`.
- Added named provider-extension fields:
  - `RawPrompt`
  - `RawConversation`
  - `RawContextManagement`
- Deep-cloned those fields in `CloneProviderExtensions`.
- Removed those official fields from generic `RawTopLevelFields` classification.
- Replayed named native raw fields before generic unknown raw top-level fields.
- Extended provider-extension non-serialization test to cover the new named raw fields.

## Green test evidence

Commands:

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'TestOutboundTransformer_TransformRequest_(ClassifiesNativeTopLevelFieldsSeparatelyFromUnknownRaw|ReplaysRawTopLevelFields|RawTopLevelDoesNotOverrideStructuredFields|ReplaysProviderRawToolsAndToolChoice|ReplaysProviderRawInputItems|DoesNotReplayRawToolWhenToolsChanged)$|TestProviderExtensions_NotSerializedWithLLMRequest$' -count=1

go test ./transformer/openai/responses -count=1
```

Result:

- Both commands passed.

## Self-review

Pass:

- Official Responses fields are now visible in the Responses native request model.
- Union/complex fields are kept as `json.RawMessage`, avoiding false narrowing.
- Official native fields no longer pollute generic unknown `RawTopLevelFields`.
- Unknown future/profile top-level fields still use `RawTopLevelFields`.
- Raw replay does not override structured outbound fields.
- No Chat, Anthropic, Gemini, OpenRouter, or stream code was changed.
- No dead prompt/conversation/context_management parsing path remains in `parseRawRequestFragments`; those fields now come from `responses.Request`.

Follow-up:

- P1c should clean up naming/field-list locality and decide whether the long empty-extension check should be factored into a helper before module-level review.
