# P1c result — OpenAI Responses request preservation cleanup

Date: 2026-07-07
Implementation worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
Branch: `codex/responses-top-level-preservation-clean`

## Scope

P1c is a cleanup/refactor slice for Module 1. It does not add new protocol behavior.

In scope:

- Make the extension empty-check readable.
- Rename the known top-level exclusion list to reflect field ownership instead of only structural JSON emission.
- Add comments explaining the split between owned native/common fields and unknown/profile raw fallback.

Out of scope:

- Chat.
- Anthropic.
- Stream events.
- Cross-protocol diagnostics.

## Baseline evidence

Before cleanup:

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -count=1
```

Result: passed.

## Implementation

Changed file:

- `llm/transformer/openai/responses/request_extensions.go`

Implementation summary:

- Replaced a long inline empty-extension condition with `hasOpenAIResponsesRequestExtensionData`.
- Renamed `openAIResponsesStructuredRequestFields` to `openAIResponsesOwnedRequestFields`.
- Added a comment documenting that fields in this list are owned by common model, Responses request model, or named provider-extension preservation; anything else goes to same-protocol unknown/profile `RawTopLevelFields`.

## Green test evidence

Commands:

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'TestOutboundTransformer_TransformRequest_(ClassifiesNativeTopLevelFieldsSeparatelyFromUnknownRaw|ReplaysRawTopLevelFields|RawTopLevelDoesNotOverrideStructuredFields|ReplaysProviderRawToolsAndToolChoice|ReplaysProviderRawInputItems|DoesNotReplayRawToolWhenToolsChanged)$|TestProviderExtensions_NotSerializedWithLLMRequest$' -count=1

go test ./transformer/openai/responses -count=1
```

Result: both passed.

## Self-review

Pass:

- No behavior change beyond P1a/P1b preservation already tested.
- No Chat, Anthropic, Gemini, OpenRouter, or stream files touched.
- Helper name describes the module-level intent.
- Field ownership list comment explains how future fields should be routed.
- No dead prompt/conversation/context_management parsing path remains.

Ready for Module 1 review.
