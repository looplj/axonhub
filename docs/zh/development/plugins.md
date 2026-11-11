# 插件

---

AxonHub 插件是一个 `.axplugin` 压缩包。Owner 在「插件」页（`/plugins`）上传，按 manifest 的子集授权，然后启用。本文描述发布前修订的目标契约；钩子签名、注册和返回值已同步到运行时及示例脚本，见 [实现状态](../../solutions/plugin-system/README.md#9-任务管理)。pluginApiVersion 保持 1。

## 插件能做什么

- **事件。** 导出 `onEvent`，并在 `events` 里声明下面 8 种开放事件中的若干种。
- **请求钩子。** `onRequest` 每个请求调用一次，位于配额检查、模型权限检查、模型映射和选路之前。`onAttempt` 在每次匹配的上游尝试时接收目标协议的 JSON body，位于协议转换、pass-through 和 body 适配之后，渠道 override 之前。`onAttempt` 的改写只作用于这一次尝试。
- **页面。** `ui.pages` 的每一项是侧栏入口，也是一整页 iframe。
- **插件 KV。** `ctx.kv` 和页面上的 GraphQL KV 字段读写同一批记录。键属于这一次安装。
- **宿主代发的请求。** `plugin.js` 里用 `ctx.fetch`，页面里用 `axonhub.fetch`。目标必须匹配已授予的 `network` 规则。

`onRequest` 可以修改 `model`。随后的模型权限检查和模型映射按新模型执行。两个请求钩子都保持 `stream` 与输入一致。`onAttempt` 还要保持当次输入的 `model` 不变。

8 种事件：

| 事件 | 发布时机 |
| --- | --- |
| `channel.auto_enabled` | 渠道被自动重新启用 |
| `channel.credential_disabled` | 一把上游凭证被停用 |
| `channel.credential_enabled` | 一把上游凭证被启用 |
| `request.attempt_failed` | 一次上游尝试失败 |
| `request.failover` | 路由离开当前渠道 |
| `request.failed` | 请求在多次尝试后失败 |
| `request.completed` | 请求成功 |
| `api_key.quota_exceeded` | API Key 超出配额 |

事件对象使用 camelCase。带凭证的事件只有指纹和掩码，没有凭证原文。事件不含请求体和响应体。投递最多一次。插件不会收到自己造成的事件。`onEvent` 的返回值不会被当作改写；抛错或超时会丢掉这一次投递，并且不重试。

下面 4 个名字仍由现有 WebHook 发送，写进 `events` 会被拒绝：`channel.auto_disabled`、`provider.quota.warning`、`provider.quota.exhausted`、`provider.quota.recovered`。

### 事件 payload

`onEvent(ctx, event)` 的 `event` 是普通 JS 对象，所有事件共用以下字段：

| 字段 | 类型与含义 |
| --- | --- |
| `id` | string，事件 UUID |
| `type` | string，上表中的事件名 |
| `severity` | string，当前事件使用 `info`、`warning` 或 `error`，具体见下表 |
| `occurredAt` | string，UTC RFC 3339 时间 |
| `subject` | object，相关对象；没有上下文的成员直接省略，不填 `null` |
| `data` | object，各事件的数据；无数据时为 `{}` |
| `origin` | string，普通业务事件为 `""`，插件引起的事件为 `"plugin:<插件 id>"` |
| `depth` | number，普通业务事件为 `0`，插件引起的后续事件递增 |

`subject` 成员的结构如下。这里的 `id` 都是数字，`model` 是字符串；不要把这些 id 当作 GraphQL 全局 ID。

| 成员 | 结构 |
| --- | --- |
| `channel` | `{ id, name, type, baseUrl, status }`，除 id 外都是字符串 |
| `credential` | `{ fingerprint, masked }`，都是字符串；普通密钥的 fingerprint 为 SHA-256 的前 12 位十六进制，OAuth 的两个值均为 `"oauth"` |
| `model` | 模型名；没有模型时省略 |
| `apiKey` | `{ id, name }`，name 为字符串 |
| `project` | `{ id }` |
| `request` | `{ id }` |

下表的 subject 列列出可能提供的成员。请求尚未持久化、没有 API Key（例如 Playground）或没有取得上游凭证时，对应成员可能缺省。应先检查成员存在再访问其字段。

| `type` | `severity` | `subject` 成员 | `data` 字段及类型 |
| --- | --- | --- | --- |
| `channel.auto_enabled` | `info` | `channel` | `{}` |
| `channel.credential_disabled` | `warning` | `channel`, `credential` | `errorCode: number`, `reason: string`, `expiresAt: string 或 null`（UTC RFC 3339；null 表示没有到期时间） |
| `channel.credential_enabled` | `info` | `channel`, `credential` | `trigger: "manual" 或 "expired"` |
| `request.attempt_failed` | `warning` | `channel`, `credential`, `model`, `apiKey`, `project`, `request` | `attempt: number`, `statusCode: number`, `errorType: string`, `errorMessage: string` |
| `request.failover` | `warning` | `channel`（离开的渠道）, `model`, `apiKey`, `project`, `request` | `toChannelId: number`, `toChannelName: string`, `attempt: number`（离开的尝试） |
| `request.failed` | `error` | `channel`（最后一次渠道）, `model`, `apiKey`, `project`, `request` | `statusCode: number`, `errorType: string`, `errorMessage: string`, `attempts: number` |
| `request.completed` | `info` | `channel`, `model`, `apiKey`, `project`, `request` | `latencyMs: number`, `promptTokens: number`, `completionTokens: number`, `attempts: number`, `stream: boolean` |
| `api_key.quota_exceeded` | `warning` | `apiKey`, `project`, `model`, `request` | `profile: string`（配额配置名）, `window: { start: string 或 null, end: string 或 null, endInclusive: boolean }` |

`attempt` 是尝试编号，`attempts` 是尝试次数，未获得计数时为 `0`。没有 HTTP 状态时 `statusCode` 为 `0`；没有错误类型时 `errorType` 为 `""`。请求错误的 `errorMessage` 最多保留 512 个 UTF-8 字节。没有 usage 时 token 数为 `0`，不能据此断言实际未消耗 token。配额窗口的 start/end 为 UTC RFC 3339，`null` 表示该侧没有边界，endInclusive 表示结束边界是否包含在窗口内。

```js
export function onEvent(ctx, event) {
  if (event.type !== "request.failover") return;
  const channel = event.subject.channel;
  if (!channel) return;
  ctx.log.info("request failover", {
    eventId: event.id,
    fromChannelId: channel.id,
    toChannelId: event.data.toChannelId,
    attempt: event.data.attempt,
  });
}
```

## 尚未开放的能力

这些名字会被拒绝，或者当前没有对应的作者接口：

- 响应钩子（`onResponse`、`onStreamChunk`）
- Dashboard 插槽
- 自定义渠道 Provider
- 按 project、API Key 或模型收窄请求钩子
- 内置插件
- `ctx.models`
- 包签名
- 浏览器直接连接外部 URL

## 包结构

`.axplugin` 是 zip：

```text
manifest.json
plugin.js          可选；声明了 events 或 hooks 时必须有
ui/**              html、js、css、svg、png、woff2
```

只有页面的插件可以省略 `plugin.js`。页面脚本在浏览器里运行，不会被当成后端钩子。

`plugin.js` 可以 import 包内相对路径，例如 `import { failoverKey } from "./lib/key.js"`。打包和安装接受 `manifest.json`、`ui/` 下的文件，以及 `plugin.js` 的静态 import 闭包。`node:` 和裸 npm 模块名会被拒绝。npm 依赖由作者自己打进包里。`eval` 和动态代码生成已禁用。顶层 `await` 会导致安装失败。钩子导出可以是 async。

## 打包

打包器是独立模块 `github.com/looplj/axonhub/cmd/pack-plugin`，只依赖 moejs。它复制了服务端的 zip 读取和静态 import 编译，不执行插件。安装时的检查仍会再看 manifest、AxonHub 版本范围和命名导出，打包通过的 zip 仍可能被安装拒绝。

安装 `unstable` 分支上的打包器：

```bash
go install github.com/looplj/axonhub/cmd/pack-plugin@unstable
pack-plugin <plugin-directory> -o <outside>.axplugin
```

`@unstable` 选择该分支的当前提交，Go 把它记成 `v0.0.0-<时间>-<提交>`。打包器发布子目录标签 `cmd/pack-plugin/vX.Y.Z` 之后，`@latest` 选择这些标签里的最新一个。AxonHub 根模块的 `vX.Y.Z` 不会成为这个模块的版本；要和某次 AxonHub 发布用同一套检查时，安装同号的 `cmd/pack-plugin/vX.Y.Z`。

在本仓库里：

```bash
go run -C cmd/pack-plugin . ../../plugins/examples/<name>
```

默认产物是插件目录旁边的 `<目录名>.axplugin`。输出必须写在插件目录外面，写进目录里会失败。`*.axplugin` 已被 Git 忽略。包里只有 `manifest.json`、`ui/` 下的文件、根目录 `plugin.js`，以及 `plugin.js` 通过静态相对 import 到达的文件。其他文件，例如 `README.md`、`.git`、`.github`、`.claude`、`node_modules`、测试文件和 `package.json`，都不会进包，命令会列出这些被跳过的路径。整个目录都没有文件进包时只列一行，例如 `.git/`。只通过动态 `import()` 加载的文件不会被收集，上传前请检查跳过列表。符号链接只有在会进包的位置才会让打包失败：`manifest.json`、`plugin.js`、`ui/` 下的任意路径，或被 import 的文件。

本地试用时，把这个 zip 上传到 `/plugins`。发布到官方索引是后面的另一步。

## manifest.json

字段使用 camelCase。未开放的字段在安装时被拒绝，不会写入。这包括 `hooks.response`、`hooks.streamChunk`、`ui.panels`、`providers`、权限名 `fetch`，以及任何未知字段。

| 字段 | 必填 | 规则 |
| --- | --- | --- |
| `schemaVersion` | 是 | `1` |
| `id` | 是 | `^[a-z0-9]+(\.[a-z0-9-]+)+$`，最长 64 个字符。安装后不可改。`axonhub.webhook` 是保留 id |
| `name` | 是 | 非空字符串 |
| `version` | 是 | 语义化版本 |
| `description` | 否 | 字符串 |
| `axonhubVersion` | 是 | npm 风格的版本范围。建议写 `^1.0.0`。比较前会去掉正在运行的 AxonHub 版本的预发布后缀，因此 `v1.0.0-beta9` 满足 `^1.0.0` |
| `pluginApiVersion` | 是 | `1` |
| `permissions` | 否 | 取下面的权限名。省略为 `[]` |
| `network` | 否 | 最多 32 条。省略为 `[]` |
| `events` | 否 | 取上面 8 种事件名。非空时必须 `export function onEvent` |
| `hooks.request` | 否 | 对象：enabled 必填；true 时 requestTypes 必须非空且导出 onRequest；false 保留配置但不执行 |
| `hooks.attempt` | 否 | 对象：enabled 必填；true 时 apiFormats 必须非空且导出 onAttempt；false 保留配置但不执行 |
| `configSchema` | 否 | JSON Schema 对象，见下文 |
| `ui.pages` | 否 | 侧栏页面，见下文 |
| `ui.actions` | 否 | 全局工具栏操作，点击打开插件弹窗 |

已开放的权限：`read_channels`、`read_requests`、`read_dashboard`、`read_api_keys`、`read_projects`、`read_settings`、`write_channels`、`webhook`。出站 URL 写在 `network`。`read_plugins` 和 `write_plugins` 是用户 scope，不是插件权限。KV 不占用权限。

一条 `network` 规则要么是精确的 `https` URL，要么是 `{ "regex": "^https://api\\.example\\.com/v1/items/[0-9]+$" }`。精确 URL 必须带 path，不能有 userinfo、query 或 fragment，并且只匹配这条 path。正则对去掉 query 和 fragment 后的 URL 做全匹配，最长 256 个字符。实际请求仍可以带 query。宿主不跟随重定向；301、302、303、307、308 会使 `ctx.fetch` 和 `axonhub.fetch` 以 `INVALID_STATE` 拒绝 Promise。

`configSchema.type` 为 `object`。每个属性的类型是 `string`、`integer`、`number`、`boolean` 或 `array`，可以带 `enum`、`minimum`、`maximum`、`default`、`title`、`description` 和 `format: "secret"`。`array` 必须写 `"items": {"type": "string"}`，表示字符串列表（例如 `{"type": "array", "items": {"type": "string"}, "default": ["gpt-4o"]}`），不支持 `enum` 和 `format`；设置表单会显示为可增删的列表。`configSchema.required` 列出插件必需的属性名。新安装的插件默认停用；停用时可以只保存部分配置，但启用、保存已启用插件的配置、升级已启用插件时，每个必填字段都要有值或默认值（空字符串和空数组视为缺失），否则失败。未声明的字段会被拒绝。`ctx.config` 是已保存的配置，包含 secret 原文。Owner 还没保存的字段不会出现，脚本里要自己使用 schema 的默认值。设置表单会显示这些默认值。GraphQL 和页面拿到已保存 secret 时是 `******`；把这个掩码原样提交回去，已保存的值保持不变。

`ui.pages[]` 必须有 `key`、`entry` 和 `title`。`key` 匹配 `^[a-z0-9][a-z0-9-]{0,31}$`，并且在该插件内唯一。`entry` 是 `ui/` 下已经存在的 `.html`。`title` 可以是字符串，也可以是至少包含非空 `en` 的语言映射。侧栏按当前语言、`en`、`key` 的顺序取值。

## plugin.js

宿主只调用命名导出。不会从 `export default` 里发现钩子，也没有注册函数。manifest 声明了的钩子必须导出为函数。manifest 没有声明的导出不会被调用。

所有入口统一 ctx 在前：`onEvent(ctx, event)`、`onRequest(ctx, request)`、`onAttempt(ctx, body)`。

`onRequest` 按 RequestType 匹配，接收统一请求公开字段的 JS 对象，沿用 snake_case。内部记账字段和音频、图片等非公开数据由宿主保留，不参与 replace/patch。`onAttempt` 按本次实际出站 APIFormat 匹配，接收目标协议的 JSON body 对象，不是 llm.Request、字符串或字节数组。multipart、二进制及非对象 JSON body 不进入 attempt。

```json
{
  "hooks": {
    "request": { "enabled": true, "requestTypes": ["chat"] },
    "attempt": { "enabled": true, "apiFormats": ["openai/chat_completions"] }
  }
}
```

声明钩子对象时 enabled 必填；启用时范围列表必须非空。禁用时可以保留范围配置。省略钩子对象表示未声明；空列表不是“全部”，不根据列表推断启停。范围为下面枚举的精确值，不接受未知值、重复项或通配符。整个插件的 enabled 仍是总开关。两个钩子独立匹配，重试按实际出站格式重新选择 attempt；未执行 request 不妨碍 attempt 执行。

`requestTypes`：`chat`、`embedding`、`rerank`、`image`、`video`、`compact`、`completion`、`speech`、`transcription`、`translation`、`moderation`、`alpha_search`、`systemone`、`decisions`。

`apiFormats`：`openai/chat_completions`、`openai/completions`、`openai/responses`、`openai/responses-ws`、`openai/responses_compact`、`openai/image_generation`、`openai/image_edit`、`openai/image_variation`、`openai/embeddings`、`openai/video`、`openai/audio_speech`、`openai/audio_transcriptions`、`openai/audio_translations`、`openai/moderations`、`openai/alpha_search`、`gemini/contents`、`anthropic/messages`、`aisdk/text`、`aisdk/datastream`、`gemini/embeddings`、`jina/rerank`、`jina/embeddings`、`typesafe/systemone`、`openai/decisions`、`ollama/chat`、`seedance/video`、`zenmux/video`、`modelscope/image_generation`。

`onRequest` 拿到的是公开 JSON。宿主会去掉 `raw_request`、`request_type`、`api_format`、`transform_options`、`transformer_metadata` 和 provider extensions。请求类型和协议看 `ctx.request`。带 `json:"-"` 的字节不会出现，包括图片字节、音频字节和 alpha search 的原始 body。各类型上常见的文本位置：

| 类型 | 公开字段 |
| --- | --- |
| `chat` | `messages` |
| `embedding` | `embedding.input`。字符串会在；token 数组没有文本 |
| `rerank` | `rerank.query`、`rerank.documents` |
| `image` | `image.prompt` |
| `video` | `video.content` 里 `type` 为 `text` 的 `text` |
| `compact` | `compact.instructions`、`compact.input` |
| `completion` | `completion.prompt`、`completion.suffix` |
| `speech` | `speech.input`、`speech.instructions` |
| `transcription` | `transcription.prompt` |
| `translation` | `translation.prompt` |
| `moderation` | `moderation_request.input`，字符串、字符串数组或多模态 part 的 `text` |
| `alpha_search` | 原始 body 不在公开 JSON 里 |
| `systemone` | `systemone.state` |
| `decisions` | `decisions_request.input` |

宿主统一解析出站格式：优先 HTTP 请求的 APIFormat，其次是候选渠道格式，最后是 transformer 默认格式。钩子匹配与 ctx.attempt.apiFormat 使用同一个结果。格式未匹配而跳过时记录 Debug 日志，包含插件、渠道、格式和原因。

| 返回或调用 | 结果 |
| --- | --- |
| JSON 对象 | replace：整体替换公开对象，省略字段删除，不做 merge |
| 数组 | patch：RFC 6902 JSON Patch 操作列表；空数组为成功的无修改操作 |
| undefined 或无返回 | 保留调用前对象，忽略对参数的原地修改 |
| null、标量、非法返回、抛错或超时 | 保留原值，丢弃本次 state，计一次失败 |
| ctx.reject(400 / 403 / 429, message) | 仅 onRequest，停止请求；message 非空，属于业务拒绝，不计失败 |

patch 支持 [RFC 6902](https://www.rfc-editor.org/rfc/rfc6902) 的 add/remove/replace/move/copy/test，路径使用 [RFC 6901 JSON Pointer](https://www.rfc-editor.org/rfc/rfc6901)，例如 `/messages/0/content`、数组追加 `/messages/-`。它作用于调用前对象，任一步失败或最终结果不是对象都整组回滚。replace 和 patch 都先校验类型与保护字段，再提交请求和 state；后续插件看到已提交的结果。

onRequest 可以修改 model，随后仍过权限检查和映射。对于 speech 请求，onRequest 还必须保留 `speech.stream_format`（包括是否省略），因为宿主在执行钩子前已经选定响应输出方式。两个钩子均不能改变流式语义；onAttempt 也不能改变已选定模型，包括删除或通过根替换间接改变。宿主按协议校验 body、endpoint/WS 的相关控制语义；违反保护约束丢弃本次修改和 state，不计失败。ctx.reject 被捕获后仍然拒绝。

保护规则按出站 APIFormat 定义：例如聊天 stream 缺省等价于 false，Ollama 则等价于 true；type/generate 只在 Responses WebSocket 中按控制字段处理，stream_format 只在 speech 中处理。保护规则导致改写丢弃时，protectedDrops 每次调用增加一次，管理页卡片和详情可见；宿主记录含插件、钩子、请求 ID、格式、字段名和原因的 Warn，不记录字段值。丢弃请求修改和 state，连续失败计数保持不变。

```js
export function onEvent(ctx, event) {
  ctx.log.info(event.type);
}

export function onRequest(ctx, request) {
  if (request.model === "blocked-model") ctx.reject(403, "model is disabled");
  return { ...request, temperature: 0.2 }; // replace
}

export function onAttempt(ctx, body) {
  return [{ op: "add", path: "/temperature", value: 0.2 }]; // patch
}
```

插件按 priority 升序执行，新安装默认 100。每次 attempt 重新生成上游请求，不继承上次尝试的 body 修改。attempt 钩子不写回 llm.Request，也不触发整个统一请求的深拷贝。上传文件等私有缓冲区始终由宿主持有，onRequest 提交公开字段时保留原引用。

pass-through 会覆盖 onRequest 的非 model 改写，宿主记录 overriddenByPassThrough 和 Warn。onAttempt 在 pass-through 后运行，其 replace/patch 正常生效；随后渠道 override 同字段优先。使用 attempt 修改透传 body 不需要关闭 pass-through。

ctx.request 使用 camelCase：projectId、apiKeyId、model、requestType、apiFormat，值来自进入中间件时；Playground 的 apiKeyId 为 null。ctx.attempt 仅在 onAttempt 提供 channel（id、name、type）、实际 model 和出站 apiFormat，不含凭证。body 不一定含 model；从 ctx.attempt.model 读取实际模型。插件无法通过此接口修改 URL、方法、HTTP 头或凭证。

## ctx

使用本次调用传入的 `ctx`。把 `ctx` 或其方法存进模块级绑定，下一次调用也不能拿来用。

| 成员 | 适用范围 |
| --- | --- |
| `ctx.config` | 本插件已保存的配置。只读 |
| `ctx.log.info` / `warn` / `error` | 插件日志。第二个参数是 JSON 对象。不要写入密钥或完整请求体 |
| `ctx.kv.get` / `set` / `delete` / `list` | 本次安装的 KV。值是 UTF-8 字符串，最大 256 KiB。namespace 和 key 匹配 `^[A-Za-z0-9_.:-]{1,128}$`，namespace 默认 `default`。一次调用最多写 10 次，同一插件每分钟最多写 60 次。键不存在或已过期时 `get` 返回 `null`。`ttlSeconds` 为 `0` 或省略表示不过期。升级保留这些记录；删除后用同一个 id 再安装会从空存储开始 |
| `ctx.fetch(url, init)` | 宿主代发的 HTTPS。返回 `{ status, headers, body }` 的 Promise。普通非 2xx 响应也会 resolve，由脚本看 `status`；301/302/303/307/308 会以 `INVALID_STATE` reject，不跟随重定向。没有全局 `fetch` |
| `ctx.request` | 上面的请求元数据 |
| `ctx.attempt` | 仅 `onAttempt` |
| `ctx.state` | 本次请求、本插件的 JSON 对象。同一请求里的下一个钩子会看到已提交的对象，即使换了一台 VM |
| `ctx.reject(status, message)` | 仅 `onRequest` |

`ctx.state` 初始为 `{}`。钩子成功返回时提交，包括返回 `undefined`。抛错、超时、非法返回或 `ctx.reject` 会丢掉这次对 state 的修改。已经写入的 KV 不会撤回。

下面三项只在 `onEvent` 里提供：

- `ctx.counter.add(key, windowSeconds)` 固定加 1，并返回窗口内的计数。`ctx.counter.get` 只读。同一插件、同一 key、同一窗口是一个计数器。`memory` 缓存下按实例计数；配置了 Redis 时各实例共享。
- `ctx.webhook.send(targetName, payload)` 按已有 WebHook 目标名发送，不接受 URL。需要已授予 `webhook`。字符串 `payload.message` 会填进模板的 Message。
- `ctx.axonhub.channels.disable(id, options)` 禁用该渠道。需要已授予 `write_channels`。渠道已经是禁用状态时，这次调用仍然成功。

模块级变量会在 VM 复用后保留，超时后 VM 也可能被丢掉。这里只放常量和可以重建的缓存。一次请求内的数据放 `ctx.state`。重启后还要在的数据放 `ctx.kv`。

```js
ctx.kv.set("cursor", "10", { ttlSeconds: 3600 });
const raw = ctx.kv.get("cursor");
```

### KV 调用与分页

这些 `ctx.kv` 方法同步返回，失败时抛出带 `code` 的 `PluginHostError`：

| 调用 | 返回值 |
| --- | --- |
| `ctx.kv.get(key, { namespace }?)` | 字符串值；不存在或已过期时为 `null` |
| `ctx.kv.set(key, value, { namespace, ttlSeconds }?)` | `undefined`；value 必须是字符串，JSON 对象先 `JSON.stringify` |
| `ctx.kv.delete(key, { namespace }?)` | boolean，是否删除了记录 |
| `ctx.kv.list({ namespace, prefix, first, after }?)` | 下述 `KVPage` 对象 |

options 对象及其字段均可省略。`namespace` 默认 `"default"`；`prefix` 默认 `""`，按 key 前缀过滤；`first` 为 1–100 的整数，默认 100；`after` 为上页的 `nextCursor` 字符串，省略或 `""` 表示第一页。不要把末页的 `null` 传给 `ctx.kv.list`。

```ts
type KVEntry = {
  namespace: string;
  key: string;
  value: string;
  updatedAt: string;
  expiresAt: string | null;
};
type KVPage = { entries: KVEntry[]; nextCursor: string | null };

// ctx.kv.list(options?) -> KVPage
// options: { namespace?: string; prefix?: string; first?: number; after?: string }
```

结果只包含未过期记录，按 key 升序排列，并排除游标指向的 key。`nextCursor` 为下一页游标，末页为 `null`；空结果的 `entries` 为 `[]`。`updatedAt` 和非 null 的 `expiresAt` 是 UTC RFC 3339 时间，可带小数秒。翻页时保持 namespace/prefix 不变；每页读取当前数据，不提供跨页快照。

```js
export function onEvent(ctx, event) {
  let after;
  do {
    const page = ctx.kv.list({ namespace: "default", prefix: "note.", first: 50, after });
    for (const entry of page.entries) {
      ctx.log.info("stored key", { key: entry.key, updatedAt: entry.updatedAt });
    }
    after = page.nextCursor;
  } while (after !== null);
}
```

页面通过 `axonhub.graphql` 调用 `pluginKVList(namespace, prefix, first, after)`，返回字段也是 `entries` 和 `nextCursor`，但需要在 GraphQL selection 中显式选择。GraphQL 的 `after` 可以传 `null` 表示第一页。`pluginKV` 返回完整条目或 `null`，`setPluginKV` 返回条目，`deletePluginKV` 返回 boolean；与返回字符串的 `ctx.kv.get` 区分。

## 全局工具栏操作

`ui.actions` 将操作注册到公共布局右上角。宿主渲染按钮，点击后才加载插件 iframe；桌面最多直接展示两个按钮，其余进入更多菜单，移动端统一使用菜单。示例：

```json
"ui": {
  "actions": [{
    "key": "notes",
    "slot": "app.header.actions",
    "title": { "en": "Quick notes", "zh-CN": "随手记" },
    "icon": "puzzle",
    "effect": { "type": "dialog", "entry": "ui/notes.html" }
  }]
}
```

首版 `slot` 只接受 `app.header.actions`，`effect.type` 只接受 `dialog`。key、title 和 entry 遵循页面规则，action key 在 actions 内唯一。icon 可省略（默认 puzzle），支持 puzzle/search/activity/settings。未知字段、插槽或 effect 会被拒绝；不支持 panels 和渠道操作菜单。

弹窗使用同一份 Bridge、资源规则和权限机制，只有 actions、没有 pages 的插件也可调用 GraphQL、KV 和已授权的 fetch。初始化包含 `action` key、`slot`、`context: {}`、`page: null`、`panel: null`。全局操作的 `projectId` 固定为 null，调用不携带当前项目头；普通页面继续使用原有项目上下文。关闭弹窗取消在途请求，重新打开使用独立会话。navigate 会关闭弹窗。

新增 action 会进入升级预览。可打包示例见 [toolbar-notes](../../../plugins/examples/toolbar-notes/README.md)。

## 页面

每个页面是认证布局主区域里的 iframe。侧栏入口在「插件」分组。路由是 `/plugins/<插件 id>/pages/<key>`。iframe 读不到宿主的 cookie 和 `localStorage`。

资源 URL 相对于引用它的文件。以 `/` 开头的路径会丢掉插件资源 token。`bridge.js` 由宿主提供，不要放进包里。`ui/notes/index.html` 里写成 `../../_axonhub/bridge.js`。`ui/index.html` 里写成 `../_axonhub/bridge.js`。CSS 的 `url(...)` 相对于 CSS 文件。交付的构建产物使用相对 URL（Vite 配置 `base: './'`）。

页面使用 `window.axonhub`。先调用一次 `axonhub.ready()`，再用 `axonhub.graphql(query, variables)` 和 `axonhub.fetch(url, init)`。两者都由宿主代理。GraphQL 的权限是插件授权与当前登录用户权限的交集。`axonhub.fetch` 和 `ctx.fetch` 使用同一份已授予的 `network` 规则。页面 CSP 的 `connect-src` 保持 `'none'`，浏览器不会自己连接外部 URL。`axonhub.navigate` 接受以单个 `/` 开头的站内路径。`axonhub.toast` 显示一条纯文本通知。

页面 KV 使用插件 GraphQL 字段 `pluginKV`、`pluginKVList`、`setPluginKV`、`deletePluginKV`，和 `ctx.kv` 是同一批记录。能打开页面的人都可以读。写入要求当前登录用户有 `write_plugins`。`init.config` 与设置表单一致：补上 schema 默认值，已保存的 secret 显示为 `******`。

### 页面桥接 API

加载宿主提供的 `bridge.js` 后使用 `window.axonhub`。`ready()` 返回初始化快照；`onInit` 收到相同结构，`onTheme` 只收到主题部分：

```ts
type ThemeSnapshot = {
  theme: "light" | "dark";
  colorScheme: string;
  tokens: Record<string, string>;
};
type InitSnapshot = ThemeSnapshot & {
  locale: string;
  projectId: number | null;
  page: string | null;
  panel: null;
  action: string | null;
  slot: "app.header.actions" | null;
  context: Record<string, unknown>;
  config: Record<string, unknown>;
  canWriteKV: boolean;
};
```

`locale` 是当前界面语言；`projectId` 是所选项目的数字 ID，没有有效选择时为 null；`page` 是 manifest 中的页面 key。`config` 包含配置默认值，secret 为掩码；`canWriteKV` 表示当前用户能否写入插件 KV。快照深度冻结，不可修改。

| 方法 | 参数与返回值 |
| --- | --- |
| `ready()` | `Promise<InitSnapshot>`；开始握手，重复调用返回同一个 Promise |
| `onInit(handler)` | handler 接收 `InitSnapshot`；返回取消订阅函数 `() => void` |
| `onTheme(handler)` | handler 接收 `ThemeSnapshot`；返回取消订阅函数 `() => void` |
| `graphql(query, variables?, options?)` | `Promise`，返回完整 GraphQL 响应 `{ data?, errors?, extensions? }`；不是直接返回 data |
| `navigate(path)` | `Promise<null>`；path 是单个 `/` 开头的站内路径，不能含 `//`、反斜杠或控制字符 |
| `toast(message, options?)` | `Promise<null>`；message 为非空纯文本；options 为 `{ kind?: "info" / "success" / "warning" / "error" }`，默认 info |
| `fetch(url, init?)` | `Promise<{ status: number, headers: Record<string, string>, body: string }>`；不是浏览器的 Response 对象 |

`onInit` 在握手成功后调用一次；在初始化完成后订阅，会在下一次微任务收到初始化快照。`onTheme` 在初始化时先提供一次主题，之后在收到主题更新时再次调用；初始化后订阅同样会异步收到当前主题。两者取消订阅后都不再调用。仅注册监听器不会启动握手，仍需调用 `ready()` 或其他异步方法。回调中的同步异常会报告为插件页面错误，不会阻断其他监听器；回调里启动的异步工作应自行处理 rejection。

`graphql` 的 query 必须是非空字符串，variables 是 JSON 对象，默认 `{}`；options 为 `{ operationName?: string }`，query 包含多个 operation 时必须指定非空 operationName。页面没有直接的 `axonhub.kv`，KV 查询示例见下方。

`fetch` 的 init 为 `{ method?, headers?, body? }`：method 只能是大写的 `GET`、`POST`、`PUT`、`PATCH`、`DELETE`，默认 GET；headers 是字符串值的对象，body 是字符串。响应 headers 的名称为小写，body 是 UTF-8 文本；JSON 响应需自行 `JSON.parse(result.body)`。普通 HTTP 错误（例如 404、500）仍返回结果，检查 result.status。**301、302、303、307、308 是例外：宿主不跟随重定向，也不返回该响应，而是以 `INVALID_STATE` 拒绝 Promise。** `ctx.fetch` 也遵循这个重定向规则。

#### 返回值与错误处理

`graphql`、`navigate`、`toast` 和 `fetch` 在握手完成前自动等待 ready。握手超时为 10 秒；发送后的桥接调用超时为 15 秒。`navigate` 的成功只表示宿主接受了导航，不保证目标页面已加载；若导航销毁当前 iframe，未完成调用可能以 `DISPOSED` 结束。

桥接失败时 Promise reject 一个 `Error`，`name` 为 `AxonhubBridgeError`，有 `code` 和 `message`，可能有数字 `status`。status 是宿主接口的 HTTP 状态，不是 fetch 目标响应里的 result.status。错误消息供展示，分支判断使用 code。

| `code` | 含义 |
| --- | --- |
| `INVALID_ARGUMENT` | 桥接参数校验失败 |
| `NOT_READY` | 没有可用的握手会话 |
| `UNAUTHENTICATED` / `FORBIDDEN` / `NOT_FOUND` / `RATE_LIMITED` | 宿主接口分别返回 401 / 403 / 404 / 429 |
| `TIMEOUT` | 握手、调用或代理请求超时 |
| `NETWORK_ERROR` | 网络或代理传输失败 |
| `INVALID_STATE` | 出站请求遇到上述重定向，或响应不是有效 UTF-8 等状态错误 |
| `DISPOSED` | 页面或桥接会话已销毁 |
| `INTERNAL` | 其他宿主错误或无效响应 |

GraphQL 的 HTTP 请求成功时，即使响应包含 `errors`，Promise 仍 resolve；必须检查 `result.errors`，按需读取 `errors[].extensions.code`，并决定是否使用部分 data。这与 `catch` 捕获的桥接错误是两个层次。

```js
const axonhub = window.axonhub;
const stopInit = axonhub.onInit((init) => {
  document.documentElement.lang = init.locale;
});
const stopTheme = axonhub.onTheme(({ theme }) => {
  document.documentElement.dataset.theme = theme;
});
window.addEventListener("pagehide", () => {
  stopInit();
  stopTheme();
}, { once: true });

async function readFirstPage() {
  try {
    const init = await axonhub.ready();
    const result = await axonhub.graphql(`query Notes($after: String) {
      pluginKVList(prefix: "note.", first: 20, after: $after) {
        entries { namespace key value updatedAt expiresAt }
        nextCursor
      }
    }`, { after: null });
    if (result.errors?.length) {
      console.error("GraphQL errors", result.errors);
      return;
    }
    console.log(init.canWriteKV, result.data.pluginKVList);
  } catch (error) {
    console.error("Bridge call failed", error.code, error.status, error.message);
  }
}
readFirstPage();
```

### 页面主题

插件页面跑在沙箱 iframe 里，样式表由插件自己携带。宿主只把已经算好的主题变量写进页面，不提供组件，也不把控制台的 CSS 文件交给插件。

握手成功时，以及用户切换明暗或配色时，`bridge.js` 把父页面 `<html>` 上的计算值写到插件页面的 `<html>`。同时按当前明暗设置 `color-scheme`。页面用同名变量引用：

```css
body {
  background: var(--background, #f4f5f7);
  color: var(--foreground, #1c1f24);
  font-family: var(--font-sans, ui-sans-serif, system-ui, sans-serif);
}

code, .mono {
  font-family: var(--font-mono, ui-monospace, monospace);
}

.card {
  background: var(--card, #ffffff);
  color: var(--card-foreground, inherit);
  border: 1px solid var(--border, #e3e6eb);
  border-radius: var(--radius, 0.75rem);
  box-shadow: var(--shadow-sm);
}

button {
  background: var(--primary, #1c1f24);
  color: var(--primary-foreground, #ffffff);
  border-radius: var(--radius, 0.75rem);
}

button.secondary {
  background: var(--secondary, transparent);
  color: var(--secondary-foreground, inherit);
}

input, textarea {
  border: 1px solid var(--input, #e3e6eb);
  border-radius: var(--radius, 0.75rem);
}

input:focus-visible {
  outline: 2px solid var(--ring, transparent);
}

.popover {
  background: var(--popover, #ffffff);
  color: var(--popover-foreground, inherit);
  box-shadow: var(--shadow-md);
}
```

`var()` 的第二个参数只在握手完成前，或当前配色没有这个变量时生效。宿主写上变量之后，回退色不再使用。

`--muted` 是浅色表面，次要文字用 `--muted-foreground`。`--accent` 也是表面色，不是按钮主色；按钮主色是 `--primary`。

#### 会传递的变量

| 用途 | 变量 |
|------|------|
| 页面底色和正文 | `--background`、`--foreground` |
| 卡片 | `--card`、`--card-foreground` |
| 弹出层 | `--popover`、`--popover-foreground` |
| 主按钮 | `--primary`、`--primary-foreground` |
| 次按钮 | `--secondary`、`--secondary-foreground` |
| 弱化表面和次要文字 | `--muted`、`--muted-foreground` |
| 高亮表面 | `--accent`、`--accent-foreground` |
| 危险操作 | `--destructive`、`--destructive-foreground` |
| 边框、输入框、焦点环 | `--border`、`--input`、`--ring` |
| 圆角 | `--radius` |
| 字体族 | `--font-sans`、`--font-mono` |
| 阴影 | `--shadow-2xs`、`--shadow-xs`、`--shadow-sm`、`--shadow`、`--shadow-md`、`--shadow-lg`、`--shadow-xl`、`--shadow-2xl` |

字体只传家族名称，字体文件不会进入 iframe。系统没有对应字体时，页面应在 `font-family` 里保留通用族作为回退。

侧栏、图表、表格、字距和间距不传。这些变量跟着控制台内部布局走，不是插件页面的契约。

空值，以及含控制字符、`;`、`{}`、`<>`、`\` 的值会被丢掉。某次消息里没有的白名单变量会从插件页面上移除，避免上一个配色的值留下来。

## 授权

Owner 启用插件时，授予的是 `permissions` 的子集和 `network` 的子集。宿主 API 和页面 GraphQL 检查的是这份授权。给一个已启用的插件新增权限、出站规则、事件、钩子或页面时，Owner 确认后，新包才会替换旧包。

`write_*` 会在授权对话框里高亮，因为页面可以执行该 scope 下的 mutation。授予 `write_channels` 后，只要当前用户也有这个权限，页面就能执行渠道 mutation，并读取凭证明文，包括修改渠道地址。页面仍然不能自己开连接。数据只能经已经授予的 `axonhub.fetch` 规则发出。

## 发布

索引地址是 [https://raw.githubusercontent.com/looplj/axonhub/unstable/plugins/registry.json](https://raw.githubusercontent.com/looplj/axonhub/unstable/plugins/registry.json)。AxonHub 每次按这个 URL 拉取，所以改索引不用发新版本。示例插件不在市场里。

把 zip 发布到你自己的 HTTPS 地址，例如 GitHub Release。sha256 是 zip 原始字节的 64 位小写十六进制：

```bash
shasum -a 256 temperature-cap-1.0.0.axplugin
```

要让它出现在市场里，向本仓库提交 PR，在 `plugins` 数组里加上一条插件。`versions` 里每一项都要有 `version`、`url`、`sha256` 和 `axonhubVersion`：

```json
{
  "id": "acme.temperature-cap",
  "name": "Temperature Cap",
  "description": "Caps temperature on chat requests.",
  "versions": [
    {
      "version": "1.0.0",
      "url": "https://github.com/acme/temperature-cap/releases/download/v1.0.0/temperature-cap-1.0.0.axplugin",
      "sha256": "zip 的 64 位小写十六进制",
      "axonhubVersion": "^1.0.0"
    }
  ]
}
```

Hub 会核对下载字节和 `sha256`。没有发布者签名。

## 示例

| 示例 | 演示的能力 |
| --- | --- |
| [Notes](../../../plugins/examples/notes/README.md) | 页面、`configSchema`、插件 GraphQL，以及本插件的 KV |
| [Event Log](../../../plugins/examples/event-log/README.md) | `onEvent` 把一条短记录写入 KV，并在页面上列出 |
| [Failover Guard](../../../plugins/examples/failover-guard/README.md) | `onEvent` 使用 `ctx.counter`、`ctx.axonhub.channels.disable` 和 `ctx.webhook.send` |
| [Star 历史](../../../plugins/examples/star-history/README.md) | 页面通过 `axonhub.fetch` 访问已授予的 `network` 规则 |
| [Temperature Cap](../../../plugins/examples/temperature-cap/README.md) | `onRequest` 按配置返回带 `temperature` 的完整请求 |

## 限制

作者最先碰到的限制：

- zip 最大 5 MiB。
- `plugin.js` 最大 1 MiB。
- `onEvent`、`onRequest`、`onAttempt` 单次调用 10 秒超时，已 `await` 的 `ctx.fetch` 算在里面。
- 脚本出错时 fail-open：保持上一个请求，HTTP 状态不变。事件投递被丢掉。
- 插件借不到 VM 时，请求返回 429 `plugin_overloaded`，并且不会发往上游。过载不会跳过这个钩子。
