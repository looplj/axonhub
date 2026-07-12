# Latest Codex protocol-delta classification and remediation

## Goal

以旧 Codex 基线 `1f0566d` 与最新 `9e552e9d1` 的真实源码差异为依据，找出所有可能经过 AxonHub 的协议变化，归类到既有 G 工作组或明确的新工作组；只修复已证实的 AxonHub 转换缺口，不把 Codex 客户端本地行为伪装为协议要求。

## Confirmed baseline

- Codex 两个本地副本均已同步到 `9e552e9d1`（2026-07-11）。
- 差异范围：`1f0566d3f59298d1bb88820a0d35294f1eeb07ea..9e552e9d15ba52bed7077d5357f3e18e330f8f38`。
- 已完成的历史编号不能重用：
  - G9：Responses `stream_options` raw nested preservation；
  - G10：raw JSON clone helper；
  - G11：lossy-downgrade recorder；
  - G12：raw top-level capture helper。
- 本范围未新增 OpenAI Responses 工具声明 schema：`tool_search`、`defer_loading`、namespace、apply_patch、web search、MCP declaration 均无新的 production wire shape。
- Codex 的多 agent、approval、sandbox、MCP OAuth、WebSocket telemetry、app-event/turn lifecycle 为客户端控制面，不进入 AxonHub Responses/Chat/Anthropic transformer。

## New protocol-delta groups

| Group | Codex delta | Why it belongs here | Scope boundary |
|---|---|---|---|
| **G13** | 每个 Responses 请求始终携带 `reasoning`，并始终请求 `include: ["reasoning.encrypted_content"]`（`d2d00b663`） | Responses request reasoning/encrypted content preservation | Hub 只保真实际收到的字段；不模仿 Codex 的“永远发送”客户端策略 |
| **G14** | 仅当模型能力允许时发送 `reasoning.summary` 和 summary-delivery `stream_options`（`dffe1f02a`） | Responses reasoning-summary / stream-options compatibility | Hub 不维护 Codex model catalog；同协议保真 supplied values，不伪造 capability gate |
| **G15** | 请求 `input[]` 的 response item id 仅在具有合法 Codex 类型前缀时发送；空/无前缀 id 出站省略（`c9d52de5c`） | Responses input item identity/presence behavior | Hub 不生成或强制 Codex 前缀；必须保留已有 id、允许无 id，并审计是否意外重写 |

## Requirements

1. 对每个 G13–G15，分别审计 AxonHub 当前 `OpenAI Responses -> llm -> OpenAI Responses` 行为，使用 public transformer seam 和定向 fixture 证明实际结果。
2. G13：保留客户端实际发送的 `reasoning` 和 `include`（包括 `reasoning.encrypted_content`）；未发送时不可由 Hub 擅自注入。
3. G14：保留输入中的 `reasoning.summary` 与 `stream_options`；不得在 Hub 内复制 Codex 的 model-specific capability gate，也不得因未知模型删除已提供字段。
4. G15：同协议路径中保留非空已有 item id；没有 id 时保持没有 id；不得把 `item_*` 或 Codex 前缀表当作全局规范，不得为跨协议合成 Codex id。
5. 所有工具/MCP、approval、sandbox、OAuth、multi-agent 和 telemetry 的本次变化必须写明“不进入 Hub”的证据和理由，避免后续误桥。
6. 思考等级仅作为 G13/G14 的一个字段值域注意项：当前 Codex `ReasoningEffort` 支持自定义字符串，Hub OpenAI same-family 路径必须保持未知字符串；不把 Codex `ultra -> max` 客户端策略写成全局协议映射。
7. 每个 G 先做独立 5 分钟级别切片（TDD → 定向验证 → 自审）；每个 G 全部切片结束后才运行独立模块审查。审查失败必须回对应切片。

## Out of scope

- 不把 Codex client-only approval、MCP OAuth、sandbox、multi-agent event、WebSocket telemetry 转换成 Responses 字段。
- 不在 Hub 内维护 Codex 模型能力表或强制使用 Codex 的 item-id 前缀。
- 不因本次差异重开已完成的 G9–G12 架构清理。
- 不运行全量 lint/build，不重启服务。

## Acceptance criteria

- [x] 三份 Codex delta 研究报告中的每一项都有统一的 G13–G15 / existing / client-only 分类，且无别名冲突。
- [x] G13、G14、G15 各自有 AxonHub public-seam 现状审计；每项都有“已正确 / 测试文档缺口 / 代码缺口”结论和证据。
- [x] 任何代码改动都由该组 RED fixture 触发，并保持与该 G 的边界一致。
- [x] OpenAI same-family 未知 `reasoning.effort` 字符串测试存在并通过；不假称这证明跨协议可映射。
- [x] 所有不适用的工具/MCP/客户端控制面变化记录理由，不进入转换实现。
- [ ] 每个完成 G 的自审和独立模块审查通过；所有已修改文档、测试和代码在本地相关提交中可追溯。
  - G13/G14/G15 模块审查已通过；parent review 与 scoped commit 尚未完成，本项暂不勾选。
