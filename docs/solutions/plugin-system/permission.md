# 插件权限

> 状态：待实施方案。Plugin Principal 尚未实现。当前实现仍使用 `BuildPluginScopedUser`（原 `DowngradeUser`）；不得在替代授权路径完整接入并通过测试前移除它。

本篇定义插件授权的目标模型、迁移步骤和新增功能时的要求。[manifest.md](manifest.md) 继续作为权限词表和授权升级规则的唯一来源；[pages.md](pages.md) 定义页面入口和桥接；[scope.md](scope.md) 定义插件执行范围，执行范围不等于数据访问权限。

## 1. 当前实现与问题

插件页面通过 `POST /admin/plugins/:pluginId/graphql` 调用宿主。该入口与 `/admin/graphql` 复用 schema，但增加插件身份绑定、授权上限、复杂度、速率和响应大小限制。

当前授权步骤：

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

目标是在 `authz.PrincipalType` 中新增 `PrincipalTypePlugin`，用独立主体表达委托调用。目标实现删除 `BuildPluginScopedUser`，不再通过修改用户实体表示权限上限。

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

授权上下文另保存由服务端解析的插件授权快照。快照应复制输入集合，不能让调用方修改原切片来扩大权限；不向插件暴露可写的主体或授权集合。真实用户、角色和项目成员信息保留原样，身份数据本身不等于允许执行操作。

### 创建与派生

增加受控的用户到插件委托函数，具体名称以实施时为准：

1. 要求父主体是已经认证的 User，且主体 UserID 与 context 用户一致。
2. 入口从服务端解析有效安装和授权，禁止使用客户端提供的 grants、安装 ID 或主体类型。
3. 创建独立的请求上下文，父请求的用户和可变容器不能被改写。
4. 派生后的授权主体是 Plugin；`contexts.GetUser` 仍返回真实委托用户。两者是不同职责的上下文状态，不能根据哪个状态更有权限来选择放行路径。
5. 拒绝缺失用户、无效安装、API Key/System 到页面插件的转换及插件嵌套委托。
6. 不继承父 context 中的全局 privacy Allow/系统 bypass；遇到这类上下文应拒绝创建，避免绕过后续规则。

### 主体覆盖与系统 bypass 的约束

当前并没有全局的主体 set-once 保证：只有 `WithPrincipal` 检查已有主体；`NewUserContext`、`NewAPIKeyContext`（`internal/authz/principal.go`）和 `NewSystemContext`（`internal/authz/system.go`）直接通过 `context.WithValue` 覆盖主体。`WithSystemBypass`、`RunWithSystemBypass`、`RunWithSystemBypassVoid` 先创建 System 主体再注入 privacy Allow。因此，即使插件入口使用 `WithPrincipal`，后续调用这些函数仍能静默把 Plugin 换成 System/User/APIKey。现有审计日志也不能替代授权拒绝。

目标实施必须在主体构造和 bypass 入口强制执行以下规则，不能只靠调用方约定：

1. 在 Plugin context 上调用 `NewSystemContext`、`NewUserContext`、`NewAPIKeyContext` 或普通主体替换入口，返回错误并记录被拒绝的主体转换；不返回已替换身份的 context。测试主体构造入口也不得成为插件链路的替换通道。
2. `WithSystemBypass` 和所有 `RunWithSystemBypass*` 包装入口在覆盖主体之前检查父主体。父主体为 Plugin 时拒绝，闭包不执行，不能注入 privacy Allow。`WithBypassPrivacy` 继续只接受 System/Test 主体。
3. 现有只返回 context 的函数需要增加可报告错误的受检接口，并迁移插件可达调用点；不能用“记录日志后继续执行”“返回原 context 当作成功”或忽略错误的适配器维持旧签名。保留的兼容入口也必须执行相同拒绝策略，不能留下可绕过的旧路径。
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

当前系统已经同时保存两套与调用身份相关的状态：

- `authz.Principal`：独立 context key 下的主体类型和身份 ID，由 `authz.HasScope` 等入口读取。
- `contexts` 中的 `User` / `APIKey` 实体：包含用户、角色、成员关系或 API Key 权限。`scopes` 当前的 `getUserFromContext` / `getAPIKeyFromContext` 直接读取这些实体，不读取 Principal，也不验证实体是否匹配当前主体。

因此，新增 `PrincipalTypePlugin` 并不自动约束 Ent privacy。目标 Plugin 路径保留真实用户，`contexts.GetUser` 和 `getUserFromContext` 都能读到真实的 `IsOwner=true`、完整角色和项目成员信息。若只切换 Principal 而仍让旧规则凭这些字段放行，就会出现“authz 认为是受限插件，scopes 却按真实 Owner 授权”的危险中间态。

迁移后明确两者的职责：

1. Principal / 受控委托状态决定当前调用者类别、插件安装身份及授权上限；User/APIKey 实体提供已认证身份数据和用户原有权限，不能独立决定当前调用是否获得这些权限。
2. 公共授权入口必须先识别当前主体及委托限制，再读取实体计算权限交集和数据过滤条件。只调用 `getUserFromContext` 取得用户不等于已完成授权。
3. 插件页面只允许从直接 User 主体派生。创建时校验 Principal.UserID 与实体用户 ID 一致；Plugin 路径不得因为 context 中残留 APIKey 实体而转入 API Key 授权分支。
4. 实体替换、重新加载用户、请求上下文复制或 fork，均不能移除插件委托约束。主体、委托记录与实体身份不一致时拒绝，不回退为普通用户权限。
5. 切换页面入口、保留真实用户之前，必须先迁移所有插件可达的 scope、Owner、本人资源及直接身份判断路径；不能仅修改 `authz.HasScope` 后就删除用户权限副本。

| 入口/规则 | 目标行为 |
|---|---|
| `authz.HasScope`、`RequireScope`、`WithScopeDecision` | 识别 Plugin，检查插件授权与委托用户权限交集；未接入的主体默认拒绝。Plugin 的 scope-decision 成功不得注入通用 Ent Allow，后续仍执行资源及租户授权规则 |
| 系统 scope 的 Ent 规则 | 使用带 context 的有效权限检查，不能只检查 `user.Scopes` 或 `user.IsOwner` |
| 项目 scope 的 Ent 规则 | 检查授权上限并保留项目成员校验、项目过滤和既有资源可见性约束 |
| `OwnerRule` | Plugin 不通过 Owner 身份捷径，继续评估其他显式授权规则 |
| `UserOwnedQueryRule`、`UserOwnedMutationRule` | Plugin 不因用户 ID 相同而放行，返回 Skip，让其他显式规则决定；最终仍未允许则拒绝 |
| 仅凭项目 Owner 放行的规则 | Plugin 不通过身份捷径；需要有明确的 scope 路径 |
| 直接在 resolver/service 中判断 Owner 的代码 | 改为主体感知的检查，不能重新加载用户后按 IsOwner 放行 |
| 自助操作 | 明确要求直接 User 主体；未迁移的操作继续保留入口限制 |

`Skip` 表示该身份规则不适用，不是允许，也不能用无条件 Deny 阻断同一 policy 后续合法的显式 scope 规则。公共 policy 必须保持默认拒绝。

### 依赖方向

当前 `authz` 已依赖 `scopes`，而 `scopes` 已依赖 `contexts`；直接让 `scopes` import `authz` 会造成循环依赖。依赖调整尚未选型，至少有以下两个候选方案：

| 候选方案 | 做法 | 需要验证的约束 |
|---|---|---|
| 下沉共享身份基础设施 | 将 Principal 类型和 context 存取放入不依赖 Ent、scopes、biz 或 plugin 的内部包，例如 `internal/authz/identity`；`authz` 保留对外封装，`scopes` 读取共享主体 | 明确共享 Principal 与现有 User/APIKey 实体的关联检查；保证迁移不改变普通认证路径 |
| 在 `contexts` 承载插件委托信息 | 利用已有依赖方向，在 `contexts` 提供服务端设置的安装身份、委托用户及授权上限访问接口；`authz` 和 `scopes` 都读取它，并与 `PrincipalTypePlugin` 联动 | 创建、复制和主体转换入口必须维护两者一致；不能出现委托标记丢失后恢复普通用户权限，或 Principal/委托记录安装 ID 不一致仍被放行 |

第二种方案中的委托信息不是客户端可指定的请求来源字段，也不能复用现有业务 `Source`。单独放一个可被清除的布尔标记不足以保证身份和安装隔离。

选型前先确定各状态的权威来源、关联校验及生命周期。两种方案都必须保留真实实体而限制其授权用途，避免再增加可独立修改、互相漂移的权限副本；不以“当前只有一套主体状态”为前提。包名不是安全保证，最终以同一组越权和兼容性测试验收。业务权限计算仍由授权层负责，不要求为选型统一更换所有调用方 import。

## 6. 私有数据、自助接口与 bypass

### 私有模板

渠道覆盖模板目前只有 Owner/本人授权路径，没有插件显式权限入口。迁移后，Plugin 不能通过这些路径读取或修改模板。限制应在 Ent privacy 层覆盖列表、用户关联、直接 ID、`node/nodes`，不能依赖 GraphQL 字段名枚举。

今后如果确实要向插件开放模板，先定义独立能力和可暴露字段，再增加显式授权路径；不能因为用户拥有模板就默认授权插件读取其中的凭证。

### 自助操作和 `me`

修改个人资料、密码、解绑身份等操作要求直接用户主体。既有自助 mutation 名单在所有相关 service 具备主体检查前继续保留，不因新增枚举就删除。

`me` 要单独决定允许暴露的用户信息。若保留读取自己的基础资料，应提供明确、有限的投影；不能为了兼容它恢复通用的“本人数据自动放行”，也不能让返回关联边带出私有资源。迁移验收必须覆盖这个行为，不能依靠重新加载真实用户绕过权限检查。

### 插件管理、KV 与内部操作

- 安装、授权、启停、删除等管理操作仍要求直接 Owner；插件不能修改自己的授权。
- 页面 KV 只使用入口绑定的安装 ID。写入继续要求委托用户具备系统级 `write_plugins`，系统 Owner 视为拥有该权限，与 `pages.md` 的“Owner 或系统级 write_plugins”一致。这是宿主定义的独立资格，不是给插件授予插件管理 scope。
- 当前 KV/管理器存在系统 bypass 调用。它们是迁移清单，不能作为 Plugin → System 的通用例外；引入主体拒绝机制时必须同步处理，否则会导致插件 KV 不可用。
- 插件 KV 应改为按服务端绑定的安装 ID 和写资格授权，或通过宿主封装的单项存储操作执行。若可信宿主内部仍需绕过 privacy，必须在校验身份、安装归属和操作资格之后限定到最小操作；不得向调用方返回 System/Allow context，不得接受任意回调或绕过入口，并记录原插件身份及原因。
- Plugin 本身不能取得 System 权限；在插件 context 上调用通用 `WithSystemBypass` / `RunWithSystemBypass*` 必须按第 3 节拒绝。仅写“不得使用”或只记录成功切换日志，不满足本方案的授权要求。
- 当前 `HasScope` 的未知主体分支返回 false，因此遗漏 Plugin 分支会拒绝访问；应保留这一默认拒绝行为。
- 当前 `WithScopeDecision` 在成功时注入通用 Ent Allow，它不绑定所检查的 scope、资源或查询。即使封装在闭包里，闭包内部级联执行其他 Ent 操作仍会继承 Allow；“仅传给受控操作”不是自动隔离保证。
- 目标 Plugin 分支在 scope 校验成功后保留受限 context，不注入通用 Allow；失败返回 Deny。后续 Ent 操作仍须通过资源级权限及项目过滤规则，`RunWithScopeDecision` 等包装接口采用相同语义。非 Plugin 的既有行为单独回归验证。若某个宿主操作确实需要特许访问，按前述单项存储操作方式处理，不能把任意 service 闭包整体放行。

## 7. 实施顺序

1. 为现有行为建立回归基线：普通用户、Owner、API Key、项目角色、插件读写矩阵及私有数据路径。
2. 按第 5 节确定 Principal、委托信息与实体状态的权威来源、关联校验及依赖方案，再引入 Plugin 类型，将安装 ID 纳入 `principalEqual` 和幂等/冲突检查，实现受控委托；同时给全部主体覆盖构造函数和 system bypass 包装入口接入转换检查、错误传播及审计。未知主体默认拒绝。
3. 接入系统/项目 scope、Owner、本人资源等公共规则，并审计所有直接用户身份判断及 bypass 调用。
4. 迁移页面入口：创建 Plugin 主体，保留真实用户；绑定安装和授权快照，不再构建用户副本。
5. 明确并测试 `me`、自助接口、私有模板、KV 和插件管理行为，补上未经过 Ent 的 service 检查。
6. 同一组交付中删除 `BuildPluginScopedUser` 及仅验证副本结构的测试，用主体授权测试替代。
7. 当 Ent 通用授权规则已覆盖私有模板的列表、用户关联、直接 ID、`node/nodes` 等访问路径，并在不依赖字段拦截的条件下通过越权测试后，才删除 `internal/server/gql/graphql.go` 中的 `rejectPluginPrivateFields` 及其 `AroundFields` 注册；保留相应安全回归测试。不能仅因引入 Plugin Principal 就提前删除，也不能误删 `rejectPluginGraphqlOperation` 中的自助操作限制；后者按 service 主体检查的接入进度逐项迁移。
8. 更新本文、页面契约和审计说明，运行相关测试、race 检查和 lint。若涉及 Ent/GraphQL schema，再按仓库规则生成代码。

不能发布“真实 Owner 用户已保留，但旧规则还只看 IsOwner”的中间状态。实现可分步骤开发，授权闭环必须一起交付。

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
| Plugin context 中真实用户仍为 Owner、角色完整，但插件未获得 scope | 原用户实体不变；authz 与 Ent privacy 均拒绝，不能凭真实 IsOwner 或本人规则放行 |
| Principal、委托记录、User/APIKey 实体的身份或安装 ID 不一致 | 拒绝，不回退为普通用户或 API Key 权限 |
| 重新加载用户、替换实体或 fork 请求上下文 | 委托限制持续有效，不能恢复完整用户权限；委托状态缺失或不一致时拒绝 |
| 原 context 含 privacy Allow/bypass | 不得派生成可继承放行状态的 Plugin |
| Plugin context 调用 `NewSystemContext` / `NewUserContext` / `NewAPIKeyContext`（含兼容入口） | 返回错误并记录拒绝审计；父主体不变，不产生新主体权限 |
| Plugin context 调用 `WithSystemBypass` / `RunWithSystemBypass*` | 在切换主体前拒绝并审计；闭包不执行、不注入 Allow、无数据库或网络副作用 |
| Plugin 类型尚未接入 `HasScope` 分支 | 默认拒绝，不回退为 User/System |
| service 内用 Plugin 的 scope-decision ctx 级联查询其他资源 | 已授权资源按规则可读；未授权资源、私有模板和越界项目仍拒绝。成功检查一种 scope 不产生通用 Allow；直接调用与闭包包装均覆盖 |
| 通用 bypass 被禁止后调用合法插件 KV | 专用安装级授权路径正常工作，不能通过返回 System context 或任意回调恢复通用提权 |
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

权限相关缓存与 dataloader 必须隔离授权上下文。若缓存值已经按调用者权限过滤，key 至少包含主体类型、用户 ID、插件安装 ID（Plugin 路径）及相关项目范围；权限或授权快照变化时，还需通过权限版本、快照摘要或可靠失效机制避免复用旧结果。不能只按用户 ID 共享直接用户和插件结果，也不能在不同安装之间共享过滤后的结果。请求内 dataloader 不跨直接用户/插件派生上下文复用；若必须共享，执行同样的隔离。缓存未过滤的原始实体时，每次使用仍要执行当前调用者的授权和字段过滤，不要求所有纯数据缓存都改成主体级缓存。

审计至少记录主体类型、插件安装 ID、manifest ID（辅助信息）、委托者、项目、所需权限和允许/拒绝结果；不得记录 secret、Authorization 请求头或 KV value。授权变更在新调用创建上下文时生效；跨实例的生效延迟沿用运行时同步契约，不承诺立刻撤销已在途调用。
