# 页面

[插件系统](README.md)的 UI 扩展点。v1 只有独立页面：侧栏入口加一个整页 iframe。dashboard 面板等插槽见 [slots.md](slots.md)，未排期，排期时复用本篇的 iframe 宿主、桥接和插件 GraphQL。manifest 的 `ui.pages` 字段见 [manifest.md](manifest.md)。数值限制见 [运行限制](README.md#运行限制)。任务见 [任务表](README.md#9-任务管理)，本篇对应 T-04 至 T-07。

> 页面使用独立 Plugin Principal，context 中的用户为收窄视图（权限与插件授权取交集、无 Owner、无角色），详见 [permission.md](permission.md)。

## 1. 背景

### 现状

- 前端没有 iframe 桥接，也没有 `postMessage`。后端没有 CSP。access token 在 localStorage `axonhub_access_token`。GraphQL 走 `frontend/src/gql/graphql.ts` 的 `graphqlRequest`，端点 `POST /admin/graphql`。
- Ent privacy（`internal/scopes/rule*.go`）全部从 context 里的 `*ent.User` 读 `IsOwner`、`Scopes`、`Edges.Roles`、`Edges.ProjectUsers`。管理端 GraphQL 由 `gql.NewGraphqlHandlers` 创建，已有 `apiKeyReadOnly`：`AroundOperations` 拒绝非 Query。没有复杂度限制。`Channel.credentials` 的 resolver 要求 `write_channels`。
- 有几个 mutation 不按 scope 授权，只认 context 里的用户本人：`me.graphql` 的 `updateMe`、`updateMyPassword`（OIDC-only 用户不需要旧密码，见 `me.resolvers.go`）、`unlinkOIDCIdentity`，以及 `ChannelOverrideTemplate` 的增删改（schema 用 `UserOwnedMutationRule`）。
- 认证布局在 `frontend/src/routes/_authenticated/route.tsx`。侧栏项来自 `frontend/src/config/nav-items.ts`，再由 `sidebar.ts` 的 `useSidebarData` 按 `config/route-permission.ts` 过滤。动态路由的现成写法是 `routes/_authenticated/project/threads/$threadId.tsx`。
- `internal/scopes/scopes.go` 没有插件相关的 scope。

插件页面如果做成普通路由组件，就要进前端构建。做成 iframe 后，页面内容跟着 `.axplugin` 走，不改 AxonHub 前端。

## 2. 目标

1. manifest 的 `ui.pages` 每个条目在侧栏的「插件」分组里有一个入口。打开 `/plugins/$pluginId/pages/$page` 时，主区域渲染该页面的 iframe。iframe 是 `sandbox="allow-scripts allow-forms"`，不能读 cookie 和 localStorage；表单可以用 `submit` 事件处理，但不能真正提交。
2. 页面经桥接执行 GraphQL query 和 mutation。执行权限是插件的 `granted_permissions` 与当前用户权限的交集。
3. 页面能读写本插件的 KV，读写不到其他插件的 KV。写入要求当前用户有 `write_plugins`。
4. 插件被禁用、页面 key 不存在、或用户未登录时，不渲染 iframe。页面 10 秒内不握手或脚本报错时，显示错误和重试。

### 非目标

- 插槽：dashboard 面板、渠道编辑对话框、系统设置。见 [slots.md](slots.md)。
- 页面直接访问网络。
- 按 mutation 细分插件权限。授予 `write_channels` 就是授予渠道的全部写能力，见 [妥协 1](#10-风险与妥协)。
- 把用户 token 交给 iframe；向页面传入请求体。
- 用插件页面替换 dashboard、渠道、请求这些内置页面。
- 页面自己的二级导航或菜单注入。v1 一个 page key 对应侧栏一项。

## 3. User Stories

1. 作为控制台用户，我希望从侧栏进入插件提供的整页；我没有某类数据的读权限时，页面也读不到这类数据。
2. 作为系统 Owner，我希望授权时敏感权限被高亮，并写明授予后插件页面能做什么，以便我知道装上它意味着什么。
3. 作为插件作者，我希望页面把规则和阈值存进本插件的 KV，后端 `plugin.js` 用 `ctx.kv` 读同一份数据。
4. 作为有 `write_plugins` 的管理员，我希望在插件页面修改插件数据；没有这个权限的用户只能查看。
5. 作为控制台用户，我希望关掉插件后侧栏入口消失，以便不会点进一个已经停用的页面。

## 4. 决策

| 方案 | 结论 |
|------|------|
| 页面提交 SQL | 放弃。绕过 Ent privacy，能读到凭证列，还要写三种方言 |
| iframe 直连 `/admin/graphql` | 放弃。iframe 要拿到用户 token，插件获得用户的全部权限 |
| 宿主代理 GraphQL，使用 Plugin 主体和收窄用户视图 | 采用。公共 privacy 与业务授权检查识别委托上限；直接读 Owner 标记的代码默认拿不到 Owner 权限 |
| 在 context 里只放一个独立 scope 上限 | 不采用。身份、安装和授权快照必须保持一致 |
| 合成 API Key principal | 放弃。`WithPrincipal` 不允许同一请求混用 User 和 APIKey |
| 页面写入：开放 mutation，权限上限包含 `write_*` | 采用。敏感权限在授权时高亮，风险记为妥协 |
| 页面写入：插件动作 `onAction`，后端只能调宿主 API | 放弃。写的范围更小，但每种写操作都要先加宿主 API，且页面依赖 JS 执行运行时 |
| 页面写入：开放 mutation，再加危险 mutation 黑名单 | 放弃。名单要随 schema 维护，漏一个就越权。v1 用授权高亮和妥协说明代替 |
| KV 写入：能打开页面的用户都能写 | 放弃。页面隐藏按钮挡不住直接调用端点 |
| KV 写入：新增用户 scope `write_plugins` | 采用。scope 定义见 [README](README.md#plugin) |

Plugin 主体记录真实用户 ID 用于身份和归属；context 中的用户是收窄视图，系统与项目授权分别取交集。Owner、本人身份不再自动放行插件；私有模板由 Ent privacy 拒绝。`me` 只暴露基础资料，不返回权限或关联资源。自助操作由业务入口的 `RequireDirectUser` 拒绝插件，私有资源由 Ent 本人资源规则限制；GraphQL 不维护 mutation 黑名单。详见 [权限方案](permission.md#6-私有数据自助接口与-bypass)。

## 5. 路由与侧栏

新增 `frontend/src/routes/_authenticated/plugins/$pluginId/pages/$page.tsx`。

```text
1. 路由组件读 pluginUIEntries.pages
2. 没有匹配的 pluginID + key：按现有 404 页面
3. 匹配到：渲染 PluginFrame，entry 用该 page 的 entry，占满认证布局的主区域
```

侧栏的「插件」分组在 `nav-items.ts` 里静态定义，位于「项目」和「设置」之间，静态项只有「插件管理」（`/plugins`，需要 `read_plugins`）。`sidebar.ts` 的 `useSidebarData` 在权限过滤之后把页面项追加到这一组；用户看不到或隐藏了「插件管理」时，按原位置重建只含页面项的分组。项来自 `pluginUIEntries.pages`，`title` 为字符串时直接显示，为对象时按当前语言 → `en` → `key` 回退，统一遵循 [title](manifest.md#title)。页面项不进 `nav-items.ts`，也不进 `route-permission.ts`：能看到项的条件是「已登录且该插件启用并声明了页面」。页面内部的数据由 [插件 GraphQL](#7-插件-graphql) 限制。

`PluginUIPage.title` 使用现有 `JSONRawMessage!` scalar 保留字符串或对象形式；前端声明为 `string | Record<string, string>`，用统一的标题解析函数处理，不在 GraphQL 中按某个用户语言提前转成字符串。

未登录用户打到这个路由时，由现有 `AuthGuard` 送到登录页，不签发 UI token。

## 6. 资源与桥接

```text
1. PluginFrame 调 createPluginUISession(pluginID, entry)
   token = base64url(payload{plugin_id, package_sha256, entry, user_id, exp})
         + "." + HMAC-SHA256(SystemKeySecretKey, "plugin-ui|" + payload)
   entry 必须是该插件 manifest 里某个 page 的 entry，并绑定到 token；资源读取范围见下文
2. iframe src=/admin/plugin-ui/<token>/<entry>
   sandbox="allow-scripts allow-forms"，不加 allow-same-origin
   referrerpolicy="no-referrer"
   allow-forms 只为让 submit 事件触发（没有它浏览器在派发 submit 前就拦截表单），
   真正的表单提交由 CSP form-action 'none' 拦截
3. 资源响应头：
   Content-Security-Policy: sandbox allow-scripts allow-forms; default-src 'none';
     script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline';
     img-src 'self' data:; font-src 'self'; connect-src 'none'; form-action 'none';
     frame-ancestors 'self'
   X-Content-Type-Options: nosniff
   Referrer-Policy: no-referrer
   Access-Control-Allow-Origin: *
   不设置 Access-Control-Allow-Credentials；只对 token 保护的 UI 资源路由启用，见下文 CORS 契约
4. 页面加载 /admin/plugin-ui/<token>/_axonhub/bridge.js，axonhub.ready() 发 hello
5. 宿主只接受 event.source === iframe.contentWindow 的消息
   回 init{theme, colorScheme, tokens, locale, projectId, page, panel: null, slot: null, config, canWriteKV}
   tokens 是宿主已计算的样式变量，规则见 [UI Bridge](ui-bridge.md#1-sdk-契约)
   config 去掉 secret；canWriteKV 是当前用户是否有 write_plugins
6. axonhub.graphql(query, variables)
   → PluginFrame 用自己渲染时的 pluginID POST /admin/plugins/:pluginId/graphql
   带用户 Bearer token 和当前 X-Project-ID
   iframe 消息里的任何插件 ID 都不使用
```

### 页面目录与资源路径

不要求有 `pages/` 目录。页面入口和静态资源放在 `ui/` 下，manifest 的 `ui.pages[].entry` 指向具体 HTML 文件；可以按页面分目录，也可以共享 assets。字段规则见 [Manifest](manifest.md#3-字段)。例如：

```text
manifest.json
plugin.js                 # 只有页面的插件可省略
ui/
  overview/
    index.html            # entry: ui/overview/index.html
    app.js
    shared.js             # app.js 静态 import
    details.js            # app.js 动态 import
    style.css
  assets/
    logo.svg
    font.woff2
```

HTML 使用相对路径，浏览器按入口 URL 解析，自动保留插件资源路径中的 token。插件不需要获取或拼接 token，也不能把安装路径、版本或临时目录写进资源 URL。

`ui/overview/index.html`：

```html
<!doctype html>
<html>
  <head>
    <meta charset="utf-8">
    <link rel="stylesheet" href="./style.css">
  </head>
  <body>
    <img src="../assets/logo.svg" alt="Logo">
    <main id="app"></main>
    <script crossorigin="anonymous" src="../../_axonhub/bridge.js"></script>
    <script type="module" crossorigin="anonymous" src="./app.js"></script>
  </body>
</html>
```

宿主先加载 bridge，`app.js` 再注册页面逻辑并调用 `axonhub.ready()` 完成握手。`_axonhub/bridge.js` 由宿主提供，不需要放进插件包。

`app.js` 可以使用 `import { render } from './shared.js'` 和 `import('./details.js')`。这些请求均按下文的匿名 CORS 契约加载；动态 import 不等同于页面直接使用 fetch 调用业务 API。

若入口 URL 是 `/admin/plugin-ui/<token>/ui/overview/index.html`，浏览器解析结果为：

| HTML 中的引用 | 请求路径 |
|---------------|----------|
| `./app.js` | `/admin/plugin-ui/<token>/ui/overview/app.js` |
| `./style.css` | `/admin/plugin-ui/<token>/ui/overview/style.css` |
| `../assets/logo.svg` | `/admin/plugin-ui/<token>/ui/assets/logo.svg` |
| `../../_axonhub/bridge.js` | `/admin/plugin-ui/<token>/_axonhub/bridge.js` |

路径规则：

- 不使用 `/assets/logo.svg` 这类以 `/` 开头的站点根路径，否则会丢掉插件路径和 token。外部 CDN 不在资源授权范围内。
- bridge 的相对路径取决于入口深度：`ui/index.html` 使用 `../_axonhub/bridge.js`；上面的 `ui/overview/index.html` 使用 `../../_axonhub/bridge.js`。
- CSS 的 `url(...)` 相对于 CSS 文件所在目录。例如上述 `style.css` 可用 `url('../assets/font.woff2')` 引用字体。
- JS 模块的相对 `import` 相对于该模块 URL；HTML 的资源引用相对于页面 URL。不能把两种基准混用。相对路径只解决 URL 定位，资源仍须满足 sandbox、CSP 和宿主校验。
- 交付构建后的静态文件。打包工具应生成相对资源 URL，例如 Vite 配置 `base: './'`；构建产物必须保留这里的目录对应关系和 bridge 引用。检查最终 HTML、CSS、JS，不依赖开发服务器的根路径。
- 示例中的 `../` 由浏览器在请求前解析，最终请求仍位于该 token 的 `ui/` 资源范围内；宿主对收到的路径继续做规范化和目录穿越校验。bridge 是唯一单独列出的宿主资源例外。

### 资源读取范围

UI token 的资源读取范围：

- HTML 只允许 token 绑定的 entry，签发时校验它属于 manifest 声明的页面。
- 允许同一插件、同一 `package_sha256` 的包中 `ui/` 下的 `.js`、`.css`、`.svg`、`.png`、`.woff2` 静态资源，供页面加载脚本、样式、图片和字体。包作者应将 `ui/` 视为可交给页面的公开资源目录，不放后端代码或密钥。
- 额外允许宿主提供的 `/_axonhub/bridge.js`，不能由包内文件覆盖。
- 不允许读取 `manifest.json`、根目录的 `plugin.js`、其他后端模块、未绑定的 HTML 或其他插件包。路径解码并校验后按包内规范路径查找，拒绝目录穿越和逃出 `ui/` 的请求。
- 所有资源（含 bridge）均校验签名、有效期、插件启用状态及当前包的 sha256；升级后旧包 token 失效。资源按扩展名白名单返回明确的 MIME，保留 `nosniff`。

包内 UI 资源来自本实例的版本临时目录，存放与清理见 [资源加载与释放](README.md#资源加载与释放)。鉴权成功后持有该版本资源引用，从文件流式返回，传输结束释放；不把整份文件读进 Go 内存，也不开放目录列表。宿主 bridge 仍使用内嵌资源。目录丢失或正在重建时返回 503，通知管理器重建后允许页面重试。

### UI 资源 CORS 契约

不带 `allow-same-origin` 的 sandbox iframe 使用不透明 origin，模块资源请求的 Origin 序列化为 `null`。即使模块 URL 是相对路径，也必须通过 CORS；只允许 CSP 的 script-src 并不足以让模块加载。参见 [HTML 模块脚本规则](https://html.spec.whatwg.org/multipage/scripting.html#attr-script-crossorigin)和 [Fetch 的 CORS 与凭据规则](https://fetch.spec.whatwg.org/#cors-protocol-and-credentials)。

- **范围**：仅 `GET/HEAD /admin/plugin-ui/:token/*path`，包含绑定的 HTML、允许的 UI 静态资源和宿主 bridge；静态 import、动态 import、modulepreload 及字体均使用同一规则。其他方法返回 405。正常模块资源 GET 不需要预检，第一版不开放携带自定义请求头的资源预检。
- **响应**：该路由固定返回 `Access-Control-Allow-Origin: *`，不反射请求 Origin，也不使用 `Access-Control-Allow-Origin: null`。不返回 `Access-Control-Allow-Credentials` 或 Set-Cookie；不开放 Authorization 等自定义请求头。固定通配符不需要增加 `Vary: Origin`，保留其他已有 Vary 值。成功、条件响应（若返回 304）及错误响应均保留相同 CORS 头，错误响应不能包含受保护的资源内容。
- **鉴权**：URL token 是资源访问凭据；CORS 不替代签名、有效期、启用状态、包版本和路径范围校验。所有读取（含 HEAD、条件请求）均先鉴权，不能因有 Cookie、用户 JWT 或 `Origin: null` 而放行无效 token。持有有效 token 的调用方可匿名读取其范围内的静态资源，因此包内 UI 文件不得包含密钥。
- **加载方式**：示例使用 `crossorigin="anonymous"`，Vite 产物的模块脚本和 modulepreload 也使用匿名模式；禁止 `crossorigin="use-credentials"`。不向模块请求附加 Cookie、HTTP 身份认证或用户 Bearer token，不需要 SDK 在 JS 中拼接认证头。
- **隔离保持**：iframe 和响应 CSP 仍为 `sandbox allow-scripts allow-forms`，不增加 `allow-same-origin`，不放宽 `connect-src 'none'` 和 `form-action 'none'`。这套静态资源 CORS 不能挂到整个 `/admin`，也不能应用到主 GraphQL 或插件 GraphQL 端点；数据访问继续由父页面 Bridge 代理。CORS 不改变 SDK 的 source/session 校验。
- **响应类型**：JS 模块及其依赖返回 `Content-Type: text/javascript`，CSS 返回 `text/css`，字体返回 `font/woff2`，保留 `nosniff`。缺失的 JS 返回真实 404，不能用 HTML 入口兜底，否则模块会因 MIME 不匹配失败。

`navigate` 只允许站内路径。`toast` 走宿主的通知。主题变化用 `onTheme` 再推一次。

失败：token 过期或签名错误返回 403，重试会重新申请 token。10 秒内没有 hello，或脚本报错，主区域显示「页面出错」和重试。GraphQL `FORBIDDEN` 与主端点的 ErrorPresenter 相同，由页面自己处理。

#### `internal/server/plugin/ui.go`
- 用 `SystemKeySecretKey` 签发和校验绑定 entry 的 token。签发时校验 plugin 存在、已启用、entry 是 manifest 声明的某个 page entry；读取资源时按上文分别校验 HTML、静态资源和宿主 bridge。
- 路径校验、包版本校验与 MIME 白名单；按 UI 资源 CORS 契约返回响应头，从版本临时目录流式读取，传输期间持有资源引用，目录丢失时触发重建。嵌入 `/_axonhub/bridge.js`。

#### `internal/server/plugin/uisdk/bridge.js`
- `axonhub.ready`、`onInit`、`graphql`、`navigate`、`toast`、`onTheme`。

#### `internal/server/routes.go`
- `unSecureAdminGroup`：`GET/HEAD /admin/plugin-ui/:token/*path`。token 是凭据，不走用户 JWT；CORS 只绑定这条资源路由，不能借用允许凭据的全局策略。
- 插件 GraphQL 不放在这个组，见下一节。

`pluginUIEntries` 对任意登录用户可查。实现时用系统上下文读已启用插件，只返回 UI 描述，不返回包和配置密钥。

### 共用 UI Bridge

浏览器 SDK 的方法签名、初始化上下文、消息格式、错误和会话生命周期统一见 [ui-bridge.md](ui-bridge.md)。页面与未来的 slots 共用这套契约，本篇只定义页面挂载、资源和 GraphQL 端点。

## 7. 插件 GraphQL

`POST /admin/plugins/:pluginId/graphql` 放在新的 JWT 组：`WithJWTAuth` 只接受用户 JWT，不接受 service account，加 `WithProjectID`、限流和 10 秒超时。

```text
a. 插件存在、已启用、声明了 ui.pages；否则 404
b. kvWritable = 委托用户是 Owner，或有系统级 write_plugins。写入 context
c. ceiling = granted_permissions 去掉 webhook，即插件被授予的 read_* 与 write_channels
d. authz.DelegatePlugin 派生 Plugin 主体，复制授权快照并隔离请求容器；pluginID 和步骤 a 解析到的 installationID（plugins.id）绑定本次操作
e. 接受 Query 和 Mutation，拒绝 Subscription。个人账号操作在业务入口检查直接用户身份，Owner 操作检查直接 Owner，私有资源通过 Ent privacy 检查；拒绝时返回 FORBIDDEN。
f. FixedComplexityLimit(1000)。响应上限 2 MiB
```

`DelegatePlugin` 要求已认证且身份一致的直接 User，拒绝继承 privacy Allow 或系统 bypass。context 中的用户替换为 `scopes.DelegatedUserView` 生成的收窄视图，并固定在委托快照中；每次资源授权检查 Plugin 的授权上限和用户在系统或对应项目的权限。Owner-only 操作不对 Plugin 开放。主体替换属于非法宿主调用，先审计再 panic；通用 system bypass 对 Plugin 默认拒绝（审计并返回错误或拒绝所有 Ent 操作）；普通权限不足仍返回授权错误。

页面 KV 由固定宿主操作校验安装 ID、用户身份和写资格，不向插件返回特权 context。插件获授 `write_channels` 且用户本人也拥有该权限时，凭证明文读取遵循相应显式权限；资源查询仍通过 Ent privacy。

个人资料、密码和 OIDC 解绑通过业务入口的直接用户检查保护；私人模板通过 Ent 本人资源规则保护。测试枚举 schema 的全部 Mutation 根字段并与快照比较：新增 mutation 时确认其 scope、直接用户或资源归属检查后更新快照。限制不依赖字段名、别名或 fragment。多个 mutation 的拒绝发生在字段执行时；数据库事务内的写入会随错误回滚，已发生的外部副作用不能靠此机制撤销。

`graphql.go` 抽出 `newSchemaFromDeps`。`GraphqlHandler` 增加 `PluginGraphql`，复用同一 schema，只注册 POST，`AroundOperations` 仅拒绝 Subscription，复用 ErrorPresenter，不启用 APQ。

### KV 字段

`plugin.graphql` 增加：

```graphql
type PluginKVPage {
  entries: [PluginKVEntry!]!
  nextCursor: String
}

type PluginKVEntry {
  namespace: String!
  key: String!
  value: String!
  expiresAt: Time
  updatedAt: Time!
}

extend type Query {
  pluginKV(namespace: String, key: String!): PluginKVEntry
  pluginKVList(namespace: String, prefix: String, first: Int = 100, after: String): PluginKVPage!
}

extend type Mutation {
  setPluginKV(namespace: String, key: String!, value: String!, ttlSeconds: Int): PluginKVEntry!
  deletePluginKV(namespace: String, key: String!): Boolean!
}
```

- `pluginKVList` 按 key 升序分页：`first` 默认 100，范围 1–100；首请求省略 `after`，后续传上页的 `nextCursor`。返回 `entries` 和 `nextCursor`，游标为空表示没有下一页。游标是上一页最后一个 key，查询使用排他边界；翻页时保持 namespace 和 prefix 不变。分页不提供跨请求快照，并发新增在游标之前的键需重新从首页读取。
- 字段没有插件 ID 参数。resolver 从 context 取 installationID，只有插件端点会写入；在 `/admin/graphql` 上调用返回 FORBIDDEN。不能按 manifest ID 重新查找安装，避免在途操作写入同名重装插件的 KV。
- 读：能打开页面的用户都可以。写：要求 context 里的 `kvWritable`，否则 FORBIDDEN。
- 直接调用 [`kv.go`](README.md#pluginstorage)，和 `ctx.kv` 是同一份数据、同一套校验（key、value 大小、TTL、list 条数）。namespace 省略时为 `default`。

## 8. 改动清单

### 后端
- `internal/server/plugin/ui.go`、`uisdk/bridge.js`：见上文。
- `internal/authz/plugin.go`、`internal/authz/identity/`：受控委托、安装身份和授权快照。
- `internal/server/plugin/graphql.go`：插件端点、业务主体鉴权、kvWritable、pluginID 与 installationID 写入 context。
- `internal/server/gql/graphql.go`：抽出 `newSchemaFromDeps`，增加 `PluginGraphql`。
- `internal/server/gql/plugin.graphql` 与 resolver：KV 字段。执行 `make generate` 后手写 `pluginKV`、`pluginKVList`、`setPluginKV`、`deletePluginKV`。
- `internal/server/routes.go`：两个路由组。

### 前端
- `features/plugins/components/plugin-frame.tsx`：握手、主题、GraphQL 转发、navigate、toast、错误与重试。
- `routes/_authenticated/plugins/$pluginId/pages/$page.tsx`
- `sidebar.ts`：把页面项追加到「插件」分组。
- `locales/en/plugins.json`、`locales/zh-CN/plugins.json`：分组名、404、页面出错文案。key 与管理页共用这两个文件。

## 9. 验收

| 类别 | 标准 | 验证方式 |
|------|------|----------|
| 功能 | 声明了一个 page 的已启用插件出现在侧栏；点击后 iframe 完成握手、主题切换、GraphQL 查询 | `cd frontend && npm run test:unit`；`make e2e-test` 的 plugin-page |
| 功能 | 字符串 title 在各语言下原样显示；对象 title 随当前语言切换，缺少当前语言时回退 en；GraphQL 保留两种 JSON 形式 | GraphQL resolver 测试及前端标题解析单测 |
| SDK | 重复 ready 只握手一次；晚注册 onInit 能收到快照，取消订阅不再回调；并发调用乱序回复正确配对，旧会话/错误 source 的消息不生效；超时和销毁清理 pending；GraphQL 部分 data/errors 保留，HTTP 错误映射稳定且不自动重试 | bridge 单测与 plugin-page E2E |
| 功能 | `ui/index.html` 和 `ui/overview/index.html` 两种入口深度均能通过相对路径加载独立 JS、CSS、共享图片、CSS 引用的字体及宿主 bridge，并完成握手；请求 URL 保留 token | plugin-page E2E，使用含独立资源的插件包并检查资源请求路径 |
| 模块加载 | 在真实 `sandbox="allow-scripts"` 下，模块页面的静态 import、动态 import、modulepreload 和字体成功加载并完成 ready；覆盖一个 `base: './'` 的 Vite 构建产物；无 CORS/MIME/CSP 错误 | plugin-page 浏览器 E2E，不使用取消 sandbox 的测试替身；检查模块请求 Origin 为 null、无 Cookie/Authorization，响应含 ACAO `*` 且不含 ACAC |
| CORS 权限 | 合法 token 的 GET/HEAD 能读取模块资源；无效、过期、升级前 token 及越界路径在 Origin 为 null 时仍被拒；404 不返回 HTML 兜底；错误及条件响应保留 CORS 头；GraphQL 未继承这套资源 CORS | `go test ./internal/server/plugin/ -run TestPluginUI` 与路由集成测试 |
| 隔离 | 模块成功执行后仍无法访问父窗口 DOM、宿主 cookie 或 localStorage，直接 fetch GraphQL 仍被 CSP 阻止，Bridge 查询正常 | plugin-page 浏览器 E2E |
| 权限 | UI token 可读取绑定的 HTML 与同包白名单静态资源；读取 manifest、后端模块、其他 HTML、其他插件资源或目录穿越路径被拒；包升级后旧 token 被拒 | `go test ./internal/server/plugin/ -run TestPluginUI` |
| 功能 | 禁用插件后侧栏项消失，直接打开旧 URL 得到 404 | 同上 |
| 权限 | 插件被授予 `read_channels`：能读 channels，`credentials` 为 null，任何渠道 mutation 被拒 | `go test ./internal/server/plugin/ -run TestPluginGraphql` |
| 权限 | 插件被授予 `write_channels`，用户也有：渠道 mutation 成功；用户没有时被拒 | 同上 |
| 权限 | 用户没有 `read_requests`，即使插件被授予了 `read_requests`，requests 仍被拒 | 同上 |
| 权限 | Owner 打开页面时，`backup` 等只认 Owner 的查询和 mutation 被拒 | 同上 |
| 权限 | 自助 mutation 全部被拒；Mutation 根字段快照与 schema 一致 | 同上 |
| KV | 页面写入后 `ctx.kv.get` 读到同一值；插件 A 的页面读写不到插件 B 的键；在 `/admin/graphql` 调用 KV 字段被拒 | `go test ./internal/server/plugin/ -run TestPluginKVGraphql` |
| KV | 没有 `write_plugins` 的用户写入被拒，读取成功；Owner 写入成功 | 同上 |
| 错误处理 | 不握手或脚本抛错时显示错误和重试，侧栏与其他路由不受影响 | E2E |
| i18n | `en` 与 `zh-CN` 的 key 一致 | 前端既有 i18n key 校验 |

## 10. 风险与妥协

| 风险 | 触发条件 | 影响 | 对策 |
|------|----------|------|------|
| 降权被绕过 | 新 resolver 按用户 ID 从数据库重新加载用户再做权限判断 | 页面得到用户的完整权限 | 替换 context 用户无效（`GetUser` 固定返回收窄视图）；重新加载的查询仍受 Plugin privacy 约束；Owner 判断走 `scopes.IsDirectOwner`。降权矩阵覆盖全部 Query 根字段，`me` 除外 |
| 新增自助操作缺少主体检查 | 新业务入口只认用户 ID | 页面能以用户身份修改其个人数据 | 业务入口统一调用 `RequireDirectUser`，保留跨入口授权回归测试 |
| 页面滥用写权限 | 插件被授予 `write_channels`，有该权限的用户打开页面 | 页面在用户不知情时改渠道，包括改渠道地址把凭证送到外部，或读出凭证明文 | 授权对话框高亮并写明影响；妥协 1 |

```text
1. 妥协：页面的写权限按 scope 整类授予，不细分到 mutation
   - 原因：scope 只有 read / write 两档，按 mutation 细分要为插件单独设计一套权限
   - 影响：授予 write_channels 后，页面能执行全部渠道 mutation，并能读取凭证明文。
     iframe 不能联网，但改渠道地址后，凭证会随下一次请求发到该地址
   - 升级触发：出现需要写权限、但不应接触凭证的插件，或要开放给不可信第三方
   - 后续方向：增加插件专用的细分权限（例如只允许启停渠道），或改为插件动作 onAction

2. 妥协：页面在用户打开时即可执行写入，宿主不逐次确认
   - 原因：逐次确认会让正常的管理页面无法使用
   - 影响：用户只要打开页面，插件就能以其身份写入
   - 升级触发：同妥协 1
   - 后续方向：对高亮权限的写入增加宿主确认对话框
```
