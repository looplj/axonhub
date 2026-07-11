# G5b Slice Ledger — Deprecated Chat functions / function_call

## Workflow
TDD per ~5-minute slice; self-review each slice; real module sub-agent review after all slices.

| Slice | Outcome | Evidence | Self-review | Status |
|---|---|---|---|---|
| S1 request `functions` red/green | Chat->Chat re-emits original `functions` | fixture openai-deprecated-functions.request.json + TestOpenAIChatRequestDeprecatedFunctionsRawRoundTrip | pass | completed |
| S2 request `function_call` | Chat->Chat re-emits original `function_call` | fixture openai-deprecated-function-call.request.json + TestOpenAIChatRequestDeprecatedFunctionCallRawRoundTrip | pass | completed |
| S3 legacy+modern precedence | modern tools/tool_choice common path intact; legacy raw preserved | TestOpenAIChatRequestDeprecatedAndModernToolsPrecedence | pass | completed |
| S4 response `message.function_call` | parse upstream function_call into tool semantic; client re-emits legacy shape | TestOpenAIChatResponseDeprecatedMessageFunctionCallBridge | pass | completed |
| S5 cross-protocol + modern regression | Responses no-synth; Anthropic lossy; modern tool path unchanged | NotSynthesizedForResponses + DiagnosesChatDeprecatedFunctionsLoss + ModernToolPathUnaffected | pass | completed |
| S6 package verification | openai/anthropic package tests green | go test ./transformer/openai -count=1; go test ./transformer/anthropic -count=1 | pass | completed |

## Implementation notes
- Request side: `functions` / `function_call` added to `openAIChatRawPreserveFields`; do not widen `llm.Request`.
- Response side: `openai.Message.FunctionCall` native field; ToLLMMessage bridges to modern tool_calls when tool_calls absent; ChoiceFromLLM re-emits function_call when finish_reason=function_call.
- Cross-protocol: Responses no-synth; Anthropic records LossyDowngrade for both fields.

## Module review gate
- review: pending real sub-agent
- report: research/reviews/
- commit: pending
