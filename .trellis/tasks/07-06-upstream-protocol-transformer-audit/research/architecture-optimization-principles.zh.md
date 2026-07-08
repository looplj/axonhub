# 架构优化底层逻辑：协议字段归属与 deep module 判断

## 目标

本任务不是重写 AxonHub transformer，而是在保证功能完好的前提下，让协议转换架构更清晰、更好维护、更容易测试，避免为了补字段继续堆烂代码、绕远路、魔法 metadata 和 provider-specific patch。

## 外部参考原则

- Deep module 原则：module 应该用较小 interface 承载较多 implementation，避免 shallow module 让调用者知道太多细节。参考：`https://softengbook.org/articles/deep-modules`。
- `module / interface / depth / seam / adapter / leverage / locality` 词汇：优先寻找能提升 locality 和 leverage 的 seam，而不是随手加类。参考：`https://github.com/mattpocock/skills/blob/main/skills/engineering/codebase-design/SKILL.md`。
- Deepening 原则：先判断删除一个 module 会不会让复杂度扩散；如果只是搬运复杂度，就是浅层；如果能集中复杂度，就是值得加深。参考：`https://github.com/mattpocock/skills/blob/main/skills/engineering/codebase-design/DEEPENING.md`。
- 过度架构风险：不要为了“看起来干净”引入过多层、样板和 mapping；只有复杂度真实存在且需要被隐藏时才加抽象。参考：`https://github.com/sairyss/domain-driven-hexagon`。

## 对 AxonHub 的判断

作者的大框架合理：

```text
协议 HTTP 入站
  -> inbound transformer
  -> llm.Request / llm.Response 作为 CrossProtocolCanonical
  -> outbound transformer
  -> provider HTTP 出站
```

问题不是框架错，而是字段归属和 preservation seam 不够深：

- `llm.Request` 容易被误用成所有协议字段的公共桶。
- `TransformerMetadata` 容易变成 magic key 垃圾桶。
- `ProviderExtensions` 容易承载过多职责。
- raw fallback、native struct、provider-specific adapter、cross-protocol diagnostic 的责任边界需要明确。
- 很多 provider/channel 虽然类名不同，但底层实际复用 OpenAI Chat、OpenAI Responses 或 Anthropic Messages 这些协议族；因此不能只按 provider 名字补字段，必须先按协议族和字段语义分类。

## 是否要新增“转换类 / 转换 module”的判断规则

不要因为字段多就先加类。只有满足下面条件时，才新增 module / seam：

1. **复杂度会复用**：至少两个 adapter 或多个调用点需要同一套转换/保真规则。
2. **删除测试成立**：如果删除这个 module，复杂度会扩散到多个 transformer 文件，而不是简单少一层 wrapper。
3. **interface 小于 implementation**：调用方只需要表达“保真/降级/发射”意图，不需要知道所有字段细节。
4. **locality 提升**：字段新增、字段丢失、diagnostic 规则能集中在一个地方修改。
5. **leverage 提升**：一个测试 surface 能覆盖多个 provider/channel 或多个字段族。
6. **不会变成 generic dumping ground**：module 有明确字段 owner，不接收所有未知问题。

如果不满足这些条件，不新增类；直接在现有 protocol package 内做小而清晰的函数/struct 即可。

## 当前建议的架构层级

### 1. CrossProtocolCanonical

位置：`llm.Request` / `llm.Response`。

只放跨协议稳定等价字段，例如：

- model
- common messages/content
- common sampling fields
- common function tools
- common usage/response fields

禁止放：

- OpenAI Responses-only 字段
- Codex Responses lazy-loading 字段
- Anthropic companion 字段
- provider 私有字段
- stream event 字段

### 2. Protocol Native Preservation

位置：各协议 transformer package。

协议原生字段应优先落这里：

- OpenAI Responses: `llm/transformer/openai/responses`
- OpenAI Chat: `llm/transformer/openai`
- Anthropic Messages: `llm/transformer/anthropic`

这层负责 same-protocol preservation。

### 3. Provider/Channel Adapter

位置：provider-specific transformer package，例如：

- `openrouter`
- `zai`
- `deepseek`
- `moonshot`
- `gemini/openai`
- `openai/codex`
- `openai/copilot`
- `anthropic/claudecode`

这些 adapter 可以有 provider-specific policy，但不能自己重新定义协议字段归属。

### 4. Raw Fallback

只用于 same-protocol unknown/future fields。

禁止跨协议静默 replay。

### 5. LossyDowngrade Diagnostic

用于 cross-protocol 无法表达的字段。

它只记录损失，不存 native 字段，不做假语义映射。

### 6. Stream Event Fidelity

stream event 单独处理，不塞 request/response body model。

## 三个协议族的初步字段归属原则

| 协议族 | 字段例子 | 正确归属 | 不应放入 |
|---|---|---|---|
| OpenAI Responses | `conversation`, `context_management`, `prompt` | Responses native request | `llm.Request` |
| OpenAI Responses / Codex profile | `tool_search`, `defer_loading`, `additional_tools`, `namespace` | Responses native preservation / CodexResponsesProfile | Chat builder / Anthropic adapter |
| OpenAI Responses server-side MCP | `tools[].type=mcp`, `response.mcp_*` | Responses native tool/stream fidelity | Anthropic MCP connector |
| OpenAI Chat | `web_search_options`, `prediction`, top-level `audio` | Chat native + emission policy | Shared provider builder without gating |
| Anthropic Messages | `container`, `inference_geo` | Anthropic native | OpenAI common structures |
| Anthropic MCP connector | `mcp_servers`, `mcp_toolset` | Anthropic companion/native | OpenAI Responses MCP |
| Stream events | `response.reasoning_*`, `message_delta`, `thinking_delta` | Stream fidelity module | Request body model |

## 当前最小架构优化路线

1. 不新增全局“万能转换类”。
2. 先在 `llm/transformer/openai/responses` 内加深 Responses native preservation seam。
3. 只当多个 provider/channel 共享同一问题时，再提取小型 policy module。
4. OpenAI-compatible Chat 因为多个 provider 复用 `openai.RequestFromLLM`，后续可能需要 Chat emission policy seam。
5. Anthropic 先保持在 Anthropic native package 内，不和 OpenAI MCP 混。
6. stream event fidelity 后续独立做，不进入 request preservation 第一轮。

## 实施前硬门槛

开始写业务代码前，必须完成最终字段归属矩阵：

```text
字段
协议族
字段类别
作者当前位置
当前问题
正确归属
same-protocol 行为
cross-protocol 行为
是否属于第一轮实现
```

没有这个矩阵，不进入 P1a TDD。
