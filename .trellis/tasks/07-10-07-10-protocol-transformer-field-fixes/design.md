# Design — Protocol Transformer Gap Remediation

## Architecture Constraints

保留既有链路：

```text
source protocol adapter -> llm common model + provider extensions -> target protocol adapter
```

字段只能按以下分类落位：

| Class | Owner / rule |
|---|---|
| `common-abstraction` | 仅稳定跨协议语义，落 `llm.Request` / `llm.Response` / `llm.Message` 等。 |
| `native-field` | 目标协议 native struct。 |
| `adapter-specific` | provider adapter 或 named provider extension。 |
| `raw-preserve` | named raw sidecar；仅同协议 family re-emit。 |
| `lossy-conversion` | target outbound adapter 写 diagnostic。 |
| `unsupported/absent` | 不 fake-map；明确记录。 |
| `deprecated-compat` | 独立兼容路径，不能污染现代字段。 |

## Global Data-Flow Rules

1. same-protocol round-trip 优先于 cross-protocol conversion。
2. request、response item、usage、stream option、stream event 必须分开建模和测试。
3. object / array / typed union 必须按子字段 / element / variant 拆行。
4. `TransformerMetadata` 只存 bridge hint，不得成为 protocol body 字段垃圾桶。
5. `ProviderExtensions` / raw sidecar 必须有 named owner。
6. OpenAI Responses namespace、Codex app/sub-agent namespace 是 P1 usage profile；不得扩大为 P0 公共语义。
7. OpenAI Responses MCP tool 与 Anthropic MCP connector 不是同一个抽象。

## Module Order

| Order | Task | Reason |
|---:|---|---|
| Slice 0 | `07-09-fix-anthropic-opus-adaptive-thinking` | 已实现的线上 bug，必须先隔离收尾。 |
| 1 | `07-10-protocol-chat-n` | 小而明确，验证任务循环。 |
| 2 | `07-10-protocol-openai-cache-state` | 同 provider cache/state 保真，风险可控。 |
| 3 | `07-10-protocol-anthropic-container-geo` | Anthropic native/raw preservation。 |
| 4 | `07-10-protocol-chat-output-controls` | Chat top-level output controls，必须和 message/tool 语义隔离。 |
| 5 | `07-10-protocol-chat-web-search-options` | Chat native request option，不能和 hosted tool 混淆。 |
| 6 | `07-10-protocol-chat-deprecated-functions` | deprecated compatibility，需与现代 tools 分离。 |
| 7 | `07-10-protocol-anthropic-mcp-connector` | adapter-specific MCP connector。 |
| 8 | `07-10-protocol-responses-reasoning-stream` | 最大/最高风险模块；内部继续拆 5 slice。 |
| 9 | `07-10-protocol-high-priority-fixtures-matrix-sync` | 不引入新 feature，只补 fixture 与同步证据。 |

## Slice Quality Gate

```text
context -> TDD red/fixture proof -> minimal green -> local refactor
-> targeted test + diff check + baseline self-review
-> slice report -> module review
```

模块审查至少覆盖：bug、协议一致性、架构/可维护性。任意 fail：

```text
bug -> TDD / diagnosis
protocol mismatch -> diagnosis + TDD
architecture drift -> architecture planning + minimum repair
```

未通过不得推进下一个模块。

