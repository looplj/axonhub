# 插件系统

> 档位：完整档。运行时在本文。当前交付三个扩展点：事件、请求钩子（只有 `onRequest` / `onAttempt`）、独立页面。响应钩子、插槽、自定义 Provider、按 project / API Key / 模型收窄各自另篇，都不在任务表。

扩展点按用户能装上的能力拆开，而不是按「通知产品」拆开。各扩展点共用的 `manifest.json` 定义在 [manifest.md](manifest.md)，各篇只引用。WebHook 是事件的一个内置订阅者，所以不单独成篇，放在 [events.md](events.md)。页面与插槽共用的 SDK 和消息协议写在 [ui-bridge.md](ui-bridge.md)，iframe 资源与插件 GraphQL 写在 [pages.md](pages.md)。自定义 channel provider 与请求钩子分开：钩子发生在选路之前，Provider 是选中渠道之后的出站转换。

| 模块 | 文档 | 做什么 |
|------|------|--------|
| Manifest | [manifest.md](manifest.md) | 包内 `manifest.json` 的字段、校验、权限词表、升级比较。下列模块都依赖它 |
| 脚本入口 | [plugin.js.md](plugin.js.md) | `plugin.js` 的命名导出、参数与返回值、ctx、示例及加载生命周期 |
| 权限 | [permission.md](permission.md) | Plugin Principal 的待实施方案、授权规则、迁移和验收；当前仍使用用户权限副本 |
| UI Bridge | [ui-bridge.md](ui-bridge.md) | 页面与未来 slots 共用的浏览器 SDK、初始化上下文、消息协议及错误处理 |
| 运行时 | 本文 | 安装、启停、市场、KV、JS 引擎 |
| 事件 | [events.md](events.md) | 事件总线、12 种事件、内置 WebHook、`onEvent` |
| 请求 | [request.md](request.md) | `onRequest` / `onAttempt` |
| 响应 | [response.md](response.md) | `onResponse` / `onStreamChunk`，按 chunk 类型过滤。未排期 |
| 页面 | [pages.md](pages.md) | 插件独立页面、侧栏入口、iframe、插件 GraphQL（含 mutation 与 KV） |
| 插槽 | [slots.md](slots.md) | dashboard 面板。未排期 |
| Provider | [provider.md](provider.md) | 自定义 channel 的出站转换。未排期 |
| 作用范围 | [scope.md](scope.md) | 按 project、API Key、模型收窄请求钩子。可选，未排期 |

## 1. 背景

用户反馈：现有 WebHook 只能监听少数事件，不能按凭证或模型过滤，也不能在「故障转移超过 N 次」之后做处置。运维同时希望不改前端源码就能放自定义页面。

现状、问题与约束分到各篇。对所有模块都成立的约束：

- 对外 OpenAI / Anthropic / Gemini 兼容接口不能被插件机制本身破坏。插件执行出错时请求按无插件处理；请求钩子取不到 VM 时返回 429，见 [request.md](request.md#vm-不足)。
- 兼容 PostgreSQL、MySQL/TiDB、SQLite。改 Ent / GraphQL 后执行 `make generate`，不手写 schema migration SQL。
- 现有 `webhook_notifier_config` 和 4 个旧事件的模板变量必须原样可用。细节见 [events.md](events.md)。
- Redis 可选。`memory` 缓存模式下功能可用，降级见本文已知妥协。
- 只有 Go 内置插件和 JS 插件。JS 引擎是 `github.com/MoeForge/moejs` v0.1.0-alpha.11，纯 Go，无 CGO。
- 不改 `llm/`。请求钩子只通过已有 `pipeline.Middleware` / `OutboundLlmRequestMiddleware` 接入。自定义 Provider 只实现已有的 `transformer.Outbound`，见 [provider.md](provider.md)。

## 2. 目标

1. **运行时**：Owner 可上传 `.axplugin` 或从市场索引安装 JS 插件，授予权限后启用，按 manifest 修改配置。变更在 30 秒内生效；`cache.mode=two-level` 时为 2 秒内。
2. **事件、请求、页面**：见各篇目标。
3. **KV**：JS 插件用 `ctx.kv` 读写只属于自己的键。重启后和多实例之间是同一份数据。

### 非目标

1. 进程外插件（gRPC 子进程、Bun 宿主、WASM）。
2. 包签名与发布者校验。市场只校验索引里的 sha256。
3. 声明式规则引擎。v1 用 JS 插件加计数器，见 [events.md](events.md)。
4. 插件选择渠道或实现路由策略。自定义 channel provider 见 [provider.md](provider.md)，不在当前任务表。
5. 插件页面直接访问网络。页面的写权限不细分到 mutation，见 [pages.md](pages.md#10-风险与妥协)。
6. 插件主动发 HTTP（`ctx.fetch`）。插件只能用 `ctx.webhook.send` 发到 Owner 已配置的 WebHook 目标。
7. 插件插槽：dashboard 面板、渠道编辑对话框、系统设置。v1 只有独立页面，见 [slots.md](slots.md)。
8. Owner 自定义市场源。只用官方索引，地址写在代码里，见 [市场](#市场)。
9. 宿主按 project、API Key、模型收窄请求钩子。当前请求钩子对全部请求生效，插件自行判断，见 [scope.md](scope.md)。

### 成功信号

| 信号 | 验证方式 |
|------|----------|
| 上传合法包后授权，30 秒内各实例状态为 active | `go test ./internal/server/plugin/...` |
| sha256 不符的市场包不写库；相符的包与本地上传同一包结果相同 | `go test ./internal/server/plugin/ -run TestMarket` |
| `ctx.kv.set` 的值重启后仍能 `get`；插件 A 读不到插件 B 的键 | `go test ./internal/server/plugin/ -run TestKV` |
| 各扩展点的成功信号 | 见各篇 |

## 3. User Stories

1. 作为系统 Owner，我希望安装时看到插件要的权限，并显式授予，以便它不能越权操作渠道或外发数据。
2. 作为系统 Owner，我希望插件出错只停用它自己，以便不影响网关。
3. 作为系统 Owner，我希望从官方索引里选一个版本安装，以便不用自己找包和计算校验和。
4. 作为插件作者，我希望把游标和去重记录放进只属于本插件的 KV，以便重启后接着处理。
5. 作为维护者，我希望 biz 不 import 插件包，以便核心功能在无插件时独立测试。
6. 作为系统 Owner，我希望把插件的查看、配置和数据维护交给有 `read_plugins` / `write_plugins` 的管理员，而安装、启停和授权只由我做，以便插件拿到的权限不超过我的决定。

事件、请求、页面的使用方故事见各篇。

## 4. 决策

| 题目 | 选择 | 不选 |
|------|------|------|
| JS 引擎 | moejs v0.1.0-alpha.11。原生 ESM，`Runtime.Interrupt` 可中断，`DisableDynamicCode` 禁用 `eval` | goja：不支持 ESM。Lua：和页面语言割裂 |
| 市场 | 一个官方索引，地址写在代码里，索引仓库走 PR 维护。索引给出包地址和 sha256，校验后走本地安装 | Owner 配置多个源：目前没有第三方索引。只允许本地上传：用户要自己找包 |
| KV | 表 `plugin_storages`，按安装 ID（`plugins.id`）隔离；同名重装使用新 ID | 只用 Redis：`memory` 模式没有 Redis，重启即丢。复用计数器：计数器没有按键删除 |
| 事件投递 | 进程内总线 | 见 [events.md](events.md) |
| 页面读写数据 | 宿主代理 GraphQL，降权用户，开放 mutation | 见 [pages.md](pages.md#4-决策) |
| 升级时多要了权限 | 先返回预览，Owner 确认后再写库，确认前旧版本照常运行 | 自动停用或要求先停用再上传：插件承担的功能会中断 |
| VM 生命周期 | 按次借用，调用返回即归还；请求内状态存宿主 | 按请求独占：流式请求整段占着 VM，并发流一多池子就满 |
| 钩子执行出错 | 跳过该插件，请求按钩子前的值继续。插件要拦截请求就显式调 `ctx.reject` | 按插件配置失败时拒绝请求：脚本异常会中断正常网关请求 |
| 取不到 VM | 请求钩子返回 429 `plugin_overloaded`，见 [VM 不足](request.md#vm-不足) | 跳过插件放行：高负载时插件规则悄悄失效。返回 503：前置负载均衡可能把实例判为故障 |

原则：

1. 没有订阅者的事件不构造对象。没有请求钩子的插件时，不追加中间件。
2. 事件处理和请求钩子执行出错（抛错、超时、非法返回）时 fail-open。请求钩子取不到 VM 时拒绝请求，不跳过。权限判断 fail-closed。
3. 事件只有白名单字段，不含请求体、响应体和凭证明文。插件后端代码的副作用只通过宿主 API；页面的写入只通过插件 GraphQL 端点。
4. 插件定义在 `plugins`，KV 在 `plugin_storages`。编译结果、熔断、统计、市场索引缓存不跨实例共享。
5. `internal/server/events` 不依赖 biz。biz 只发布事件，不 import 插件包。

| 假设 | 如果不成立 | 应对 |
|------|------------|------|
| 安装插件的是可信的系统 Owner | 要开放给不可信第三方 | 包签名，见妥协 1 |
| moejs 满足 [运行限制](#运行限制) 里的钩子耗时 | 基准超出 [request.md](request.md) 的阈值 | 只开放 `onAttempt` 或推迟请求钩子，事件、页面照常交付 |
| 多实例要全局阈值时配置了 Redis | 只有 memory 模式 | 计数器按实例计数，管理页提示。见 [events.md](events.md) |
| 管理前端与 `/admin` 同源 | 前后端跨域 | v1 不支持。token 与 `frame-ancestors` 要改成配置项 |

## 5. 运行时

### Plugin

`internal/ent/schema/plugin.go`。使用 SoftDeleteMixin，与渠道、项目一致：删除后行保留，能追溯装过哪些插件。唯一索引建在 `(plugin_id, deleted_at)` 上，删除后同一 `plugin_id` 可以重装。重装是新的一行，不继承旧行的配置和授权。

```text
plugin
- id
- plugin_id: 未删除的行里唯一，不可变。^[a-z0-9]+(\.[a-z0-9-]+)+$，≤ 64
- name, version, description：内置插件为空
- manifest: JSON objects.PluginManifest，内置插件为空
- package: bytes，内置插件为空。MySQL 用 longblob
- package_sha256：内置插件为空
- enabled: 默认 false
- priority: 默认 100，越小越先执行
- granted_permissions: []string，默认 []。Owner 实际授予的权限，必须是 manifest.permissions 的子集
- config: JSON，默认 {}
- created_at, updated_at
- deleted_at：SoftDeleteMixin，未删除为 0
```

- 内置插件只存 `plugin_id` 和 `enabled`，其余列为空。manifest、名称、版本来自代码注册表，随 AxonHub 升级自动更新，不在启动时同步。没有行时按注册表的默认值（`axonhub.webhook` 默认启用）；Owner 第一次启停时按 `plugin_id` upsert 这一行。内置插件的配置沿用原有系统设置（`axonhub.webhook` 读 `webhook_notifier_config`），不用 `config` 列；它是代码里的实现，不走权限授予。管理页可以启停，不能删除。
- 同 `plugin_id` 再上传就是升级。新版本多要了权限、事件、钩子或 UI 入口时，先让 Owner 确认，确认前旧版本照常运行，见 [安装与同步](#安装与同步)。
- `config` 按 manifest 的 [`configSchema`](manifest.md#configschema) 校验，secret 字段的掩码规则也在那里。
- Policy：Query 为 `OwnerRule` + `UserReadScopeRule(read_plugins)`；Mutation 为 `OwnerRule` + `UserWriteScopeRule(write_plugins)`，安装、升级、启停、删除、授权在 resolver 里另外要求 Owner。管理器用 `authz.WithSystemBypass(ctx, "plugin-manager")` 读。
- 新增两个系统级用户 scope（`internal/scopes/scopes.go`）。`read_plugins`：查看插件列表、配置（secret 掩码）和运行状态。`write_plugins`：修改插件配置，以及在插件页面写入本插件的 KV。安装、升级、启停、删除、授权仍只允许 Owner：它们决定插件能拿到哪些权限，非 Owner 做这件事可能授出自己没有的权限。这两个 scope 授予用户，不进入插件的权限词表。
- 只有 `(plugin_id, deleted_at)` 唯一索引，StorageKey `plugins_by_plugin_id`。未删除的行是个位数到几十。
- 删除在一个事务里完成：先清空 `package`（`package_sha256` 保留，用于追溯），再按该行的 `id` 删除对应安装的 KV，最后软删除这一行。不清空的话，每次删除重装都会在库里多留一份最大 5 MiB 的包。旧调用的迟到写入与清理见 [PluginStorage](#pluginstorage)。

`granted_permissions` 与 manifest 的 `permissions` 的关系见 [权限词表](manifest.md#5-权限词表)。

### Manifest

字段、安装校验、权限词表和升级比较见 [manifest.md](manifest.md)。

### 包

`.axplugin` 是 ZIP。

```text
manifest.json
plugin.js          只有 UI 的插件可省略
ui/**              html/js/css/svg/png/woff2
```

`plugin.js` 是 ESM。`import` 只允许包内相对路径，解析后仍在包内。裸模块名和 `node:` 拒绝，npm 依赖由作者打进 `plugin.js`。拒绝符号链接、绝对路径、逃出包外的 `..`，以及既不是 `manifest.json`、也不在 import 闭包里、也不在 `ui/` 下的文件。包字节存 `plugins.package`；加载时后端脚本进入内存，UI 文件解压到本实例的临时目录。

`plugins.package` 保存上传或下载的原始 ZIP 字节，不存解压后的文件，也不再套一层压缩；`package_sha256` 对这些原始字节计算。实际压缩率由打包时的 ZIP 压缩方式决定，ZIP 也可以包含未压缩的条目。包大小限制针对原始 ZIP，解压总大小限制另行校验。manifest、config 的 JSON 列和 KV value 不做应用层压缩。

### 资源加载与释放

- **后端脚本**：启用插件时加载 `plugin.js` 及其包内 import 闭包，完成编译和链接。当前代保留编译结果与创建 VM 必需的模块数据；每个 VM 首次创建时初始化模块并执行顶层，随后复用。钩子调用只借 VM，不重新读库、解压或编译。事件、请求钩子，以及未来的响应钩子共用本插件的 VM 池。只包含 UI 的插件不创建 VM。
- **UI 文件**：`ui/` 下的 HTML、JS、CSS、图片、字体按文件流式解压到临时目录，Go 堆不保留整份资源内容。浏览器请求资源时，宿主先按 [页面资源契约](pages.md#6-资源与桥接) 鉴权，再从文件流式返回。前端 JS 在浏览器运行，不进入后端 VM；若某个 `ui/` 文件同时属于后端 import 闭包，仅该模块的运行所需数据也保留在内存。
- **临时目录**：进程启动时用 `os.MkdirTemp` 创建独占根目录，内部按安装 ID 和包 sha256 分版本。目录仅当前进程用户可读写，解压使用路径校验，不接受符号链接；先在暂存目录完成校验、写入，再发布给新 Generation，失败删除暂存目录。数据库是持久来源，不需要共享磁盘；每个实例自行重建缓存。加载完成后释放 ZIP 字节和解压缓冲的引用，不让 Generation 或 Ent 对象长期持有包字节。
- **生命周期**：停用插件只保留管理元数据，不常驻脚本和 UI 文件；启用时按需读包。升级、停用、删除后，旧代不再接收新调用，已有请求、事件调用和 UI 文件传输持有资源引用，最后一个引用释放后清理旧 VM、编译数据和目录。相同安装、相同包可跨配置代共享不可变资源，清理按引用计数执行；进行中的流式请求持有引用直到结束或取消。
- **恢复与退出**：正常退出等待在途调用释放后删除本进程临时目录。异常退出可能留下目录，第一版交给系统临时目录清理，不扫描删除其他进程的目录。临时目录被外部清理时，资源请求返回可重试错误并通知管理器重建该版本缓存；重建完成前不提供不完整文件。解压失败或磁盘空间不足时该实例的插件状态为 error，记录原因，不回退为整包常驻内存。

包和解压大小上限继续保留，用于限制数据库单行大小及解压消耗；解压总大小不是常驻内存配额，也不限制 JS 运行后创建的对象大小。

### 运行限制

本表是全文唯一的数值定义。各篇引用这里，不另写一份。

| 项 | 限制 |
|----|------|
| 包大小 / 解压总大小 / 文件数 / 单文件 / `plugin.js` | 5 MiB / 20 MiB / 200 / 5 MiB / 1 MiB。包大小不能再调高：包和 manifest 等列存在同一行，TiDB 单行默认上限 6 MiB（`txn-entry-size-limit`） |
| 已安装插件数 | 不限制 |
| `config` JSON | 64 KiB |
| 脚本顶层执行 | 1 s |
| `onEvent` 单次 | 10 s |
| 写类宿主 API | 每次调用最多 10 次；每插件每实例每分钟 60 次 |
| 事件队列 | 每插件每实例 1024；满则丢弃并计数 |
| 事件链深度 | 插件动作产生的事件 `Depth+1`，`Depth > 2` 丢弃；不投递给 `Origin` 对应的插件 |
| `onRequest` / `onAttempt` | 200 ms |
| 钩子输入 | 序列化后超过 2 MiB 则跳过并计数 |
| VM 池 | 每插件每实例最多 16 个 VM，按次借用、调用返回即归还。事件 worker 每插件一个，最多占 1 个 |
| 取 VM | 请求钩子最多等 100 ms，不计入调用超时；等不到返回 429，不计入熔断。事件 worker 一直等到有空闲 VM，等待不计入 `onEvent` 的 10 s |
| `ctx.state` | 每请求每插件 64 KiB，只存 JSON 值 |
| 熔断 | 同一插件连续 20 次失败，本实例暂停 5 分钟。过载不算失败 |
| 计数器 | 窗口 1 s–24 h；key ≤ 128 字符 |
| 页面 GraphQL | 复杂度 ≤ 1000；响应 ≤ 2 MiB；超时 10 s；每用户每插件 5 req/s，突发 10 |
| UI 资源 token | 1 h |
| 页面握手 | 10 s 内需发 `hello` |
| 页面 Bridge 调用 | 握手等待与调用分开计时；握手完成后每次调用最多 15 s，覆盖 GraphQL 的 10 s 服务端预算及消息传输 |
| 宿主 channels.list | limit 默认 100，最大 100；按 ID 游标分页 |
| KV key / namespace | `^[A-Za-z0-9_.:-]{1,128}$`；namespace 省略时为 `default` |
| KV value | UTF-8，最大 256 KiB |
| KV TTL | 省略或 0 表示不过期；最长 90 天 |
| KV list | 按 key 升序游标分页；first 默认 100，范围 1–100；nextCursor 为空表示末页 |
| 市场索引 | 只接受 `https`；不跟随重定向；超时 15 s；响应最大 1 MiB；内存缓存 5 分钟 |
| 市场包下载 | 同样只接受 `https`、不跟随重定向、超时 15 s；大小用包大小上限；sha256 必须与索引一致 |

VM 复用：事件和请求钩子的每次 JS 调用都从该插件的池子借一个 VM。调用结束后，宿主清除中断标记、释放本次调用的参数引用，再把 VM 放回池子。被 `Runtime.Interrupt` 中断的 VM 直接丢弃，下次按需新建。模块级变量会跨调用、跨请求保留，只能放常量和缓存。需要跨钩子保留的请求数据放 `ctx.state`，由宿主保存，下次调用时再交给 JS。

### PluginStorage

`internal/ent/schema/plugin_storage.go`。Ent 类型单数 `PluginStorage`，表名 `plugin_storages`，与 `DataStorage` → `data_storages` 相同。主 GraphQL 不暴露；本插件的页面经插件 GraphQL 端点读写，见 [KV 字段](pages.md#kv-字段)。

```text
plugin_storage
- id
- installation_id: plugins.id，安装行的数字主键，由宿主绑定，不接受插件传入
- namespace: 默认 default
- key
- value: MySQL 用 mediumtext
- expires_at: 可空，空表示不过期
- created_at, updated_at
```

唯一约束 `(installation_id, namespace, key)`。不建外键，KV 不用软删除。升级、启停、进程重启保留安装 ID 和 KV；删除后同名重装产生新安装 ID，不继承旧 KV。删除插件时在同一事务里按安装 ID 删光当前 KV。不缓存。读写走 `authz.WithSystemBypass`，审计原因包含 manifest ID 和安装 ID。

宿主创建 JS 调用上下文时，从该调用固定的 Generation 绑定安装 ID；页面 GraphQL 在入口解析当前插件行时绑定安装 ID，resolver 沿用这一值。后续 KV 操作不得再按 manifest ID 查找当前安装，JS 和页面也不能指定安装 ID。因此旧调用即使在删除后继续执行，写入仍属于旧安装，不能覆盖重装插件的数据。

过期行在 get/list 时删除并视为不存在，不设置后台定时清理任务。删除插件时显式清理该安装的 KV。第一版接受旧调用在删除后继续写入旧安装的 KV，不增加逐次写入的事务校验；这些迟到写入可能残留在数据库中，但不会影响重装后的新安装。未再次访问的过期行也可能保留，直到对应安装被删除。

`ctx.kv.get/set/delete/list` 由事件和请求插件共用；页面的 KV 字段调用同一个 `kv.go`，读写同一份数据。计数器 `ctx.counter` 不是这张表：它是滑动窗口，memory 模式下按实例计数。

### 市场

只有一个官方索引，地址是代码常量 `plugin.MarketIndexURL`：`https://raw.githubusercontent.com/looplj/axonhub-plugins/main/index.json`。Owner 不能改地址，也不能加源。

索引仓库 `looplj/axonhub-plugins` 的做法与 `looplj/reveric-skills` 的 `catalog.json` 相同：

- 仓库根目录一个 `index.json`，`plugins` 数组的顺序就是展示顺序。
- 收录插件、发布新版本都走 PR。仓库里的生成工具校验字段、更新 README 表格，`--check` 保证两者一致。
- 包发布在插件作者自己的仓库（例如 GitHub Release），索引只记下载地址和 sha256。

索引：

```json
{
  "schemaVersion": 1,
  "plugins": [
    {
      "id": "acme.failover-guard",
      "name": "Failover Guard",
      "description": "Notify and disable channels on failover storms",
      "versions": [
        {
          "version": "1.2.0",
          "url": "https://github.com/acme/failover-guard/releases/download/v1.2.0/failover-guard-1.2.0.axplugin",
          "sha256": "64位小写hex",
          "axonhubVersion": "^1.0.0"
        }
      ]
    }
  ]
}
```

`axonhubVersion` 按 [版本兼容](manifest.md#版本兼容) 比较。每个插件只展示最新的一个兼容版本；已安装版本更低时条目带 `updateFrom`。sha256 只保证包和这份索引一致，不识别发布者。

### ERD

```text
Plugin
  +-- granted_permissions --> scopes 词表
  +-- config              --> systems.webhook_notifier_config 的 target 名

PluginStorage -- installation_id --> Plugin.id（无外键；软删除时显式清理，允许迟到写入残留）

System(webhook_notifier_config) 由内置插件 axonhub.webhook 读取
```

### 安装与同步

```text
上传：
1. Owner 在 侧栏「插件」›「插件管理」（`/plugins`）上传 .axplugin
   → installPlugin(file: Upload!, grantedPermissions: [String!])，复用 backup restore 的 multipart transport
2. ParsePackage 校验 ZIP → 按 [安装校验](manifest.md#4-安装校验) 校验 manifest；有 plugin.js 时编译、链接包内模块并做导出校验
3. 新装：写入 plugins，enabled=false → 通知管理器刷新
   → 授权对话框列出 permissions、events、hooks、ui
   → enablePlugin(id, grantedPermissions)：granted ⊆ manifest.permissions，然后 enabled=true
4. 升级（plugin_id 已存在）：
   - 所有路径先执行 manifest.md「升级比较」中的授权收敛、secret 降级校验和配置迁移；失败不写库
   - 插件已停用，或新版本没有多出权限、事件、钩子、UI 入口：直接替换包和 manifest，
     同一事务写入迁移后的 config 和收敛后的 grantedPermissions，enabled 不变 → 通知管理器刷新
   - 插件已启用且有多出的项：不写库，返回 PluginUpgradePreview
     （当前版本、新版本、多出的 permissions、events、hooks、ui）
     → 前端弹出确认对话框，Owner 勾选要授予的新权限
     → 前端用同一个文件再调 installPlugin(file, grantedPermissions)，提交完整的新授权集合
     → 后端基于事务内的当前行重新校验，一个事务里替换包、manifest、迁移后的 config、grantedPermissions，enabled 不变 → 通知管理器刷新
     → Owner 取消就不写库，旧版本继续运行
   - 「多出」、授权收敛和配置迁移的定义见 [升级比较](manifest.md#6-升级比较)

市场：
1. pluginMarket：拉官方索引（5 分钟内存缓存），校验 schemaVersion，过滤兼容版本，对比已安装插件
2. installPluginFromMarket(pluginID, version, grantedPermissions)：下载，比对 sha256，交给 ParsePackage
3. 之后与上传的第 3、4 步相同。升级确认时再调一次，重新下载并比对 sha256

失败：
- 校验失败、sha256 不一致、非 https、重定向、超时、超限：不写库
- 索引拉取失败：市场页显示错误，本地上传不受影响
- plugin_id 与内置插件相同：拒绝
- 某实例编译或 UI 解压失败：该实例状态 error，其他实例不受影响。状态只反映当前实例，见妥协 3
```

插件变更（安装、升级、启停、删除、改配置）写库成功后，当前实例立即 ForceRefresh，并通过 notifier 广播：`two-level` 时走 Redis `axonhub:cache:plugins`，其他实例收到后立即刷新；memory 模式下通知只到本进程。管理器用 `live.NewCache`，另外每 30 秒比较一次未删除行的 `count(*)` 和 `max(updated_at)` 兜底，覆盖 memory 模式下的其他实例和 Redis 漏掉的消息，做法与渠道缓存相同。被软删除的行不再计入 `max(updated_at)`，只比它发现不了别的实例删除插件，所以要带上行数。

刷新是增量的，只为需要加载或重建的插件读包：

1. 查全部行，但不查 `package` 列。剩下的列（manifest、config 等）每行最多几十 KiB。
2. 已停用的行只保留元数据。已启用的行按 `(id, package_sha256)` 查找本实例资源；相同且资源完整就复用编译结果与 UI 目录，不读包。同名同包重装的安装 ID 不同，不能复用旧安装的 KV 绑定或 VM。
3. 已启用且资源不存在、sha256 变化或缓存损坏时，按 `id` 单独读出 `package`，核对 sha256，按上文加载脚本和 UI，完成后释放包字节。刷新周期也重试加载失败的已启用插件，不只依赖元数据变化。
4. 当前代有、查询结果里没有的插件视为已删除，按下文的禁用流程卸下。

这样停用、改配置、改优先级都不读包；启用时需要重新加载，升级一个已启用插件只读它的包。

构建不可变 Generation（按 priority 排序的插件、事件订阅表、请求钩子列表、UI 入口）并原子替换。一次请求从 `Process` 开始用同一代，含重试。被禁用的插件：旧代里正在执行的调用跑完，事件 worker 排空当前事件后退出。

内置插件在包 `init` 里 `plugin/builtin.Register`。管理器对已启用的内置插件调 `Apply`，禁用或停止时倒序 `OnDispose`。`Ctx` 提供 `Subscribe`、`OnDispose`。v1 只有 `axonhub.webhook`，行为见 [events.md](events.md)。

### 改动清单（运行时）

#### `go.mod`
- 新增 `github.com/MoeForge/moejs v0.1.0-alpha.11`。

#### `internal/server/server.go`
- 注册 `events.Module` 和 `plugin.Module`。

#### `internal/ent/schema/plugin.go`、`plugin_storage.go`
- 字段见上文。两者都 `entgql.Skip(entgql.SkipAll)`。`package` 用 MySQL `longblob`，`value` 用 `mediumtext`。

#### `internal/scopes/scopes.go`
- 新增 `ScopeReadPlugins`、`ScopeWritePlugins`，只有系统级。`scopes_test.go` 的全量列表同步。

#### `internal/objects/plugin.go`
- manifest 类型与校验，见 [manifest.md](manifest.md#7-改动清单)。

#### `internal/server/plugin/`
- `fx_module.go`：`*Manager`、`orchestrator.PluginIntegration`。`OnStart` 加载插件，对已启用的内置插件调 `Apply`；`OnStop` 停止。
- `package.go`、`manifest.go`、`manager.go`：解析、校验、代际；后端模块与 UI 资源分离加载，独占临时目录、资源引用计数、缓存重建与退出清理。
- `jsruntime.go`：编译缓存、`DisableDynamicCode`、`Runtime.Interrupt`、`*moejs.InterruptedError`、返回 Promise 视为非法、VM 池与取 VM 等待（等不到返回 `ErrVMUnavailable`）、熔断、`ctx.state` 存储。
- `kv.go`、`market.go`。
- `builtin/registry.go`。
- 事件、请求、UI 的文件见各篇。

#### `internal/server/gql/plugin.graphql`
- 运行时用到的类型：`Plugin`、`PluginRuntimeStatus`、`PluginMarket*`、`installPlugin`、`installPluginFromMarket`（都返回 `PluginInstallResult { plugin, upgradePreview }`）、`PluginUpgradePreview`、`enablePlugin`、`disablePlugin`、`updatePluginSettings`、`deletePlugin`。`deletePlugin` 按上文的删除事务清空包、删除 KV，再软删除。
- UI 类型 `PluginUIEntries`、`PluginUIPage`、`PluginUISession`、`createPluginUISession` 与 KV 字段的行为见 [pages.md](pages.md)。schema 仍是这一个文件。
- `plugins`、`pluginRuntimeStatuses`：Owner 或 `read_plugins`。JS 插件和注册表里的内置插件一起返回，内置插件带 `builtin: true`，没有 config 和权限。`enablePlugin`、`disablePlugin` 对内置插件只改 `enabled`；`deletePlugin` 拒绝内置插件。`updatePluginSettings`：Owner 或 `write_plugins`；其余 mutation：Owner。`Plugin.config` 掩码 secret。

#### 前端
- `features/plugins/data/plugins.ts`
- `features/plugins/index.tsx`：插件管理页，`plugins-table.tsx` 列表（启停、删除、运行状态），`plugin-install-dialog.tsx` 上传，`plugin-detail-sheet.tsx` 详情与配置抽屉。memory 模式显示「计数为单实例」。
- `features/plugins/components/plugin-market.tsx`：条目、安装和升级。
- `features/plugins/components/plugin-grant-dialog.tsx`（新装授权和升级确认共用）：`write_*` 权限高亮，并写明授予后插件页面能执行该类全部 mutation，`write_channels` 还能读取凭证明文、修改渠道地址。`plugin-config-form.tsx`
- `routes/_authenticated/plugins/index.tsx`：独立路由 `/plugins`，需要系统级 `read_plugins`（不依赖 `read_settings`），在 `config/route-permission.ts` 登记。侧栏入口「插件管理」在 `nav-items.ts` 的 `plugins` 分组，位于「项目」和「设置」之间。安装、启停、删除、授权按钮只对 Owner 显示，配置表单需要 `write_plugins`。
- `locales/en/plugins.json`、`locales/zh-CN/plugins.json`
- `locales/en/base.json`、`locales/zh-CN/base.json`：`scopes.read_plugins`、`scopes.write_plugins`。

### Migration

```text
from 最新发布版本:
1. 新增 plugin.go 与 plugin_storage.go，执行 make generate。创建 plugins、plugin_storages 及唯一索引
2. 无历史数据回填。内置插件在 Owner 第一次启停时才写行
3. webhook_notifier_config 不变
4. make migration-test-all-dbs TAG=<最新发布版本>
```

回滚：旧二进制忽略这两张表。WebHook 配置未改，回滚后旧事件仍按旧逻辑发送。

`package` 在 PostgreSQL 是 `bytea`，SQLite 是 `blob`，由 Ent 生成。MySQL/TiDB 要在 schema 里用 `SchemaType` 指定 `longblob`，Ent 默认生成的 `blob` 只有 64 KiB。

MySQL 的 `max_allowed_packet` 至少要 8 MiB，才能写入 5 MiB 的包。8.0 默认 64 MiB；5.7 默认 4 MiB，部分云数据库也会调低。写库返回 packet too large 时，`installPlugin` 返回可读错误「插件包超过数据库允许的单次写入大小（max_allowed_packet）」，不写库。

## 6. 验收

| 类别 | 标准 | 验证方式 |
|------|------|----------|
| 功能 | 上传后出现 disabled 行；授权后 30 秒内各实例 active | manager 测试，两个 Manager 共享同一 DB |
| 功能 | 停用、改配置后刷新不读 `package` 列；启用时按需读包；升级一个已启用插件只读它的包；另一个实例删除插件后 30 秒内本实例卸下 | manager 测试，两个 Manager 共享同一 DB，统计读包次数 |
| 资源 | UI 文件落在独占临时目录，加载后资源对象不持有 ZIP 或 UI 完整内容；只有 UI 的插件不创建 VM；连续钩子调用不重复读包、解压和编译 | plugin 单测，检查资源对象并统计加载、编译及 VM 创建次数 |
| 资源 | 升级或删除时旧调用仍能完成；引用释放后旧目录清除，新目录不受影响；停用后释放资源，重新启用可恢复；目录丢失后可重建，磁盘写入失败显示 error | manager/UI 测试，临时目录与受控在途调用，注入文件写入失败 |
| 功能 | 已启用插件的新版本多要了权限：第一次调用返回预览且不写库，旧版本继续处理事件；带授权再调用后替换，`enabled` 不变 | `go test ./internal/server/plugin/ -run TestUpgrade` |
| 功能 | 市场 sha256 不符不写库；相符与上传同一包相同 | `go test ./internal/server/plugin/ -run TestMarket` |
| 功能 | 删除后插件行的 `deleted_at` 非 0、`package` 为空；同一 `plugin_id` 可以重装，新行不带旧配置和授权 | `go test ./internal/server/plugin/ -run TestDelete` |
| 功能 | KV 升级、启停和重启后可读；TTL 到期为空；插件间隔离；无在途写入时删除插件后该安装 KV 行数为 0 | `go test ./internal/server/plugin/ -run TestKV` |
| 功能 | 暂停旧安装的 JS 或页面 KV 写入，删除并同名同包重装后恢复旧写入：新安装读不到旧值，同名键不被覆盖；允许旧安装迟到写入残留，新安装与停用安装的数据保持不变 | 同上，两个 Manager 共享 DB，控制调用停点 |
| 权限 | `read_plugins` 能查插件列表、不能改；`write_plugins` 能改配置、不能安装启停授权；都没有时查询被拒 | resolver 测试 |
| 错误处理 | `..`、超限、manifest 不合法（细项见 [manifest.md](manifest.md#8-验收)）：可读错误且不写库 | `go test ./internal/server/plugin/ -run TestParsePackage` |
| 错误处理 | 连续 20 次失败后熔断 5 分钟 | 插件单测，注入时钟 |
| 迁移 | 三种数据库从最新发布版本升级成功 | `make migration-test-all-dbs TAG=<最新发布版本>` |
| 可维护性 | `events` 不 import `biz` 与 `plugin`；biz 不 import plugin | `go list -deps` 断言 |
| i18n | `en/plugins.json` 与 `zh-CN/plugins.json` key 一致 | 前端既有 i18n key 校验 |

扩展点的验收在各篇。

## 7. 风险

| 风险 | 触发条件 | 影响 | 对策 |
|------|----------|------|------|
| 插件死循环占满 CPU | `while(true)` | 单核被占到超时 | `Runtime.Interrupt`；熔断 |
| 插件内存耗尽 | 构造超大数组 | 进程 OOM | 输入体积上限；妥协 2 |
| 请求数据跨请求残留 | 插件把请求数据存进模块级变量 | 后续请求读到别的请求的内容 | VM 按次复用，模块级变量会保留。开发者文档明确禁止；每次调用后宿主释放本次调用的参数引用 |
| 多实例代际不一致 | memory 缓存 | 最长 30 秒内各实例跑不同版本；实例间时钟偏差时可能更久 | 文档说明；多实例建议 two-level；妥协 5 |
| 官方索引被篡改 | 索引仓库或插件作者的发布账号被盗 | 装上与索引一致的恶意包 | 索引改动走 PR 审核；sha256 只绑定索引；妥协 1 |
| 插件装得很多 | 不限制已安装插件数 | 已启用插件的脚本和 VM 占用内存，UI 文件占用临时磁盘；启动时需加载已启用的包 | UI 按需从文件读取；停用后释放；增量刷新；需要时再加数量上限 |
| 临时磁盘不可用 | 磁盘写满、目录无权限或被系统清理 | UI 资源不可读或插件加载失败 | 状态 error 与可读日志；重建损坏缓存；正常退出清理本进程目录；异常退出残留由系统清理 |
| KV 被写满 | 循环 set 不同的 key | 数据库膨胀 | 单值 256 KiB；删除插件时按安装 ID 删除，允许旧调用的迟到写入残留 |
| secret 明文 | 数据库泄露 | 配置密钥和 KV value 一并泄露 | 妥协 4 |

## 8. 已知妥协

```text
1. 妥协：无包签名与发布者校验
   - 原因：还没有发布者密钥体系
   - 影响：市场只能保证包和索引里的 sha256 一致
   - 升级触发：需要区分多个发布者，或允许第三方索引
   - 后续方向：Ed25519 签名加发布者白名单

2. 妥协：moejs 无法限制单个 VM 的内存
   - 原因：引擎不提供内存配额
   - 影响：有缺陷的插件可能导致 OOM
   - 升级触发：出现由插件导致的 OOM，或要开放给不可信第三方
   - 后续方向：评估 wazero

3. 妥协：运行状态与统计只反映当前实例
   - 原因：运行状态不写库
   - 影响：管理页看到的是处理该请求的实例
   - 升级触发：多实例需要汇总视图
   - 后续方向：各实例周期上报到 systems 或 Redis

4. 妥协：secret 配置和 KV value 明文存库
   - 原因：与渠道凭证一样，仓库没有通用字段加密
   - 影响：数据库泄露时一起泄露
   - 升级触发：引入字段级加密
   - 后续方向：与渠道凭证一起接入

5. 妥协：兜底刷新依赖各实例的本机时钟
   - 原因：updated_at 由写库实例的本机时间生成，系统没有中央时钟；做法与渠道缓存相同
   - 影响：写库实例的时钟偏慢时，max(updated_at) 可能不变。若行数也没变（启停、改配置、升级），memory 模式下其他实例的兜底刷新会漏掉这次变更，直到下一次改动或重启。two-level 模式有 Redis 通知，只在通知丢失时受影响
   - 升级触发：多实例运行在 memory 模式，或出现插件状态长时间不一致
   - 后续方向：改用数据库时间（如 CURRENT_TIMESTAMP）写 updated_at，或比较不含 package 的元数据摘要
```

事件、请求、页面各自的妥协见各篇，不在这里重复。

## 9. 任务管理

核对日期：2026-11-05。分支 `unstable`，提交 `29267ce5f`。仓库里没有插件实现，下列任务都是已确认缺口。响应钩子见 [response.md](response.md)，自定义 channel provider 见 [provider.md](provider.md)，按 project / API Key / 模型收窄见 [scope.md](scope.md)，都不在本表。

实施分五段，按段推进；段内在依赖允许时可以并行。分段只表示顺序，不对应发布版本。

1. 插件基础设施：T-01 至 T-03。本地上传、授权、升级确认、启停、删除、配置。这一段只接受不带 `plugin.js` 的包。
2. UI 插件：T-04 至 T-07。独立页面、插件 GraphQL（含 mutation）、页面读写本插件 KV。不需要 JS。
3. WebHook 与事件插件：T-08 至 T-12。内置 WebHook 改为事件订阅者；引入 moejs，开始接受带 `plugin.js` 的包，开放 JS `onEvent`。
4. 请求插件：T-13。放在最后：按 [关键假设](#4-决策)，moejs 性能不达标时只推迟这一段，前三段不受影响。
5. 市场与收尾：T-14、T-15。T-14 只依赖 T-01、T-03，人手富余时可在第 1 段之后并行。

| ID | 任务 | 状态 | 依赖 | 当前记录 |
|----|------|------|------|----------|
| T-01 | [Plugin 实体、包解析与 manifest 校验](#plugin) | 待办 | 无 | schema（SoftDeleteMixin，唯一索引 `(plugin_id, deleted_at)`）、`package.go`；[manifest](manifest.md) 的类型、结构校验、升级比较与未开放字段拒绝；在 T-10 完成前，含 `plugin.js` 的包拒绝安装，返回可读错误；`events`、`hooks` 按 [字段表](manifest.md#3-字段) 的「开放」列拒绝；`make generate` |
| T-02 | [插件管理器](#安装与同步) | 待办 | T-01 | `manager.go`、FX 接线；按安装 ID 与 `package_sha256` 增量加载，停用释放资源；临时目录、代际引用计数、缓存重建与退出清理；多实例同步及资源生命周期验收 |
| T-03 | [插件管理 GraphQL 与管理页](#改动清单运行时) | 待办 | T-02 | `read_plugins` / `write_plugins` scope 与 i18n；`plugin.graphql` 与 resolver；plugins tab、本地上传、授权对话框（`write_*` 高亮）与升级确认（提交完整授权集合）、配置表单、i18n；本地和市场共用升级事务，落实 [授权收敛与配置迁移验收](manifest.md#8-验收)，包含 secret 降级拒绝和删除字段清理 |
| T-04 | [插件 KV 存储](#pluginstorage) | 待办 | T-01 | `plugin_storage` schema、`kv.go`；按 `plugins.id` 绑定安装身份、删除时显式清理、get/list 时清理过期数据；`make generate`；重启可读、同名重装隔离、旧调用迟到写入隔离验收 |
| T-05 | [插件 GraphQL 端点](pages.md#7-插件-graphql) | 待办 | T-03, T-04 | `scoped_user.go`、`graphql.go` 抽取、路由；开放 mutation、自助 mutation 名单与根字段快照；KV 字段与 `write_plugins` 校验；插件权限矩阵测试 |
| T-06 | [UI 资源与桥接](pages.md#6-资源与桥接) | 待办 | T-02 | `ui.go`、`uisdk/bridge.js`、CSP；token 绑定入口，允许同包 UI 白名单资源与宿主 bridge；从版本临时目录流式返回并持有资源引用；路径和包版本校验、资源路由匿名 CORS 与 MIME、缓存丢失重建、后端文件访问拒绝测试、ES module/Vite 页面加载验收 |
| T-07 | [页面挂载](pages.md#5-路由与侧栏) | 待办 | T-03, T-05, T-06 | `plugin-frame.tsx`、页面路由、侧栏插件分组；单测与真实 sandbox 的模块加载及隔离 E2E |
| T-08 | [事件总线与事件类型](events.md#5-事件) | 待办 | 无 | 新建 `internal/server/events`，含 Bus、指纹、FX 模块与单测 |
| T-09 | [内置插件注册表与 WebHook 迁移](events.md#6-发布与-webhook) | 待办 | T-02, T-03, T-08 | `builtin/registry.go`（`Apply`、`Subscribe`、`OnDispose`），管理器启停内置插件；`plugins` 查询与管理页加入内置插件；12 种事件的发布点、`NotifyEvent`、快照测试、前端事件选项与 i18n |
| T-10 | [JS 编译与安装校验](manifest.md#4-安装校验) | 待办 | T-01, T-02 | 引入 moejs v0.1.0-alpha.11；ESM 编译与包内链接、`DisableDynamicCode`、顶层执行 1 s 超时、检查命名导出存在且不是 async；管理器刷新时编译并按 `package_sha256` 复用编译结果；去掉 T-01 对 `plugin.js` 的临时拒绝；T-10 至 T-13 共用 [脚本入口契约与验收](plugin.js.md#7-实现与验收对应) |
| T-11 | [JS 执行运行时](#运行限制) | 待办 | T-04, T-10 | 按次借还的 VM 池、`Runtime.Interrupt` 超时中断并丢弃被中断的 VM、返回 Promise 视为非法、熔断；按 [宿主 API 契约](plugin.js.md#公共类型与调用规则) 实现签名、错误码、KV 与上下文状态语义 |
| T-12 | [事件插件与宿主 API](events.md#7-js-处置) | 待办 | T-09, T-11 | `dispatcher.go`、`hostapi.go`、`counter.go`；开放 manifest 的 `events`；故障转移阈值集成测试 |
| T-13 | [请求钩子](request.md#4-钩子) | 待办 | T-11 | 按用户决定，请求对象的字段命名、可编辑范围及内部字段保留/合并在本任务实施时明确；先跑 `go test -bench Hook ./internal/server/plugin/`，不达标按 [关键假设](#4-决策) 处理；`onRequest`、`onAttempt`、`ctx.state`；`hooks.go`、orchestrator 插入点与构造参数；[VM 不足](request.md#vm-不足) 等待 100 ms 后返回 429，`onAttempt` 过载时不切渠道；[pass-through 渠道](request.md#5-pass-through-渠道) 的计数和日志；开放 manifest 的 `hooks.request`、`hooks.attempt`；兼容与性能测试 |
| T-14 | [插件市场](#市场) | 待办 | T-01, T-03 | `market.go`、`MarketIndexURL`、市场页；sha256 不一致不写库；`looplj/axonhub-plugins` 仓库的 `index.json` 与校验工具 |
| T-15 | [迁移、E2E 与开发者文档](#6-验收) | 待办 | T-07, T-09, T-12, T-13, T-14 | `make migration-test-all-dbs`、`make e2e-test`；`docs/` 中英文开发者文档与示例插件 |

本方案参考了 new-api、CLIProxyAPI、sub2api 和 magpie。
