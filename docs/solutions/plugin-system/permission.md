# 插件权限

> 状态：页面 Plugin Principal 已接入。共享身份基础设施位于 `internal/authz/identity`，页面由 `authz.DelegatePlugin` 受控派生：主体为 Plugin，context 中的用户为 `scopes.DelegatedUserView` 生成的收窄视图（见第 5 节“主体与用户视图”）。本文同时保留迁移前问题和验收约束。
>
> 主体构造入口保留原函数签名，在 Plugin context 上替换主体是宿主编程错误：先记录拒绝审计，再 panic。通用系统 bypass 对 Plugin 默认拒绝：审计后 `RunWithSystemBypass*` 返回错误且不执行闭包，`WithSystemBypass` 返回保留 Plugin 主体、拒绝所有 Ent 操作的 context。普通业务权限不足仍返回拒绝/error。需要以宿主身份执行的固定操作统一使用 `authz.RunAsHost`。插件页面 KV 使用安装绑定的固定宿主存储操作。`me` 仅返回 ID、姓名、邮箱、语言及头像，Owner、scope、角色、项目和 OIDC 关联不透传。Plugin dashboard 请求在检查 `read_dashboard` 后，与直接用户共用统计缓存和 singleflight；统计结果不随调用方变化。

本篇定义插件授权的目标模型、迁移步骤和新增功能时的要求。[manifest.md](manifest.md) 继续作为权限词表和授权升级规则的唯一来源；[pages.md](pages.md) 定义页面入口和桥接；[scope.md](scope.md) 定义插件执行范围，执行范围不等于数据访问权限。

## 1. 迁移前实现与问题

插件页面通过 `POST /admin/plugins/:pluginId/graphql` 调用宿主。该入口与 `/admin/graphql` 复用 schema，但增加插件身份绑定、授权上限、复杂度、速率和响应大小限制。

迁移前授权步骤（已由独立主体替代）：

1. 认证用户 JWT，解析已启用的插件安装和其 `granted_permissions`。
2. `BuildPluginScopedUser` 创建用户副本，将系统和项目权限分别与插件授权取交集，清除副本的 Owner 标记及可重新引入权限的角色边。
3. 把副本放入本次请求 context，供现有 Ent privacy 和业务检查使用。原用户不修改。
4. 按安装 ID 绑定插件 KV。写 KV 的资格由入口根据原用户判断：系统 Owner，或拥有系统级 `write_plugins`；Owner 视为拥有该权限。
5. 对未经过 scope 检查的自助操作保留入口限制。
6. 插件 GraphQL handler 通过 `AroundFields(rejectPluginPrivateFields)` 加入临时字段拦截（`internal/server/gql/graphql.go`）：字段所属对象为 `ChannelOverrideTemplate`，或返回类型为 `ChannelOverrideTemplate` / `ChannelOverrideTemplateConnection` 时返回 `FORBIDDEN`。它按类型限制私有模板访问，不只检查根查询字段名；普通 `/admin/graphql` 不注册该拦截器。

这能约束基于 scope 的判断，但不能完整表达“插件代表用户调用”：

- `UserOwnedQueryRule`、`UserOwnedMutationRule` 可以根据用户 ID 放行，私有资源不一定需要 scope。
- resolver/service 可能重新读取真实用户，恢复其 Owner 或角色权限。
- 只接收 `*ent.User` 的检查函数不知道当前调用来自插件。
- 新增私有字段后，GraphQL 字段限制需要补充，容易遗漏嵌套边、`node/nodes` 等路径。

重命名只是使函数用途更明确，不声称解决上述模型问题。

## 2. 决策与边界

实现已在 `authz.PrincipalType` 中新增 `PrincipalTypePlugin`，用独立主体表达委托调用。原 `BuildPluginScopedUser` 移入 `scopes` 并更名为 `DelegatedUserView`，作为 context 用户的收窄视图保留，和 Plugin 主体共同生效：主体负责显式区分，视图负责让遗漏的直接判断默认安全。

- 继续复用 GraphQL schema，不单独维护一套插件 schema。
- 不新增 `ScopeSource`；主体类型已经表达插件与直接用户调用的区别。
- 不新增数据库字段。沿用安装 ID、manifest 和 `granted_permissions`。
- 普通用户、API Key、系统任务的既有授权语义不变。
- 本篇不调整数据库主从路由、资源生命周期或插件运行范围。
- 本阶段只迁移用户 JWT 驱动的插件页面。后台事件和请求钩子的主体创建规则另见第 9 节，不把它们伪装成用户调用。

## 3. 主体与上下文

目标数据形状如下，字段名可随实现细化：

```go
Principal{
    Type:                 PrincipalTypePlugin,
    UserID:               userID,
    PluginInstallationID: installationID,
}
```

`UserID` 表示代表谁调用，供审计和数据归属使用；`PluginInstallationID` 固定本次安装身份。同名重装不能让旧请求取得新安装的授权或 KV。manifest ID 可作为日志标签，不能替代安装 ID。

### 主体相等与幂等设置

当前 `principalEqual` 只比较 `Type`、`UserID`、`APIKeyID`、`ProjectID`。新增 `PluginInstallationID` 时，必须同步将其纳入相等判断和 `WithPrincipal` 的冲突检查，不能仅新增字段而沿用原比较逻辑。

- 同一用户、不同安装 ID 的两个 Plugin 主体不相等，`WithPrincipal` 必须返回主体冲突，不能进入幂等成功分支。
- 同名插件删除重装后安装 ID 已变化，即使用户及 manifest ID 相同，也必须视为不同主体。
- 只有主体类型、用户、API Key、项目及安装 ID 等身份字段全部一致时，才允许幂等设置；Plugin 主体的安装 ID 必须有效，不能用缺失或零值匹配任意安装。
- 幂等设置只确认已有身份，不重新派生上下文，也不能借此刷新、替换或扩大本次请求的授权快照。

授权上下文另保存由服务端解析的插件授权快照。快照应复制输入集合，不能让调用方修改原切片来扩大权限；不向插件暴露可写的主体或授权集合。快照同时固定收窄后的用户视图；原用户实体不被修改，但 Plugin context 内不可再通过 `contexts.GetUser` 取得。

### 创建与派生

增加受控的用户到插件委托函数，具体名称以实施时为准：

1. 要求父主体是已经认证的 User，且主体 UserID 与 context 用户一致。
2. 入口从服务端解析有效安装和授权，禁止使用客户端提供的 grants、安装 ID 或主体类型。
3. 创建独立的请求上下文，父请求的用户和可变容器不能被改写。
4. 派生后的授权主体是 Plugin；`contexts.GetUser` 返回收窄视图：ID 和资料与真实用户相同，权限与授权快照取交集，`IsOwner` 为 false，无角色边。视图固定在委托快照中，之后即使替换容器中的用户，`GetUser` 仍返回该视图。
5. 拒绝缺失用户、无效安装、API Key/System 到页面插件的转换及插件嵌套委托。
6. 不继承父 context 中的全局 privacy Allow/系统 bypass；遇到这类上下文应拒绝创建，避免绕过后续规则。

### 主体覆盖与系统 bypass 的约束

迁移前没有全局的主体 set-once 保证：只有 `WithPrincipal` 检查已有主体；`NewUserContext`、`NewAPIKeyContext`（`internal/authz/principal.go`）和 `NewSystemContext`（`internal/authz/system.go`）直接通过 `context.WithValue` 覆盖主体。`WithSystemBypass`、`RunWithSystemBypass`、`RunWithSystemBypassVoid` 先创建 System 主体再注入 privacy Allow。因此，即使插件入口使用 `WithPrincipal`，后续调用这些函数仍能静默把 Plugin 换成 System/User/APIKey。现有审计日志也不能替代授权拒绝。

目标实施必须在主体构造和 bypass 入口强制执行以下规则，不能只靠调用方约定：

1. 在 Plugin context 上调用 `NewSystemContext`、`NewUserContext`、`NewAPIKeyContext` 或普通主体替换入口，记录被拒绝的主体转换并 panic；不返回已替换身份的 context。测试主体构造入口也不得成为插件链路的替换通道。
2. `WithSystemBypass` 和所有 `RunWithSystemBypass*` 包装入口在覆盖主体之前检查父主体。父主体为 Plugin 时记录拒绝审计并默认拒绝：`RunWithSystemBypass*` 返回错误且不执行闭包；`WithSystemBypass` 返回保留 Plugin 主体、注入 privacy Deny 的 context，后续 Ent 操作全部拒绝。不能注入 privacy Allow。`WithBypassPrivacy` 继续只接受 System/Test 主体。选择默认拒绝而非 panic：插件可达的 bypass 调用点很多，panic 在 goroutine 中无人恢复会使进程退出，且会把遗漏变成错误日志噪音。
3. 保留 `NewUserContext`、`NewAPIKeyContext`、`NewSystemContext`、`NewTestContext`、`WithSystemBypass`、`WithTestBypass` 的原签名，不为编程不变量引入业务错误返回值。主体构造入口的 Plugin 非法转换必须 panic；不得记录后继续执行、返回原 context 冒充成功或吞掉 panic 后继续特权操作。原本返回 error 的 `WithPrincipal`、`WithBypassPrivacy` 保持原契约。
4. `WithPrincipal` 保留 set-once 和冲突检查，但只作为该入口的约束。受控 User → Plugin 委托是专门允许的转换；其他构造路径都要纳入主体转换检查。普通 User/APIKey/System 的现有合法内部操作单独回归验证，不因插件限制被无差别禁止。
5. 记录原主体类型、目标主体类型、插件安装 ID、委托用户、调用原因和拒绝结果；拒绝必须先于数据库、网络等副作用。日志不记录凭证或业务数据。

JWT 中间件先创建 User，插件入口通过专用委托函数创建 Plugin。这套机制约束宿主公开授权 API，不是 Go 进程内恶意代码的隔离沙箱；禁止业务代码直接写主体 context key 或直接注入 privacy Allow 的仓库检查仍需保留。

## 4. 权限计算

对页面请求所需的 scope `s`：

```text
系统权限允许(s) = 插件被授予 s ∧ 用户实际拥有系统级 s
项目权限允许(p, s) = 插件被授予 s ∧ 用户按既有规则拥有项目 p 的 s
```

用户实际权限继续按现有直接 scope、系统角色、项目成员和项目角色规则计算。系统与项目权限分别判断，不将所有项目的权限合并成一个全局集合。

- 用户是系统 Owner：可以作为“用户拥有某项显式 scope”的依据，但插件仍必须被授予该 scope。
- 用户是项目 Owner：同样只能在该项目及插件授权上限内使用显式权限。
- Owner-only 操作：要求真实当前主体具有 Owner 权限，Plugin 一律不能凭委托用户的 Owner 标记通过。
- 插件授权为空：不得获得业务 scope、Owner-only 权限或“本人数据”的隐式访问。插件自身 KV 和必要的宿主协议按专门规则处理。
- 项目查询仍需注入项目过滤条件；有 scope 不等于可以忽略租户边界。
- 不属于页面业务权限的 manifest 能力不能混入通用业务 scope 判断。权限词表仍以 manifest 契约为准。

不得只在 GraphQL 入口算一次交集，随后继续调用忽略主体类型的 Owner 或本人检查。最终放行必须同时考虑当前主体和对应资源规则。

## 5. 公共授权规则的接入

### 现有两套状态与迁移后的关系

迁移前系统同时保存两套与调用身份相关的状态：

- `authz.Principal`：独立 context key 下的主体类型和身份 ID，由 `authz.HasScope` 等入口读取。
- `contexts` 中的 `User` / `APIKey` 实体：包含用户、角色、成员关系或 API Key 权限。`scopes` 当时的 `getUserFromContext` / `getAPIKeyFromContext` 直接读取这些实体，不读取 Principal，也不验证实体是否匹配当前主体。

因此，新增 `PrincipalTypePlugin` 并不自动约束 Ent privacy。若 context 中保留真实用户，`contexts.GetUser` 和 `getUserFromContext` 都能读到真实的 `IsOwner=true`、完整角色和项目成员信息，旧规则凭这些字段就会放行，出现“authz 认为是受限插件，scopes 却按真实 Owner 授权”的危险状态。

首版实现确实保留了真实用户，并逐处为规则加 Plugin 判断。review 发现仍有遗漏：个人 API Key 可见性、请求审计范围、邀请的项目角色、个人 Key 修改的 Owner 例外等直接读 `IsOwner` 或角色的代码，都会把 Owner 权限交给插件。逐处修补无法防止以后新增代码再次遗漏，因此改为下述“主体与用户视图”结构。

### 主体与用户视图

- **Plugin 主体**（`identity`）：安装 ID、真实用户 ID 和授权快照。用于需要显式区分插件的判断：`RequireDirectUser`、`scopes.IsDirectOwner`、审计。
- **收窄用户视图**（`scopes.DelegatedUserView`）：`contexts.GetUser` 在 Plugin context 中返回的用户实体。系统和项目权限（含角色带来的权限）分别与授权取交集，用户及所有项目成员的 `IsOwner` 为 false，角色边为 nil。直接读 Owner 标记或角色的代码对插件默认拿不到 Owner 权限。
- **视图固定**：视图保存在委托快照中，`contexts.GetUser` 优先返回它。`contexts.WithUser` 原地修改请求容器，若有代码重新加载真实用户并写回（例如 API Key 授权检查），也不会恢复完整权限。
- **按 ID 重新加载**：从数据库重新查询用户得到的是真实实体，但该查询本身受 Plugin privacy 约束；Owner 判断仍须走 `scopes.IsDirectOwner` / `Effective*Scope`。

迁移后明确两者的职责：

1. Principal / 受控委托状态决定当前调用者类别、插件安装身份及授权上限；Plugin context 中的 User 实体是收窄视图，只携带授权上限内的权限。
2. 公共授权入口必须先识别当前主体及委托限制，再读取实体计算权限交集和数据过滤条件。只调用 `getUserFromContext` 取得用户不等于已完成授权。
3. 插件页面只允许从直接 User 主体派生。创建时校验 Principal.UserID 与实体用户 ID 一致；Plugin 路径不得因为 context 中残留 APIKey 实体而转入 API Key 授权分支。
4. 实体替换、重新加载用户、请求上下文复制或 fork，均不能移除插件委托约束。主体、委托记录与实体身份不一致时拒绝，不回退为普通用户权限。
5. 公共规则仍使用 `EffectiveSystemScope` / `EffectiveProjectScope` 校验授权快照，与视图形成双重约束；不能因为视图已收窄就删除规则层的 Plugin 判断。

| 入口/规则 | 目标行为 |
|---|---|
| `authz.HasScope`、`RequireScope`、`WithScopeDecision` | 识别 Plugin，检查插件授权与委托用户权限交集；未接入的主体默认拒绝。对 Plugin 与直接用户语义一致：交集校验通过注入 Allow，失败注入 Deny |
| 系统 scope 的 Ent 规则 | 使用带 context 的有效权限检查，不能只检查 `user.Scopes` 或 `user.IsOwner` |
| 项目 scope 的 Ent 规则 | 检查授权上限并保留项目成员校验、项目过滤和既有资源可见性约束 |
| `OwnerRule` | Plugin 不通过 Owner 身份捷径，继续评估其他显式授权规则 |
| `UserOwnedQueryRule`、`UserOwnedMutationRule` | Plugin 不因用户 ID 相同而放行，返回 Skip，让其他显式规则决定；最终仍未允许则拒绝 |
| 仅凭项目 Owner 放行的规则 | Plugin 不通过身份捷径；需要有明确的 scope 路径 |
| 直接在 resolver/service 中判断 Owner 的代码 | 改为主体感知的检查，不能重新加载用户后按 IsOwner 放行 |
| 自助操作 | 明确要求直接 User 主体；未迁移的操作继续保留入口限制 |

`Skip` 表示该身份规则不适用，不是允许，也不能用无条件 Deny 阻断同一 policy 后续合法的显式 scope 规则。公共 policy 必须保持默认拒绝。

### 依赖方向

当前 `authz` 已依赖 `scopes`，而 `scopes` 已依赖 `contexts`；直接让 `scopes` import `authz` 会造成循环依赖。实现选择第一种方案，以下保留选型依据：

| 候选方案 | 做法 | 需要验证的约束 |
|---|---|---|
| 下沉共享身份基础设施 | 将 Principal 类型和 context 存取放入不依赖 Ent、scopes、biz 或 plugin 的内部包，例如 `internal/authz/identity`；`authz` 保留对外封装，`scopes` 读取共享主体 | 明确共享 Principal 与现有 User/APIKey 实体的关联检查；保证迁移不改变普通认证路径 |
| 在 `contexts` 承载插件委托信息 | 利用已有依赖方向，在 `contexts` 提供服务端设置的安装身份、委托用户及授权上限访问接口；`authz` 和 `scopes` 都读取它，并与 `PrincipalTypePlugin` 联动 | 创建、复制和主体转换入口必须维护两者一致；不能出现委托标记丢失后恢复普通用户权限，或 Principal/委托记录安装 ID 不一致仍被放行 |

第二种方案中的委托信息不是客户端可指定的请求来源字段，也不能复用现有业务 `Source`。单独放一个可被清除的布尔标记不足以保证身份和安装隔离。

选型前先确定各状态的权威来源、关联校验及生命周期。收窄视图只在委托时由真实用户和授权快照一次性派生、固定在快照中，不持久化也不可修改，因此不是可独立漂移的权限副本；不以“当前只有一套主体状态”为前提。包名不是安全保证，最终以同一组越权和兼容性测试验收。业务权限计算仍由授权层负责，不要求为选型统一更换所有调用方 import。

## 6. 私有数据、自助接口与 bypass

### 私有模板

渠道覆盖模板目前只有 Owner/本人授权路径，没有插件显式权限入口。迁移后，Plugin 不能通过这些路径读取或修改模板。限制应在 Ent privacy 层覆盖列表、用户关联、直接 ID、`node/nodes`，不能依赖 GraphQL 字段名枚举。

今后如果确实要向插件开放模板，先定义独立能力和可暴露字段，再增加显式授权路径；不能因为用户拥有模板就默认授权插件读取其中的凭证。

### 自助操作和 `me`

修改个人资料、密码、解绑身份等操作要求直接用户主体。业务入口使用 `RequireDirectUser`，私有资源使用 Ent 本人规则；不依赖 GraphQL mutation 名单。插件的普通用户/角色管理仍按 scope 授权，但不能设置所有权字段，也不能通过追加 scopes 或添加角色超出安装授权。

`me` 要单独决定允许暴露的用户信息。若保留读取自己的基础资料，应提供明确、有限的投影；不能为了兼容它恢复通用的“本人数据自动放行”，也不能让返回关联边带出私有资源。迁移验收必须覆盖这个行为，不能依靠重新加载真实用户绕过权限检查。

### 渠道测试

插件暂不支持 `testChannel`、`testChannelAPIKey` 和 `testChannelAPIKeys`。三个测试方法在业务入口按 Plugin 主体返回 `FORBIDDEN`，在读取渠道凭据、发送上游请求或持久化测试记录之前拒绝；即使插件已获渠道权限也不例外。此限制不依赖 GraphQL 字段黑名单，直接用户保留原来的权限检查和测试行为。以后开放时，先完成调用方授权，再将完整测试及流消费封装为固定宿主操作。

### 插件管理、KV 与内部操作

- 安装、授权、启停、删除等管理操作仍要求直接 Owner；插件不能修改自己的授权。
- 页面 KV 只使用入口绑定的安装 ID。写入继续要求委托用户具备系统级 `write_plugins`，系统 Owner 视为拥有该权限，与 `pages.md` 的“Owner 或系统级 write_plugins”一致。这是宿主定义的独立资格，不是给插件授予插件管理 scope。该资格由入口在委托前根据真实用户计算并写入 `Call.KVWritable`，KV 只认这个值（收窄视图不含 Owner 和插件管理权限）。
- 插件 KV 按服务端绑定的安装 ID 和写资格授权，通过宿主封装的单项存储操作（`runKV` → `authz.RunAsHost`）执行，不向调用方返回 System/Allow context，不接受任意回调，并记录原插件身份及原因。
- Plugin 本身不能取得 System 权限；在插件 context 上调用通用 `WithSystemBypass` / `RunWithSystemBypass*` 按第 3 节默认拒绝。
- 宿主需要以 System 身份替已授权插件执行的固定操作，统一使用 `authz.RunAsHost(ctx, client, reason, fn)`：只保留 Ent client（通常是调用方事务）、取消和截止时间，脱离调用方授权；闭包只返回数据。不在调用点手写脱离 context 的逻辑，也不在 `WithSystemBypass` 外包 `IsPlugin` 分支来放行。当前使用者：插件 KV、API Key 配额用量统计、请求体存储配置查找、项目可用渠道读取项目 profile、系统设置读取（`SystemService.querySystemRow`）。这些都是对所有调用方一致的宿主读取，授权在调用点之前完成，不按主体分路径。
- `read_dashboard` 授权的是宿主定义的固定统计结果，不要求插件额外持有 `read_requests`。dashboard 与 analytics resolver 对所有主体走同一条路径：`WithScopeDecision(read_dashboard)` 后执行聚合查询，只返回统计 DTO，结果不随调用方变化，因此插件与直接用户共用统计缓存。Allow 只在 resolver 内有效，插件读取原始 requests / usageLogs 仍须各自的数据权限。
- 当前 `HasScope` 的未知主体分支返回 false，因此遗漏 Plugin 分支会拒绝访问；应保留这一默认拒绝行为。
- 非 Plugin 的 `WithScopeDecision` 在成功时注入通用 Ent Allow，它不绑定所检查的 scope、资源或查询。即使封装在闭包里，闭包内部级联执行其他 Ent 操作仍会继承 Allow；“仅传给受控操作”不是自动隔离保证。
- Plugin 与直接用户采用相同语义：`HasScope` 对 Plugin 已检查「插件授权 ∩ 委托用户权限」，通过后同样注入 Allow，失败返回 Deny。理由：全部调用点都是 scope 检查之后在 resolver 或闭包内的内部读取（gqlgen 子字段拿不到 resolver 改过的 ctx），插件在这里的暴露面与持有同样 scope 的直接用户相同；若对插件不注入 Allow，持有 `read_channels` 的插件会因内部读取需要其他 scope 而意外失败，只能再用 `IsPlugin` 分支绕开。新增调用点同样只能用于 scope 检查之后的受控内部读取，不得把 Allow ctx 交给插件提供的回调。

## 7. 实施顺序

1. 为现有行为建立回归基线：普通用户、Owner、API Key、项目角色、插件读写矩阵及私有数据路径。
2. 按第 5 节确定 Principal、委托信息与实体状态的权威来源、关联校验及依赖方案，再引入 Plugin 类型，将安装 ID 纳入 `principalEqual` 和幂等/冲突检查，实现受控委托；同时给全部主体覆盖构造函数和 system bypass 包装入口接入转换检查（主体构造 panic、system bypass 默认拒绝）及审计。未知主体默认拒绝。
3. 接入系统/项目 scope、Owner、本人资源等公共规则，并审计所有直接用户身份判断及 bypass 调用。
4. 迁移页面入口：创建 Plugin 主体，context 用户替换为固定在快照中的收窄视图；绑定安装和授权快照。
5. 明确并测试 `me`、自助接口、私有模板、KV 和插件管理行为，补上未经过 Ent 的 service 检查。
6. 将 `BuildPluginScopedUser` 移入 `scopes` 并更名为 `DelegatedUserView`，保留其结构测试，同时补充主体授权测试。
7. 当 Ent 通用授权规则已覆盖私有模板的列表、用户关联、直接 ID、`node/nodes` 等访问路径，并在不依赖字段拦截的条件下通过越权测试后，才删除 `internal/server/gql/graphql.go` 中的 `rejectPluginPrivateFields` 及其 `AroundFields` 注册；保留相应安全回归测试。不能仅因引入 Plugin Principal 就提前删除，个人账号操作由业务入口的 `RequireDirectUser` 检查，私有模板由 Ent 本人资源规则拒绝；`rejectPluginGraphqlOperation` 仅保留 Subscription 限制，不再维护自助 mutation 黑名单。
8. 更新本文、页面契约和审计说明，运行相关测试和 race 检查；lint 遵循仓库规则，仅在用户明确要求时执行。若涉及 Ent/GraphQL schema，再按仓库规则生成代码。

不能发布“context 中是真实 Owner 用户，但旧规则还只看 IsOwner”的状态。实现可分步骤开发，授权闭环必须一起交付。

## 8. 验收矩阵

| 场景 | 预期 |
|---|---|
| Owner + 无授权插件 | 普通业务数据、Owner-only 操作、私有模板均拒绝 |
| Owner + `read_channels` | 允许渠道读取；渠道写入及凭证读取按各自显式权限拒绝 |
| 用户缺少权限、插件已获该权限 | 拒绝；插件不能替用户增加权限 |
| 用户拥有权限、插件未获该权限 | 拒绝；用户角色与 Owner 身份不能绕过上限 |
| 用户和插件都拥有权限 | 原合法业务操作继续成功 |
| 只有项目 A 的角色权限 | A 的授权操作成功，B 的同类操作拒绝；系统级资源不借项目 scope 放行 |
| 私有模板列表、用户关联、node/nodes、别名和 fragment | 都不能读取保存的 headers、凭证或其他模板内容 |
| 调用自助 mutation、备份恢复、授权插件 | Plugin 拒绝；直接用户的原合法调用不受影响 |
| 派生后修改原始 grants 切片、替换用户或再次派生 | 不能扩大授权；身份不匹配的请求拒绝 |
| 真实用户为 Owner、角色完整，但插件未获得 scope | 原用户实体不变；`GetUser` 返回无 Owner、无角色的视图；authz 与 Ent privacy 均拒绝，不能凭 IsOwner 或本人规则放行 |
| Plugin context 中用 `contexts.WithUser` 写回真实用户 | `GetUser` 仍返回固定视图，不恢复 Owner 或完整权限 |
| 项目 Owner 的插件读取个人 API Key、请求记录，或通过项目角色发邀请 | 不按 Owner 放宽可见性；项目角色权限只在授权上限内生效 |
| Principal、委托记录、User/APIKey 实体的身份或安装 ID 不一致 | 拒绝，不回退为普通用户或 API Key 权限 |
| 重新加载用户、替换实体或 fork 请求上下文 | 委托限制持续有效，不能恢复完整用户权限；委托状态缺失或不一致时拒绝 |
| 原 context 含 privacy Allow/bypass | 不得派生成可继承放行状态的 Plugin |
| Plugin context 调用 `NewSystemContext` / `NewUserContext` / `NewAPIKeyContext`（含兼容入口） | 先记录拒绝审计再 panic；父主体不变，不产生新主体权限 |
| Plugin context 调用 `WithSystemBypass` / `RunWithSystemBypass*` | 在切换主体前审计并默认拒绝：`Run*` 返回错误且闭包不执行；`WithSystemBypass` 返回的 context 保留 Plugin 主体、拒绝所有 Ent 操作；不注入 Allow |
| Plugin 类型尚未接入 `HasScope` 分支 | 默认拒绝，不回退为 User/System |
| Plugin 调用使用 `WithScopeDecision` 的 resolver | 插件授权与用户权限交集包含该 scope 时与直接用户结果一致；任一方缺失时拒绝；直接调用与闭包包装均覆盖 |
| 通用 bypass 被禁止后调用合法插件 KV 及其他宿主固定操作 | 经 `RunAsHost` 正常工作，不能通过返回 System context 或任意回调恢复通用提权 |
| 同一用户、不同安装 ID 的 Plugin 主体调用 `WithPrincipal` | 返回主体冲突，原 context 的身份及授权快照保持不变；同名重装也不例外 |
| 身份字段完全一致的 Plugin 主体重复调用 `WithPrincipal` | 幂等成功，不改变原 context 或替换授权快照 |
| 创建安装 ID 缺失或无效的 Plugin 主体 | 拒绝，不能以零值绕过安装身份比较 |
| 同名插件删除重装 | 旧调用仍绑定旧安装，不跨安装访问 KV 或授权 |
| KV 无写资格/有写资格 | 分别拒绝/允许写入自身安装；其他安装始终不可访问 |
| Owner 未显式列出 `write_plugins` / 非 Owner 有系统级 `write_plugins` | 两者均可写本插件安装的 KV；仅有项目级同名权限不能替代系统级写资格 |
| 同一用户分别直接调用、通过不同安装或不同授权快照调用 | 权限过滤后的缓存和 dataloader 结果不混用；授权撤销后不命中旧权限结果 |
| `me` 和嵌套关联 | 仅返回明确允许的资料，不形成私有资源入口 |
| 原 User/APIKey/System 路径 | 既有认证、权限和租户隔离测试全部通过 |

## 9. 非页面入口与扩展规则

新增 scope、资源或 resolver 时，明确它是显式业务权限、直接用户自助能力，还是宿主内部能力。使用带 context 的公共授权入口，不新增只根据 `GetUser().IsOwner` 或用户 ID 放行的插件可达接口。

后台事件没有必然存在的用户；请求钩子可能由 API Key 触发。因此未来须为这些入口单独定义委托来源、数据范围和能力集合，再接入 Plugin 主体。不得伪造一个 Owner 用户，也不得把 Plugin 标记成 System 来获得全部权限。接口落地前继续遵守对应扩展点的最小宿主 API 契约。

权限相关缓存与 dataloader 必须隔离授权上下文。若缓存值已经按调用者权限过滤，key 至少包含主体类型、用户 ID、插件安装 ID（Plugin 路径）及相关项目范围；权限或授权快照变化时，还需通过权限版本、快照摘要或可靠失效机制避免复用旧结果。不能只按用户 ID 共享直接用户和插件结果，也不能在不同安装之间共享过滤后的结果。请求内 dataloader 不跨直接用户/插件派生上下文复用；若必须共享，执行同样的隔离。缓存未过滤的原始实体时，每次使用仍要执行当前调用者的授权和字段过滤，不要求所有纯数据缓存都改成主体级缓存。系统设置读取（`getSystemValue` 及品牌、初始化状态等直接行读取）是宿主读取：缓存存未过滤的原始值，未命中时经 `RunAsHost` 查库，结果与回填都不依赖触发者，插件、API Key、后台任务触发 miss 填进去的数据相同。对外暴露的授权在 GraphQL resolver 边界：品牌、初始化状态、onboarding、`codexCompatibilityEnabled`（只暴露开关，API Key 详情据此展示 Codex 配置示例）对任意调用方开放；`retryPolicy` 需要 `read_settings` 或 `write_channels`（渠道可用性对话框展示全局自动禁用规则）；其余设置需要 `read_settings`。签名密钥不经任何 resolver 暴露，`System` 实体的 Ent policy 对插件也会过滤。dashboard 统计结果不随调用方变化，插件与直接用户共用缓存。请求体存储配置查找属于固定宿主读取：调用方先通过请求本身的授权，查找再经 `RunAsHost` 执行，不向插件返回特权 context。插件请求触发的其他惰性缓存加载若内部调用通用 system bypass，会按第 3 节默认拒绝；确需对插件开放时在调用点改用 `RunAsHost`，不放开通用 bypass。

审计至少记录主体类型、插件安装 ID、manifest ID（辅助信息）、委托者、项目、所需权限和允许/拒绝结果；不得记录 secret、Authorization 请求头或 KV value。授权变更在新调用创建上下文时生效；跨实例的生效延迟沿用运行时同步契约，不承诺立刻撤销已在途调用。
