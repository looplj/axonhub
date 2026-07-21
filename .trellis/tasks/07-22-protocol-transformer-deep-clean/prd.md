# PRD: Protocol transformer deep clean (S1–S7)

## Goal

把 AxonHub 协议转换栈（`llm/transformer/**` + 相关 orchestrator 协议逻辑）收成**贴作者架构、无双路径屎山、尽量无损、可合 PR** 的代码。禁止重复实现作者已有能力；该删的删；可加深 module，但 interface 要小、归属清晰。

## Locked decisions (user + recommended, 2026-07-22)

| # | Decision | Locked choice |
|---|---|---|
| 1 | Scope this parent task | **S1–S7** (主干 P1–P3). S8–S10 = follow-up parent task |
| 2 | Merge / PR | **Multiple PRs to `origin/unstable` by package**; create PR, **do not merge** |
| 3 | Bridge aggressiveness | **Strict**: only bridges with tests + doc acknowledgment; no name-only fake bridges |
| 4 | PassThroughBody | **Keep as explicit mitigation adapter** (not architecture fix) |
| 5 | Codex | **Responses native + usage-profile**; not a separate protocol |
| 6 | Dual-path tests | **Rewrite tests to PE**; delete metadata body-write paths |
| 7 | Tests / commands | **Targeted `go test` per slice** under `llm/` (and orch when touched); no full-repo lint/build unless user asks |
| 8 | Review bar | **Per-slice reviewer sub-agent**; **Phase complete → code-review skill** |
| 9 | Goal stop valve | **Only irreversible/external**: push default branch, merge, product default changes, delete user data |
| 10 | Working tree hygiene | **Do not commit unrelated `.trellis` script noise**; only task files |

Doc freeze (S0) already landed: ADR 0001/0002, CONTEXT, guidelines, matrix owner notes, `llm.Request` metadata comments (`d1b5dec7` / related).

## Background (confirmed)

- Author architecture: CrossProtocolCanonical = `llm.Request`/`Response`; protocol-private → PE / native / raw; LossyDowngrade for cross-protocol; same-protocol first; no universal AST.
- Friction: dual homes (PE vs TransformerMetadata vs RawRequest.Body), custom-tool policy in orchestrator + Chat + Anthropic, pass-through body dump after transform, opaque strip field knowledge in orch.
- Branch `codex/grok-chat-custom-tool-compat` is far ahead of `origin/unstable`; PR packing by package is mandatory.

## Requirements

### R1 FieldOwnership purity (S1, S4, S5)

- Responses body natives: **only** `ProviderExtensions.OpenAIResponses.*` (no metadata dual-write for body).
- Chat same-protocol natives currently on body reparse: move to named Chat PE owner (or equivalent), single attach/merge path.
- Anthropic container/geo/mcp_servers etc.: migrate primary owner to `ProviderExtensions.Anthropic` (not metadata body dump).
- Delete dead dual-path code after tests green.

### R2 Diagnostics single surface (S2)

- Formal loss/preservation diagnostics primarily under `ProviderExtensions.Diagnostics`.
- No triple recording of the same custom-tool / field loss across orch + Chat + Anthropic without a single owner decision.

### R3 Custom tool lifecycle module (S3)

- One `llm` (or transformer/shared) module: preserve | bridge-to-function | drop + rehydrate.
- Orchestrator: candidate eligibility + invoke only; no shadow conversion policy.

### R4 Opaque reasoning (S6)

- Field strip shape in `llm` helper; orchestrator only decides **when** (channel/model switch, recover).

### R5 Pass-through contract (S7)

- Explicit ConvertOutbound vs PassThroughOutbound (or equivalent clear seam).
- Pass-through remains mitigation; PE convert path not silently discarded without mode selection.

### R6 Conversion quality

- Same-protocol: near-lossless; silent drop = bug.
- Cross-protocol: semantic / near-semantic bridges **with tests**; no-synth ecosystems → Lossy / unsupported.
- Codex: normalize to Responses where possible; profile-only stays PE/native.

### R7 Process

- Strict slice order S1→S7; no cross-slice batching.
- Each slice: red test → implement → delete dead dual path → short review → only then next slice.
- Phase (P1 after S2, P2 after S6, P3 after S7): full code-review before declaring phase done.

## Acceptance criteria

### Per-slice (must hold before next slice)

- [ ] Scope files only; no drive-by refactors outside slice list in `implement.md`.
- [ ] Targeted tests for the slice contract pass.
- [ ] Dual path replaced by slice is **deleted**, not commented out.
- [ ] Short review (reviewer sub-agent) reports no ADR/guidelines violation for the slice.
- [ ] Commit message names slice id (S1…S7).

### Parent task / Goal exit (all required)

See **Goal exit standard** below — Goal mode ends only when that checklist is fully true.

## Out of scope (this parent task)

- S8 approximate-bridge expansion beyond already-tested bridges (new parent).
- S9 PersistenceState full split (optional later; only if blocking S1–S7 tests).
- S10 full multi-PR merge campaign finish (PRs may be opened during task; merge is human).
- Full vendor doc re-crawl; full-repo lint/build; frontend; non-protocol features.
- Widening `llm.Request` with protocol-private fields.
- Independent “Codex protocol” module.

## Goal exit standard (退出 Goal 的硬标准)

Goal 模式**完成并退出**当且仅当下列全部满足。任一不满足 → **不得**宣称 Goal 完成；继续修或降级范围须符合 stop valve。

### A. 切片交付

1. **S1–S7 全部**达到各自 `implement.md` 完成标准，且每刀有对应 commit（可 squash 叙述，但历史可追溯切片 id）。
2. 无「半刀」：禁止「S3 做了一半先开 S4」。
3. 每刀短审结论为 **pass**（或 findings 已在同刀修完再 pass）。

### B. 代码质量（无屎山门禁）

4. 本任务触及的双路径目标（S1 metadata body、S3 三套 custom 策略、S4 body reparse 保真、S5 Anthropic metadata body 主路径、S6 orch 内协议字段 strip 形状）在 **scope 文件内已删除或只剩单一 owner**；reviewer 抽查无「兼容 shim 当完成」。
5. 无新增：把协议私有字段塞进 `llm.Request`、把 `TransformerMetadata` 当 body 垃圾场、把 pass-through 当唯一同协议解。
6. 无未删除的死代码/注释掉的大块替代实现（本任务引入的）。

### C. 正确性

7. 本任务声明的 targeted test 集 **全绿**（每刀跑过的包在最终再跑一轮相关合集）。
8. 同协议保真：本任务触及字段无已知静默丢（有测覆盖）。
9. 跨协议：无新增 name-only 假桥；新 Lossy 有诊断路径。

### D. 文档一致性（影响后续的活文档）

10. ADR 0001/0002、CONTEXT、guidelines、strict matrix **Owner 与本任务结论不冲突**（本任务改码导致的 owner 变化已回写矩阵/指南必要处）。
11. 不依赖过时「第一刀冻结」叙述指导实现。

### E. 可合并形态

12. 已按包准备好 **指向 `origin/unstable` 的 PR（create，不 merge）**，或等价：清晰的 PR 拆分说明 + 本地 branch 可推；diff 不含无关 trellis 噪声。
13. PR 描述含：切片列表、测试命令、已知 residual（S8+）、Goal exit checklist 勾选状态。

### F. 审查

14. **P1（S1–S2）、P2（S3–S6）、P3（S7）** 各至少一次 Phase 级 code-review（Standards + Spec/ADR）；blocking 项已清零。
15. 无 open blocking review item 挂在 task notes。

### 明确 **不** 作为退出条件

- 全仓 lint/全量 e2e 绿（除非用户另开要求）。
- `origin/unstable` 已 merge。
- S8–S10 完成。
- 101 行矩阵全部 `CONFIRMED`。
- 跨协议「完美无损」。

### 中途退出 / 中止 Goal（非完成）

仅当 stop valve 触发且用户改决策，或发现 **阻塞级** 事实（例如作者主干大改导致架构 ADR 需重开）时：写清 blocked 原因到 task notes，`task.py finish` 不标记验收完成，列出已完成切片与 residual。

## Open questions

**None blocking.** All ten pre-goal decisions locked.
