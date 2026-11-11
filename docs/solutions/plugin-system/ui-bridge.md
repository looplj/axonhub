# UI Bridge：页面与插槽共用 SDK

[插件系统](README.md)所有 iframe UI 共用的宿主通信契约。当前供独立页面使用，T-17 的工具栏操作弹窗复用同一份 bridge.js、PluginFrame、消息协议和 GraphQL 权限规则。工具栏扩展实施状态见总览 T-17。

[pages.md](pages.md)定义页面入口、资源加载和插件 GraphQL；[slots.md](slots.md)定义操作注册、弹窗及 action 上下文。后端 JS 的 ctx API 见 [plugin.js.md](plugin.js.md)，不与浏览器 SDK 混用。数值预算只在[总览运行限制](README.md#运行限制)维护；实施归入总览 T-06/T-07，不另建任务表。

页面和操作弹窗的模块脚本均遵循 [UI 资源 CORS 契约](pages.md#ui-资源-cors-契约)：匿名加载 token 保护的静态资源，保持 sandbox 的不透明 origin。资源 CORS 不开放直接 GraphQL 访问，也不改变本文的父窗口消息校验。

## 1. SDK 契约

`bridge.js` 提供全局 `window.axonhub`。页面和 `plugin.js` 都可以 async/await。外部 URL 不由浏览器直接请求：页面调用 `fetch`，宿主按已授权规则代发，CSP 的 `connect-src` 保持 `'none'`。下面的 JSONValue/JSONObject 沿用 [JS 公共类型](plugin.js.md#公共类型与调用规则)。

```ts
interface ThemeState {
  theme: "light" | "dark"; // 宿主解析 system 设置后的实际主题
  colorScheme: string; // 宿主配色名称，页面不应枚举固定名单
  tokens: Record<string, string>; // 宿主 <html> 上已计算的样式变量
}
interface UIInit extends ThemeState {
  locale: string;
  projectId: number | null;
  page: string | null; // 独立页面的 key；操作弹窗为 null
  panel: null; // 兼容旧字段，不开放 panels
  action: string | null; // T-17：操作 key，独立页面为 null
  context: JSONObject; // T-17：全局操作为 {}
  slot: "app.header.actions" | null; // T-17 操作插槽；独立页面为 null
  config: JSONObject; // 不含 secret
  canWriteKV: boolean;
}
interface GraphQLError {
  message: string;
  path?: Array<string | number>;
  locations?: Array<{ line: number; column: number }>;
  extensions?: JSONObject;
}
interface GraphQLResult<T> {
  data?: T | null;
  errors?: GraphQLError[];
  extensions?: JSONObject;
}
interface BridgeError extends Error {
  name: "AxonhubBridgeError";
  code: "INVALID_ARGUMENT" | "NOT_READY" | "UNAUTHENTICATED"
    | "FORBIDDEN" | "NOT_FOUND" | "RATE_LIMITED" | "TIMEOUT"
    | "DISPOSED" | "NETWORK_ERROR" | "INTERNAL";
  status?: number; // 有 HTTP 响应时提供
}
interface AxonhubBridge {
  ready(): Promise<Readonly<UIInit>>;
  onInit(handler: (init: Readonly<UIInit>) => void): () => void;
  onTheme(handler: (theme: Readonly<ThemeState>) => void): () => void;
  graphql<T = JSONObject>(query: string, variables?: JSONObject,
    options?: { operationName?: string }): Promise<GraphQLResult<T>>;
  navigate(path: string): Promise<void>;
  toast(message: string, options?: {
    kind?: "info" | "success" | "warning" | "error";
  }): Promise<void>;
  fetch(url: string, init?: {
    method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
    headers?: Record<string, string>;
    body?: string;
  }): Promise<{ status: number; headers: Record<string, string>; body: string }>;
}
```

上下文由宿主根据挂载位置构造：独立页面设置 page，action/slot 为 null；操作弹窗设置 action/slot，page/panel 为 null，context 为 {}，projectId 为 null。T-17 新增 action/context 字段，panel 兼容保留为 null。每个 iframe 独立签发 token、建立会话和 pending 调用表；同插件同 entry 的不同实例也不能复用凭据或串用消息。

- ready 首次调用发送 hello，收到 init 后 resolve；重复调用返回同一个 Promise，不重复握手。握手超过总览预算则 reject `TIMEOUT`，宿主显示错误和重试；重试创建新的 iframe 会话。它不是顶层窗口登录 API。
- onInit 注册监听，返回取消订阅函数。初始化成功后调用一次；初始化后才注册的监听在下一次微任务收到缓存的 init，取消后不再回调。onTheme 同样返回取消订阅函数，初始化后先交付当前值，之后在主题、配色或 `tokens` 变化时通知。回调收到只读快照，修改不影响宿主；回调抛错交给页面错误处理，不破坏其他回调。
- `tokens` 是父页面 `<html>` 的计算样式。白名单和页面用法见 [插件开发文档](../../zh/development/plugins.md#页面主题)。空值，以及含控制字符、`;`、`{}`、`<>`、`\` 的值不发送。`bridge.js` 在通知监听之前把这些值写到 iframe 的 `documentElement`，白名单里这次没有的项会移除，并按 `theme` 设置 `color-scheme`。名单以外的变量仍由页面自己的 CSS 定义。宿主在微任务里读取，等配色 class 写上 `<html>` 之后再取计算值。切换主题沿用原来的 `theme` 消息，不轮询。
- graphql/navigate/toast/fetch 在握手完成前等待 ready；握手失败则一起失败。query 必须是非空字符串，variables 默认 `{}`，不接受非 JSON 值；多 operation 文档须提供 operationName。页面没有直接 KV 方法，通过插件 GraphQL 的 KV 字段访问。
- graphql 返回完整的 GraphQL envelope，不自动提取 data；HTTP 成功且包含 `errors` 时仍 resolve，保留部分 data。页面检查 `errors[].extensions.code` 处理业务错误。网络失败或非成功 HTTP 状态则 reject BridgeError：401/403/404/429 分别映射到 `UNAUTHENTICATED`/`FORBIDDEN`/`NOT_FOUND`/`RATE_LIMITED`，超时为 `TIMEOUT`，其他 HTTP 错误为 `INTERNAL`；message 不暴露服务端内部信息。
- navigate 只接受以单个 `/` 开头的站内路径，可带 query/hash；拒绝 `//`、反斜杠、控制字符及解析后不同源的地址。沿用宿主路由权限检查，不能绕过 AuthGuard。宿主接受导航动作即 ack；若导航使页面卸载，尚未完成的调用按销毁规则结束。
- toast 的 kind 默认 info，message 必须非空，按纯文本显示；宿主接收并显示通知后 ack，不等待通知消失。
- fetch 与 `ctx.fetch` 使用同一套 [出站规则](plugin.js.md#出站请求)。目标不匹配已授权的路径或正则时返回 `FORBIDDEN`，不发出请求。非 2xx 仍 resolve，传输失败 reject `NETWORK_ERROR`。浏览器不直接连接该 URL。
- 不自动重试 graphql（可能含 mutation）或其他动作。页面卸载、重试、切换语言时销毁旧会话；独立页面切换 project 也重新创建 PluginFrame，让 init、请求头和配置视图保持一致。全局操作不使用当前项目，选中项目变化不重建它。

```js
axonhub.onTheme(({ theme }) => {
  document.documentElement.dataset.theme = theme;
});

async function load() {
  const init = await axonhub.ready();
  const result = await axonhub.graphql(
    "query ReadSetting($key: String!) { pluginKV(key: $key) { value } }",
    { key: "settings" }
  );
  if (result.errors?.length) {
    await axonhub.toast(result.errors[0].message, { kind: "error" });
    return;
  }
  // 不存在时 pluginKV 为 null；存在时 value 为字符串。
  render(init, result.data?.pluginKV ?? null);
}

load().catch(error => {
  document.getElementById("app").textContent = error.message;
});
```

## 2. 消息与生命周期

宿主与 SDK 使用结构化 postMessage 对象，不通过拼接 JS 执行。消息协议为：

```ts
type Envelope = { protocol: "axonhub.plugin-ui"; version: 1 };
type Hello = Envelope & { type: "hello" };
type Init = Envelope & { type: "init"; sessionId: string; payload: UIInit };
type Call = Envelope & {
  type: "call"; sessionId: string; id: string;
  method: "graphql" | "navigate" | "toast" | "fetch"; params: JSONObject;
};
type Reply = Envelope & {
  type: "reply"; sessionId: string; id: string;
} & ({ ok: true; result: JSONValue } | {
  ok: false; error: { code: BridgeError["code"]; message: string; status?: number };
});
type ThemeChange = Envelope & {
  type: "theme"; sessionId: string; payload: ThemeState;
};
type PageErrorReport = Envelope & {
  type: "pageError"; sessionId: string; payload: { message: string };
};
```

params 按方法对应：graphql 为 `{query, variables, operationName?}`，navigate 为 `{path}`，toast 为 `{message, kind}`，fetch 为 `{url, method?, headers?, body?}`；SDK 先填充默认值再发出。成功的 void 调用在消息中使用 result: null，SDK 转为 undefined；GraphQL 的 result 是原始 envelope；fetch 的 result 是 `{status, headers, body}`。

宿主在每次 iframe load 时生成新的随机 sessionId；只接受该 iframe.contentWindow 的 hello/call，检查协议版本、参数形状和 sessionId。SDK 只接受 `event.source === window.parent` 的 init/reply/theme，并检查宿主同源 origin（从 iframe 的资源 URL 获取）；宿主向 sandbox 的不透明 origin 发消息使用 `targetOrigin: "*"`，不能以此放宽 source/session 校验。SDK 发给父窗口使用明确的宿主 origin。

每次调用生成本会话唯一的 id，可并发，reply 按 id 匹配，不按返回顺序匹配。SDK 对已完成、超时或旧会话的 reply 直接忽略。未支持的方法/版本拒绝处理，参数错误返回 `INVALID_ARGUMENT`；没有有效握手的调用不得转发到后端。

宿主只使用当前 PluginFrame 自己的 pluginID、用户凭据和 project 上下文；任何消息里的额外身份字段都不能覆盖它们。SDK/宿主均有调用超时，预算见总览；超时或销毁时清除 pending，reject `TIMEOUT` 或 `DISPOSED`，宿主取消尚在进行的 fetch。取消不是已提交 mutation 的回滚。全局 error/unhandledrejection 由 bridge 报告给宿主用于显示页面错误；错误报告只接受相同 source/session，不携带用户 token。

### 页面出站的服务端入口（T-16）

父页面收到 `fetch` 消息后，使用自己的用户 JWT 调用 `POST /admin/plugins/:pluginId/fetch`，JSON 请求体为 `{url, method?, headers?, body?}`。pluginId 取当前 PluginFrame；用户 JWT 只用于宿主接口认证，不转发给目标 URL。成功响应为 `{status, headers, body}`，外部 HTTP 状态放在 `status`，不作为宿主接口的 HTTP 状态。

该路由复用插件 GraphQL 的 `WithJWTAuth`（仅用户 JWT）、`WithProjectID` 和已启用 UI 插件的查找逻辑（T-17 接受 pages/actions）。服务端独立验证用户身份，解析安装 ID，并从当前插件加载 `granted_network`；不接受客户端提交的授权规则或安装身份。插件不存在、已停用或未声明页面或操作返回 404，目标未授权返回 403。检查通过后调用与 `ctx.fetch` 共用的出站服务。无需增加 GraphQL 数据权限或新的出站次数、并发限制；页面 GraphQL 同样不设请求频率限制。

该路由单独设置总览中 Bridge 调用的超时预算，并继承请求取消；父页面的 Bridge 超时或会话销毁时中止该请求。鉴权错误沿用 GraphQL 的 Bridge 映射；出站超时为 `TIMEOUT`，传输失败为 `NETWORK_ERROR`，重定向或非法 UTF-8 为 `INVALID_STATE`。升级、停用和授权变更按管理器现有代际同步生效，新调用重新读取当前授权，已经开始的调用沿用其快照。

验收覆盖直接请求接口时缺少 JWT、service account、已停用或无页面插件、未授权目标，以及管理器刷新后撤销的规则不再接受新调用；同时验证用户 JWT 和 Cookie 不会被附加到出站请求。

## 3. 操作弹窗复用与验收

T-17 复用 ready/onInit/onTheme/graphql/navigate/toast/fetch，只增加 action/slot/context 初始化信息，不增加 resize。全局操作不继承当前项目；navigate 成功及路由变化时关闭弹窗。关闭、语言切换、重试均销毁会话，细则见 [slots.md](slots.md)。GraphQL 与 fetch 的准入同时扩展为 pages-or-actions，只有 actions 的插件也可使用原有数据及出站能力。

公共验收覆盖重复握手、初始化后订阅、取消订阅、并发乱序回复、错误 source/session、超时、销毁、GraphQL 部分结果与错误码。操作弹窗复用实际资源 URL 和 sandbox 测试，另加与独立页面同屏时的实例隔离、关闭清理及全局项目作用域测试。
