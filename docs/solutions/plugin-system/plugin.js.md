# plugin.js：脚本入口与调用契约

[插件系统](README.md)的后端 JS 编写说明。本文定义脚本的导出形式，并串起已有的钩子、上下文和生命周期约定；描述的是待实现方案，不表示当前仓库已经支持插件。

manifest 字段与开放范围以 [manifest.md](manifest.md) 为准，数值限制以 [运行限制](README.md#运行限制) 为准。业务事件、请求链路和未来响应链路分别见 [events.md](events.md)、[request.md](request.md)、[response.md](response.md)。不另建任务表，实现与验收归入总览的 T-10 至 T-13，响应能力仍按原计划另行排期。

> 本文的钩子签名、注册和返回值为未发布接口的修订契约，仍使用 pluginApiVersion 1；运行时及示例脚本已按 T-13 同步。

## 1. 入口形式

包根目录的 `plugin.js` 是 ESM 模块。插件导出约定名称的函数，manifest 声明使用哪些能力，宿主在对应时机调用。

```js
export function onRequest(ctx, req) {
  return { ...req, temperature: 0.2 };
}
```

- 第一版只识别命名导出，不从 `export default { ... }` 中寻找钩子，也不提供工厂函数或动态 `registerHook`。
- manifest 启用了某个钩子，必须有同名的可调用导出；缺失或导出不是函数时安装失败。只导出而没有声明的函数不会被调度。
- 辅助函数可以自由定义；只有宿主约定且已开放的导出才是入口。
- 只有 UI 的插件可以省略 `plugin.js`。页面自己的 JS 在浏览器里运行，通过 UI Bridge 交互，不会作为后端钩子执行。

## 2. 导出函数

| 导出 | manifest 声明 | 调用时机 | 当前方案范围 |
|------|---------------|----------|--------------|
| `onEvent(ctx, event)` | 非空 `events` | 收到订阅的业务事件，例如 `request.failover` | 事件扩展点 |
| `onRequest(ctx, req)` | `hooks.request.enabled: true` + `requestTypes` | 每个请求一次，模型权限检查、模型映射和选路之前 | 请求扩展点 |
| `onAttempt(ctx, body)` | `hooks.attempt.enabled: true` + `apiFormats` | 每次匹配的上游尝试；协议转换及 pass-through 后，渠道 override 前 | 请求扩展点 |
| `onResponse(ctx, res)` | `hooks.response: true` | 成功的非流式响应 | 未开放，见响应篇 |
| `onStreamChunk(ctx, chunk)` | `hooks.streamChunk` 类型列表 | 命中声明类型的流式 chunk | 未开放，见响应篇 |

这里的 `onEvent` 是事件总线的业务事件，不是 SSE/token 事件。流式响应改写使用独立的 `onStreamChunk` 名称。

Provider 的 `transformRequest`、`transformResponse` 另见 [provider.md](provider.md)，也未开放；不能通过额外导出这些函数绕过 manifest 的能力校验。

### 输入与返回值

所有入口的第一个参数都是 `ctx`。`onRequest(ctx, request)` 接收统一请求的公开 JSON 对象，按 RequestType 匹配。`onAttempt(ctx, body)` 接收当前上游协议的 JSON body 对象，按实际出站 APIFormat 匹配；不是 llm.Request，也不是字符串或字节。非 JSON 对象 body（包括 multipart、二进制）不调度 attempt。两者独立匹配，第一次执行 attempt 且尚无已提交状态时 state 为 `{}`，后续重试保留成功提交的 state。

| 返回或动作 | 请求钩子的含义 |
|------------|----------------|
| 返回 JSON 对象 | replace：整体替换公开输入，省略字段删除，不做 merge；通过校验后交给下一个插件 |
| 返回数组 | patch：RFC 6902 JSON Patch，作用于调用前对象；空数组为成功的无修改操作 |
| 返回 `undefined`，或没有 return | 保持调用前的请求，不提交对参数对象的原地修改 |
| 返回 `null`、标量或其他非法值 | 本次调用失败，保留调用前的请求 |
| `onRequest` 中调用 `ctx.reject(status, message)` | 主动拒绝请求，允许的状态码和错误路径见请求篇 |
| 抛错、超时，或 Promise 被拒绝 | 本次调用失败，按请求篇处理 |
| 返回 Promise | 宿主等待结算。结算值按上面各行处理；等待计入该钩子的预算 |

- `onRequest` 可以修改 model，修改后仍走模型权限检查和映射；不能修改 stream。
- `onAttempt` 的返回值只影响当次尝试，不污染下一次重试；模型与传输控制字段按出站 APIFormat 校验，详见请求篇。保护约束丢弃改写时记录 Warn、累加 protectedDrops、丢弃 state，并保持连续失败计数不变。
- `onEvent` 通过宿主 API 执行动作，普通返回值不作为事件改写结果。它失败后事件不重试，也不改变已经发生的网关请求结果。
- 不把请求对象回滚理解为宿主副作用回滚：已经成功执行的 KV 或事件处置操作不会因随后抛错而自动撤销。
- 未来响应钩子的返回语义以 [response.md](response.md) 为准，其中 `null` 丢弃 chunk 的规则只针对流式 chunk，不能套用到请求。

patch 使用 [RFC 6902](https://www.rfc-editor.org/rfc/rfc6902) 的 add/remove/replace/move/copy/test，路径为 [RFC 6901 JSON Pointer](https://www.rfc-editor.org/rfc/rfc6901)。宿主在副本上按顺序应用，任一步失败或最终值不是对象则全部回滚。replace/patch 都先完成类型与保护字段校验，再记录成功和提交 state；不能先清空失败计数。

pass-through 只可能覆盖 onRequest 除 model 外的改写，onAttempt 在其后运行，改写正常生效；渠道 override 在 onAttempt 之后执行，具体优先级及提示见 [request.md](request.md#5-pass-through-渠道)。

## 3. ctx 与状态

`ctx` 由宿主为本次调用提供，不能由插件自行构造出额外权限。

| 成员 | 用途与范围 |
|------|------------|
| `ctx.config` | 按 manifest schema 保存的本插件配置；页面拿到的是去掉 secret 的单独配置视图 |
| `ctx.log.info/warn/error` | 记录插件日志，不应记录密钥或完整请求内容 |
| `ctx.kv.get/set/delete/list` | 插件持久数据；事件与请求插件共用，按本次安装 ID 隔离 |
| `ctx.fetch` | 宿主代发的外部请求，只到达已授权的路径或正则。见 [出站请求](#出站请求)。`onEvent`、`onRequest`、`onAttempt` 都可 `await` |
| `ctx.request` | 请求钩子的请求元数据：`projectId`、`apiKeyId`、客户端 `model`、`requestType`、`apiFormat`；playground 的 `apiKeyId` 为 null |
| `ctx.attempt` | `onAttempt` 的当前渠道、实际模型和上游格式 |
| `ctx.state` | 请求钩子之间共享的、按请求和插件隔离的 JSON 状态；未来响应钩子沿用 |
| `ctx.reject` | `onRequest` 的主动拒绝入口 |

事件处置还使用 `ctx.counter`、`ctx.webhook.send` 和 `ctx.axonhub` 下的渠道、请求元数据接口。JS 调用签名统一定义如下，事件发布和处置流程见 [JS 处置](events.md#7-js-处置)。这里的 TypeScript 仅描述接口，交付脚本仍是 JavaScript。

状态分三类：

- **一次请求内**：放 `ctx.state`，由宿主保存；下一次钩子可能换了 VM，宿主重新交付状态。请求结束后释放，不与异步 `onEvent` 共享。
- **跨请求、重启或实例**：放 `ctx.kv`，升级保留，同名重装隔离。键、值、TTL 与安装身份规则见 [PluginStorage](README.md#pluginstorage)。
- **模块级变量**：只能放常量及可重建缓存。VM 会复用，也可能因中断或升级被销毁；模块变量不能用作全局计数器，不能保存某个用户的请求上下文。

宿主 API 查实际授予的权限，不以 manifest 声明替代授权；KV 的安装 ID 由宿主绑定，插件不能传入其他安装的 ID。

`ctx.config` 是只读配置快照，后端包含本插件配置的 secret；修改不会保存配置。`ctx.state` 初始为 `{}`，必须保持 JSON 对象：成功返回（包括 undefined）且请求返回值通过校验后保存本次状态；抛错、超时、非法返回或 reject 时丢弃本次状态修改，保留调用前的状态。KV 写入是独立持久操作，不随 state 回滚。

### 公共类型与调用规则

```ts
type JSONValue = null | boolean | number | string | JSONValue[]
  | { [key: string]: JSONValue };
type JSONObject = { [key: string]: JSONValue };
type ID = number; // 正的 JS 安全整数，使用宿主交付的 ID
type Timestamp = string; // RFC 3339，UTC

interface PluginHostError extends Error {
  name: "PluginHostError";
  code: "INVALID_ARGUMENT" | "PERMISSION_DENIED" | "NOT_FOUND"
    | "CONFLICT" | "LIMIT_EXCEEDED" | "RATE_LIMITED"
    | "DEADLINE_EXCEEDED" | "CANCELED" | "UNAVAILABLE"
    | "INVALID_STATE" | "INTERNAL";
}
```

后端宿主 API 里，KV、计数器、日志、渠道和 WebHook 同步返回。`ctx.fetch` 返回 Promise，由钩子 `await`。失败抛出 `PluginHostError`；错误 message 可读但不作为稳定判断依据，不含密钥。JSON 数字必须有限，拒绝循环对象、函数、BigInt 和非 JSON 值。省略参数或传 undefined 使用默认值，null 只有在签名明确允许时才合法；不隐式转换参数类型。时间、TTL 和窗口单位在签名中区分。

API 只在有效的钩子调用中可用，不能把 ctx 或其方法保存到模块变量后跨调用使用。宿主 I/O 继承本次调用的取消与剩余执行时间，不因进入 Go 宿主函数而延长预算；取消后不再接受新的副作用调用。超时不承诺撤销已提交的数据库写入或已发送的通知。

权限不足统一为 `PERMISSION_DENIED`。参数不合法为 `INVALID_ARGUMENT`；超过体积或条数上限为 `LIMIT_EXCEEDED`；写限流为 `RATE_LIMITED`；不允许访问的方法为 `INVALID_STATE`。插件可 try/catch 处理宿主错误；未捕获的错误按钩子失败规则处理。

### KV、计数器与日志

```ts
interface KVEntry {
  namespace: string;
  key: string;
  value: string;
  expiresAt: Timestamp | null;
  updatedAt: Timestamp;
}
interface KVPage {
  entries: KVEntry[];
  nextCursor: string | null;
}
interface KV {
  get(key: string, options?: { namespace?: string }): string | null;
  set(key: string, value: string,
      options?: { namespace?: string; ttlSeconds?: number }): void;
  delete(key: string, options?: { namespace?: string }): boolean;
  list(options?: { namespace?: string; prefix?: string; first?: number; after?: string }): KVPage;
}
interface Counter {
  add(key: string, windowSeconds: number): number;
  get(key: string, windowSeconds: number): number;
}
interface Log {
  info(message: string, fields?: JSONObject): void;
  warn(message: string, fields?: JSONObject): void;
  error(message: string, fields?: JSONObject): void;
}
```

- `ctx.kv`：namespace 默认 `default`；get 对不存在或已过期的键返回 null，空字符串是合法值。set 为覆盖写入，value 必须是 UTF-8 字符串，存 JSON 需显式 `JSON.stringify`；TTL 省略或为 0 表示不过期，覆盖时重设 TTL。delete 仅在删除未过期的现存键时返回 true，否则 false。list 默认空 prefix，按 key 升序返回该 namespace 内匹配前缀的未过期条目；返回 `{ entries, nextCursor }`，无结果时 entries 为 `[]`、nextCursor 为 null。first 默认 100、范围 1–100；after 传上一页 nextCursor，按 key 排他边界继续读取，nextCursor 为 null 表示末页。prefix 是字面前缀，不是正则或 SQL 通配符。
- `ctx.counter`：add 固定递增 1 并返回递增后的窗口计数；get 只读，不存在时为 0。窗口必须是整数秒。同一插件、同一 key、同一 windowSeconds 才是同一计数器，不同窗口独立。Redis 通过 Lua 原子完成过期清理、递增和总数返回，使用服务端时间；原子计数不保证后续宿主动作恰好执行一次。计数器不是持久 KV。
- `ctx.log`：fields 默认 `{}`；宿主另附插件、安装和调用信息，插件不能覆盖这些身份字段。日志接口不因日志落地失败中断钩子。
- KV、counter、log 在 `onEvent`、`onRequest`、`onAttempt` 都可用。KV set/delete 计入写类 API 限额；counter.add 用于每次事件计数，不计入处置类写限额。

```js
ctx.kv.set("rules", JSON.stringify({ enabled: true }), {
  namespace: "settings", ttlSeconds: 3600
});
const raw = ctx.kv.get("rules", { namespace: "settings" });
const rules = raw === null ? null : JSON.parse(raw);
```

### 事件处置 API

```ts
interface ChannelInfo {
  id: ID;
  name: string;
  type: string;
  baseUrl: string;
  status: string; // 宿主渠道状态，不包含 credentials
}
interface RequestInfo {
  id: ID;
  projectId: ID;
  apiKeyId: ID | null;
  channelId: ID | null;
  model: string;
  apiFormat: string;
  status: "pending" | "processing" | "completed" | "failed" | "canceled";
  createdAt: Timestamp;
}
interface Channels {
  get(id: ID): ChannelInfo | null;
  list(options?: { afterId?: ID; limit?: number }): {
    items: ChannelInfo[]; nextAfterId: ID | null;
  };
  disable(id: ID, options?: { reason?: string }): void;
  enable(id: ID, options?: { reason?: string }): void;
  disableCredential(id: ID, fingerprint: string,
    options?: { reason?: string; expiresAt?: Timestamp | null }): void;
  enableCredential(id: ID, fingerprint: string,
    options?: { reason?: string }): void;
}
interface EventActions {
  axonhub: {
    channels: Channels;
    requests: { get(id: ID): RequestInfo | null };
  };
  webhook: { send(targetName: string, payload: JSONObject): void };
}
```

这些处置 API 仅在 `onEvent` 提供，不在请求钩子开放渠道管理或 WebHook 发送。channels.get/list 要求 `read_channels`；四个启停方法要求 `write_channels`；requests.get 要求 `read_requests`；webhook.send 要求 `webhook`。先检查权限，再查询资源。

- get 对不存在或已删除的行返回 null；返回对象只含上面的白名单字段，requests.get 不返回请求/响应 body、客户端 IP 或用户凭证。
- channels.list 按 ID 升序，afterId 省略从头开始，游标为排他下界；空结果为 `items: []`，没有下一页时 nextAfterId 为 null。limit 的默认值和上限见总览，必须为正整数。
- 启停方法成功返回 undefined，目标已处于所需状态也视为成功；目标不存在抛 `NOT_FOUND`。reason 省略为 `plugin action`，只作日志和业务处置说明。凭证通过该渠道内的 fingerprint 定位，无匹配抛 `NOT_FOUND`，多匹配抛 `CONFLICT`，不能猜选；`oauth` 不支持这两个凭证级动作，返回 `INVALID_ARGUMENT`。expiresAt 省略或 null 为不自动恢复，非空必须是未来时间。
- webhook.send 只接受已有目标名称，不接受 URL、headers 或凭证覆盖。payload 作为自定义模板 Data，字符串 payload.message 同时填充 Message；宿主从当前事件填充 Event、Severity、OccurredAt，并将 event.Subject.Channel 映射到 Channel。模板上下文不提供 Subject，Data 也不是 event.Data；按目标已有模板渲染发送。目标不存在抛 `NOT_FOUND`，未启用抛 `INVALID_STATE`，非成功 HTTP 状态或传输失败抛 `UNAVAILABLE`；调用成功表示已收到成功 HTTP 响应。此入口只尝试一次，不自动重试，防止通知重复。
- 四个启停方法与 webhook.send 计入写类 API 限额。具体事件来源和递归深度沿用事件篇。

### 出站请求

`ctx.fetch` 由宿主发出 HTTP，脚本里没有全局 `fetch`。页面不直接连接外部主机，浏览器侧走 [axonhub.fetch](ui-bridge.md#1-sdk-契约)，用同一份已授权规则。CSP 的 `connect-src` 保持 `'none'`。`ctx.models` 本轮不提供。

登记在 manifest 的 `network`，Owner 授予的子集写在 `granted_network`。比较时去掉 query 和 fragment，只看 scheme、host、port、path。query 可以出现在实际请求上，但不参与授权。

```ts
interface FetchResult {
  status: number;
  headers: Record<string, string>;
  body: string; // UTF-8
}
interface FetchInit {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  headers?: Record<string, string>;
  body?: string;
}
interface Fetch {
  fetch(url: string, init?: FetchInit): Promise<FetchResult>;
}
```

- 精确规则是完整的 `https` URL，必须带 path，`https://example.com/v1/check` 只等于这条 path，不含其子路径。默认端口 443 不写入比较串。
- 正则规则写成 `{ "regex": "^https://api\\.example\\.com/v1/items/[0-9]+$" }`。宿主用 Go `regexp` 对去掉 query、fragment 后的 URL 做全匹配，模式本身要带 `^` 和 `$`。
- 未授权、不是 `https`、或带 userinfo：不发出请求，抛 `PERMISSION_DENIED`。上游返回 301、302、303、307、308 时不请求 Location，本次调用抛 `INVALID_STATE`。
- `Host` 与逐跳头由宿主丢弃。插件自己的 `Authorization` 可以放在 `headers` 里，宿主不附带渠道凭证或用户 Cookie。
- 非 2xx 仍 resolve，由插件看 `status`。传输失败或超时抛 `UNAVAILABLE` / `DEADLINE_EXCEEDED`。响应不是合法 UTF-8 时抛 `INVALID_STATE`。
- 体积和条数见 [运行限制](README.md#运行限制)。等待计入当前钩子预算；页面调用计入 Bridge 的 15 s。VM 在钩子返回的 Promise 结算前不归还；未等待的任务按 [异步调用收尾](#异步调用收尾) 清理。

```js
const res = await ctx.fetch("https://moderation.example.com/v1/check", {
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ text: "..." }),
});
```

### 主动拒绝请求

```ts
interface RequestControl {
  reject(status: 400 | 403 | 429, message: string): never;
}
```

`ctx.reject` 仅在 `onRequest` 可用。合法调用记录宿主侧拒绝标记，并立即终止当前 JS 执行；即使插件 try/catch 捕获内部控制信号，宿主也忽略返回值、停止后续请求钩子并返回拒绝结果。非法状态码或空白 message 抛 `INVALID_ARGUMENT`，不设置拒绝标记。拒绝是业务结果，不计入插件失败或熔断。

## 4. 最小示例

以下示例用于事件与请求扩展点均开放后。manifest 字段在相应任务完成之前仍按开放规则拒绝安装。

`manifest.json`：

```json
{
  "schemaVersion": 1,
  "id": "example.request-policy",
  "name": "Request Policy",
  "version": "1.0.0",
  "axonhubVersion": "^1.0.0",
  "pluginApiVersion": 1,
  "permissions": [],
  "events": ["request.failover"],
  "hooks": {
    "request": { "enabled": true, "requestTypes": ["chat"] },
    "attempt": { "enabled": true, "apiFormats": ["openai/chat_completions"] }
  }
}
```

`plugin.js`：

```js
export function onEvent(ctx, event) {
  ctx.log.info(`收到业务事件：${event.type}`);
}

export function onRequest(ctx, req) {
  if (req.model === "blocked-model") {
    ctx.reject(403, "该模型已禁用");
    return;
  }

  ctx.state.clientModel = req.model;
  // 返回完整请求对象，保留自己不修改的字段。
  return { ...req, temperature: 0.2 };
}

export function onAttempt(ctx, body) {
  // body 已是实际上游协议；元信息不依赖 body 中是否存在 model。
  ctx.log.info(`上游模型：${ctx.attempt.model}`);
  return [{ op: "add", path: "/temperature", value: 0.2 }];
}
```

## 5. 模块与运行生命周期

允许包内相对 import，例如 `import { normalize } from "./lib/normalize.js"`。依赖必须在包内，npm 依赖由作者提前打包。裸模块名、`node:` 和包外路径被拒绝；运行时不下载依赖，也不提供 Node.js 文件系统和进程 API。`eval` 与动态代码生成由 `DisableDynamicCode` 禁用。

函数可以是 async。宿主等待钩子返回的 Promise，等待计入该钩子的预算；Promise 被拒绝视同抛错。顶层 `await` 拒绝安装。运行时不提供全局 `fetch`，外部请求只走 `ctx.fetch`，目标见 [出站请求](#出站请求)。`ctx.webhook.send` 仍只发往 Owner 已配置的 WebHook 目标。

### 异步调用收尾

每次钩子调用拥有独立的调用 ID、可取消 context 和宿主异步任务集合。`ctx.fetch` 发起时登记任务，完成结果只投递到该调用的执行队列；HTTP worker 不直接操作 VM，Promise 的 resolve/reject 与 JS 回调都在 VM 所属执行线程处理。

钩子返回（含返回的 Promise 结算）、抛错或超时时，宿主先将调用标记为结束并关闭结果投递，再取消仍在进行的 I/O。迟到结果只释放响应体等 Go 资源，不能再 resolve/reject Promise 或进入 JS，也不能投递到下一次调用。未等待的 fetch 不会延长钩子生命周期；已经发送的 HTTP 请求不承诺撤销。

结束时仍有未结算的宿主异步任务或待执行的回调，则丢弃该 VM，不带着遗留 Promise 归还池子。只有任务和执行队列均已清空的正常调用才复用 VM；被中断的 VM 仍直接丢弃。这样 `void ctx.fetch(...)`、`Promise.race(...)` 提前返回和超时都不会让旧回调进入后续请求。

验收覆盖：未 await 的 fetch 后立即返回、两个 fetch 的 `Promise.race`、请求取消、超时后 HTTP 才完成；确认迟到回调不执行、不修改后续调用状态，I/O 被取消且响应资源被释放。异步调度与 VM 收尾归 T-11，真实 fetch 集成用例归 T-16。

加载和调用顺序：

```text
安装校验：编译、链接 → 临时 VM 执行顶层 → 检查声明的导出
启用/升级：加载包 → 编译结果与模块数据进入内存 → 发布 Generation
首次需要 VM：创建 VM → 初始化模块、执行顶层
每次调用：借 VM → 交付参数与 ctx → 调用命名导出并等待返回值 → 校验结果 → 异步调用收尾 → 归还或丢弃 VM
停用/删除/升级：旧调用结束 → 释放旧执行资源
```

顶层代码可能在安装校验及多个 VM 初始化时重复执行，不是“安装一次只执行一次”的生命周期钩子。顶层只做纯初始化；业务副作用放在宿主调用的钩子内。

编译结果可复用，VM 按次借还；事件与请求共享同一插件的池。未来响应钩子也复用该池，不为每个 chunk 加载脚本。请求固定使用同一 Generation，流式期间不会因升级提前释放旧脚本。

UI 文件保存在实例临时目录，后端模块和执行数据保存在内存；完整加载、目录清理与恢复见 [资源加载与释放](README.md#资源加载与释放)。VM 数量、调用预算、状态大小和熔断阈值只在 [运行限制](README.md#运行限制) 维护。

调用出错时请求钩子保留原请求，事件不重试；请求钩子取不到 VM 则返回过载错误，不能与普通脚本错误混为一谈。被中断的 VM 丢弃，后续按需新建。精确错误路径见各扩展点文档。

## 6. 接口取舍

第一版不同时支持 default 对象、工厂函数、动态注册，避免同一种能力出现多种发现与生命周期规则。以后扩展宿主 API 或新增钩子仍经过 manifest 能力校验；不兼容变化按 `pluginApiVersion` 管理。

## 7. 实现与验收对应

- **T-10，安装与导出校验**：声明但缺少命名导出、只有 default 对象、导出不是函数、顶层 await、包外 import 均失败；async 钩子和包内相对 import 通过。
- **T-11，运行时**：按次借还 VM，等待 Promise 到结算或超时，中断后丢弃 VM；不同请求的 `ctx.state` 隔离，同一请求换 VM 后仍能读取自己的状态。调用阶段不重复读库、解压或编译。`ctx.fetch` 不在本任务。
- **T-12，事件**：只分发声明的事件；业务事件不进入流式钩子；宿主权限与 KV 安装隔离遵循事件篇和总览验收。
- **T-13，请求**：对象 replace、数组 JSON Patch 原子提交，原地修改后返回 undefined 不生效；非法返回、patch 失败或 stream 改写回滚；`onRequest` 改 model 后仍过权限检查，`onAttempt` 改 model 被拒；未声明钩子不调用。

宿主 API 契约测试还覆盖：KV 缺失/空串/过期、覆盖重设 TTL、list 排序与超限；不同窗口计数器独立；渠道分页和不存在返回值；凭证歧义与 OAuth 拒绝；未授权调用无副作用；WebHook 失败不重试；宿主 I/O 取消；catch reject 后仍不能放行请求；失败调用不提交 ctx.state。出站的路径匹配、正则全匹配、重定向不跟随归入 T-16。

验证归入 `go test ./internal/server/plugin/...` 和请求篇的 orchestrator 契约测试；本文没有运行测试的记录，不将示例视为已实现或已验收。
