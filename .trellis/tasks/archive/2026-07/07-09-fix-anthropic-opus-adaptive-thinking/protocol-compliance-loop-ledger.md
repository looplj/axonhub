# Protocol Transformer Compliance Loop Ledger

Date: 2026-07-10
Active goal: repair all identified implementable protocol-transformer gaps across OpenAI Responses, OpenAI Chat Completions, and Anthropic Messages, using the existing AxonHub transformer architecture.

## Scope boundary

In scope:

- Implementable gaps listed in `.agent/summary/2026-07-10-protocol-transformer-compliance-backlog.md`.
- High-priority fixture-only gaps when tests are required to prove existing behavior.
- Existing AxonHub transformer architecture only: common abstraction for stable overlap, adapter-native fields, provider extensions/raw sidecars, and lossy/unsupported diagnostics.
- Small vertical slices with TDD, targeted checks, self-review, module review, parent review, spec update, and local commits.

Out of scope unless separately authorized:

- Rewriting the transformer architecture into a universal AST.
- Forcing lossy/unsupported fields into fake cross-protocol mappings.
- Pushing to GitHub.
- Building, starting, restarting, initializing, logging in to, or replaying requests against runtime services. This goal uses code-only verification unless the user explicitly reauthorizes runtime validation.

## Source references

- `.trellis/spec/backend/protocol-transformer-guidelines.md`
- `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md`
- `docs/specs/protocols/openai-responses-protocol.md`
- `docs/specs/protocols/openai-chat-completions-protocol.md`
- `docs/specs/protocols/anthropic-claude-messages-protocol.md`
- `docs/specs/protocols/drafts/batch-common-fields.md`
- `docs/specs/protocols/drafts/batch-limits-state-cache.md`
- `docs/specs/protocols/drafts/batch-output-message-content.md`
- `docs/specs/protocols/drafts/batch-reasoning-stream.md`
- `docs/specs/protocols/drafts/batch-tools-mcp.md`
- `.agent/summary/2026-07-10-protocol-transformer-compliance-backlog.md`

## Stop Condition

| Gate | Status | Evidence |
|---|---|---|
| All implementation slices pass targeted verification | pending | Slice ledger below. |
| All slices pass self-review | pending | Slice self-review records. |
| Module review findings closed | pending | Module review after each group. |
| Parent review passes | pending | Parent review after all groups. |
| Required checks pass or are explicitly skipped | pending | Targeted Go tests per slice; broader tests only when authorized/relevant. |
| Durable knowledge updated or marked unnecessary | pending | `.trellis/spec/backend/protocol-transformer-guidelines.md` updates. |
| Local commits created by slice | pending | Git commit hashes per slice. |
| Remaining risks stated | pending | Final report. |

## Slice Ledger

| Group | Slice | Outcome | Seam | Verification | Write set | Status | Review | Notes |
|---|---|---|---|---|---|---|---|---|
| G0 | S0 Anthropic adaptive/effort-first thinking routing | Centralized Anthropic thinking capability policy; adaptive-only/adaptive-preferred/manual-supported/unknown routing fixed; DeepSeek kept separate (including adaptive metadata replay); invalid manual budgets rejected; unknown capability records lossy diagnostic instead of guessing | `llm/transformer/anthropic` outbound reasoning/thinking (`OutboundTransformer.TransformRequest` + request conversion helpers) | `cd llm && go test ./transformer/anthropic -count=1` passed on 2026-07-10; `git diff --check -- llm/transformer/anthropic` passed; targeted tests cover Opus 4.8 serialized request body, `minimal -> low`, DeepSeek isolation and adaptive-metadata downgrade, explicit/configured manual-budget rejection, adaptive-only explicit budget rejection, unknown-capability lossy diagnostic | `llm/transformer/anthropic/outbound.go`, `llm/transformer/anthropic/outbound_convert.go`, `llm/transformer/anthropic/outbound_test.go`, `llm/transformer/anthropic/integration_test.go`, `llm/transformer/anthropic/thinking.go`, `llm/transformer/anthropic/thinking_test.go`, `llm/transformer/anthropic/testdata/anthropic-thinking.request.json`, `llm/transformer/anthropic/testdata/anthropic-tool-result-mixed.request.json` | passed | bug reviewer PASS; protocol reviewer PASS; architecture reviewer PASS | Closed prior FAIL findings: removed scattered model-name routing from request builder, fixed `minimal` handling, isolated DeepSeek including metadata replay, restored fixture legality without unrelated signature churn. |
| G1 Common fields | S1 Chat `n` storage/preservation decision | Preserve Chat-native `n` or document unsupported with diagnostics/tests | OpenAI Chat request native model / outbound/inbound | pending TDD | pending | pending | pending | First new slice. Smallest workflow validation slice. |
| G2 OpenAI state/cache | S2 Chat/OpenAI `prompt_cache_retention` coverage | Same-OpenAI typed/raw preservation | OpenAI Chat + Responses adapters | pending TDD | pending | pending | pending | Do not bridge unless documented/testable. |
| G3 Anthropic request native fields | S3 Anthropic `container` | Anthropic adapter-specific typed/raw preserve | Anthropic request model/inbound/outbound | pending TDD | pending | pending | pending | Same-protocol first. |
| G3 Anthropic request native fields | S4 Anthropic `inference_geo` | Anthropic adapter-specific typed/raw preserve | Anthropic request model/inbound/outbound | pending TDD | pending | pending | pending | Same-protocol first. |
| G4 Chat request native fields | S5 Chat top-level `audio` | Chat-native preservation | OpenAI Chat request model/inbound/outbound | pending TDD | pending | pending | pending | Distinct from assistant message audio. |
| G4 Chat request native fields | S6 Chat `prediction` | Chat-native preservation | OpenAI Chat request model/inbound/outbound | pending TDD | pending | pending | pending | No fake Responses bridge. |
| G4 Chat request native fields | S7 Chat `moderation` | Chat-native preservation | OpenAI Chat request model/inbound/outbound | pending TDD | pending | pending | pending | Distinct from Responses image moderation. |
| G5 Tools/MCP | S8 Chat `web_search_options` | Chat-native preservation | OpenAI Chat request model/inbound/outbound | pending TDD | pending | pending | pending | Not Responses hosted web_search. |
| G5 Tools/MCP | S9 Deprecated Chat `functions` | Deprecated compatibility | OpenAI Chat request model/inbound/outbound | pending TDD | pending | pending | pending | Precedence vs tools. |
| G5 Tools/MCP | S10 Deprecated Chat request `function_call` | Deprecated compatibility | OpenAI Chat request model/inbound/outbound | pending TDD | pending | pending | pending | Precedence vs tool_choice. |
| G5 Tools/MCP | S11 Deprecated Chat response `message.function_call` | Deprecated compatibility | OpenAI Chat response/stream handling | pending TDD | pending | pending | pending | Separate from tool_calls. |
| G6 Anthropic MCP | S12 Anthropic `mcp_servers` | Adapter-specific connector/raw preserve | Anthropic request model/inbound/outbound | pending design/TDD | pending | pending | pending | Larger design slice. |
| G6 Anthropic MCP | S13 Anthropic `tools[].type="mcp_toolset"` | Adapter-specific tool variant | Anthropic tools model/inbound/outbound | pending design/TDD | pending | pending | pending | Larger design slice. |
| G7 Responses reasoning | S14 Responses request `reasoning` overview/sidecar | Responses native typed/raw preserve | OpenAI Responses request model/inbound/outbound | pending TDD | pending | pending | pending | Split children. |
| G7 Responses reasoning | S15 Responses `reasoning.context` | Responses native preserve | OpenAI Responses request model/inbound/outbound | pending TDD | pending | pending | pending | Cross-protocol unsupported/lossy. |
| G7 Responses reasoning | S16 Responses deprecated `reasoning.generate_summary` | Deprecated compatibility | OpenAI Responses request model/inbound/outbound | pending TDD | pending | pending | pending | Separate from `summary`. |
| G7 Responses reasoning | S17 Responses reasoning item `content[]` / `reasoning_text` | Response item support/raw preserve | OpenAI Responses response/stream model | pending TDD | pending | pending | pending | Do not collapse to Anthropic thinking. |
| G7 Responses reasoning | S18 Responses reasoning stream events | Stream fidelity | OpenAI Responses stream handling | pending TDD | pending | pending | pending | Stream option != stream event. |
| G7 Responses reasoning | S19 Unknown nested reasoning/future variants | Raw preserve or diagnostic | OpenAI Responses request/response/stream | pending TDD | pending | pending | pending | Never silently drop unknown variants. |

## Failed Gates

| Gate | Finding | Evidence | Route | Owner slice | Status |
|---|---|---|---|---|---|
|  |  |  |  |  |  |

## Review Findings

| Finding | Axis | Evidence | Owner slice | Route | Status |
|---|---|---|---|---|---|
|  |  |  |  |  |  |

## Current checkpoint

Latest phase: S0 code-level verification and module review passed on all required axes. Runtime validation is explicitly excluded by user instruction.

Next phase: begin G1/S1 Chat `n` verification design with TDD.

## Goal Mode Contract

Goal status: active.

Goal exit is forbidden until all of the following are true and verified from current state:

1. Code-level completion:
   - all implementable protocol-transformer gaps in the backlog are fixed in code, or explicitly reclassified with evidence as lossy/unsupported/not-applicable;
   - no known protocol conversion bug from the baseline docs remains open without an owner slice.
2. Test/fixture completion:
   - every completed slice has targeted tests or documented non-applicability;
   - high-priority fixture-only rows are covered or explicitly deferred with risk.
3. Slice quality loop completion:
   - every small slice passes TDD verification;
   - every small slice passes self-review before the next slice starts.
4. Module review completion:
   - after a coherent group of slices closes, run multiple review sub-agents using the same model class as the main session;
   - review axes must include bugs, protocol/document conformance, architecture, code structure, maintainability, dead code, useless code, and over-broad abstractions;
   - if review fails, route the finding back to TDD / diagnosing-bugs / architecture planning for the smallest responsible slice.
5. Parent review completion:
   - after all module groups close, run parent-level architecture and compliance review;
   - no accepted parent review finding may remain open.
6. Durable knowledge and commits:
   - update Trellis/spec docs for durable protocol rules;
   - make local commits by slice/module as appropriate;
   - do not push unless explicitly authorized.
7. No forced lossy conversion:
   - fields classified as lossy/unsupported must not be fake-mapped into unrelated target fields.

MCP fallback rule:

- Prefer codebase-memory MCP graph tools for code discovery.
- If MCP transport/tooling is unavailable or stale, use:
  `/Users/asuan/.local/bin/codebase-memory-mcp cli <tool> '<json>'`
- If that CLI is also unavailable, use read-only `rg` / `sed` / `python` as fallback and record the fallback in the slice evidence.

Loop hierarchy:

```text
Parent goal
  -> module / big task group
      -> small vertical slice
          -> TDD
          -> implementation
          -> targeted verification
          -> slice self-review
          -> pass?
              no  -> TDD / diagnosing-bugs / architecture planning for same slice
              yes -> next slice or module review
      -> module multi-agent review
          -> pass?
              no  -> smallest responsible slice
              yes -> next module
  -> parent multi-agent/architecture review
  -> spec update
  -> local commit / finish-work
```
