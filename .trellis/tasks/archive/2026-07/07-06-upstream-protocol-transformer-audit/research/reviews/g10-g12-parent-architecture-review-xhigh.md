# G10-G12 parent architecture/code review（xhigh）

日期：2026-07-08  
审查仓库：`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`  
审查范围：`88795be0...c62c111f`，即最近三个架构加深提交：

- `d41d4077 refactor(llm): share raw JSON clone helpers`
- `a09276c2 refactor(llm): centralize lossy downgrade recording`
- `c62c111f refactor(llm): share raw top-level capture helper`

## 结论

**PASS**。

这三个切片组合后仍符合作者 transformer 框架：`llm` common carrier 保持为公共语义层，协议 sidecar 仍承载协议私有/原始字段，target adapter 仍拥有 downgrade policy。未发现过度抽象、屎山化、死代码、命名不一致或协议行为破坏。

可以进入当前 G10-G12 parent stop-condition / finish gate。代码层面无 must-fix。

## 最终验证

已在审查 worktree 执行并核实：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm && go test ./... -count=1
```

结果：**PASS**，所有 `llm` module packages 通过。

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean && git diff --check
```

结果：**PASS**，无输出。

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean && git status --porcelain=v1 -uall
```

结果：**clean**，无输出。当前 HEAD：`c62c111f`。

附加只读格式检查：

```bash
gofmt -l <G10-G12 touched go files>
```

结果：**PASS**，无输出。

## 关键证据

### 1. 提交范围干净且切片边界明确

`git log 88795be0..HEAD --oneline` 只包含目标三枚提交：

```text
c62c111f refactor(llm): share raw top-level capture helper
a09276c2 refactor(llm): centralize lossy downgrade recording
d41d4077 refactor(llm): share raw JSON clone helpers
```

组合 diff 只触及 14 个 Go/test 文件，集中在：

- `llm/internal/pkg/xjson/raw.go` / `raw_test.go`
- `llm/lossy_downgrade.go` / `lossy_downgrade_test.go`
- `llm/provider_extensions.go`
- OpenAI Chat / OpenAI Responses / Anthropic 的 extension 和 lossy recorder 文件

未触碰 pipeline/orchestrator/router/provider selection，也未给 `llm.Request` 增加协议专属字段。

### 2. 没有背离作者 transformer 框架

作者框架要求：common carrier 只放跨协议公共语义，协议私有字段进入 sidecar/native adapter，target outbound adapter 决定目标协议能否表达。

当前代码符合：

- `llm/provider_extensions.go:9-15` 仍定义 `ProviderExtensions` 为 provider/API-format private data，所有字段均 `json:"-"`，不通过 common request/response JSON 序列化。
- `llm/provider_extensions.go:62-67` 的 `OpenAIChatRequestExtensions.RawTopLevelFields` 注释仍限定为 OpenAI Chat same-protocol replay。
- `llm/provider_extensions.go:69-98` 的 OpenAI Responses request sidecar 仍区分 raw tools/input/tool_choice、typed/raw Responses native fields、same-protocol unknown/profile top-level fields。
- G10-G12 没有把 Responses/Chat/Anthropic 字段移入 `llm.Request`，也没有新增 universal protocol AST。
- `llm/transformer/openai/chat_extensions.go:11-15` 明确 Chat raw replay 是 same-protocol，cross-protocol unsupported fields 属于 LossyDowngrade。

判断：**llm common carrier + protocol sidecar + target adapter policy 仍成立**。

### 3. `llm/internal/pkg/xjson` 新 helper 没有协议语义泄漏

`llm/internal/pkg/xjson/raw.go:1-54` 只包含三类低层 JSON 工具：

- `CloneRawMessage(json.RawMessage)`：深拷贝 raw bytes，空输入归一为 `nil`。
- `CloneRawMessageMap(map[string]json.RawMessage)`：深拷贝 map 与 value raw bytes。
- `CaptureRawTopLevelFields(body []byte, fieldNames []string)`：按调用方传入的字段名白名单，从 JSON body 捕获 top-level raw bytes。

关键点：

- `xjson` 只导入 `encoding/json`，没有导入 `llm`、OpenAI、Anthropic 或 transformer 包。
- helper 不包含任何协议字段名、field matrix、same-protocol/cross-protocol 判断或 target policy。
- 字段列表仍在协议 adapter 本地：OpenAI Chat 在 `llm/transformer/openai/chat_extensions.go:16-30`，Anthropic request 在 `llm/transformer/anthropic/request_extensions.go:10-17`，Anthropic response 在 `llm/transformer/anthropic/response_extensions.go:10-14`。
- OpenAI Responses 的 owned/unknown top-level 分流仍留在 `llm/transformer/openai/responses/request_extensions.go`，没有被粗暴塞进通用 xjson helper。

判断：**xjson 只承载通用 JSON byte/raw capture 工具，没有协议语义泄漏**。

### 4. LossyDowngrade helper 只收敛 recorder 样板

`llm/lossy_downgrade.go:31-48` 的 `AddLossyDowngrade` 仍负责校验和去重。G11 新增的 `AddLossyDowngradeIfPresent` 位于 `llm/lossy_downgrade.go:50-65`，行为只是在 `present == true` 时构造标准 diagnostic，然后委托 `AddLossyDowngrade`。

没有中央化字段矩阵：

- Anthropic target 的字段存在性与 target protocol 仍在 `llm/transformer/anthropic/lossy_downgrade.go:18-73` 本地判断。
- OpenAI Chat target 的字段存在性与 target protocol 仍在 `llm/transformer/openai/lossy_downgrade.go:18-63` 本地判断。
- OpenAI Responses target 的字段存在性与 target protocol 仍在 `llm/transformer/openai/responses/lossy_downgrade.go:14-48` 本地判断。
- raw top-level 字段排序仍由 `llm.SortedRawFieldNames` 控制，helper 不读取 sidecar、不枚举字段、不知道目标协议能力。

判断：**LossyDowngrade helper 只减少重复 struct literal / present guard，没有隐藏 target policy，也没有形成中央字段矩阵**。

### 5. 组合后无重复 helper、死代码或命名不一致

已做精确残留检查：旧 helper 名称无残留调用/定义：

```text
cloneRawMessageMap
cloneAnthropicRawMessage
cloneOpenAIChatRawMessage
cloneRaw(
cloneRawMessage(
```

唯一保留的近似名称是 `llm/transformer/openai/responses/websocket_executor.go` 的 `cloneRawMessages`，它是 `[]json.RawMessage` slice clone，作用域是 WebSocket session input 缓存，不是 G10-G12 抽取的单个 raw/map helper 残留。

命名一致性：

- JSON byte clone/capture 统一命名在 `xjson.CloneRawMessage`、`xjson.CloneRawMessageMap`、`xjson.CaptureRawTopLevelFields`。
- downgrade 统一命名为 `AddLossyDowngradeIfPresent`，各 target adapter 保留本地 `record...Downgrade` seam。
- Anthropic raw field spec 从单字段 struct 改为 `[]string` 后，`Message.MarshalJSON`、request/response preserve/apply 循环都同步使用 `name`，未发现 `field.name` 残留。

判断：**无阻塞重复/死代码/命名问题**。

### 6. 测试覆盖与协议行为

新增测试：

- `llm/internal/pkg/xjson/raw_test.go` 覆盖 clone nil/empty、raw bytes 不别名、map value 深拷贝、top-level capture 的 invalid/no-match/null/白名单/clone 行为。
- `llm/lossy_downgrade_test.go` 覆盖 `present=false` 不记录、`present=true` 记录标准 diagnostic、重复记录仍去重。

组合验证：

- `go test ./... -count=1` 在 `llm` module 全量通过。
- `git diff --check` 通过。
- worktree status clean。

行为判断：

- G10/G12 是 helper seam 抽取，保留旧逻辑的空值处理、raw bytes clone、map clone、JSON null 保留、invalid JSON 忽略语义。
- G11 保留原 reason/severity：`no_equivalent_semantics` + `warning`，并继续由 target adapter 决定字段是否 present。
- 未发现同协议 raw replay 泄漏到跨协议路径，也未发现 cross-protocol loss 被静默吞掉。

## must-fix

无。

## 建议项

1. 后续不要把 OpenAI Responses 的 owned/unknown top-level 分流强行下沉到 `xjson.CaptureRawTopLevelFields`；该分流有协议所有权语义，应继续留在 Responses adapter。
2. 如果未来继续扩展 `xjson`，保持当前约束：只处理 JSON byte/map/raw clone，不接受协议常量、target protocol、field ownership 或 downgrade reason。
3. 可选增强测试：为 `CaptureRawTopLevelFields` 补一个重复 fieldNames 的用例；当前实现与旧 map capture 行为一致，不作为阻塞项。

## 是否可以进入 stop-condition / finish

**可以**。

从 G10-G12 parent architecture/code review 角度：无 must-fix，验证命令通过，审查 worktree clean，可以进入 stop-condition / finish。此结论只覆盖本报告列出的三枚架构加深提交组合，不扩大为未审查提交的结论。
