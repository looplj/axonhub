# Git comparison evidence

Generated: 2026-07-06T17:56:28+08:00
Current repo: /Users/asuan/项目/AI/axonhub
Upstream clone: /tmp/axonhub-upstream-20260706-175405
Current branch: codex-transformer-field-fixes
Current HEAD: c798c6e9e206e4d6ca44cae49c3d586e4e23c962
Upstream remote HEAD: 97c9351a23df5a3c302cf1c35bf5ca39caf7208f
Merge base: 6831e03ce7cf1efbc3eb4d2e2eb84bf0cb1722a3

## Local commits not in upstream latest
c798c6e9 feat: preserve OpenAI Responses native request fields
915962a8 docs: define OpenAI Responses native round-trip plan
8a3c8090 docs: archive OpenAI Codex Responses references
eab76a20 Merge remote-tracking branch 'origin/unstable' into codex-transformer-field-fixes
02593b7f fix(transformer): 协议转换字段丢失修复 — 7类24处
994b005c merge: 合并上游 origin/unstable (S3优化/Ollama URL/OpenCode配额/modelMappings/models端点)
9bd45b78 refactor(transformer): 提取 OpenAI 兼容渠道共用 chat 请求 builder
1eb4567c refactor(transformer): 裸字符串 metadata key 全部补常量,消除手打拼写隐患
9a97ec44 docs(specs): 归档跨协议元数据传播收尾(openrouter/nanogpt/gemini-openai + responses 重构 + copilot)
b762f877 fix(transformer): copilot chat 出站补元数据传播(PropagateRequestMetadata + MergeResponseMetadata)
3f377e1c refactor(transformer): responses 出站统一改用 shared 传播 helper,消除内联孤例
aa3bcfed fix(transformer): 补齐 openrouter/nanogpt/gemini-openai 跨协议元数据传播缺口 + 跨协议往返测试
b6ffc6eb docs(specs): 跨协议修复验收 APPROVED(Faraday REJECTED→补 5 出口→Rawls APPROVED)
4628e538 fix(transformer): 补齐 5 个独立构建出口的 PropagateRequestMetadata(deepseek/moonshot/zai/doubao/openrouter)
9e3905b2 fix(transformer): 流式跨协议元数据传播 — shared.PropagateStreamMetadata 包装三个出口流
a1b10836 fix(transformer): 跨协议元数据传播 — chat/anthropic/gemini 出口补 PropagateRequestMetadata + MergeResponseMetadata
7d6db1c7 docs(specs): 总交接 handoff — 全 7 切片 23 项结清(14 修复 + 4 不修)
397c7bce fix(transformer): #1a/D1 namespace 工具组经 TransformerMetadata 映射表往返闭合(非流式+流式)
7e32bf05 docs(specs): ζ 切片 F2 stream_options 复核无 bug 不修
72d07e00 fix(transformer): #11 chat/responses 顶层 cache_control 经 shared key + json.RawMessage 三方统一透传往返闭合
13aab1cc docs(specs): #10 session_id body 变体设计性不修文档化
e12f1fc3 fix(transformer): #13 user 跨 anthropic 边界单向兜底桥接,身份不再跨格式泄漏
6173e8c6 docs(specs): δ 切片收尾 handoff + 协议转换 bug 审计总报告
6bf3b379 fix(transformer): #4 chat reasoning 对象 + responses reasoning.enabled 经 canonical 三槽/TransformerMetadata 透传往返闭合
f1bff385 docs(specs): δ-5 #5 thinking utils.go:34 复核结论——无 bug 不修
0e8f0120 fix(transformer): #6 anthropic output_config format/task_budget 经 TransformerMetadata 透传
d9fe8bed fix(transformer): F21 anthropic context_management 经 TransformerMetadata 透传往返闭合
b1698131 fix(transformer): F19 responses prompt 存储模板引用经 TransformerMetadata 透传往返闭合
7aeadd9f fix(transformer): C7/C8/C9 rep_penalty/min_p/top_a chat 往返闭合,OpenRouter 扩展采样旋钮不丢
9b0a2688 fix(transformer): C3 top_k 三方对称化,shared 中性 key + chat/responses 加 TopK 字段
3a57d0ac fix(transformer): C13 logit_bias 放宽 map[string]float64,修复浮点值解码中断
69a733cb docs(specs): β 切片收尾——#1b 按作者设计不修,β-1/β-2 已验收
6dc2ff42 fix(transformer): #1c/D11 非流式 custom_tool_call 补 Namespace 字段往返闭合
d8a4479c docs(specs): 回补切片 α/β PRD(B 折中纪律:每原子 Problem/Solution/Testing/Out-of-scope/Status)
2a06d2a9 fix(transformer): #3/#1e anthropic parallel_tool_calls ↔ disable_parallel_tool_use 极性映射
98fb0a16 fix(transformer): D12 流式 custom_tool_call 补 Namespace 字段往返闭合
46503802 docs(specs): 归档三协议字段审计底稿并锁定 F8-F20/#7verbosity 业务决议
812c9077 fix(transformer): 协议转换丢字段修复(21处,Codex审查发现)

## Upstream commits not in current branch
97c9351a ci: publish Helm chart to GHCR --issue=#1965 (#1966)
3da59d10 chore: sync model developers data (#1964)
e412fab1 feat: add codex headers (#1963)
84344a5f feat: show reasoning token usage (#1960)
91b0f39a chore: remove the test source filter for request, close #1913 (#1957)
07d464fc feat: ip access control (#1956)
0ad31228 fix(httpclient): cap upstream error response body at 1 MB (#1955)

## Committed diff stat: origin/unstable..HEAD
 .agent/rules/spec-audit-method.md                  |    26 +
 .../handoff-round2-field-audit-2026-06-30.md       |    90 +
 .github/workflows/helm-chart.yml                   |    54 -
 AGENTS.md                                          |     1 +
 CONTEXT.md                                         |    27 +
 cmd/schema/go.mod                                  |    14 +-
 cmd/schema/go.sum                                  |    28 +-
 config.example.yml                                 |    10 -
 deploy/helm/README.md                              |     2 +-
 deploy/helm/values-production.yaml                 |     3 +-
 deploy/helm/values.yaml                            |     3 +-
 ...eparate-openai-responses-native-preservation.md |    15 +
 docs/audit-bug-report.md                           |    44 +
 docs/fix-tracker.md                                |    48 +
 docs/handoff-final.md                              |    58 +
 "docs/handoff-\316\264.md"                         |    53 +
 docs/specs/_parts/part-A-tools.md                  |    40 +
 docs/specs/_parts/part-B-content.md                |    48 +
 docs/specs/_parts/part-C-sampling.md               |    71 +
 docs/specs/_parts/part-D-reasoning-cache.md        |    84 +
 docs/specs/_parts/part-F-stream-platform.md        |   102 +
 docs/specs/audit-anthropic-messages.md             |    84 +
 docs/specs/audit-chat-completions.md               |   112 +
 docs/specs/audit-openai-responses.md               |    84 +
 docs/specs/field-fate-matrix.md                    |    67 +
 docs/specs/field-matrix-core.md                    |    41 +
 docs/specs/field-matrix-full.md                    |   127 +
 docs/specs/master-conversion-table.md              |   258 +
 ...openai-responses-native-field-classification.md |   479 +
 .../specs/openai-responses-native-roundtrip-prd.md |   101 +
 .../openrouter-chat-messages-responses.min.yaml    | 14808 ++++++++
 docs/specs/openrouter-openapi.yaml                 | 34171 +++++++++++++++++++
 docs/specs/openrouter-protocol-baseline.md         |   417 +
 docs/specs/slices/alpha-prd.md                     |    11 +
 docs/specs/slices/beta-prd.md                      |    23 +
 docs/specs/slices/delta-prd.md                     |    64 +
 docs/specs/slices/epsilon-prd.md                   |    42 +
 docs/specs/slices/eta-prd.md                       |    45 +
 docs/specs/slices/gamma-prd.md                     |    36 +
 ...ai-responses-native-request-roundtrip-issues.md |   292 +
 docs/specs/slices/zeta-prd.md                      |    17 +
 docs/specs/vendor/openai/api/conversation-state.md |     5 +
 docs/specs/vendor/openai/api/function-calling.md   |     7 +
 .../vendor/openai/api/migrate-to-responses.md      |    45 +
 .../openai/api/responses-create-reference.md       |   268 +
 .../specs/vendor/openai/api/streaming-responses.md |     1 +
 .../vendor/openai/api/tools-connectors-mcp.md      |   101 +
 docs/specs/vendor/openai/api/tools-tool-search.md  |     1 +
 docs/specs/vendor/openai/codex-source/README.md    |    19 +
 .../codex-source/codex-rs/codex-api/src/common.rs  |   335 +
 .../codex-source/codex-rs/core/src/client.rs       |  2402 ++
 .../codex-rs/core/src/mcp_tool_exposure.rs         |   100 +
 .../codex-rs/core/src/tool_search_handler.rs       |   350 +
 .../codex-rs/core/src/tool_search_spec.rs          |   113 +
 .../codex-source/codex-rs/protocol/src/models.rs   |  3660 ++
 .../codex-rs/tools/src/responses_api.rs            |   140 +
 .../codex-rs/tools/src/tool_discovery.rs           |   150 +
 .../codex-source/codex-rs/tools/src/tool_search.rs |   136 +
 .../codex-source/codex-rs/tools/src/tool_spec.rs   |   135 +
 docs/specs/vendor/openai/codex/codex-manual.md     | 12751 +++++++
 docs/specs/vendor/openai/codex/config-basic.md     |     1 +
 docs/specs/vendor/openai/codex/config-reference.md |     1 +
 docs/specs/vendor/openai/codex/config.md           |   268 +
 docs/specs/vendor/openai/codex/mcp.md              |     1 +
 frontend/src/features/models/data/providers.json   |   773 +-
 .../requests/components/data-table-toolbar.tsx     |     5 +-
 .../requests/components/request-detail-content.tsx |    28 +-
 .../requests/components/requests-columns.tsx       |     7 +-
 frontend/src/features/requests/data/requests.ts    |     2 -
 frontend/src/locales/en/requests.json              |     1 -
 frontend/src/locales/zh-CN/requests.json           |     1 -
 frontend/src/routeTree.gen.ts                      |    23 -
 go.mod                                             |    20 +-
 go.sum                                             |    28 +-
 internal/server/config.go                          |    15 +-
 internal/server/middleware/ip_access_control.go    |    88 -
 .../server/middleware/ip_access_control_test.go    |   239 -
 internal/server/orchestrator/prompt.go             |    26 +
 internal/server/orchestrator/prompt_test.go        |    52 +
 internal/server/routes.go                          |     7 -
 llm/completion.go                                  |     2 +-
 llm/go.mod                                         |    10 +-
 llm/go.sum                                         |    20 +-
 llm/httpclient/client.go                           |    28 +-
 llm/model.go                                       |    21 +-
 llm/provider_extensions.go                         |    89 +-
 llm/tools.go                                       |    21 +
 llm/transformer/anthropic/cache_control_test.go    |    10 +-
 .../anthropic/context_management_test.go           |    99 +
 llm/transformer/anthropic/inbound_convert.go       |   151 +-
 llm/transformer/anthropic/inbound_test.go          |   158 +
 llm/transformer/anthropic/model.go                 |    48 +-
 llm/transformer/anthropic/outbound.go              |     8 +-
 llm/transformer/anthropic/outbound_convert.go      |   215 +-
 llm/transformer/anthropic/outbound_convert_test.go |   144 +
 llm/transformer/anthropic/outbound_stream.go       |    46 +-
 .../outbound_stream_server_tool_use_test.go        |     8 +-
 llm/transformer/anthropic/outbound_test.go         |   369 +
 .../anthropic/output_config_format_test.go         |   128 +
 .../anthropic/redacted_thinking_stream_test.go     |    69 +
 llm/transformer/anthropic/service_tier_test.go     |    91 +
 llm/transformer/anthropic/top_k_test.go            |   127 +
 llm/transformer/anthropic/user_bridge_test.go      |   140 +
 llm/transformer/deepseek/outbound.go               |    24 +-
 llm/transformer/doubao/outbound.go                 |    27 +-
 llm/transformer/doubao/video_outbound.go           |     5 +-
 llm/transformer/gemini/image.go                    |     5 +-
 llm/transformer/gemini/inbound_convert.go          |     2 +-
 llm/transformer/gemini/inbound_convert_test.go     |    42 +
 llm/transformer/gemini/inbound_stream.go           |     2 +-
 llm/transformer/gemini/inbound_stream_test.go      |    62 +
 llm/transformer/gemini/openai/outbound.go          |    29 +-
 llm/transformer/gemini/outbound.go                 |    11 +-
 llm/transformer/gemini/outbound_convert.go         |    44 +-
 llm/transformer/gemini/outbound_stream.go          |     9 +-
 llm/transformer/gemini/outbound_test.go            |   113 +
 llm/transformer/moonshot/outbound.go               |    25 +-
 llm/transformer/nanogpt/outbound.go                |    12 +-
 llm/transformer/openai/aggregator.go               |    33 +
 llm/transformer/openai/aggregator_test.go          |    51 +
 llm/transformer/openai/cache_control_test.go       |    81 +
 .../openai/codex/codex_simulator_test.go           |    36 +-
 llm/transformer/openai/codex/constants.go          |     2 -
 llm/transformer/openai/codex/headers.go            |     4 -
 llm/transformer/openai/codex/outbound.go           |    29 +-
 llm/transformer/openai/completion.go               |     2 +-
 llm/transformer/openai/copilot/outbound.go         |    11 +-
 llm/transformer/openai/google.go                   |    16 +
 llm/transformer/openai/image_outbound.go           |     5 +-
 llm/transformer/openai/inbound_convert.go          |    63 +
 llm/transformer/openai/inbound_test.go             |   164 +
 llm/transformer/openai/model.go                    |    49 +-
 llm/transformer/openai/outbound.go                 |    20 +-
 llm/transformer/openai/outbound_convert.go         |    45 +-
 llm/transformer/openai/outbound_convert_test.go    |    87 +
 llm/transformer/openai/outbound_test.go            |    42 +
 llm/transformer/openai/responses/aggregator.go     |    13 +-
 .../openai/responses/aggregator_test.go            |    27 +
 .../openai/responses/cache_control_test.go         |    82 +
 .../openai/responses/compact_inbound.go            |     7 +
 .../openai/responses/compact_inbound_test.go       |    33 +
 .../openai/responses/compact_outbound.go           |     2 +-
 .../responses/cross_protocol_copilot_test.go       |   133 +
 .../openai/responses/cross_protocol_test.go        |   822 +
 llm/transformer/openai/responses/image_request.go  |     2 +-
 llm/transformer/openai/responses/inbound.go        |   206 +-
 llm/transformer/openai/responses/inbound_stream.go |    65 +-
 .../openai/responses/inbound_stream_test.go        |   179 +
 llm/transformer/openai/responses/inbound_test.go   |   382 +
 llm/transformer/openai/responses/model.go          |    77 +-
 llm/transformer/openai/responses/outbound.go       |    47 +-
 .../openai/responses/outbound_convert.go           |   131 +-
 .../openai/responses/outbound_convert_test.go      |   377 +-
 .../openai/responses/outbound_stream.go            |    30 +-
 .../openai/responses/outbound_stream_test.go       |    92 +
 llm/transformer/openai/responses/outbound_test.go  |   804 +-
 .../openai/responses/request_extensions.go         |   360 +-
 .../testdata/encrypted_content.response.json       |    69 +-
 .../testdata/encrypted_content.stream.jsonl        |    12 +-
 .../testdata/encrypted_only.response.json          |    22 +-
 .../responses/testdata/encrypted_only.stream.jsonl |     4 +-
 .../openai/responses/testdata/tool-2.response.json |    54 +-
 .../openai/responses/testdata/tool-2.stream.jsonl  |   292 +-
 llm/transformer/openai/video_outbound.go           |    11 +-
 llm/transformer/openrouter/model.go                |    27 +
 llm/transformer/openrouter/model_test.go           |    31 +
 llm/transformer/openrouter/outbound.go             |    41 +-
 llm/transformer/shared/cache.go                    |    11 +
 llm/transformer/shared/chat_request.go             |    46 +
 llm/transformer/shared/chat_request_test.go        |    40 +
 llm/transformer/shared/metadata.go                 |   100 +
 llm/transformer/shared/metadata_keys.go            |    17 +
 .../shared/responses_lossy_downgrade.go            |   115 +
 llm/transformer/shared/sampling.go                 |    19 +
 llm/transformer/zai/image.go                       |     3 +-
 llm/transformer/zai/outbound.go                    |    26 +-
 176 files changed, 80440 insertions(+), 1930 deletions(-)

## Committed diff names: origin/unstable..HEAD
A	.agent/rules/spec-audit-method.md
A	.agent/summary/handoff-round2-field-audit-2026-06-30.md
D	.github/workflows/helm-chart.yml
M	AGENTS.md
A	CONTEXT.md
M	cmd/schema/go.mod
M	cmd/schema/go.sum
M	config.example.yml
M	deploy/helm/README.md
M	deploy/helm/values-production.yaml
M	deploy/helm/values.yaml
A	docs/adr/0001-separate-openai-responses-native-preservation.md
A	docs/audit-bug-report.md
A	docs/fix-tracker.md
A	docs/handoff-final.md
A	"docs/handoff-\316\264.md"
A	docs/specs/_parts/part-A-tools.md
A	docs/specs/_parts/part-B-content.md
A	docs/specs/_parts/part-C-sampling.md
A	docs/specs/_parts/part-D-reasoning-cache.md
A	docs/specs/_parts/part-F-stream-platform.md
A	docs/specs/audit-anthropic-messages.md
A	docs/specs/audit-chat-completions.md
A	docs/specs/audit-openai-responses.md
A	docs/specs/field-fate-matrix.md
A	docs/specs/field-matrix-core.md
A	docs/specs/field-matrix-full.md
A	docs/specs/master-conversion-table.md
A	docs/specs/openai-responses-native-field-classification.md
A	docs/specs/openai-responses-native-roundtrip-prd.md
A	docs/specs/openrouter-chat-messages-responses.min.yaml
A	docs/specs/openrouter-openapi.yaml
A	docs/specs/openrouter-protocol-baseline.md
A	docs/specs/slices/alpha-prd.md
A	docs/specs/slices/beta-prd.md
A	docs/specs/slices/delta-prd.md
A	docs/specs/slices/epsilon-prd.md
A	docs/specs/slices/eta-prd.md
A	docs/specs/slices/gamma-prd.md
A	docs/specs/slices/openai-responses-native-request-roundtrip-issues.md
A	docs/specs/slices/zeta-prd.md
A	docs/specs/vendor/openai/api/conversation-state.md
A	docs/specs/vendor/openai/api/function-calling.md
A	docs/specs/vendor/openai/api/migrate-to-responses.md
A	docs/specs/vendor/openai/api/responses-create-reference.md
A	docs/specs/vendor/openai/api/streaming-responses.md
A	docs/specs/vendor/openai/api/tools-connectors-mcp.md
A	docs/specs/vendor/openai/api/tools-tool-search.md
A	docs/specs/vendor/openai/codex-source/README.md
A	docs/specs/vendor/openai/codex-source/codex-rs/codex-api/src/common.rs
A	docs/specs/vendor/openai/codex-source/codex-rs/core/src/client.rs
A	docs/specs/vendor/openai/codex-source/codex-rs/core/src/mcp_tool_exposure.rs
A	docs/specs/vendor/openai/codex-source/codex-rs/core/src/tool_search_handler.rs
A	docs/specs/vendor/openai/codex-source/codex-rs/core/src/tool_search_spec.rs
A	docs/specs/vendor/openai/codex-source/codex-rs/protocol/src/models.rs
A	docs/specs/vendor/openai/codex-source/codex-rs/tools/src/responses_api.rs
A	docs/specs/vendor/openai/codex-source/codex-rs/tools/src/tool_discovery.rs
A	docs/specs/vendor/openai/codex-source/codex-rs/tools/src/tool_search.rs
A	docs/specs/vendor/openai/codex-source/codex-rs/tools/src/tool_spec.rs
A	docs/specs/vendor/openai/codex/codex-manual.md
A	docs/specs/vendor/openai/codex/config-basic.md
A	docs/specs/vendor/openai/codex/config-reference.md
A	docs/specs/vendor/openai/codex/config.md
A	docs/specs/vendor/openai/codex/mcp.md
M	frontend/src/features/models/data/providers.json
M	frontend/src/features/requests/components/data-table-toolbar.tsx
M	frontend/src/features/requests/components/request-detail-content.tsx
M	frontend/src/features/requests/components/requests-columns.tsx
M	frontend/src/features/requests/data/requests.ts
M	frontend/src/locales/en/requests.json
M	frontend/src/locales/zh-CN/requests.json
M	frontend/src/routeTree.gen.ts
M	go.mod
M	go.sum
M	internal/server/config.go
D	internal/server/middleware/ip_access_control.go
D	internal/server/middleware/ip_access_control_test.go
M	internal/server/orchestrator/prompt.go
M	internal/server/orchestrator/prompt_test.go
M	internal/server/routes.go
M	llm/completion.go
M	llm/go.mod
M	llm/go.sum
M	llm/httpclient/client.go
M	llm/model.go
M	llm/provider_extensions.go
M	llm/tools.go
M	llm/transformer/anthropic/cache_control_test.go
A	llm/transformer/anthropic/context_management_test.go
M	llm/transformer/anthropic/inbound_convert.go
M	llm/transformer/anthropic/inbound_test.go
M	llm/transformer/anthropic/model.go
M	llm/transformer/anthropic/outbound.go
M	llm/transformer/anthropic/outbound_convert.go
M	llm/transformer/anthropic/outbound_convert_test.go
M	llm/transformer/anthropic/outbound_stream.go
M	llm/transformer/anthropic/outbound_stream_server_tool_use_test.go
M	llm/transformer/anthropic/outbound_test.go
A	llm/transformer/anthropic/output_config_format_test.go
A	llm/transformer/anthropic/redacted_thinking_stream_test.go
A	llm/transformer/anthropic/service_tier_test.go
A	llm/transformer/anthropic/top_k_test.go
A	llm/transformer/anthropic/user_bridge_test.go
M	llm/transformer/deepseek/outbound.go
M	llm/transformer/doubao/outbound.go
M	llm/transformer/doubao/video_outbound.go
M	llm/transformer/gemini/image.go
M	llm/transformer/gemini/inbound_convert.go
M	llm/transformer/gemini/inbound_convert_test.go
M	llm/transformer/gemini/inbound_stream.go
M	llm/transformer/gemini/inbound_stream_test.go
M	llm/transformer/gemini/openai/outbound.go
M	llm/transformer/gemini/outbound.go
M	llm/transformer/gemini/outbound_convert.go
M	llm/transformer/gemini/outbound_stream.go
M	llm/transformer/gemini/outbound_test.go
M	llm/transformer/moonshot/outbound.go
M	llm/transformer/nanogpt/outbound.go
M	llm/transformer/openai/aggregator.go
M	llm/transformer/openai/aggregator_test.go
A	llm/transformer/openai/cache_control_test.go
M	llm/transformer/openai/codex/codex_simulator_test.go
M	llm/transformer/openai/codex/constants.go
M	llm/transformer/openai/codex/headers.go
M	llm/transformer/openai/codex/outbound.go
M	llm/transformer/openai/completion.go
M	llm/transformer/openai/copilot/outbound.go
M	llm/transformer/openai/google.go
M	llm/transformer/openai/image_outbound.go
M	llm/transformer/openai/inbound_convert.go
M	llm/transformer/openai/inbound_test.go
M	llm/transformer/openai/model.go
M	llm/transformer/openai/outbound.go
M	llm/transformer/openai/outbound_convert.go
M	llm/transformer/openai/outbound_convert_test.go
M	llm/transformer/openai/outbound_test.go
M	llm/transformer/openai/responses/aggregator.go
M	llm/transformer/openai/responses/aggregator_test.go
A	llm/transformer/openai/responses/cache_control_test.go
M	llm/transformer/openai/responses/compact_inbound.go
M	llm/transformer/openai/responses/compact_inbound_test.go
M	llm/transformer/openai/responses/compact_outbound.go
A	llm/transformer/openai/responses/cross_protocol_copilot_test.go
A	llm/transformer/openai/responses/cross_protocol_test.go
M	llm/transformer/openai/responses/image_request.go
M	llm/transformer/openai/responses/inbound.go
M	llm/transformer/openai/responses/inbound_stream.go
M	llm/transformer/openai/responses/inbound_stream_test.go
M	llm/transformer/openai/responses/inbound_test.go
M	llm/transformer/openai/responses/model.go
M	llm/transformer/openai/responses/outbound.go
M	llm/transformer/openai/responses/outbound_convert.go
M	llm/transformer/openai/responses/outbound_convert_test.go
M	llm/transformer/openai/responses/outbound_stream.go
M	llm/transformer/openai/responses/outbound_stream_test.go
M	llm/transformer/openai/responses/outbound_test.go
M	llm/transformer/openai/responses/request_extensions.go
M	llm/transformer/openai/responses/testdata/encrypted_content.response.json
M	llm/transformer/openai/responses/testdata/encrypted_content.stream.jsonl
M	llm/transformer/openai/responses/testdata/encrypted_only.response.json
M	llm/transformer/openai/responses/testdata/encrypted_only.stream.jsonl
M	llm/transformer/openai/responses/testdata/tool-2.response.json
M	llm/transformer/openai/responses/testdata/tool-2.stream.jsonl
M	llm/transformer/openai/video_outbound.go
M	llm/transformer/openrouter/model.go
M	llm/transformer/openrouter/model_test.go
M	llm/transformer/openrouter/outbound.go
A	llm/transformer/shared/cache.go
A	llm/transformer/shared/chat_request.go
A	llm/transformer/shared/chat_request_test.go
A	llm/transformer/shared/metadata.go
A	llm/transformer/shared/metadata_keys.go
A	llm/transformer/shared/responses_lossy_downgrade.go
A	llm/transformer/shared/sampling.go
M	llm/transformer/zai/image.go
M	llm/transformer/zai/outbound.go

## Uncommitted diff stat
 llm/provider_extensions.go                         | 20 +++++---
 .../openai/responses/cross_protocol_test.go        | 58 ++++++++++++++++++++++
 llm/transformer/openai/responses/outbound_test.go  | 46 ++++++++++++++++-
 .../openai/responses/request_extensions.go         | 44 ++++++++++------
 .../shared/responses_lossy_downgrade.go            | 28 +++--------
 5 files changed, 151 insertions(+), 45 deletions(-)

## Working tree status
 M llm/provider_extensions.go
 M llm/transformer/openai/responses/cross_protocol_test.go
 M llm/transformer/openai/responses/outbound_test.go
 M llm/transformer/openai/responses/request_extensions.go
 M llm/transformer/shared/responses_lossy_downgrade.go
?? .agents/
?? .codex/
?? .trellis/
?? Dockerfile.cn
?? docker-compose.8091.yml
?? docker-compose.8092.yml
?? docker-compose.override.yml
?? docs/specs/protocols/
?? docs/specs/vendor/anthropic/
?? docs/specs/vendor/openai/official-2026-07-06/
?? docs/specs/vendor/protocol-canonical-2026-07-06/
?? llm/openai_responses_classification.go
?? llm/openai_responses_classification_test.go

## LLM committed diff names
M	llm/completion.go
M	llm/go.mod
M	llm/go.sum
M	llm/httpclient/client.go
M	llm/model.go
M	llm/provider_extensions.go
M	llm/tools.go
M	llm/transformer/anthropic/cache_control_test.go
A	llm/transformer/anthropic/context_management_test.go
M	llm/transformer/anthropic/inbound_convert.go
M	llm/transformer/anthropic/inbound_test.go
M	llm/transformer/anthropic/model.go
M	llm/transformer/anthropic/outbound.go
M	llm/transformer/anthropic/outbound_convert.go
M	llm/transformer/anthropic/outbound_convert_test.go
M	llm/transformer/anthropic/outbound_stream.go
M	llm/transformer/anthropic/outbound_stream_server_tool_use_test.go
M	llm/transformer/anthropic/outbound_test.go
A	llm/transformer/anthropic/output_config_format_test.go
A	llm/transformer/anthropic/redacted_thinking_stream_test.go
A	llm/transformer/anthropic/service_tier_test.go
A	llm/transformer/anthropic/top_k_test.go
A	llm/transformer/anthropic/user_bridge_test.go
M	llm/transformer/deepseek/outbound.go
M	llm/transformer/doubao/outbound.go
M	llm/transformer/doubao/video_outbound.go
M	llm/transformer/gemini/image.go
M	llm/transformer/gemini/inbound_convert.go
M	llm/transformer/gemini/inbound_convert_test.go
M	llm/transformer/gemini/inbound_stream.go
M	llm/transformer/gemini/inbound_stream_test.go
M	llm/transformer/gemini/openai/outbound.go
M	llm/transformer/gemini/outbound.go
M	llm/transformer/gemini/outbound_convert.go
M	llm/transformer/gemini/outbound_stream.go
M	llm/transformer/gemini/outbound_test.go
M	llm/transformer/moonshot/outbound.go
M	llm/transformer/nanogpt/outbound.go
M	llm/transformer/openai/aggregator.go
M	llm/transformer/openai/aggregator_test.go
A	llm/transformer/openai/cache_control_test.go
M	llm/transformer/openai/codex/codex_simulator_test.go
M	llm/transformer/openai/codex/constants.go
M	llm/transformer/openai/codex/headers.go
M	llm/transformer/openai/codex/outbound.go
M	llm/transformer/openai/completion.go
M	llm/transformer/openai/copilot/outbound.go
M	llm/transformer/openai/google.go
M	llm/transformer/openai/image_outbound.go
M	llm/transformer/openai/inbound_convert.go
M	llm/transformer/openai/inbound_test.go
M	llm/transformer/openai/model.go
M	llm/transformer/openai/outbound.go
M	llm/transformer/openai/outbound_convert.go
M	llm/transformer/openai/outbound_convert_test.go
M	llm/transformer/openai/outbound_test.go
M	llm/transformer/openai/responses/aggregator.go
M	llm/transformer/openai/responses/aggregator_test.go
A	llm/transformer/openai/responses/cache_control_test.go
M	llm/transformer/openai/responses/compact_inbound.go
M	llm/transformer/openai/responses/compact_inbound_test.go
M	llm/transformer/openai/responses/compact_outbound.go
A	llm/transformer/openai/responses/cross_protocol_copilot_test.go
A	llm/transformer/openai/responses/cross_protocol_test.go
M	llm/transformer/openai/responses/image_request.go
M	llm/transformer/openai/responses/inbound.go
M	llm/transformer/openai/responses/inbound_stream.go
M	llm/transformer/openai/responses/inbound_stream_test.go
M	llm/transformer/openai/responses/inbound_test.go
M	llm/transformer/openai/responses/model.go
M	llm/transformer/openai/responses/outbound.go
M	llm/transformer/openai/responses/outbound_convert.go
M	llm/transformer/openai/responses/outbound_convert_test.go
M	llm/transformer/openai/responses/outbound_stream.go
M	llm/transformer/openai/responses/outbound_stream_test.go
M	llm/transformer/openai/responses/outbound_test.go
M	llm/transformer/openai/responses/request_extensions.go
M	llm/transformer/openai/responses/testdata/encrypted_content.response.json
M	llm/transformer/openai/responses/testdata/encrypted_content.stream.jsonl
M	llm/transformer/openai/responses/testdata/encrypted_only.response.json
M	llm/transformer/openai/responses/testdata/encrypted_only.stream.jsonl
M	llm/transformer/openai/responses/testdata/tool-2.response.json
M	llm/transformer/openai/responses/testdata/tool-2.stream.jsonl
M	llm/transformer/openai/video_outbound.go
M	llm/transformer/openrouter/model.go
M	llm/transformer/openrouter/model_test.go
M	llm/transformer/openrouter/outbound.go
A	llm/transformer/shared/cache.go
A	llm/transformer/shared/chat_request.go
A	llm/transformer/shared/chat_request_test.go
A	llm/transformer/shared/metadata.go
A	llm/transformer/shared/metadata_keys.go
A	llm/transformer/shared/responses_lossy_downgrade.go
A	llm/transformer/shared/sampling.go
M	llm/transformer/zai/image.go
M	llm/transformer/zai/outbound.go

## LLM uncommitted diff names
M	llm/provider_extensions.go
M	llm/transformer/openai/responses/cross_protocol_test.go
M	llm/transformer/openai/responses/outbound_test.go
M	llm/transformer/openai/responses/request_extensions.go
M	llm/transformer/shared/responses_lossy_downgrade.go
