# 请求钩子

[插件系统](README.md)的请求扩展点。`onRequest(ctx, request)` 修改统一请求的公开字段，`onAttempt(ctx, body)` 修改当前尝试的上游 JSON 请求体。所有入口统一以 `ctx` 为第一个参数。manifest 见 [manifest.md](manifest.md)，宿主 API 见 [plugin.js.md](plugin.js.md)，限制见 [运行限制](README.md#运行限制)。

> 本篇是未发布接口的修订设计，`schemaVersion` 和 `pluginApiVersion` 保持 `1`，不保留旧签名或布尔 hooks 的兼容层。运行时、示例和回归测试已同步此契约，见 T-13。

## 1. 背景

统一请求适合协议无关的业务处理，上游请求体适合供应商协议适配。两个钩子分别处理这两个阶段，避免为了增加一个上游字段而扩展 `llm.Request`，也允许在 pass-through 渠道上精确修改上游 body。

## 2. 目标

1. `onRequest` 按 `RequestType` 注册，每个匹配请求调用一次。
2. `onAttempt` 按当前尝试的实际出站 `APIFormat` 注册，每次尝试重新匹配。
3. 两个钩子返回对象表示整体替换，返回数组表示 RFC 6902 JSON Patch，返回 `undefined` 表示不修改。
4. 非法返回、patch 失败、抛错、超时保留调用前请求并计一次失败；VM 过载返回 429，不跳过插件。
5. 不匹配时不借 VM、不为插件序列化请求。插件不引入统一请求的 attempt 隔离复制。没有请求插件时不增加插件中间件。

### 非目标

- 新的 channel provider、路由策略、响应改写，分别见 [provider.md](provider.md)、[response.md](response.md)。
- `onAttempt` 修改 HTTP URL、方法、头、凭证、multipart 或二进制 body。
- 按 project、API Key、模型收窄；这是 [scope.md](scope.md) 的额外可选过滤。

## 3. 注册与匹配

```json
{
  "hooks": {
    "request": {
      "enabled": true,
      "requestTypes": ["chat", "completion"]
    },
    "attempt": {
      "enabled": true,
      "apiFormats": ["openai/chat_completions", "anthropic/messages"]
    }
  }
}
```

- 省略整个钩子对象表示未声明；声明时 `enabled` 必填且为布尔值，不从列表内容推断启停。`enabled: false` 保留范围配置但不调度；`true` 要求对应范围列表非空，并有同名函数导出。没有隐含的“全部”，空列表不是通配符。
- `requestTypes` 和 `apiFormats` 各自只接受 `llm/constants.go` 对应枚举的精确值；列表内部为 OR，重复项、未知值、错误类型和未知字段安装时拒绝。禁用配置中的非空列表也照常校验。
- `onRequest` 使用进入中间件时的 RequestType；兼容已有请求中空 RequestType 表示 chat 的约定。不依赖入站 API 格式。
- `onAttempt` 使用统一解析的出站格式：优先 `httpclient.Request.APIFormat`，为空时回退到 candidate APIFormat，再回退到 wrapped transformer 的 APIFormat。orchestrator 在上游转换完成后将解析结果写回 HTTP 请求；钩子匹配、`ctx.attempt.apiFormat`、pass-through 和持久化共用该结果。同渠道重试和故障转移都重新匹配，不使用入站 `ctx.request.apiFormat`。已启用的 attempt 钩子因格式不匹配而跳过时记录 Debug，包含 plugin_id、channel_id、api_format、reason；格式仍无法确定时 reason 为 api_format_unresolved，否则为 api_format_not_matched。
- 两个钩子独立匹配。只声明 attempt 或 request 不匹配时，attempt 仍可执行，此时该插件尚未写入的 `ctx.state` 为 `{}`。
- 整个插件的安装记录 `enabled` 是总开关；manifest 内的 `enabled` 是对应钩子的开关，两者均开启才调度。
- 请求固定持有进入时的 Generation；重试只重新匹配该 Generation 内的插件，不切换版本。各阶段按 priority 顺序调用。

## 4. 钩子

### 输入与执行顺序

```text
客户端请求 → inbound transformer → 统一 llm.Request
→ processor middlewares、billing
→ onRequest(ctx, request)
→ 配额、模型权限、模型映射、选路、提示词处理
→ 每次尝试：实际模型、渠道 transform options、reasoning 映射、默认参数
→ provider transformer → 上游 HTTP 请求
→ pass-through body、Responses 传输与续接 body 适配
→ onAttempt(ctx, body)
→ 渠道 override body / headers
→ 最终协议校验、传输整理、持久化、限流与发送
```

`onRequest` 的 `request` 是统一请求的公开 JSON 对象，沿用 snake_case 标签。宿主保留 `RawRequest`、`RequestType`、`APIFormat`、`TransformOptions`、`TransformerMetadata`、`ProviderExtensions` 及嵌套的非公开字段；这些字段不交付 JS，也不接受通过 replace 或 patch 注入。音频文件及文件名、图片字节与 mask、AlphaSearch/Decisions 原始正文等不因公开对象回填而丢失。它们不进入 JS，onRequest 回填时由插件内部保留原引用，不复制文件，也不参与 attempt 隔离；未修改的业务类型仍可正常发送。

`onAttempt` 的 `body` 是目标协议的 JSON 对象，不是 `llm.Request`、JSON 字符串、字节数组或整个 HTTP 请求。它已包含实际模型、渠道变换、pass-through 与续接适配的结果，字段保持上游协议本身的命名。只调度 JSON 对象 body；multipart、二进制、空 body、JSON 标量或数组 body 跳过，不计插件失败。声明的 APIFormat 不会让非 JSON body 自动变成 JSON。

`ctx.request` 使用 camelCase：`projectId`、`apiKeyId`、`model`、`requestType`、`apiFormat`，保存中间件进入时的元数据，Playground 的 apiKeyId 为 null。`ctx.attempt` 提供 `channel: {id, name, type}`、实际 `model` 和出站 `apiFormat`，不含凭证。模型在 URL 中编码的协议也通过 `ctx.attempt.model` 读取实际模型，不能假定 body 一定存在 model。

### 返回值：replace / patch

两种请求钩子使用同一套返回契约；async Promise 的结算值也适用。

| 返回值 | 含义 |
| --- | --- |
| JSON 对象（非 null、非数组） | replace：整体替换本次输入的公开对象，不做 merge；省略字段即删除，onRequest 解码后为对应零值 |
| JSON 数组 | patch：按 RFC 6902 依次应用操作；不是把 body 替换成数组 |
| `undefined` 或无返回 | 保持调用前值，忽略对参数对象的原地修改 |
| `null`、标量、非法 JSON 值 | 调用失败，保留原值并计一次失败 |

patch 使用 [JSON Patch（RFC 6902）](https://www.rfc-editor.org/rfc/rfc6902) 的 `add`、`remove`、`replace`、`move`、`copy`、`test` 六种操作，路径使用 [JSON Pointer（RFC 6901）](https://www.rfc-editor.org/rfc/rfc6901)。不使用 sjson 点号路径或 Merge Patch。`add` 对对象属性可新增或覆盖；`replace`、`remove` 要求目标存在；数组 `/-` 用于追加且父数组必须存在；`~0`、`~1` 分别表示键名中的 `~`、`/`。空数组是成功的无修改操作。

宿主把 patch 应用到调用前对象的副本，而非插件可能原地修改过的参数。任一步非法、目标不存在、test 失败或最终结果不是 JSON 对象，都回滚整组操作。根路径 `""` 也按 RFC 处理，但最终必须通过对象形态和保护字段校验。没有部分成功提交。

replace 和 patch 都在提交前完成 JSON 校验、onRequest 的类型解码及保护字段校验。只有全部通过才记录调用成功、保存 body/request 和 `ctx.state`，然后作为下一个插件的输入。失败时不得先清空连续失败计数。`undefined`、空 patch 是成功调用，可提交 state；KV 等已经发生的宿主副作用不随请求修改回滚。

```js
export function onRequest(ctx, request) {
  // replace：未修改的公开字段仍需保留。
  return { ...request, temperature: 0.2 };
}

export function onAttempt(ctx, body) {
  // patch：只修改本次上游请求中的一个字段。
  return [{ op: "add", path: "/temperature", value: 0.2 }];
}
```

### 保护字段与隔离

- `onRequest` 可以改 model，随后仍过模型权限检查和映射；不能改变流式/非流式语义，nil 与 false 均为非流式。
- `onAttempt` 不能改变已选定的模型和响应传输模式。宿主按实际 APIFormat 校验协议对应的模型、stream/传输控制字段，包括删除字段和根替换；模型在 URL、流式由 endpoint 或 WebSocket 控制时，插件不能通过 body 覆盖或矛盾地声明这些值。不向 JS 暴露 URL 修改能力。
- 保护字段变化时丢弃本次全部修改及 state，每次调用累加一次 protectedDrops，并记录 Warn（plugin_id、hook、request_id、api_format、changed_fields、reason=protected_fields_changed），不记录请求内容。这不是成功或失败，不增加也不清零连续失败计数；非法返回、patch 失败和类型解码失败计一次失败。protectedDrops 与 overloaded 一样按插件、按实例累计，通过 pluginRuntimeStatuses 和管理页卡片/详情展示。
- 每次 attempt 由 provider transformer 重新生成上游请求；onAttempt 只修改当次 HTTP 请求的 body，不写回 llm.Request，也不复用上次被插件修改的 body。因此不保存 HookBaseline，不为插件深拷贝统一请求或对其做 JSON 往返；有无插件均沿用同一套 orchestrator 请求处理。渠道 transform options 自身是否跨尝试污染属于独立问题，不能通过启用插件才触发的隔离机制解决。
- `ctx.state` 按请求、插件保存，跨 attempt 保留；body 修改只影响当前 attempt。响应流结束前若仍可能重试，Generation 和请求状态不得提前释放。

### 按协议定义保护规则

规则集中在 `plugin/protected_fields.go`，使用本次解析后的 APIFormat。只检查最终 replace/patch 结果，删除、根替换、move/copy 不能绕过检查；同一 patch 临时修改再恢复原值可以通过。

| 协议 | 保护语义 |
| --- | --- |
| OpenAI chat/completions、HTTP Responses、Anthropic | model 值及存在性不变；stream 缺省与 false 等价 |
| Ollama chat | model 不变；stream 缺省与 true 等价，删除显式 false 会被丢弃 |
| Responses WebSocket | model 不变；stream 按适配前请求语义保护；type 缺省为 response.create、generate 缺省为 true，不能改成其他帧或切换 warmup |
| OpenAI speech | model 不变；stream_format 缺省与 audio 等价，不能切换 audio/SSE |
| Gemini contents | URL 控制模型和流式动作；新增 body.model / body.stream 不得与宿主选定模型和 URL 动作冲突；type、generate 不作为通用保护字段 |
| Gemini embeddings | URL 与 body.model 必须一致；批量请求的 requests 中各 model 也不能指向其他模型 |
| 其他已支持格式 | 按表中对应协议只保护其实际控制字段，不全局保留 type/generate/stream_format |

新增 APIFormat 时须显式补充规则及测试；缺少规则时拒绝该次改写并计 protectedDrops，不推断未知协议的控制字段。

### 拒绝、失败与 VM 不足

`ctx.reject(status, message)` 仅在 onRequest 提供，status 为 400、403、429，message 非空。合法拒绝停止后续钩子，沿 `llm.ResponseError` 返回对应状态，不计失败；捕获内部控制异常仍然拒绝。非法参数按宿主 API 错误处理。

抛错、超时、非法返回、patch 失败、类型解码失败、序列化输入超过 2 MiB：保留调用前值，丢弃本次 state，计一次失败，不改变 HTTP 状态。熔断时跳过钩子。调用预算 10 秒，含已等待的 Promise 和 ctx.fetch；state 限制 64 KiB。

借 VM 最多等待 100 ms，不计入调用预算。取不到时返回 429 `plugin_overloaded` / `rate_limit_error`，消息为 `plugin <id> is at capacity; please retry shortly`；记 overloaded 和 Warn，不计插件失败、不发上游、不重试同渠道、不切渠道、不发布 failover、不影响渠道或凭证健康。

onAttempt 在 raw-request 阶段过载时设置 PluginOverloaded 标记。渠道队列、RPM、插件容量三类本地拒绝统一由 isLocalAdmissionError 分类，性能统计、模型熔断、渠道冷却和凭证失败规则共用它，不能将本地拒绝当成上游故障；同渠道重试也使用该分类，但插件过载禁止故障转移，队列/RPM 仍可换渠道。

ApplyRequestHookAttempt 位于性能记录、执行持久化、渠道限流之前。它提前失败时，后续 OnOutboundRawRequest 没有执行，但 pipeline 仍向所有中间件广播 OnOutboundRawError，因此 state.Perf / RequestExec 可能仍属于上一轮 attempt。插件过载时，错误回调及 orchestrator 最终收尾都不能覆盖上一轮执行记录；队列/RPM 拒绝通常已有本次记录，仍需持久化失败。资源清理回调仍须执行并支持尚未初始化的情况。

## 5. pass-through 渠道

pass-through 仍先回放客户端原始 body，并合并实际模型及 prompt protection 结果。onRequest 的非 model 改写仍可能在此被覆盖；宿主只对这些 onRequest 改写记录 `overriddenByPassThrough`，按插件、每次 attempt 至多计一次，并记 Warn。

onAttempt 在 pass-through 和协议 body 适配完成后运行，因此它的 replace/patch 正常生效，不记入 `overriddenByPassThrough`。未知上游字段在 patch 模式下保持；replace 模式下由插件负责返回希望保留的完整对象。之后渠道 override 对同一字段具有最终优先级，最终协议校验仍由宿主执行。

需要在 pass-through 上改供应商参数时，应注册对应 APIFormat 的 onAttempt；无须关闭 pass-through。multipart/二进制请求不进入 onAttempt。

## 6. 改动清单

- `internal/objects/plugin.go`、`plugin/manifest.go`：hooks 从布尔值改为包含显式 enabled 和匹配列表的对象；未知字段、枚举、重复值、非空范围、导出校验；升级范围扩大提示。
- `plugin/invoke.go`、事件派发：所有入口以 ctx 为第一个参数；在成功计数前完成返回契约校验。
- `plugin/hooks.go`：request 类型匹配，attempt 出站格式匹配；统一 replace/patch 处理、原子提交、保护字段、state、失败计数。
- `orchestrator`：将 onAttempt 从 OutboundLlmRequestMiddleware 移到 pass-through/Responses body 适配之后、渠道 override 之前的 raw-request 中间件；使用实际出站格式，修正过载分类和 pass-through 计数；不引入统一请求的隔离复制。
- manifest 展示、安装/升级预览、授权 UI：展示 enabled 与范围，新启用或扩大范围触发已有升级确认流程。
- 示例插件、运行时测试、双语开发指南：同步签名、manifest 和返回约定；保持 pluginApiVersion 1，不兼容未发布的旧契约。

## 7. 验收

| 类别 | 标准 |
| --- | --- |
| 注册 | enabled 必填；true 配空列表拒绝；false 保留列表但不执行；未知/重复枚举拒绝；省略不注册 |
| 匹配 | RequestType 相同但入站协议不同均进入 onRequest；onAttempt 按实际出站格式匹配，切渠道重新判断；只声明 attempt 也可执行 |
| 性能 | 不匹配时不借 VM、不做插件序列化；有无插件均不引入统一请求的 attempt 深拷贝 |
| replace | 对象整体替换，省略字段删除；onRequest 内部和二进制字段保留，音频/图片等接口不回归 |
| patch | 六种操作、嵌套对象、数组插入/追加/移动、Pointer 转义、根路径、空 patch；中途失败整组回滚 |
| 校验 | 各 APIFormat 的默认值与业务字段差异、保护丢弃的 Warn/计数/state 回滚、失败计数保持；patch 基于调用前对象；对象与数组返回都经过最终形态、类型和保护字段校验；连续解码失败可触发熔断 |
| 状态 | 成功含 undefined/空 patch 提交 state；失败、保护字段变化、reject 丢弃 state；跨 attempt 保留成功 state |
| 隔离 | 同渠道重试、故障转移均重新生成 wire body，不残留上次插件修改；私有 Go 值和 state.LlmRequest 的模型更新行为不因启用插件而改变；Generation 不因重试切换或提前释放 |
| 协议 | onAttempt 接收目标协议对象；multipart、二进制、非对象 JSON 跳过；模型在 URL、流式在 endpoint/WS 的情况也受保护 |
| 优先级 | pass-through 后 onAttempt 改写可达上游；仅 onRequest 被覆盖时增加计数；渠道 override 同字段优先 |
| 错误 | 抛错/超时与无插件行为一致；过载 429 不调用上游、不重试、不影响渠道/凭证健康和插件熔断 |
| 本地拒绝 | 队列/RPM/插件过载不推进凭证自动禁用计数，真实上游错误仍按阈值处理；第一次上游 500、第二次插件过载时客户端收到 429，第一次执行记录保留 500，主请求标为失败；队列/RPM 本次记录仍保存拒绝原因 |
| 签名 | onEvent(ctx, event)、onRequest(ctx, request)、onAttempt(ctx, body)；示例和调用端一致 |

## 8. 风险与妥协

onAttempt 第一版只支持 JSON 对象 body；multipart、二进制修改需另立契约。replace 由插件负责保留所需字段，patch 适合局部修改。插件仍受调用预算、输入限制、VM 池和熔断约束。响应钩子尚未开放，见 [response.md](response.md)。
