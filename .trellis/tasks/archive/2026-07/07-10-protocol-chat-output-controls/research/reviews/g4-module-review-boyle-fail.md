# G4 Module Review — Boyle

Agent: 019f5271-70a5-7663-bb34-e2185c7ea405 (Boyle)
Commit: 9a2692ed
Result: FAIL

## Findings
1. major: openAIChatRawPreserveFields / Anthropic lossy list over-scoped to web_search_options/functions/function_call
2. major: PRD requires separate fixtures per field; only one combined fixture exists
3. minor: core audio/prediction/moderation path is directionally correct

## Required fix before re-review
- Remove tool/deprecated fields from G4 lists
- Split fixtures and tests by audio / prediction / moderation
- Keep same-protocol preserve + Responses no-synth + Anthropic lossy
