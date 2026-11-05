# 响应钩子

[插件系统](README.md)的一个扩展点：插件改写统一的 `llm.Response` 和流式 chunk。

本篇不在 [任务表](README.md#9-任务管理) 里，不阻塞当前交付。当前交付只有 [请求钩子](request.md) 的 `onRequest` 和 `onAttempt`。manifest 的 `hooks` 里出现 `response` 或 `streamChunk` 时拒绝安装，见 [字段表](manifest.md#3-字段)。下面是排期时的实现合同。排期后加入任务表，依赖 T-13。

## 1. 背景

`llm/pipeline/middleware.go` 有两个响应侧的切入点：

- `OnOutboundLlmResponse`：每次成功的非流式 attempt 调用一次。
- `OnOutboundLlmStream`：每次成功的流式 attempt 调用一次，中间件包装 `streams.Stream[*llm.Response]` 后逐 chunk 处理。

两者都按注册顺序的逆序执行（`llm/pipeline/pipeline.go` 231–280 行）。请求钩子中间件注册在 `persistRequestExecution` 之前，所以响应侧会晚于持久化执行。

chat 流一个请求可能有几百个 chunk。每个 chunk 都调用一次 JS 的话，要序列化 chunk、执行脚本、把返回值转回 Go，还要启动超时看门狗，调用次数会随 token 数线性增长。

## 2. 目标

1. 非流式响应调用 `onResponse(res, ctx)`；流式响应按 chunk 调用 `onStreamChunk(chunk, ctx)`。返回对象则替换，返回 `null` 丢弃该 chunk，返回 `undefined` 保持原样。
2. `onStreamChunk` 只收到插件声明过的 chunk 类型，其余 chunk 由宿主透传，不进 JS。
3. 钩子抛错、超时、返回非法值时按原值继续，与 [请求钩子](request.md#4-钩子) 的失败处理相同。
4. 同一请求里，同一插件的请求钩子和响应钩子共用 `ctx.state`。

### 非目标

- 改写上游的非 2xx 错误响应。
- 改变响应形态：流式不能变成非流式，反之也不行。
- 审计客户端实际收到的、被改写后的内容。见妥协 1。
- 攒批调用。多个 chunk 合成一次调用会增加首字延迟和每个 chunk 的延迟。

## 3. User Stories

1. 作为插件作者，我希望只在流结束时拿到 `finish_reason` 和 usage，以便统计或补充字段，而不用处理每个 token。
2. 作为 API 使用方，我希望插件没有声明 `content` 时，内容 chunk 的延迟和没装插件时一样。
3. 作为插件作者，我希望在 `onRequest` 里放进 `ctx.state` 的数据，在 `onResponse` 里还能读到。

## 4. 决策

| 方案 | 结论 |
|------|------|
| 每个 chunk 都调用 JS | 放弃。调用次数随 token 数线性增长 |
| 按请求独占 VM，省掉每个 chunk 的借还 | 放弃。借还只是两次 channel 操作加清理，贵的是 JS 调用本身；独占会让流式请求长时间占满池子。见 [README 决策](README.md#4-决策) |
| 攒批调用 | 放弃。增加延迟 |
| 插件声明关心的 chunk 类型，宿主分类后只把命中的 chunk 交给 JS，每次调用按次借 VM | 采用 |

## 5. 设计

脚本入口和共用运行时约定见 [plugin.js.md](plugin.js.md)；下面的响应导出仅在本扩展点开放后生效。

### manifest

开放时并入 [manifest.md](manifest.md)。

`hooks.response` 是布尔值，`true` 时必须导出 `onResponse`。`hooks.streamChunk` 是 chunk 类型的非空列表，有值时必须导出 `onStreamChunk`：

```json
"hooks": { "request": true, "attempt": false, "response": true, "streamChunk": ["finish", "usage"] }
```

chunk 类型由宿主从 `llm.Response` 判断，一个 chunk 可以同时属于多种类型：

| 类型 | 判断条件 |
|------|----------|
| `content` | 任一 choice 的 delta 有文本内容 |
| `reasoning` | 任一 choice 的 delta 有推理内容 |
| `tool_call` | 任一 choice 的 delta 有工具调用 |
| `finish` | 任一 choice 的 `finish_reason` 非空 |
| `usage` | `Usage` 非空 |

### 流程

响应钩子复用 [资源加载与释放](README.md#资源加载与释放) 中的后端模块和同一插件 VM 池，不单独加载一份脚本，也不逐 chunk 读库、解压或编译。请求固定的 Generation 持有资源直到响应结束或取消，流式期间升级不会提前释放旧脚本。

```text
1. 请求钩子中间件同时实现 OnOutboundLlmResponse 和 OnOutboundLlmStream。
   只有声明了 response 或 streamChunk 的插件才进入下面的列表。
2. 非流式：OnOutboundLlmResponse(res)
   按 priority 调用 onResponse(res, ctx)，前一个插件的结果作为下一个插件的输入。
3. 流式：OnOutboundLlmStream 包装 stream，对每个 chunk：
   - 宿主判断 chunk 类型，与每个插件声明的类型取交集
   - 交集为空的插件跳过，不借 VM、不序列化
   - 都跳过时 chunk 原样透传
   - 否则按 priority 借 VM 调用 onStreamChunk(chunk, ctx)，返回后归还
4. ctx.state 与该插件在本请求里的请求钩子共用，请求结束释放。
```

响应钩子按 attempt 触发。重试或切换渠道后，新的 attempt 重新从第 2 或第 3 步开始。启用了 [作用范围](scope.md) 时，只调用匹配通过的插件。

失败：抛错、超时、非法返回，都保留原 chunk 或原响应，并计一次失败。单个请求的 `onStreamChunk` 调用次数达到上限后，剩余 chunk 不再交给该插件，并计一次。

取不到 VM 时也保留原值，计入 `overloaded`，不计失败。这里和 [请求钩子](request.md#vm-不足) 不同：上游已经返回并计费，流式响应可能已经开始写出，这时拒绝只会浪费这次调用。

### 限制

排期时并入 [运行限制](README.md#运行限制)，在此之前以本表为准。

| 项 | 限制 |
|----|------|
| `onResponse` | 200 ms |
| `onStreamChunk` | 20 ms |
| 单请求 `onStreamChunk` 调用次数 | 每插件 1000 次 |

### 改动清单

#### `internal/objects/plugin.go`
- `PluginHooksSpec` 增加 `Response bool`、`StreamChunk []string`。校验类型取值和对应导出。删除当前交付中"出现即拒绝"的校验。

#### `internal/server/plugin/hooks.go`
- 实现 `OnOutboundLlmResponse`、`OnOutboundLlmStream`。chunk 分类、按类型过滤、单请求调用计数。

#### `internal/server/plugin/manager.go`
- Generation 中为每个插件预计算 chunk 类型集合。

#### 前端
- 授权对话框和插件详情显示声明的响应钩子和 chunk 类型。`locales/en/plugins.json`、`locales/zh-CN/plugins.json` 增加类型名称。

## 6. 验收

| 类别 | 标准 | 验证方式 |
|------|------|----------|
| 功能 | 声明 `["finish", "usage"]` 的插件，在 200 个 chunk 的流里只被调用 1–2 次 | `go test ./internal/server/plugin/ -run TestStreamChunkFilter`，统计 JS 调用次数 |
| 功能 | 返回 `null` 丢弃 chunk；返回 `undefined` 保持原样；返回对象则替换 | 同上 |
| 功能 | `onRequest` 写入的 `ctx.state` 在 `onResponse` 中可读 | 同上 |
| 对外兼容 | 钩子抛错或超时时，OpenAI 和 Anthropic 入口的流式与非流式响应与无插件时一致 | orchestrator 集成测试，httptest 上游 |
| 性能 | 不命中任何声明类型的 chunk 不借 VM；命中时单次 `onStreamChunk` ≤ 0.2 ms（开发机） | `go test -bench StreamChunk ./internal/server/plugin/` |
| 当前交付 | `hooks` 含 `response` 或 `streamChunk` 的包安装失败且不写库 | `go test ./internal/server/plugin/ -run TestParsePackage` |

## 7. 风险与妥协

| 风险 | 触发条件 | 影响 | 对策 |
|------|----------|------|------|
| 声明 `content` 的插件拖慢流式输出 | 每个内容 chunk 都进 JS | 首字和 chunk 间隔变长 | 20 ms 超时、单请求调用上限、熔断。授权对话框单独标出声明了 `content` 的插件 |

```text
1. 妥协：响应钩子的改写不进入请求记录
   - 原因：响应侧按逆序执行，钩子晚于 persistRequestExecution
   - 影响：请求记录里的响应是插件改写前的内容
   - 升级触发：需要审计客户端实际收到的内容
   - 后续方向：在持久化前再调用一次响应钩子

2. 妥协：不处理非 2xx 响应
   - 原因：错误体影响故障转移判断，要和现有上游错误保持一致
   - 影响：插件不能改写上游错误
   - 升级触发：需要统一改写上游错误文案
   - 后续方向：可选的 onError，返回值限制为现有 llm.ResponseError
```
