# HPF Module Re-Review — high-priority fixtures / matrix sync

| Field | Value |
|---|---|
| **Conclusion** | **PASS** |
| **Agent** | HPF module re-review sub-agent |
| **Fix commit** | `7e07df39986083084df013377dcdad3079cf18be` (`docs(protocols): fix HPF matrix §5 elevation and draft sync`) |
| **Baseline commit** | `c8718b9a91f3cf341d260b075ec6514873ce585e` |
| **Branch** | `codex-transformer-field-fixes` |
| **HEAD** | `7e07df39` |
| **Scope** | docs/evidence consistency only; no code changes |
| **Date** | 2026-07-12 |
| **Prior review** | `research/reviews/hpf-module-review.md` → **FAIL** (4 majors) |

## Design intent check

| Intent | Result | Note |
|---|---|---|
| 1. No new protocol feature | **PASS** | Fix commit is docs/task metadata only. |
| 2. Matrix/spec evidence for G1–G7 | **PASS** | §5 authoritative rows elevated; §9 is index aligned to §5, not overlay-only. |
| 3. Residual fixture-only explicit, not faked complete | **PASS** | §0 / §9.3 / residual-gaps keep whole matrix **INCOMPLETE**; no 101-row CONFIRMED claim. |
| 4. Parent final audit input usable | **PASS** | Majors from prior FAIL closed; remaining items are minors / residual fixture-only. |

## Prior majors disposition

| # | Prior major | Status | Evidence |
|---|---|---|---|
| 1 | §5 G1–G7 still `UNCHECKED`; §9 only overlay | **CLOSED** | §5 rows updated to `PARTIAL` + Code/Test; §9.2 states §5 is authority |
| 2 | §0 stale (“only namespace closed”) | **CLOSED** | §0 rewritten: G1–G7 same-protocol closed + residual non-claims + overall INCOMPLETE |
| 3 | batch drafts not synced | **CLOSED** | All 5 `docs/specs/protocols/drafts/batch-*.md` have HPF sync pointer section |
| 4 | `task.json` completed + `commit: pending` | **CLOSED** | `status=in_progress`; commit points to base + re-review pending (not fake completed) |

## Checklist: 4 required majors

### 1. 主矩阵 §5 G1–G7 已从 UNCHECKED 提升并写入 Code/Test Evidence；§9 不是仅 overlay

**PASS**

权威文件：`docs/specs/protocols/protocol-conversion-strict-verification-matrix.md`

| Row ID | §5 line | Status | Code Evidence (excerpt) | Test Evidence (excerpt) |
|---|---:|---|---|---|
| `RSP.TOP.reasoning` | 454 | PARTIAL | `llm/transformer/openai/responses` reasoning handlers | `reasoning_context_test.go` / `reasoning_g7_test.go` + stream/aggregator tests |
| `CHAT.TOP.audio` | 475 | PARTIAL | `llm/transformer/openai/chat_n.go` | `chat_n_test.go` output-controls |
| `CHAT.TOP.moderation` | 482 | PARTIAL | `chat_n.go` | output-controls |
| `CHAT.TOP.n` | 483 | PARTIAL | `chat_n.go` | `TestOpenAIChatRequestN*` |
| `CHAT.TOP.prediction` | 485 | PARTIAL | `chat_n.go` | output-controls |
| `CHAT.TOP.prompt_cache_retention` | 488 | PARTIAL | `chat_n.go` | prompt_cache_retention tests |
| `CHAT.TOP.web_search_options` | 503 | PARTIAL | `chat_n.go` | chat_n_test + anthropic lossy notes |
| `CHAT.TOP.function_call` | 504 | PARTIAL | deprecated functions path | `chat_deprecated_functions_test.go` |
| `CHAT.TOP.functions` | 505 | PARTIAL | deprecated functions path | `chat_deprecated_functions_test.go` |
| `ANT.TOP.container` | 517 | PARTIAL | anthropic model/metadata | `container_inference_geo_test.go` |
| `ANT.TOP.inference_geo` | 518 | PARTIAL | anthropic model/metadata | `container_inference_geo_test.go` |
| `ANT.TOP.mcp_servers` | 531 | PARTIAL | MCP connector path | `mcp_connector_test.go` |
| `ANT.TOOL.mcp_toolset` | 547 | PARTIAL | MCP toolset path | `mcp_connector_test.go` order test |

核对：上述 13 行在 §5 主表（19-col）中 **均无** `UNCHECKED` 残留于 Code/Test/Status 关键。

§9 对齐声明（L885–891）：

- “§5 主表中对应 G1–G7 行已于 2026-07-12 同步更新为 `PARTIAL`，并写入 Code/Test Evidence 路径”
- “§9.1 是同一批证据的紧凑索引，**不是**替代主表的‘仅 overlay’”
- “**§5 行状态为准**”

§0 权威阅读顺序（L22）：

- “§5 行是主状态；§9 是 G1–G7 实现证据索引（与 §5 对齐，不是‘仅附录、不提升主表’）”

§9.1（L864–881）列出同一批 G1–G7 路径索引，与 §5 不冲突。

### 2. §0 当前结论已改写

**PASS**

`protocol-conversion-strict-verification-matrix.md` L8–22：

1. 整体仍 **INCOMPLETE**
2. Same-protocol 已闭环：Chat G1/G2/G4/G5；Anthropic G3/G6；Responses G7
3. 历史 namespace 闭环保留为 P1/Codex 叙事，不抬成全矩阵完成
4. **未声称**：101 行全 `CONFIRMED`、全方向跨协议语义等价、Codex P1 = 公共 P0、residual 已实现为 feature
5. Residual 指向 §9.3 + `residual-gaps.md`

与 prior FAIL 中“只确认 namespace”叙事已脱钩。

### 3. batch drafts 已同步证据指针

**PASS**（按“挂接指针 / 不重打分”策略）

五个 drafts 均含同一 HPF 段 `## 2026-07-12 Implementation Sync Pointer (HPF)`：

| File | HPF section start |
|---|---:|
| `docs/specs/protocols/drafts/batch-common-fields.md` | L373 |
| `docs/specs/protocols/drafts/batch-limits-state-cache.md` | L349 |
| `docs/specs/protocols/drafts/batch-output-message-content.md` | L551 |
| `docs/specs/protocols/drafts/batch-reasoning-stream.md` | L618 |
| `docs/specs/protocols/drafts/batch-tools-mcp.md` | L379 |

每段均：

- 声明 draft inventory **未**重新打分
- 指向主矩阵 §0/§5/§9、residual-gaps、guidelines Field Evidence Index、parent-final-audit-input
- **Claims allowed**: same-protocol G1–G7 closed with code+tests
- **Claims forbidden**: full 101-row `CONFIRMED` / full cross-protocol equivalence / Codex P1 as public P0

这关闭 prior major #3（PRD 要求 sync batch drafts；先前 commit 完全未触达）。

### 4. task.json 不再虚假 completed + commit:pending

**PASS**

`.trellis/tasks/07-10-protocol-high-priority-fixtures-matrix-sync/task.json`：

- `status`: `in_progress`（允许，等待 re-review）
- `commit`: `c8718b9a91f3cf341d260b075ec6514873ce585e (base); HPF majors fix pending re-review`
- `notes`: `HPF module review FAIL majors fixed in docs; awaiting re-review PASS before archive`

不再是 prior 的 `completed` + `commit: "pending"` 组合。

`hpf-slice-ledger.md` S5 仍 `in_progress`（re-review 路径），S2 已改为 “§5 elevated rows + §9 index”。

## Evidence path existence (code/tests referenced by elevated rows)

| Path | Exists |
|---|---|
| `llm/transformer/openai/chat_n.go` | yes |
| `llm/transformer/openai/chat_n_test.go` | yes |
| `llm/transformer/openai/chat_deprecated_functions_test.go` | yes |
| `llm/transformer/anthropic/container_inference_geo_test.go` | yes |
| `llm/transformer/anthropic/mcp_connector_test.go` | yes |
| `llm/transformer/openai/responses/reasoning_context_test.go` | yes |
| `llm/transformer/openai/responses/reasoning_g7_test.go` | yes |

（本复审以文档一致性为主；路径存在性已核对。未重跑 go test——非本轮关闭条件。）

## Review dimensions

### 协议一致性

**PASS**

- Cross-protocol 策略在 elevated 行中保持 no-synth / LossyDowngrade / 禁止伪桥（MCP ≠ namespace；reasoning ≠ thinking/`reasoning_effort`；web_search_options ≠ Responses `web_search` tool）。
- 未把 same-protocol PARTIAL 抬成跨协议 CONFIRMED。

### 文档内部一致性

**PASS with minors**

- §0 / §5 / §9 / residual-gaps / ledger 对 G1–G7 权威位置一致：§5 主状态，§9 索引。
- 整体 INCOMPLETE 与 “未声称 101 CONFIRMED” 贯穿主矩阵与 drafts HPF 段。
- Minor：draft 正文 Round 3/4 历史段落仍含过时表述（例如 `batch-common-fields.md` 仍写 `n` “no active storage / implementation-gap”）。HPF 段已明确 draft 未重打分、以主矩阵为准，故 **不回开 major**；维护时勿单独信任 Round 4 正文。

### 证据可追溯

**PASS**

- Elevated 行含 module 标签（G1–G7）、code path、test path/notes。
- §9.1 与 §5 可交叉检索。
- residual-gaps 增加 “Matrix authority note (2026-07-12 HPF fix)”。

### 是否引入虚假完成

**PASS — 未引入**

明确 **不得** / **未** 声称：

- 101 行全 `CONFIRMED`
- 全跨协议语义等价
- Codex P1 = 公共 P0
- residual fixture-only 已实现为 feature

矩阵标题与规则仍是 **INCOMPLETE**。namespace 历史闭环仍标 PARTIAL（Responses↔Chat confirmed；Anthropic unchecked）。

### 可维护性

**PASS with minors**

- 阅读顺序写清，降低 “只读 §9 / 只读 draft Round4” 误读风险。
- Minor：`task.json.commit` 仍以 `c8718b9a` 为 base 叙述，未写 `7e07df39` 为 majors-fix commit（notes 已说明 awaiting re-review；可在 archive 时改为最终 commit）。
- Minor：`completedAt: 2026-07-12` 与 `status: in_progress` 并存，略脏，不构成虚假 completed（status 字段为准）。
- Minor：G7 子路径 ID（`RSP.TOP.reasoning.context` 等）与 `CHAT.MSG.function_call` 主要出现在 §9.1，§5/§6 以父行 `RSP.TOP.reasoning` / request deprecated fields 承载；对 parent 审计可接受，若未来做子行矩阵需再拆。

## Findings

### Blockers

None.

### Majors

None open. Prior 4 majors **all closed**.

### Minors

1. **Draft Round 4 body stale vs elevated matrix**  
   Example: `batch-common-fields.md` L240/L260/L281/L298 still describe Chat `n` as missing storage. Mitigated by HPF pointer + “draft not re-scored”. Parent readers must follow pointer to §5.

2. **task.json metadata hygiene**  
   `completedAt` set while `status=in_progress`; commit string omits `7e07df39`. Safe for re-review gate; clean on archive.

3. **§9 nested IDs not mirrored as independent §5/§6 rows**  
   G7 sub-seams and response `message.function_call` live in §9.1; parent rows elevated. Acceptable for this task scope.

4. **§9.1 test globs remain slightly approximate**  
   e.g. `TestOpenAIChatRequestN*` / “output-controls” / “stream tests” — files exist; naming loose, not false.

## Residual (accepted non-blocking)

From `research/residual-gaps.md` (still fixture-only / intentional non-goals):

- token-limit multi-direction fixture expansion  
- Responses namespace / Codex P1 catalog  
- full SSE tool/audio family parity  
- Anthropic thinking multi-block ordering edge fixtures  
- Chat custom tool source gap  

Not presented as G1–G7 feature completion.

## Verdict

**PASS**

Fix commit `7e07df39` closes all four prior documentation majors without inventing features or claiming whole-matrix completion.

Parent may treat this as:

- §5-authoritative G1–G7 same-protocol evidence elevation  
- honest residual list  
- drafts linked, not re-audited  

**Must not** treat as 101-row full CONFIRMED or full cross-protocol equivalence.

## Recommendation to parent

1. Accept HPF docs gate as re-review **PASS**.  
2. On archive: set `task.json.status=completed`, `commit=7e07df39` (or final range including it), clear/adjust `completedAt` consistently.  
3. Optional follow-up (not blocking): annotate or strike stale Round 4 “implementation-gap” sentences inside drafts that contradict elevated §5 rows.
