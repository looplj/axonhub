# G11 LossyDowngrade recorder seam 审查（xhigh）

日期：2026-07-08  
审查仓库：`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`  
审查对象：当前未提交 diff（G11 `LossyDowngrade` recorder seam）

## 结论

**PASS**

可以提交，但提交时必须包含当前 untracked 文件：

- `llm/lossy_downgrade_test.go`

## 审查范围

当前工作树差异为：

- modified: `llm/lossy_downgrade.go`
- modified: `llm/transformer/anthropic/lossy_downgrade.go`
- modified: `llm/transformer/openai/lossy_downgrade.go`
- modified: `llm/transformer/openai/responses/lossy_downgrade.go`
- untracked: `llm/lossy_downgrade_test.go`

未看到字段矩阵文档、字段清单、协议模型或字段顺序表被修改。

## 关键证据

### 1. 新 helper 行为正确

`llm/lossy_downgrade.go:50-65` 新增 `AddLossyDowngradeIfPresent`：

- `present == false` 时直接 return（`llm/lossy_downgrade.go:54-56`）。
- `present == true` 时只构造标准 `LossyDowngrade` 并调用 `AddLossyDowngrade`（`llm/lossy_downgrade.go:58-64`）。
- 默认值保持为：
  - `Reason: LossyDowngradeReasonNoEquivalentSemantics`
  - `Severity: LossyDowngradeSeverityWarning`
- 校验与去重没有重写：仍委托 `AddLossyDowngrade`。`AddLossyDowngrade` 仍在 `llm/lossy_downgrade.go:31-48` 做 nil/空字段校验，并通过遍历 `diagnostics.LossyDowngrades` 对完全相同 diagnostic 去重。

### 2. 三处迁移保持 target protocol 不变

三处原本内联 `AddLossyDowngrade` 的 recorder 现在只转调 helper，target protocol 常量保持原值：

- Anthropic Messages target：`llm/transformer/anthropic/lossy_downgrade.go:71-73` 使用 `llm.APIFormatAnthropicMessage`。
- OpenAI Chat target：`llm/transformer/openai/lossy_downgrade.go:61-63` 使用 `llm.APIFormatOpenAIChatCompletion`。
- OpenAI Responses target：`llm/transformer/openai/responses/lossy_downgrade.go:46-48` 使用 `llm.APIFormatOpenAIResponse`。

diff 只删除三处重复的 `present=false return + AddLossyDowngrade(...)` 块，替换为等价 helper 调用。

### 3. 未改变字段矩阵、字段顺序、reason/severity、去重

- 字段判定仍在目标 outbound adapter 内完成：
  - Anthropic target adapter 仍在 `recordAnthropicRequestOpenAIResponsesDowngrades` / `recordAnthropicRequestOpenAIChatDowngrades` 中判断字段存在性。
  - OpenAI Chat target adapter 仍在 `recordOpenAIChatRequestOpenAIResponsesDowngrades` / `recordOpenAIChatRequestAnthropicDowngrades` 中判断字段存在性。
  - OpenAI Responses target adapter 仍在 `recordResponsesRequestAnthropicDowngrades` / `recordResponsesRequestOpenAIChatDowngrades` 中判断字段存在性。
- 字段列表没有改动：例如 `openAIChatUnsupportedAnthropicFields`、`openAIChatUnsupportedResponsesFields` 仍保持原顺序。
- raw top-level 字段顺序仍由 `llm.SortedRawFieldNames` 控制；本 diff 未改该函数。
- diagnostic reason/severity 仍是 `no_equivalent_semantics` + `warning`。
- diagnostic 去重仍由 `AddLossyDowngrade` 的 exact-struct comparison 保证。

### 4. 测试覆盖充足

新增 core helper 单测在 `llm/lossy_downgrade_test.go:9-26` 覆盖：

- `present=false` 不记录 diagnostic。
- `present=true` 记录标准 source/target/reason/severity。
- 重复调用仍只保留 1 条，证明 helper 没绕过去重。

三协议 lossy downgrade tests 已覆盖：

- OpenAI Chat target：`llm/transformer/openai/lossy_downgrade_test.go`
- OpenAI Responses target：`llm/transformer/openai/responses/lossy_downgrade_test.go`
- Anthropic Messages target：`llm/transformer/anthropic/lossy_downgrade_test.go`

已执行并通过：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./... -count=1
```

结果：全 `llm` 模块测试通过。

附加只读检查：

```bash
git diff --check
gofmt -l llm/lossy_downgrade.go llm/lossy_downgrade_test.go llm/transformer/anthropic/lossy_downgrade.go llm/transformer/openai/lossy_downgrade.go llm/transformer/openai/responses/lossy_downgrade.go
```

结果：无输出，通过。

### 5. G11b 跳过字段矩阵抽取是合理的

本 slice 是 recorder seam cleanup，不新增字段、不删除字段、不重排字段、不改变字段存在性判断，也不把字段矩阵中央化。

现有规范要求 target protocol outbound adapter 拥有 downgrade 决策，因为它知道目标协议能表达什么；见 `.trellis/spec/backend/protocol-transformer-guidelines.md` 中 LossyDowngrade 规则：target outbound adapter owns downgrade decisions。

当前实现符合该规则：helper 只接收 `present` 布尔值并写标准 diagnostic，不读取 `ProviderExtensions`、不枚举字段、不建立中央 downgrade matrix。因此 G11b 不重新做字段矩阵抽取是合理的；字段判断仍应留在各 target adapter。

### 6. 抽象质量

未发现过度抽象、屎山、死代码或误导性注释：

- helper 只消除三处完全重复 recorder boilerplate。
- 没有引入全局字段矩阵、万能 converter、协议 AST 或新的 sidecar。
- 注释准确限定为“standard cross-protocol downgrade diagnostic”，并明确字段存在性由 target adapters 决定。
- 现有私有 recorder 函数保留为每个 target adapter 的本地 seam，可读性比三处重复 struct literal 更好。

## Must-fix

代码层面：无。

提交打包层面：必须把 untracked 文件一起纳入提交：

- `llm/lossy_downgrade_test.go`

否则 core helper 单测不会进入提交，测试覆盖会退化。

## 建议项

- 可选：如果后续继续扩展 helper，可补一个 `present=true` 但 source/target/field 为空的单测，直接证明校验仍由 `AddLossyDowngrade` 承担。当前 diff 中 helper 是纯委托，现有结构证据和 full llm tests 已足够，不作为阻塞项。

## 是否可提交

**可提交**。

前提：提交包含以下 5 个文件，其中 `llm/lossy_downgrade_test.go` 不能遗漏：

- `llm/lossy_downgrade.go`
- `llm/lossy_downgrade_test.go`
- `llm/transformer/anthropic/lossy_downgrade.go`
- `llm/transformer/openai/lossy_downgrade.go`
- `llm/transformer/openai/responses/lossy_downgrade.go`
