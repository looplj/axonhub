# Residual High-Priority Fixture Gaps (Post G1–G7)

These remain **fixture-only / documentation** items or intentional non-goals. They do **not** reopen implementation modules unless evidence shows missing storage/conversion.

## Closed by G1–G7 implementation + tests
- Chat `n`
- Chat `prompt_cache_retention`
- Anthropic `container`, `inference_geo`
- Chat `audio`, `prediction`, `moderation`
- Chat `web_search_options`
- Chat deprecated `functions`, request `function_call`, response `message.function_call`
- Anthropic `mcp_servers`, `tools[].type=mcp_toolset`
- Responses `reasoning.context`, deprecated `generate_summary`, reasoning `content[]`/`reasoning_text`, stream text path, unknown nested reasoning

## Still fixture-only / intentional lossy (no new feature in this task)
1. **Token-limit precedence multi-direction table** — existing converters handle fields; full multi-protocol precedence fixture matrix can expand later without new seams.
2. **Responses namespace / Codex P1 sub-agent/codex_app catalog** — keep P1-labeled; not public P0 completion claim.
3. **Full Responses SSE tool/audio event family parity** — not required for G7 reasoning slice; remain partial.
4. **Anthropic thinking multi-block ordering edge cases** — existing adaptive/enabled paths covered; multi-block exact-order fixtures optional.
5. **Chat custom tool forms** — source gap remains; do not invent support.

## Rule
Any row still `UNCHECKED` outside the closed set above is either lower priority residual or blocked on source clarification, not an open G1–G7 implementation gap.
