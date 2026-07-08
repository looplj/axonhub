# G12 raw top-level capture helper seam 审查报告

## 结论

PASS。当前未提交 diff 可以提交。

## 关键证据

- 审查范围：`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean` 当前未提交 diff，只涉及：
  - `llm/internal/pkg/xjson/raw.go`
  - `llm/internal/pkg/xjson/raw_test.go`
  - `llm/transformer/openai/chat_extensions.go`
  - `llm/transformer/anthropic/request_extensions.go`
  - `llm/transformer/anthropic/response_extensions.go`
  - `llm/transformer/anthropic/model.go`
- `xjson.CaptureRawTopLevelFields` 行为与旧三处 capture 保持一致：
  - `len(body)==0` 或 JSON invalid 时返回 `nil`。
  - 只按传入白名单 `fieldNames` 捕获 top-level raw 字段。
  - `json.RawMessage("null")` 长度非 0，因此 JSON null 会被保留。
  - 返回前通过 `CloneRawMessage` 克隆 raw bytes，不别名源 body。
  - 无匹配字段时返回 `nil`。
- helper 位置合理：放在 `llm/internal/pkg/xjson/raw.go`，只依赖 `encoding/json`，签名只接受 `body []byte` 和 `fieldNames []string`，不包含 OpenAI/Anthropic 字段名或协议语义。
- 三处迁移保持 raw preservation：
  - OpenAI Chat request：`captureOpenAIChatRequestRawTopLevelFields` 仍写入 `req.RawTopLevelFields`，字段列表仍为 `openAIChatRawReplayFieldNames`。
  - Anthropic request：`captureAnthropicRequestRawTopLevelFields` 仍写入 `req.RawTopLevelFields`，字段列表内容未变。
  - Anthropic response：`captureAnthropicResponseRawTopLevelFields` 仍写入 `resp.RawTopLevelFields`，字段列表内容未变。
- Anthropic 字段列表从 `[]struct{name}` 改为 `[]string` 安全：旧 struct 只有 `name` 一个字段；`preserveAnthropicRequestExtensions`、`preserveAnthropicResponseExtensions`、`Message.MarshalJSON` 均已从 `field.name` 同步为 `name`；仓库内 `llm/transformer/anthropic` 下未发现残留 `field.name` 或 `anthropicRawTopLevelFieldSpec` 引用。
- OpenAI Responses owned/unknown capture 未被误动：未提交 diff 不包含 `llm/transformer/openai/responses/*`；现有 `rawTopLevelFields` 仍保留 owned 与 unknown 分流逻辑。
- map 迭代顺序未发现可见行为变化：新 helper 返回 map 后三处 capture range map，但只是写入 `RawTopLevelFields` map；最终 JSON marshal 本身无稳定字段顺序承诺，旧代码也进入 map 存储。
- 新测试覆盖 helper 的 nil/invalid/no-match、白名单捕获、JSON null 保留、忽略非白名单、clone 不别名源 body。
- 用户已提供验证结果：`cd llm && go test ./internal/pkg/xjson ./transformer/openai ./transformer/anthropic -count=1` PASS；`go test ./... -count=1` PASS；`git diff --check` PASS。

## must-fix

无。

## 建议项

- 可选：后续若继续复用该 helper，可补一个“字段名重复”用例；当前行为是后一次相同 key 覆盖同一值，不影响本次迁移。
- 可选：若团队特别在意 JSON 输出文本顺序，可避免在 capture 层引入 map range；但当前三处只写 map，且现有 marshal 路径本就不保证对象字段顺序，因此不是提交阻塞项。

## 是否可提交

可以提交。当前变更是低风险去重抽取，未改变三处 raw top-level preservation 的语义，也未触碰 OpenAI Responses owned/unknown capture。
