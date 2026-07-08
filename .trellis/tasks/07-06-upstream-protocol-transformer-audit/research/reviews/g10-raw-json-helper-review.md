# G10 raw JSON helper seam 只读审查

结论：**PASS（带提交前置条件）**

## 关键证据

- 新增 `llm/internal/pkg/xjson/raw.go`：
  - `CloneRawMessage` 对 `len(src)==0` 返回 `nil`；非空时使用 `append(json.RawMessage(nil), src...)`，保持旧 helper 的深拷贝行为。
  - `CloneRawMessageMap` 对 `len(src)==0` 返回 `nil`；非空时新建 map，并逐项调用 `CloneRawMessage`，满足 map 本体与 value 的深拷贝。
- 新增 `llm/internal/pkg/xjson/raw_test.go` 覆盖了：nil / empty 返回 nil、RawMessage 非别名、源数据变更不影响 clone、map value 非别名。
- 依赖方向合理：`llm/provider_extensions.go` 与 `llm/transformer/...` 均在 `llm` module 内部，导入 `github.com/looplj/axonhub/llm/internal/pkg/xjson` 符合 Go `internal` 可见性；`xjson` 只依赖标准库 `encoding/json`，没有反向依赖 transformer/provider 层。
- 已替换的范围覆盖关键路径：
  - ProviderExtensions：`CloneProviderExtensions` 中 OpenAI Responses request/response、OpenAI Chat、Anthropic request/response 的 raw 字段与 raw map 均改为 `xjson.CloneRawMessage*`。
  - OpenAI Responses：request extension attach、raw top-level fields、raw-only tools/input、marshal replay、stream_options 合并等 raw preservation 路径均改用新 helper。
  - OpenAI Chat：prompt_cache_retention、raw-only top-level fields、tools merge 等路径改用新 helper。
  - Anthropic：request/response top-level raw fields、container、stop_details、tools merge 等路径改用新 helper。
- 旧 helper 清理：在 `llm/` 下未发现 `cloneRawMessageMap`、`cloneAnthropicRawMessage`、`cloneOpenAIChatRawMessage`、`func cloneRaw(` 或 `cloneRaw(` 残留。
- 注意到 `llm/transformer/openai/responses/websocket_executor.go` 仍有 `cloneRawMessages([]json.RawMessage)`；这是切片 helper，不是本次新增的单值/map seam 的旧调用点，未直接影响本次 G10 结论。

## must-fix

无代码行为层面的 must-fix。

提交前必须确认两个新增文件也被纳入提交：

- `llm/internal/pkg/xjson/raw.go`
- `llm/internal/pkg/xjson/raw_test.go`

当前它们是 untracked；如果只提交已跟踪文件 diff，会导致调用 `xjson.CloneRawMessage*` 但缺少定义。

## 建议项

- 可选：后续若想继续收敛，可把 `websocket_executor.go` 的 `cloneRawMessages` 循环内部改为调用 `xjson.CloneRawMessage`；但它是 slice 专用 helper，不阻塞本次提交。
- 可选：提交说明中明确这是 helper seam 收敛，不改变 raw preservation 语义。

## 是否可提交

**可以提交**，条件是把上述两个新增 `xjson` 文件一并加入提交。若提交工具只包含已跟踪改动，则**不可提交**。
