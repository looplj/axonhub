# G5b Slice Ledger — Deprecated Chat functions / function_call

## Workflow
TDD per ~5-minute slice; self-review each slice; real module sub-agent review after all slices.

| Slice | Outcome | Evidence | Self-review | Status |
|---|---|---|---|---|
| S1 request `functions` red/green | Chat->Chat re-emits original `functions` | fixture + TestOpenAIChatRequestDeprecatedFunctionsRawRoundTrip | pass | completed |
| S2 request `function_call` | Chat->Chat re-emits original `function_call` | fixture + TestOpenAIChatRequestDeprecatedFunctionCallRawRoundTrip | pass | completed |
| S3 legacy+modern precedence | modern tools/tool_choice intact; legacy raw preserved | TestOpenAIChatRequestDeprecatedAndModernToolsPrecedence | pass | completed |
| S4 response `message.function_call` | bridge + client re-emit; drops modern tool_calls | TestOpenAIChatResponseDeprecatedMessageFunctionCallBridge + DropsModernToolCalls | pass | completed |
| S5 history/stream false-green fixes | origin metadata; stream partial/final; multi-turn history | History + Stream tests; review FAIL M1/M2 fixed | pass | completed |
| S6 cross-protocol + modern regression | Responses no-synth; Anthropic lossy; modern path | NotSynthesized + DiagnosesChatDeprecatedFunctionsLoss + ModernToolPath | pass | completed |
| S7 package verification | openai/anthropic green | go test ./transformer/openai ./transformer/anthropic -count=1 | pass | completed |

## Implementation notes
- Request top-level: `functions`/`function_call` in `openAIChatRawPreserveFields`.
- Response/history: `Message.FunctionCall` + origin metadata `openai.chat.function_call_origin`.
- Emitters: ChoiceFromLLM / MessageFromLLM restore legacy shape when origin set or finish_reason=function_call.
- Cross-protocol: Responses no-synth; Anthropic LossyDowngrade.

## Module review gate
- first review: FAIL by Aquinas `019f52ab-d3d0-73b3-8a71-bd6177020978` (M1 stream, M2 history)
- re-review: pending after fix commit
- report: research/reviews/
- commit: pending fixup
