# Provider

[插件系统](README.md)的一个扩展点。插件提供自定义 channel 的出站转换：把统一的 `llm.Request` 变成上游 HTTP 请求，再把上游响应变回 `llm.Response`。选路、重试、故障转移、配额和计费仍由宿主完成。

本篇不在 [任务表](README.md#9-任务管理) 里。当前交付只做一件事：manifest 里出现 `providers` 就拒绝安装，见 [字段表](manifest.md#3-字段)。下面的接入、验收和妥协是开放该能力时的实现合同，不是本轮要写的代码。

onRequest 修改路由前的统一请求，onAttempt 修改已有 provider 转换后的上游 JSON body，见 [request.md](request.md)。两者都不能代替这里完整的出站转换。

## 1. 背景

渠道类型是 Ent 枚举。`internal/ent/schema/channel.go` 的 `type` 是一组固定字符串，`internal/ent/channel/channel.go` 的 `TypeValidator` 拒绝列表之外的值。`ChannelService.buildChannelWithTransformer`（`internal/server/biz/channel_llm.go`）按 `c.Type` 和 endpoint 的 `api_format` 构造 `transformer.Outbound`。这个接口在 `llm/transformer/interfaces.go`：`TransformRequest` 把 `*llm.Request` 变成 `*httpclient.Request`，宿主再发 HTTP。

每接一个上游就要加一个枚举值、改这处 switch，并在 `llm/transformer/` 里加实现。`llm/` 是独立模块，插件包不能被它 import。

### 约束

- 不改 `llm/` 的类型和已有供应商实现。插件适配器放在 `internal/server/plugin`，实现已经存在的 `transformer.Outbound`。
- 不给每个插件加一个枚举值。枚举值要进 schema 并重新生成，插件安装不能触发这件事。
- 插件不选择渠道，也不改调度。路由策略仍不是插件能力。
- JS 插件拿不到渠道密钥。密钥由宿主在发出请求前写入声明的认证头。
- 转换失败不能把未转换的统一请求发给上游。该渠道这次 attempt 失败，已有的故障转移继续。

## 2. 目标

开放之后：

1. 一个已启用插件可以声明一个或多个 provider。Owner 创建 `type=plugin` 的渠道，选定 `plugin_id` 和 `key`，填写 base URL、模型和 API Key。
2. 该渠道的 chat completion 由插件转换后发出。上游 URL 的 scheme、host、port 必须与渠道 base URL 一致。
3. 转换抛错、超时或返回非法值时，这次 attempt 失败且没有发出 HTTP。其他渠道的故障转移与现在相同。
4. 禁用插件或卸掉对应 `key` 后，引用它的渠道留在库里，但不再进入可用渠道缓存。

### 非目标

- 插件实现入站协议。客户端仍然只走现有的 OpenAI / Anthropic / Gemini 入口。
- 流式、embedding、image、video、模型自动拉取、OAuth。见妥协。
- 插件指定代理、超时或 HTTP 客户端。沿用该渠道已有的 `ChannelSettings` 和宿主 HTTP 客户端。
- 在请求钩子里伪装成新供应商。

## 3. User Stories

1. 作为系统 Owner，我希望安装一个插件后就能添加它声明的上游，而不用为这个上游发一版 AxonHub。
2. 作为系统 Owner，我希望插件改写请求时不能把渠道密钥送到 base URL 以外的主机。
3. 作为插件作者，我希望只实现统一请求到上游 HTTP 的转换，以便重试、计费和故障转移不用重写。

## 4. 决策

| 方案                                              | 结论                                                                                                     |
| ------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| 每个插件一个 `channel.type` 枚举                  | 放弃。安装插件不能改 Ent 枚举                                                                            |
| 用请求钩子改 body，渠道仍选 openai                | 放弃。钩子失败要放行；供应商转换失败必须拦住                                                             |
| 一种枚举值 `plugin`，设置里写 `pluginId` 和 `key` | 采用。插件的多个 provider 用 `key` 区分                                                                  |
| JS 直接拿 API Key 拼 Header                       | 放弃。与事件里不下发凭证明文同一原则                                                                     |
| JS 返回 HTTP 请求，宿主覆盖认证头并限制 URL 源    | 采用                                                                                                     |
| Go 内置插件在 `Apply` 里注册同一套 provider       | 采用。注册表见 [builtin-plugins.md](builtin-plugins.md)，未排期。工厂函数接收宿主给出的 `APIKeyProvider` |

## 5. 接入

manifest 增加字段，开放时并入 [manifest.md](manifest.md)。开放后 `schemaVersion` 和 `pluginApiVersion` 仍是 1：旧插件没有这个字段，旧版本 AxonHub 会拒绝带这个字段的包。

```json
"providers": [
  {
    "key": "chat",
    "apiFormat": "openai/chat_completions",
    "auth": { "header": "Authorization", "scheme": "bearer" },
    "title": "Acme"
  }
]
```

`key` 在该插件内唯一，`^[a-z0-9][a-z0-9-]{0,31}$`。`apiFormat` 第一版只能是 `openai/chat_completions`。`auth.header` 是一个 HTTP 头名称。`auth.scheme` 只能是 `bearer`（宿主写 `Bearer <key>`）或 `raw`（宿主把密钥原样写入该头）。`title` 用在渠道类型选择器里，接受字符串或多语言对象，校验和显示回退复用 [title](manifest.md#title)；GraphQL 与前端类型沿用页面的约定。

JS 必须导出 `transformRequest` 和 `transformResponse`。与 `plugin.js` 相同，允许 async，宿主等待 Promise。`transformRequest(ctx, req)` 返回 `{ method, url, headers, body }`。`ctx.channel` 只有 `id`、`name`、`baseUrl`、`providerKey`。没有密钥。转换函数可以 `await ctx.fetch`，目标仍须匹配已授权的路径或正则；发往渠道上游的 URL 仍受下面的 base URL 约束。`transformResponse(ctx, res)` 把上游状态码、头和 body 变成统一响应对象。

宿主包装成 `transformer.Outbound`：

```text
1. 调用 transformRequest。超时、抛错、非法返回：不发 HTTP，本次 attempt 失败。
   取不到 VM：按本地准入失败处理，与 ChannelQueueError 相同。不发 HTTP，不做同渠道重试，
   不计渠道错误和冷却，直接切到下一个候选渠道。这里和请求钩子不同，要切渠道：别的渠道不依赖这个插件。
2. 解析 url。scheme、host、port 必须与渠道 base URL 相同，否则同样失败。
3. 按不区分大小写的名称匹配，删除 JS 返回的 auth.header、Authorization、X-Api-Key 的所有条目和值，
   包括大小写变体和重复值；再按 auth.scheme 在 auth.header 写入唯一的宿主凭证值。
   JS 自带认证头不会导致拒绝请求，清除后使用宿主凭证继续发送。
4. 用该渠道现有的 HTTP 客户端发送；在代理等客户端配置完成后调用 WithRejectRedirects()，拒绝所有重定向。
   上游返回重定向时本次 attempt 失败，不向 Location 发出后续请求；沿用现有失败处理与故障转移。
5. 上游 2xx 交给 transformResponse。非 2xx 走 TransformError：第一版不调用 JS，映射成现有的上游错误，让故障转移识别。
```

Go 内置插件在 `Apply` 里 `RegisterProvider(key, factory)`，注册表见 [builtin-plugins.md](builtin-plugins.md)，当前不实现。`factory` 收到 base URL 和 `APIKeyProvider`，返回 `transformer.Outbound`。禁用时 `OnDispose` 卸掉。一个 `key` 不能同时由 JS 导出和 Go 注册。

`PassThroughBody` 对插件 provider 固定关闭。客户端原始 body 不能绕过转换。

渠道：

- `channel.type` 增加一个枚举值 `plugin`。这是开放该能力时唯一的 schema 变更，走 `make generate`。
- `ChannelSettings` 增加 `pluginProvider`：`{ "pluginId", "key" }`。这里沿用 `internal/objects/channel.go` 已有的 camelCase，与 manifest 一致。
- `buildChannelWithTransformer` 在 `c.Type == plugin` 时从当前 Generation 取 provider。找不到、插件未启用、或 `apiFormat` 不是声明值：构造失败，该渠道不进可用缓存。渠道行不改状态。日志写 `plugin_id` 和 `key`。
- 可用渠道缓存现在约 1 分钟刷新。禁用插件后，最迟下一次刷新这些渠道退出候选。
- 凭证沿用现有 API Key 列表。不支持 OAuth。

前端在渠道类型里增加 Plugin。选项来自已启用且声明了 `providers` 的插件。保存时 `pluginId` 和 `key` 必填。插件稍后被禁用时，渠道仍可打开，但不能被请求选中。

数值在进入任务表时并入 [运行限制](README.md#运行限制)，在此之前以本表为准：

| 项                                       | 限制                                  |
| ---------------------------------------- | ------------------------------------- |
| `transformRequest` / `transformResponse` | 与请求钩子相同，10 s，含 `await ctx.fetch` |
| 交给 JS 的请求或响应                     | 序列化后 2 MiB，超出则本渠道失败      |
| URL                                      | 仅渠道 base URL 的 scheme、host、port |
| `providers` 条目                         | 每个插件最多 8 个                     |

## 6. 当前交付

安装时按 [字段表](manifest.md#3-字段) 拒绝 `providers`（包括空数组）。不增加 `plugin` 枚举，不加 `pluginProvider` 字段，不注册转换函数。

## 7. 验收

开放时：

| 类别     | 标准                                                                                                                                   | 验证方式                                                               |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| 功能     | `type=plugin` 的渠道发出的 HTTP 是 `transformRequest` 的结果，认证头由宿主写入                                                         | `go test ./internal/server/biz/ -run TestPluginChannel`，httptest 上游 |
| 安全     | JS 返回 URL 的 scheme、host 或 port 与 base URL 不同：转换失败，不发 HTTP                                                              | 插件 Provider 测试，断言上游请求数为 0                                 |
| 安全     | JS 提供 auth.header、Authorization、X-Api-Key 的大小写变体或重复值：请求继续，上游只收到宿主写入的唯一认证头和值，没有 JS 提供的认证值 | httptest 上游检查完整请求头，覆盖三类名称与重复值                      |
| 安全     | 上游返回同源或跨源重定向：不请求 Location，本次 attempt 失败，可切换其他候选渠道                                                       | 两个 httptest 服务，覆盖 301/302/303/307/308，重定向目标请求数为 0     |
| 功能     | 转换超时或抛错时该渠道失败，候选列表中的下一个渠道仍收到请求                                                                           | orchestrator 集成测试，两个上游                                        |
| 功能     | 禁用插件后，下一次渠道缓存刷新不再选出该渠道                                                                                           | biz 缓存测试                                                           |
| 当前交付 | 带 `providers` 的包安装失败且不写库                                                                                                    | `go test ./internal/server/plugin/ -run TestParsePackage`              |

## 8. 风险与妥协

| 风险                           | 触发条件                                    | 影响                                | 对策                                                           |
| ------------------------------ | ------------------------------------------- | ----------------------------------- | -------------------------------------------------------------- |
| 转换把密钥抄进 body 或非认证头 | JS 从别处拼出密钥，或 Go provider 写进 body | 密钥出现在上游请求体                | JS 路径不提供密钥。管理页说明 Go provider 与内置供应商同等可信 |
| 禁用后短时间内仍被选中         | 渠道缓存尚未刷新                            | 最多约 1 分钟继续调用正在卸下的插件 | 刷新后构造失败即退出候选；进行中的请求使用开始时的 Generation  |

```text
1. 妥协：第一版只有非流式 openai/chat_completions
   - 原因：流式要按 chunk 进出 JS，延迟和内存都比单次转换大
   - 影响：插件渠道上 stream=true 的请求在转换前失败，由其他渠道接手
   - 升级触发：要对接只提供流式接口的上游
   - 后续方向：增加 transformStream，单 chunk 20 ms，与 [response.md](response.md) 的 onStreamChunk 同一预算

2. 妥协：不支持模型自动拉取和 OAuth
   - 原因：模型拉取和 OAuth 在各内置供应商上是独立实现，没有统一接口
   - 影响：插件渠道的模型手工填写，凭证只能是 API Key
   - 升级触发：这类上游无法手工维护模型列表，或只提供 OAuth
   - 后续方向：可选的 listModels 导出，以及宿主托管的 OAuth 配置档

3. 妥协：非 2xx 不交给 JS
   - 原因：错误体形态影响故障转移判断，第一版要和现有上游错误一致
   - 影响：插件不能把上游的业务错误改写成另一种状态码
   - 升级触发：上游把业务错误放在 200 以外且现有映射无法识别
   - 后续方向：可选 transformError，返回值限制为现有 llm.ResponseError
```
