# Manifest

[插件系统](README.md)的公共契约：插件包里 `manifest.json` 的唯一定义。运行时、事件、请求钩子、页面，以及未排期的响应钩子、插槽、Provider 都依赖本篇，各篇不另写字段和校验。包结构见 [包](README.md#包)，数值限制见 [运行限制](README.md#运行限制)。任务见 [任务表](README.md#9-任务管理)；各字段由哪个任务开放见 [字段表](#3-字段)。

> hooks 对象是未发布接口的修订设计，运行时已同步；schemaVersion 和 pluginApiVersion 均保持 1，不兼容旧的布尔 hooks。

## 1. 命名

Go 类型是 `internal/objects/plugin.go` 的 `PluginManifest`。插件能看到的 JSON 都用 camelCase：manifest、市场索引、JS 侧的 `ctx`、事件数据。这与 `internal/objects` 的惯例一致，该包的 JSON tag 绝大多数是 camelCase。枚举值不跟着改，例如事件类型 `channel.auto_disabled`、权限 `read_channels`、chunk 类型 `tool_call`。`configSchema` 内部是 JSON Schema，关键字保持规范原样。

## 2. 示例

```json
{
  "schemaVersion": 1,
  "id": "acme.failover-guard",
  "name": "Failover Guard",
  "version": "1.0.0",
  "description": "Notify and disable channels on failover storms",
  "axonhubVersion": "^1.0.0",
  "pluginApiVersion": 1,
  "permissions": ["read_channels", "write_channels", "webhook"],
  "events": ["request.failover", "channel.credential_disabled"],
  "configSchema": {
    "type": "object",
    "properties": {
      "threshold": {
        "type": "integer",
        "title": "Threshold",
        "minimum": 1,
        "default": 10
      },
      "target": { "type": "string", "title": "Webhook target" },
      "token": { "type": "string", "format": "secret" }
    },
    "required": ["token"]
  },
  "ui": {
    "pages": [
      {
        "key": "main",
        "entry": "ui/page.html",
        "title": { "en": "Failover Guard", "zh-CN": "故障转移守卫" }
      }
    ]
  }
}
```

## 3. 字段

字段是逐步开放的。「开放」列是让该字段生效的任务。该任务完成前，以及标为「未排期」的字段，出现即拒绝安装，返回「规则」列里的错误，不写库，避免字段被静默忽略。开放新字段时同步更新规则、任务状态、解析及调用链；仅新增能力不改变版本号。不兼容变化遵循版本兼容规则，本次发布前修订例外见 hooks 一节。

| 字段                                  | 必填 | 规则                                                                                                                                                     | 使用方                                           | 开放   |
| ------------------------------------- | ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------ | ------ |
| `schemaVersion`                       | 是   | 只能是 `1`                                                                                                                                               | 运行时                                           | T-01   |
| `id`                                  | 是   | `^[a-z0-9]+(\.[a-z0-9-]+)+$`，≤ 64；不能与内置插件相同；安装后不可变                                                                                     | 运行时、KV、市场                                 | T-01   |
| `name`                                | 是   | 非空字符串                                                                                                                                               | 管理页、市场                                     | T-01   |
| `version`                             | 是   | semver，用 `Masterminds/semver/v3` 解析                                                                                                                  | 升级、市场                                       | T-01   |
| `description`                         | 否   | 字符串                                                                                                                                                   | 管理页、市场                                     | T-01   |
| `axonhubVersion`                      | 是   | npm 风格的 semver 范围，见 [版本兼容](#版本兼容)                                                                                                         | 运行时、市场                                     | T-01   |
| `pluginApiVersion`                    | 是   | 整数，当前只能是 `1`，见 [版本兼容](#版本兼容)                                                                                                           | 运行时                                           | T-01   |
| `permissions`                         | 否   | 只能取 [权限词表](#5-权限词表) 中已开放的权限；省略为 `[]`                                                                                               | 授权、宿主 API、页面 GraphQL                     | T-01   |
| `network`                             | 否   | 最多 32 条。字符串是精确 `https` URL，必须带 path，无 userinfo、query、fragment。对象只能是 `{ "regex": "..." }`，正则全匹配且最长 256 字符。省略为 `[]`。开放前出现：`network is not enabled` | [出站请求](plugin.js.md#出站请求)                 | T-16   |
| `configSchema`                        | 否   | 见 [configSchema](#configschema)                                                                                                                         | 管理页配置表单、`ctx.config`、页面 `init.config` | T-01   |
| `ui.actions` | 否 | 数组；key/slot/title/effect 必填，icon 可选；首版仅 app.header.actions + dialog，严格字段白名单，详见 [操作插槽](slots.md#3-manifest) | 工具栏、弹窗、UI 资源 | T-17 |
| `ui.pages[].key`                      | 是   | `^[a-z0-9][a-z0-9-]{0,31}$`，在该插件内唯一                                                                                                              | [页面](pages.md) 路由                            | T-01   |
| `ui.pages[].entry`                    | 是   | 包内 `ui/` 下存在的 `.html`                                                                                                                              | [页面](pages.md) iframe                          | T-01   |
| `ui.pages[].title`                    | 是   | 非空字符串，或语言 → 非空标题的对象，见 [title](#title)                                                                                                  | [页面](pages.md) 侧栏                            | T-01   |
| `events`                              | 否   | 只能取 [events.md](events.md#5-事件) 里当前开放的 8 种；非空时要求导出 `onEvent`。开放前出现：`events are not enabled`。旧 WebHook 的 4 个事件名暂不接受 | [事件](events.md)                                | T-12   |
| `hooks.request` | 否 | 对象；enabled 必填。true 时 requestTypes 必须非空且导出 onRequest；false 不调度 | [请求钩子](request.md) | T-13 |
| `hooks.attempt` | 否 | 对象；enabled 必填。true 时 apiFormats 必须非空且导出 onAttempt；false 不调度 | [请求钩子](request.md) | T-13 |
| `hooks.response`、`hooks.streamChunk` | —    | 出现即拒绝：`response hooks are not enabled`                                                                                                             | [响应钩子](response.md#manifest)                 | 未排期 |
| `ui.panels`                           | —    | 出现即拒绝：`ui panels are not enabled`                                                                                                                  | 未开放，暂无面板设计                      | 未排期 |
| `providers`                           | —    | 出现即拒绝，包括空数组：`custom channel providers are not enabled`                                                                                       | [Provider](provider.md#5-接入)                   | 未排期 |

`ui.actions` 已随 T-17 开放，接受 [操作插槽](slots.md#3-manifest) 定义的 action/effect 固定结构。`ui.panels` 继续拒绝，不再规划内嵌面板。

未排期字段的设计写在各自文档里，排期时并入本表。

### title

页面、操作，以及未来的 provider，统一使用 `title`，支持两种形式：

```json
{ "title": "Overview" }
```

```json
{ "title": { "en": "Overview", "zh-CN": "概览" } }
```

- 字符串：所有语言直接显示该文本。
- 对象：key 是语言标识，value 是非空字符串，至少包含 `en`。显示时优先当前语言，其次 `en`，最后用条目的 `key` 作防御性回退。
- 缺失、null、空白字符串、数组、非字符串的对象值或缺少 `en` 的对象均拒绝。字符串形式不需要额外提供语言字段。
- `configSchema` 内属性的 `title` 是配置表单标签，仍遵循该节的 JSON Schema 规则，不套用这里的联合类型。

### 版本兼容

**`axonhubVersion`** 声明插件能在哪些 AxonHub 版本上运行，写法参考 npm 的 `engines.node`：

- 范围语法同 node-semver：`^1.0.0`、`~1.2.0`、`>=1.0.0 <2.0.0`、`1.x`、`||`。Go 端用仓库已有的 `Masterminds/semver/v3`，语法基本一致。推荐写 `^`。
- 不满足时拒绝安装。npm 默认只警告，这里不跟：插件调用当前版本没有的宿主 API，只会在运行时出错。
- 比较前去掉 AxonHub 版本的预发布后缀，`v1.0.0-beta9` 按 `1.0.0` 比较。semver 默认不让预发布版本满足 `^1.0.0`，不去掉的话现在的 beta 版都装不上插件。版本来自 `internal/build.Version`：发布构建由 goreleaser 写入，其他构建读内嵌的 `internal/build/VERSION`。
- 市场索引里的 `axonhubVersion` 用同一套比较。

**`pluginApiVersion`** 声明插件按第几代插件接口编写。插件接口指 `ctx` 上的宿主 API、`onEvent` / `onRequest` / `onAttempt` 的参数与返回格式、页面桥接的 `axonhub.*`。

- AxonHub 在代码里声明支持的代，当前只有 `1`。不在支持列表里就拒绝安装，错误为 `plugin API version N is not supported`。
- 插件接口有不兼容变化时加一代。新增宿主 API、新增可选字段不算不兼容。
- 单独设这一项，是因为 AxonHub 的版本号跟着网关功能走，插件接口可能在 1.x 的小版本里变化，`axonhubVersion` 挡不住，作者写的不封顶范围更挡不住。

### hooks

```json
{
  "request": { "enabled": true, "requestTypes": ["chat", "completion"] },
  "attempt": { "enabled": true, "apiFormats": ["openai/chat_completions"] }
}
```

两个钩子独立注册。省略钩子对象表示未声明；声明时 enabled 必填，不能根据范围列表隐式推断。enabled=false 允许省略列表或保留空/非空列表，不执行且不要求对应导出；enabled=true 要求非空范围和同名函数导出。空列表不代表全部，未启用的列表也必须满足类型、枚举和去重校验。省略整个 hooks 表示不注册请求钩子。

request 对象只允许 enabled、requestTypes；attempt 只允许 enabled、apiFormats。requestTypes 使用 llm.RequestType，apiFormats 使用 llm.APIFormat 的精确枚举值（见 llm/constants.go），不接受通配符、重复项或未知值。范围列表内为 OR；attempt 按实际出站格式每次重新匹配，与 request 是否匹配无关。安装记录 enabled 仍是整个插件的总开关。

这次修订发生在发布前，因此不增加 pluginApiVersion，也不提供旧布尔值或旧参数顺序兼容；发布后仍遵循上面的版本兼容规则。

### configSchema

- 顶层只能是 `object`。
- 属性类型限 `string|integer|number|boolean`，可带 `enum`、`minimum`、`maximum`、`default`、`title`、`description`、`format: secret`。
- `format: "secret"` 的字段在 GraphQL 返回 `"******"`，原样传回表示不改；交给页面的 `init.config` 去掉这些字段。
- 顶层可带 `required`（字符串数组），每项必须是已声明且不重复的属性名。
- 值校验由 `github.com/google/jsonschema-go` 完成：服务端把本 schema 转成 JSON Schema 并补上 `additionalProperties: false`，未声明字段被拒绝。错误只含字段名和关键字，不含值。
- 停用的插件可以保存不完整的配置；启用插件、保存已启用插件的配置、升级已启用插件时都检查 `required`。没保存的字段用 `default` 补齐后再判断，空字符串视为缺失。
- 保存的 `config` 按本 schema 校验，大小见 [运行限制](README.md#运行限制)。升级时已有配置的处理见 [升级比较](#6-升级比较)，不能直接用新 schema 解释旧值。

## 4. 安装校验

脚本入口形式、命名导出与返回值约定见 [plugin.js.md](plugin.js.md)。只识别命名导出，不从 default 对象或工厂函数发现钩子。

任一项失败都返回可读错误，不写库。

**结构校验**（`manifest.go`）：

1. JSON 能解析为 `PluginManifest`，[字段表](#3-字段) 里已开放字段的规则全部满足。对固定结构逐层校验字段白名单，未知字段拒绝并报告完整字段路径，不能使用 Go JSON 解码默认的静默忽略行为。
2. 已知但未开放字段按字段表的规则拒绝，优先返回对应的专用错误，不当作普通未知字段。例如 `hooks.requset` 返回未知字段错误，`hooks.response` 返回 `response hooks are not enabled`。
3. `axonhubVersion`、`pluginApiVersion` 与当前 AxonHub 兼容。

固定结构包括 manifest 根对象、hooks、hooks.request、hooks.attempt、ui、pages/actions 条目、action.effect，以及 configSchema 的受支持关键字和每个属性的 schema。`title` 对象的语言键、`configSchema.properties` 中的属性名属于动态映射，按各自的值类型与规则校验，不能套用固定字段名单。actions/effect 已有结构白名单；未来开放 providers 时再增加对应结构，当前不提前定义未开放结构。

**导出校验**（`jsruntime.go`，T-10）：包里有 `plugin.js` 时，在 `DisableDynamicCode` 的临时 runtime 里编译并执行顶层（1 s），然后检查：

1. `events` 非空 → 导出 `onEvent`。
2. `hooks.request.enabled` 为 `true` → 导出 `onRequest(ctx, request)`；`hooks.attempt.enabled` 为 `true` → 导出 `onAttempt(ctx, body)`。事件导出统一为 `onEvent(ctx, event)`。
3. 上述导出都是函数。允许 async。顶层 `await` 拒绝。

非空 events 或任一 enabled=true 的请求钩子要求包内有 plugin.js；全部禁用的 hooks 不单独要求后端脚本。

## 5. 权限词表

| 权限                                                                                                  | 来源              | 用途                                                                                                                                                    | 开放   |
| ----------------------------------------------------------------------------------------------------- | ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| `read_channels`、`read_requests`、`read_dashboard`、`read_api_keys`、`read_projects`、`read_settings` | `internal/scopes` | 宿主 API 读接口；页面 GraphQL 上限                                                                                                                      | T-01   |
| `write_channels`                                                                                      | `internal/scopes` | 宿主 API 的渠道和凭证启停；进入页面 GraphQL 上限，页面因此能执行全部渠道 mutation 并读取凭证明文。授权对话框高亮，见 [pages.md](pages.md#10-风险与妥协) | T-01   |
| `webhook`                                                                                             | 插件专用          | `ctx.webhook.send` 使用已配置的 WebHook 目标                                                                                                            | T-01   |
| `fetch`                                                                                               | 插件专用          | 出现即拒绝：`fetch is not enabled`。出站目标写在 `network`，不使用这个权限名                                                                             | 未排期 |

- manifest 的 `permissions` 只声明插件想要什么。Owner 启用时勾选，勾选项必须是声明的子集，写入 `plugins.granted_permissions`。宿主 API 和页面 GraphQL 查的是这一列。`network` 另存为 `granted_network`，规则相同：勾选项必须是声明的子集，未提交时取旧授权与新声明的交集。
- KV 不在词表里：插件只能读写本次安装（`plugins.id`）的数据，安装 ID 由宿主绑定，JS 和页面都不能指定。升级保留安装 ID，同名重装使用新 ID，见 [PluginStorage](README.md#pluginstorage)。
- `read_plugins`、`write_plugins` 不在词表里：它们授予用户，不授予插件，见 [Plugin](README.md#plugin)。

## 6. 升级比较

同一 `id` 再次上传时，按新旧 manifest 比较「新版本声明了、旧版本没声明」的项。下面任一项非空，且插件已启用，就返回 `PluginUpgradePreview` 让 Owner 确认，流程见 [安装与同步](README.md#安装与同步)：

| 项            | 比较方式                             |
| ------------- | ------------------------------------ |
| `permissions` | 集合差                               |
| `network`     | 集合差。精确 URL 按字符串，正则按 `regex` 原文 |
| `events`      | 集合差                               |
| `hooks` | 新启用的钩子，或任一 requestTypes/apiFormats 新增匹配项；禁用配置只保存、不调度，启用时再走确认 |
| `ui.pages`    | 按 `key` 的集合差                    |
| `ui.actions`（T-17） | 新增 key，或既有 key 的 slot/effect.type 改变；详见 slots.md |

`name`、`version`、`description`、`axonhubVersion`、`entry`、`title` 的变化不需要确认。`configSchema` 的变化不单独触发确认，但必须通过下面的配置迁移校验。

所有升级路径（已启用、已停用、本地上传、市场安装）都执行以下规则：

1. **授权收敛**：没有显式提交 `grantedPermissions` 时，新授权为旧授权与新 `manifest.permissions` 的交集；不自动授予新增权限。显式提交时，该参数表示完整的新授权集合（不是增量），必须是新声明的子集，否则拒绝。`grantedNetwork` 使用同样的交集和完整集合规则，对应 `manifest.network`。包、manifest、config 和最终授权在同一事务里写入。
2. **禁止 secret 降级**：旧 schema 中标为 `format: secret` 的字段，若新 schema 仍声明同名字段，该字段必须继续标为 secret；否则拒绝升级，不写库。错误只包含字段名和原因，不包含值。即使插件已停用或 Owner 确认新增权限，也不绕过此校验。
3. **删除废弃配置**：新 config 只保留新 schema 声明的字段；被删除的字段（包括 secret）从配置中移除，不交给新版本、配置查询或页面。没有 `configSchema` 等同于没有声明字段。对保留的值按新 schema 校验，不兼容则拒绝升级，旧版本与旧配置保持不变。v1 不做隐式类型转换。
4. **已启用插件需满足 required**：迁移后的配置缺少新版本 `required` 字段时拒绝升级，Owner 先停用、升级、补配置再启用。
5. **确认时重新校验**：预览阶段只计算变更，不修改配置或授权；最终写入时，基于事务内读取的当前插件行重新执行上述规则，避免预览之后的配置变更绕过校验。

## 7. 改动清单

### `internal/objects/plugin.go`

- `PluginManifest`、`PluginHooksSpec`、`PluginUI`、`PluginUIPage`、`PluginNetworkRule`（精确 URL 或 `{ regex }`）。
- T-16 完成前，`network` 出现即返回 `network is not enabled`，不定义授予列。开放时再写入 `granted_network`。
- `PluginTitle`：支持字符串或语言映射的 JSON 编解码和校验，保留原始形式；页面、操作及未来的 provider 共用。
- 固定结构按层校验字段白名单，并保留错误字段路径；动态语言映射和配置属性名按各自契约校验。为识别未开放字段，解析时记录 `ui.panels`、`providers`、`hooks.response`、`hooks.streamChunk` 是否出现，不定义它们的结构。
- `Validate()`：结构校验。

### `internal/server/plugin/manifest.go`

- 字段开放表：字段 → 是否开放、未开放时的错误。开放字段的任务只改这张表和对应规则。
- `axonhubVersion` 比较、支持的 `pluginApiVersion` 列表、权限词表、事件类型白名单（取自 `internal/server/events`）、升级比较 `DiffManifest(old, new)`；升级授权收敛、secret 降级校验与配置迁移，供所有安装路径共用。

### `internal/server/plugin/jsruntime.go`

- 导出校验，T-10 加入。

## 8. 验收

| 类别     | 标准                                                                                                                                                                        | 验证方式                                                  |
| -------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| 结构     | 字段表每条规则各有一个违反用例，均返回可读错误且不写库                                                                                                                      | `go test ./internal/server/plugin/ -run TestManifest`     |
| 结构     | 根对象、hooks（含 request/attempt 的 enabled 与范围）、ui、pages/actions 条目、action.effect 和 configSchema 的未知固定字段均拒绝并返回完整路径；合法语言键和自定义配置属性名通过；已知未开放字段仍返回专用错误                             | 同上，包含 `hooks.requset` 与 `hooks.response` 的区分用例 |
| 标题     | `title` 的字符串与多语言对象均通过且可往返序列化；非法类型、空白值、缺少 `en` 的对象被拒                                                                                    | 同上                                                      |
| 开放     | 字段表里每个未开放字段各一个用例，错误文本与表中一致；开放任务完成后对应用例改为通过                                                                                        | 同上                                                      |
| 版本     | `axonhubVersion: "^1.0.0"` 在 `v1.0.0-beta9` 上通过，在 `0.9.0` 和 `2.0.0` 上失败；`pluginApiVersion: 2` 失败                                                               | 同上                                                      |
| 导出     | 缺少导出、顶层 await、声明了 `events` 但没有 `plugin.js`：安装失败。async 钩子可以通过                                                                                         | `go test ./internal/server/plugin/ -run TestParsePackage` |
| 升级     | 只改 `version` 或兼容的 `configSchema` 时直接替换；多一个权限、出站规则、事件、钩子或页面时返回预览                                                                                   | `go test ./internal/server/plugin/ -run TestUpgrade`      |
| 升级权限 | 新版移除 `write_channels` 后，持久化授权不再包含它，新代宿主 API 和页面渠道 mutation 被拒；停用插件升级也收敛授权；显式授权超出新声明时不写库                               | 同上，覆盖本地和市场安装                                  |
| 升级配置 | secret 改为普通字段时升级失败，包、manifest、config、授权均不变，错误不含密钥；删除 secret 字段时一并删除旧值，配置查询与页面 init 均不返回该值；保留字段类型不兼容时不写库 | 同上，覆盖启用、停用、预览后确认及两种安装入口            |
