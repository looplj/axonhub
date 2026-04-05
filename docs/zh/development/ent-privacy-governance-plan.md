# Ent Privacy 权限治理方案（架构与迁移）

## 背景

当前项目使用 Ent Privacy 作为数据访问最后防线，但在业务层存在大量 `privacy.DecisionContext(ctx, privacy.Allow)` 的直接调用。该方式虽然能快速解决"系统内部操作需要绕过权限"的问题，但会带来以下结构性风险：

1. **绕过边界不清晰**：`Allow` 是全局放行语义，容易把"局部系统动作"扩散成"整段链路放行"。
2. **主体混杂风险**：同一请求上下文可能同时携带 User、API Key、Project 信息，放行后无法区分真实授权主体。
3. **策略失效隐患**：`internal/scopes/policy.go` 的 default-deny 设计会被上层 `Allow` 直接短路。
4. **可审计性不足**：绕过动作缺少统一入口与理由记录，安全审计难以追踪。

以上问题在配额、请求流水、系统配置读取等路径中会叠加，长期可能导致鉴权失效或"偶发越权"。

---

## 现状统计

### 非测试代码中 `privacy.DecisionContext(...Allow)` 分布

| 文件 | 调用次数 | 场景分类 |
|------|---------|----------|
| `biz/system.go` | 15 | 系统配置读写，混合请求链路与内部操作 |
| `server/gc/gc.go` | 7 | 后台 GC 任务 |
| `biz/user.go` | 7 | **请求链路，用户 CRUD，跨主体风险最高** |
| `biz/role.go` | 5 | 请求链路，角色管理 |
| `biz/data_storage.go` | 4 | 数据存储操作 |
| `biz/auth.go` | 3 | 认证查找（合理但需收敛） |
| `biz/api_key.go` | 3 | API Key 管理 |
| `gql/system.resolvers.go` | 2 | **GraphQL 入口层 bypass** |
| `biz/quota.go` | 2 | 请求链路，配额检查 |
| `biz/prompt.go` | 2 | 请求链路 |
| `biz/permission_validator.go` | 2 | 权限校验辅助 |
| `biz/channel_probe.go` | 2 | 后台探测 |
| `biz/channel_model_sync.go` | 2 | 后台同步 |
| `biz/channel_llm.go` | 2 | 请求链路 |
| `biz/channel_auto_disable.go` | 2 | 后台自动禁用 |
| `biz/channel_apikey.go` | 2 | Channel API Key 管理 |
| `gql/me.resolvers.go` | 1 | **GraphQL 入口层 bypass** |
| `biz/system_onboarding.go` | 1 | 初始化引导 |
| `biz/request.go` | 1 | 请求记录 |
| `biz/provider_quota_service.go` | 1 | 后台配额轮询 |
| `biz/project.go` | 1 | 项目管理 |
| `biz/model_fetcher.go` | 1 | 模型拉取 |
| `biz/channel.go` | 1 | Channel 管理 |
| `biz/channel_metrics.go` | 1 | 后台指标 |
| `backup/autobackup.go` | 1 | 后台备份 |
| `scopes/rule_user_scope.go` | 1 | 权限规则内部 |
| `ent/migrate/datamigrate/*.go` | 3 | 数据迁移（合理） |

**合计：约 73 处非测试调用，约 200+ 处测试调用。**

### 非测试代码中 `scopes.WithUserScopeDecision` 分布

| 文件 | 调用次数 | 场景分类 |
|------|---------|----------|
| `gql/dashboard.resolvers.go` | 10 | Scope-Gated Decision，跨实体聚合查询授权 |

此类调用不是无条件 bypass，而是基于用户 scope 的有条件授权决策。当前实现直接调用 `privacy.DecisionContext`，需收敛至 `authz` 包。

---

## 目标

### 核心目标

1. **单一主体模型**：一次请求仅存在一个授权主体（System/User/APIKey）。
2. **受控绕过机制**：禁止业务层直接使用 `privacy.Allow`，所有绕过必须通过受控入口。
3. **Bypass 作用域隔离**：bypass context 不可沿调用链无限扩散。
4. **可审计可回归**：所有 bypass 动作可追踪，且具备防回归测试与静态检查。

### 非目标

1. 本期不重写全部 Ent Schema Policy。
2. 本期不引入外部策略引擎（如 OPA/Casbin）。
3. 本期不改变现有 Role/Scope 语义定义。
4. 本期不引入用例授权层（Authorizer），待后续迭代根据实际需求决定。

---

## 当前问题分层分析

### 1）身份语义层

- 当前 context 通过 `WithUser`、`WithAPIKey`、`WithProjectID` 分散存储身份信息。
- `contextContainer` 是可变指针结构，`WithUser`/`WithAPIKey` 直接修改同一指针实例，无法保证 set-once 语义。
- 缺少"主体唯一性约束"，无法防止同上下文混入多种身份。

### 2）授权执行层

- `internal/scopes` 的规则设计本身是 default-deny + 规则链放行。
- 但业务层全局 `Allow` 会直接绕过规则链，导致规则在关键路径中失效。
- `scopes.WithUserScopeDecision`（已删除）曾是有条件放行（scope-gated），但直接调用 `privacy.DecisionContext`，且仅支持 User 主体，不支持 APIKey。该函数将 Ent Privacy 规则链的判断前置到 Resolver 层，缺少统一收口。现已被 `authz.WithScopeDecision` 完全替代并删除。

### 3）系统内部操作层

- 系统任务（迁移、GC、缓存加载、后台探测）确实需要绕过。
- 但"系统内部动作"与"用户请求流程中的局部内部查询"目前没有被架构上区分。
- 后台任务（如 `provider_quota_service.go`）直接 `ent.NewContext + privacy.Allow`，未显式声明 System 主体，存在用户请求 context 泄漏风险。

---

## 目标架构

## 1. 授权主体模型（Principal）

新增统一主体定义，建议目录：`internal/authz`。

- `PrincipalTypeSystem`
- `PrincipalTypeUser`
- `PrincipalTypeAPIKey`

建议结构：

```go
type PrincipalType int

const (
    PrincipalTypeSystem PrincipalType = iota
    PrincipalTypeUser
    PrincipalTypeAPIKey
)

type Principal struct {
    Type      PrincipalType
    UserID    *int
    APIKeyID  *int
    ProjectID *int
}
```

### 存储方式

**重要**：Principal 不得放入现有 `contextContainer`（可变指针结构），必须使用独立的 `context.WithValue` + **set-once** 语义实现：

```go
// unexported key type，防止外部伪造
type principalKey struct{}

// WithPrincipal 设置 Principal，若已存在则返回 error
func WithPrincipal(ctx context.Context, p Principal) (context.Context, error) {
    if existing, ok := GetPrincipal(ctx); ok {
        if existing != p {
            return ctx, fmt.Errorf("principal conflict: existing=%v, new=%v", existing, p)
        }
        return ctx, nil // 相同主体，幂等
    }
    return context.WithValue(ctx, principalKey{}, p), nil
}

// GetPrincipal 读取 Principal
func GetPrincipal(ctx context.Context) (Principal, bool) {
    p, ok := ctx.Value(principalKey{}).(Principal)
    return p, ok
}

// MustGetPrincipal 读取 Principal，不存在时 panic（用于已确认有主体的链路）
func MustGetPrincipal(ctx context.Context) Principal {
    p, ok := GetPrincipal(ctx)
    if !ok {
        panic("authz: no principal in context")
    }
    return p
}

// NewSystemContext 创建携带 System 主体的 context（用于后台任务）
func NewSystemContext(ctx context.Context) context.Context {
    return context.WithValue(ctx, principalKey{}, Principal{Type: PrincipalTypeSystem})
}
```

约束规则：

1. 每个请求只能有一个 Principal，通过 `WithPrincipal` 的 set-once 语义保证。
2. 中间件完成身份解析后必须调用 `WithPrincipal` 写入。
3. 冲突时 `WithPrincipal` 返回 error，中间件直接拒绝请求。
4. 短期内 `contexts.WithUser`/`WithAPIKey` 保持兼容，逐步迁移。

### ProjectID 归属规则

- **APIKey 主体**：ProjectID 从 APIKey 关联的 Project 获取，属于 Principal 的派生属性。
- **User 主体**：ProjectID 是"选中的项目上下文"，需在业务边界显式校验，不属于 Principal 本身。

## 2. 受控绕过机制（Bypass）

引入**单一受控绕过原语**，禁止在业务代码直接写 `privacy.Allow`。

**设计原则**：第一期只引入一个 bypass 机制，不做 capability 分类枚举。待后续迭代中出现真正的差异化需求时再扩展。

```go
// unexported key type
type bypassKey struct{}

type bypassInfo struct {
    Reason string
}

// WithBypassPrivacy 创建局部 bypass context。
// 仅 Principal=System 或已认证的内部操作允许调用。
// reason 必须为稳定的审计标识（如 "quota-check", "auth-lookup"）。
func WithBypassPrivacy(ctx context.Context, reason string) context.Context {
    ctx = context.WithValue(ctx, bypassKey{}, bypassInfo{Reason: reason})
    // 仅在此处将 capability 转换为 Ent 可识别的放行上下文
    return privacy.DecisionContext(ctx, privacy.Allow)
}

// RunWithBypass 在闭包内执行 bypass 操作，限制 bypass 作用范围。
// 推荐优先使用此方式，防止 bypass context 沿调用链扩散。
func RunWithBypass[T any](ctx context.Context, reason string, fn func(ctx context.Context) (T, error)) (T, error) {
    bypassCtx := WithBypassPrivacy(ctx, reason)
    return fn(bypassCtx)
}
```

### Bypass 作用域隔离

**问题**：`WithBypassPrivacy` 返回的 context 如果直接传递给下游函数，bypass 会沿调用链无限扩散。

**防护措施**：

1. **优先使用 `RunWithBypass` 闭包模式**，将 bypass 限制在最小操作范围内：
   ```go
   // 推荐：bypass 仅覆盖必要的查询
   count, err := authz.RunWithBypass(ctx, "quota-request-count", func(ctx context.Context) (int, error) {
       return client.Request.Query().Where(...).Count(ctx)
   })

   // 不推荐：bypass context 传递给整个函数
   ctx = authz.WithBypassPrivacy(ctx, "quota-check")
   s.doSomethingComplex(ctx)
   ```

2. **变量命名约定**：当必须使用 `WithBypassPrivacy` 时，变量命名为 `bypassCtx`，并控制作用域。

3. **Lint 规则**：检测 `WithBypassPrivacy` 返回值被赋给 `ctx`（而非 `bypassCtx`）时发出警告。

原则：

1. `privacy.DecisionContext(...)` 只在 `internal/authz` 包内调用，其他位置一律禁止（含 `Allow` 和 `Deny`）。
2. 每次 bypass 使用必须携带稳定的 `reason`（用于审计聚合）。
3. bypass 使用记录审计日志（principal、reason、operation、entity）。

## 2.1 Scope 授权决策（Scope-Gated Decision）

### 背景

原 `scopes.WithUserScopeDecision`（已删除）在 Resolver 层使用，是一种**有条件的 scope-aware 放行**：检查当前用户是否拥有特定 scope，有则注入 `privacy.Allow`，无则注入 `privacy.Deny`。该函数已被 `authz.WithScopeDecision` 完全替代并从代码库中删除。

```go
// 旧用法（已删除）
ctx = scopes.WithUserScopeDecision(ctx, scopes.ScopeReadDashboard)

// 当前用法（dashboard.resolvers.go，共 10 处）
ctx = authz.WithScopeDecision(ctx, scopes.ScopeReadDashboard)
```

这种模式与无条件 bypass（`WithBypassPrivacy`）有本质区别：

- **Bypass**：无视权限规则，直接放行，适用于系统内部操作。
- **Scope-Gated Decision**：基于主体身份 + scope 判断后决定 Allow/Deny，是**业务授权行为**，不是绕过。

当前实现的问题：

1. 直接调用 `privacy.DecisionContext`，违反"仅 `authz` 包可调用"的约束。
2. 仅支持 User 主体，不支持 APIKey 等其他主体类型。
3. Allow 注入 context 后会沿调用链扩散，与 bypass 存在相同的作用域问题。
4. 无审计记录。

> **迁移状态**：上述问题已通过 `authz.WithScopeDecision` 解决，`scopes.WithUserScopeDecision` 已从代码库中删除。

### 设计

在 `internal/authz` 包中新增 scope 授权决策原语，统一收敛 `privacy.DecisionContext` 的调用：

```go
// WithScopeDecision 基于当前主体的 scope 做出授权决策。
// 有 scope 则注入 Allow，无则注入 Deny。
// 这是业务授权行为，不是 bypass。
func WithScopeDecision(ctx context.Context, requiredScope scopes.ScopeSlug) context.Context {
    p, ok := GetPrincipal(ctx)
    if !ok {
        return privacy.DecisionContext(ctx, privacy.Deny)
    }

    switch p.Type {
    case PrincipalTypeUser:
        if userHasScope(ctx, requiredScope) {
            return privacy.DecisionContext(ctx, privacy.Allow)
        }
    case PrincipalTypeAPIKey:
        if apiKeyHasScope(ctx, requiredScope) {
            return privacy.DecisionContext(ctx, privacy.Allow)
        }
    case PrincipalTypeSystem:
        // System 主体拥有所有 scope
        return privacy.DecisionContext(ctx, privacy.Allow)
    }

    return privacy.DecisionContext(ctx, privacy.Deny)
}

// RunWithScopeDecision 在闭包内执行 scope 授权决策，限制决策作用范围。
// 推荐优先使用此方式，防止 Allow 沿调用链扩散。
func RunWithScopeDecision[T any](ctx context.Context, requiredScope scopes.ScopeSlug, fn func(ctx context.Context) (T, error)) (T, error) {
    scopeCtx := WithScopeDecision(ctx, requiredScope)
    return fn(scopeCtx)
}

// HasScope 纯判断函数，不注入 DecisionContext，用于业务分支逻辑。
func HasScope(ctx context.Context, requiredScope scopes.ScopeSlug) bool {
    p, ok := GetPrincipal(ctx)
    if !ok {
        return false
    }

    switch p.Type {
    case PrincipalTypeUser:
        return userHasScope(ctx, requiredScope)
    case PrincipalTypeAPIKey:
        return apiKeyHasScope(ctx, requiredScope)
    case PrincipalTypeSystem:
        return true
    }

    return false
}
```

### 内部实现说明

`userHasScope` 和 `apiKeyHasScope` 从 context 中获取对应主体信息并检查 scope。迁移初期可复用 `scopes` 包的现有判断逻辑：

```go
func userHasScope(ctx context.Context, requiredScope scopes.ScopeSlug) bool {
    user, ok := contexts.GetUser(ctx)
    if !ok || user == nil {
        return false
    }
    return scopes.UserHasScope(ctx, requiredScope)
}

func apiKeyHasScope(ctx context.Context, requiredScope scopes.ScopeSlug) bool {
    apiKey, ok := contexts.GetAPIKey(ctx)
    if !ok || apiKey == nil {
        return false
    }
    return scopes.HasAPIKeyScope(apiKey.Scopes, string(requiredScope))
}
```

### 与 Bypass 的区别

| 维度 | `WithBypassPrivacy` | `WithScopeDecision` |
|------|-------------------|-------------------|
| 语义 | 绕过权限规则 | 基于 scope 做授权判断 |
| 条件 | 无条件 Allow | 有 scope → Allow，无 scope → Deny |
| 适用场景 | 系统内部操作、内部查询 | 请求链路中的业务授权 |
| 主体要求 | 通常需要 System 主体 | 支持 User / APIKey / System |
| 审计标识 | reason 字符串 | scope slug |
| 作用域隔离 | `RunWithBypass` 闭包 | `RunWithScopeDecision` 闭包 |

### 迁移示例

```go
// 之前（已删除）
ctx = scopes.WithUserScopeDecision(ctx, scopes.ScopeReadDashboard)

// 当前方式一：直接替换（作用范围覆盖整个 resolver）
ctx = authz.WithScopeDecision(ctx, scopes.ScopeReadDashboard)

// 当前方式二（推荐）：闭包限制作用范围
result, err := authz.RunWithScopeDecision(ctx, scopes.ScopeReadDashboard, func(ctx context.Context) (*DashboardOverview, error) {
    // 仅此闭包内的 Ent 操作受 scope 决策影响
    return buildDashboardOverview(ctx, client)
})
```

### 长期演进

Scope-Gated Decision 本质上是将 Ent Privacy 规则链的判断前置到 Resolver 层。对于已有对应 Ent Schema Policy 规则的实体（如 `UserReadScopeRule`），应优先依赖规则链本身，无需在 Resolver 层重复判断。`WithScopeDecision` 主要用于以下场景：

1. **跨实体聚合查询**：如 Dashboard 同时查询 Request、Channel 等多种实体，无法由单一 Schema Policy 覆盖。
2. **实体 Schema 未配置对应 scope 规则**：如 `ScopeReadDashboard` 未出现在任何 Ent Schema Policy 中。
3. **需要在 Resolver 层提前做 scope 判断以避免无意义的数据库查询**。

当条件成熟时，应将 scope 判断逐步下沉到 Ent Schema Policy（通过 `UserReadScopeRule` / `APIKeyScopeQueryRule` 等），减少 Resolver 层的 `WithScopeDecision` 调用。

## 3. 两层授权模型

### A. 入口层（Middleware + Resolver）

- **Middleware**：负责认证并通过 `WithPrincipal` 设置主体，拒绝多主体冲突，不负责复杂业务授权判断。
- **Resolver（可选）**：对于跨实体聚合查询等无法由单一 Schema Policy 覆盖的场景，通过 `authz.WithScopeDecision` 做 scope 授权决策。

### B. 数据层（Ent Privacy）

- 继续作为最后防线。
- 面向实体的规则链保留（`UserReadScopeRule`、`APIKeyScopeQueryRule` 等）。
- 仅在 `authz.WithBypassPrivacy` / `authz.RunWithBypass` 明确授权时允许 bypass。
- 仅在 `authz.WithScopeDecision` / `authz.RunWithScopeDecision` 做 scope-gated 放行时允许 scope 授权决策。

> **关于用例授权层（Authorizer）**：当前阶段不引入独立的 Authorizer 抽象。在迁移过程中如果发现反复出现 "can X do Y" 的重复判断模式，再考虑提取为统一的用例授权层。

## 4. Bypass 准入边界（文件规范）

针对“只能在 use case 做 bypass，不能污染通用底层方法”的目标，采用**文件级准入**，而非 `ForRequest/Internal` 双接口拆分。

### 文件命名规范

- 优先在 `*_internal.go` 文件中调用 `authz.WithBypassPrivacy` / `authz.RunWithBypass`。
- 推荐命名：`xxx_internal.go`（示例：`quota_internal.go`、`channel_internal.go`）。
- `*_internal.go` 文件用于承载“系统内部用例/后台作业/内部校验”的编排逻辑。
- 对于 `system.go` 等场景复杂、bypass 调用密集的文件，允许直接调用 `authz` 原语，通过 lint 白名单管理。

### 准入规则

1. **请求入口与通用层禁止 bypass**：`resolver`、`handler`、中间件、通用 service/repository/util 不得直接 bypass。
2. **bypass 只在用例边界触发**：优先在 `*_internal.go` 中使用 `RunWithBypass` 包裹最小查询/写入范围；白名单文件中也需遵循最小范围原则。
3. **底层方法保持无 bypass 语义**：底层方法只接收 `ctx` 执行逻辑，不在内部创建 bypass context。
4. **不新增 `GetXxxInternal` 并行 API**：避免“同语义双方法”长期滥用与扩散。

### Lint/CI 白名单

静态检查白名单收敛为：

- `internal/authz/`（框架实现）
- `*_test.go`（测试）
- `internal/ent/migrate/datamigrate/`（迁移）
- `internal/server/biz/*_internal.go`（受控内部用例文件）
- `internal/server/biz/system.go`（系统配置服务，bypass 场景复杂，允许直接调用 `authz` 原语）

白名单外出现 bypass 或裸 `privacy.DecisionContext(...Allow)` 一律失败。

## 5. 后台任务系统身份

所有后台任务（GC、备份、探测、配额轮询等）必须在启动时显式声明 System 主体：

```go
// 之前（直接 bypass，无主体声明）
ctx = ent.NewContext(ctx, svc.db)
ctx = privacy.DecisionContext(ctx, privacy.Allow)

// 之后（显式 System 主体 + 受控 bypass）
ctx = ent.NewContext(ctx, svc.db)
ctx = authz.NewSystemContext(ctx)
ctx = authz.WithBypassPrivacy(ctx, "provider-quota-check")
```

---

## 高风险场景优先治理

## P0：认证上下文唯一性

目标文件：`internal/server/middleware/auth.go`、`internal/authz`

问题：当前中间件分别调用 `contexts.WithUser` / `contexts.WithAPIKey`，无唯一性约束，理论上可并存。

治理方式：

1. 实现 `authz` 包的 Principal + WithBypassPrivacy 基础 API。
2. 在 `WithAPIKeyAuth`、`WithJWTAuth` 等中间件中，认证成功后调用 `authz.WithPrincipal`。
3. 冲突时直接返回 401/403，快速失败。
4. 短期保留 `contexts.WithUser`/`WithAPIKey` 兼容调用。

## P0：GraphQL 入口层 bypass

目标文件：`internal/server/gql/system.resolvers.go`、`internal/server/gql/me.resolvers.go`

问题：Resolver 层（请求入口）直接调用 `privacy.Allow`，绕过整个 Ent Privacy 规则链。入口层 bypass 风险最高，因为它覆盖了所有后续数据操作。

治理方式：

1. 将 resolver 中的裸 `Allow` 替换为 `authz.RunWithBypass`，并缩小 bypass 作用范围。
2. 评估是否可通过调整 Ent Privacy 规则本身（如为 Owner 角色添加对应规则）消除 bypass 需求。

## P1：GraphQL Resolver Scope-Gated Decision 收敛 ✅

目标文件：`internal/server/gql/dashboard.resolvers.go`（10 处）

问题：`WithUserScopeDecision`（已删除）在 `scopes` 包内直接调用 `privacy.DecisionContext`，且仅支持 User 主体。该模式虽非无条件 bypass，但违反了"仅 `authz` 包可调用 `privacy.DecisionContext`"的约束。

已完成治理：

1. 全部 10 处 `WithUserScopeDecision` 调用已替换为 `authz.WithScopeDecision`。
2. `authz.WithScopeDecision` 统一支持 User / APIKey / System 三种主体。
3. `scopes.WithUserScopeDecision` 已从代码库中删除。
4. 评估是否可通过在 Ent Schema Policy 中添加 `UserReadScopeRule(ScopeReadDashboard)` 消除 Resolver 层的 scope 决策需求（注意 Dashboard 为跨实体聚合查询，可能不适用）。

## P0：用户服务请求链路

目标文件：`internal/server/biz/user.go`（7 处）

问题：用户 CRUD 操作（CreateUser、UpdateUser、DeleteUser 等）在函数入口直接 `Allow`，用户请求链路中的 bypass 覆盖了全部后续数据操作，存在跨主体越权风险。

治理方式：

1. 逐个分析每处 `Allow` 的必要性。对于已在中间件层完成权限校验的操作，考虑通过 Ent Privacy 规则本身放行。
2. 对确需 bypass 的内部查询使用 `RunWithBypass` 缩小作用范围。

## P1：配额链路

目标文件：`internal/server/biz/quota.go`（2 处）

问题：`CheckAPIKeyQuota` 和 `GetQuota` 在函数入口直接 `Allow`，将 API Key 请求链路中的内部统计查询一并全局放行。

治理方式：

1. 移除函数入口级 `Allow`。
2. 仅对必要的内部查询（requestCount、tokenCount）使用 `RunWithBypass` 封装。
3. 增加一致性校验：请求上下文 APIKey 与入参 `apiKeyID` 不一致时拒绝。

## P1：系统配置读取路径

目标文件：`internal/server/biz/system.go`（15 处，最大热点）

问题：大量公共方法内部无条件注入 `privacy.Allow`。部分方法（如 `SecretKey`、`GeneralSettings`、`ChannelSettingOrDefault`）同时被请求链路和内部操作调用，bypass 语义混淆。

治理方式：

1. `system.go` 不再拆分 `*_internal.go`，将 `system.go` 加入 lint 白名单，允许直接调用 `authz` 原语（`RunWithBypass` / `WithBypassPrivacy`）。
2. 公共方法移除无条件 `Allow`，改用 `authz.RunWithBypass` 收敛 bypass 作用范围。
3. 禁止新增 `GetXxxInternal` 一类并行 API，避免边界漂移和滥用。

## P1：权限校验辅助服务

目标文件：`internal/server/biz/permission_validator.go`（2 处）

治理方式：

1. 把内部查询改为 `RunWithBypass` 驱动。
2. 明确"用于权限判断的内部读"不等于"业务数据可放行"。

## P1：角色管理

目标文件：`internal/server/biz/role.go`（5 处）

治理方式：

1. 分析每处 bypass 是否可通过 Ent Privacy 规则替代。
2. 确需 bypass 的操作收敛至 `RunWithBypass`。

## P2：后台作业统一 System 主体

目标文件：`server/gc/gc.go`（7 处）、`backup/autobackup.go`、`biz/channel_probe.go`、`biz/channel_model_sync.go`、`biz/channel_auto_disable.go`、`biz/provider_quota_service.go`、`biz/channel_metrics.go`

治理方式：

1. 所有后台任务启动时使用 `authz.NewSystemContext` 声明 System 主体。
2. 将裸 `Allow` 替换为 `authz.WithBypassPrivacy`。
3. 确保后台任务不会复用用户请求的 context。

## P2：其他散点 bypass

目标文件：`biz/data_storage.go`（4 处）、`biz/api_key.go`（3 处）、`biz/channel_llm.go`（2 处）、`biz/prompt.go`（2 处）、`biz/channel_apikey.go`（2 处）、其他各 1 处

治理方式：

1. 逐步使用 `RunWithBypass` 替换。
2. 评估是否可通过 Ent Privacy 规则调整消除 bypass 需求。

---

## 迁移路线图

> 说明：以下改为迁移 TODO 清单，并按当前仓库状态更新。

## 阶段 0（准备）TODO

- [x] 新建 `internal/authz` 包，实现 Principal（set-once）+ `WithBypassPrivacy` / `RunWithBypass` + `WithScopeDecision` / `RunWithScopeDecision` / `HasScope` + 审计接口。
- [x] 增加静态检查：禁止 `internal/authz` 之外直接调用 `privacy.DecisionContext(...)`，并限制 bypass 调用位置（含迁移期 allowlist）。
- [x] 补充开发规范文档（`docs/zh/development/authz-coding-guidelines.md`）。

## 阶段 1（止血）TODO

- [x] 中间件层接入 `WithPrincipal`，实现主体唯一性校验（`internal/server/middleware/auth.go`）。
- [x] 迁移 P0 路径：GraphQL resolver bypass（`gql/system.resolvers.go`、`gql/me.resolvers.go`）收敛。
- [x] 迁移 P0 路径：`biz/user.go` 请求链路 bypass 收敛。
- [x] 增加配套单测：`internal/authz/*_test.go` 覆盖混合主体拒绝、bypass/scope 作用域限制等核心不变量。

## 阶段 2（收敛）TODO

- [x] 迁移 P1 路径：`quota.go`、`permission_validator.go`、`role.go`、`auth.go` 已收敛到 `authz` 原语。
- [x] `SystemService` bypass 治理策略调整：不再拆分 `*_internal.go`，改为将 `system.go` 加入 lint 白名单，允许直接调用 `authz` 原语。
- [x] Scope-Gated Decision 收敛：`dashboard.resolvers.go` 已替换为 `authz.WithScopeDecision`。
- [x] 旧 API 清理：`scopes.WithUserScopeDecision` 已从代码库中删除，全仓无残留调用。
- [x] 扩展 bypass 审计字段（`reason`、`principal`、`operation`、`entity`）已在 `internal/authz/bypass.go` 落地。

## 阶段 3（清理）TODO

- [x] 后台作业统一 System 主体 + 收敛剩余散点 bypass（进行中）。
  - 已完成部分：`backup/autobackup.go`、`biz/provider_quota_service.go`、`biz/channel_probe.go`、`biz/channel_model_sync.go`。
  - 未完成部分：`server/gc/gc.go`。
- [x] 全仓扫描并清除白名单外裸 `Allow`（进行中，当前仍有残留）。    
  - 当前残留（非测试且非迁移）：`internal/server/gc/gc.go`、`internal/server/biz/prompt.go`、`internal/server/biz/project.go`、`internal/server/biz/api_key.go`、`internal/server/biz/channel_metrics.go`、`internal/server/biz/system_onboarding.go`、`internal/server/biz/channel.go`。
- [x] 对历史路径加 deprecated 标识与移除计划（未完成）。

## 当前交付状态汇总

- [x] `authz` 基础包（含 bypass 和 scope decision 两类原语）
- [x] lint 规则 / CI 脚本（含迁移期 allowlist）
- [x] P0/P1 关键链路改造（多数完成）
- [x] `SystemService` bypass 治理策略调整（`system.go` 加入 lint 白名单，不再要求 `*_internal.go`）
- [x] 旧 API 清理：`scopes.WithUserScopeDecision` 已删除
- [x] 全仓 bypass/裸 `Allow` 收敛（未完成）
- [x] 历史路径清理计划（未完成）

---

## 编码规范（新增）

1. **禁止**在 `internal/authz` 之外直接使用 `privacy.DecisionContext`（含 `Allow` 和 `Deny`）。
2. 所有无条件 bypass 必须经过 `authz.WithBypassPrivacy` 或 `authz.RunWithBypass`，且仅允许出现在 `*_internal.go` 及 lint 白名单文件（以及测试/迁移/`internal/authz`）。
3. 所有 scope 授权决策必须经过 `authz.WithScopeDecision` 或 `authz.RunWithScopeDecision`。
4. 纯 scope 判断（不注入 DecisionContext）使用 `authz.HasScope`。
5. **优先使用闭包模式**（`RunWithBypass` / `RunWithScopeDecision`），将 Allow/Deny 限制在最小操作范围内。
6. 当必须使用 `WithBypassPrivacy` 时，返回值变量命名为 `bypassCtx`，禁止赋给 `ctx`。
7. 所有 bypass 使用必须携带简洁且稳定的 reason（用于审计聚合）。
8. Middleware 必须通过 `authz.WithPrincipal` 保证 Principal 唯一。
9. 后台任务必须通过 `authz.NewSystemContext` 显式声明 System 主体。
10. 测试文件中的 `privacy.Allow` / `privacy.Deny` 允许直接使用（白名单），但鼓励逐步迁移到 `authz` API。
11. `scopes.WithUserScopeDecision` 已删除，所有 scope 授权决策统一使用 `authz.WithScopeDecision`。

---

## 测试与验收

## 1. 核心不变量测试（第一期聚焦）

第一期聚焦以下三个不变量的测试覆盖：

1. **混合主体拒绝**：同一 context 写入两个不同 Principal 时返回 error。
2. **请求路径无裸 Allow**：CI 静态检查通过（白名单外无 `privacy.DecisionContext(...Allow)`）。
3. **Bypass 调用位置合规**：lint 白名单外的业务文件无 bypass 调用。
4. **Bypass 审计记录**：每次 `WithBypassPrivacy` / `RunWithBypass` 调用产生审计记录，包含 principal + reason。

## 2. 关键用例

1. APIKey 请求链路中不可借由内部流程越权访问其他 Project 数据。
2. 用户请求链路中 bypass 不可无理由触发。
3. 后台任务可在 System 主体下完成必要操作。
4. 混合主体上下文必须被拒绝。
5. `RunWithBypass` 闭包外的操作不受 bypass 影响。
6. `WithScopeDecision`：User 有 scope 时 Allow，无 scope 时 Deny。
7. `WithScopeDecision`：APIKey 有 scope 时 Allow，无 scope 时 Deny。
8. `WithScopeDecision`：System 主体始终 Allow。
9. `WithScopeDecision`：无 Principal 时 Deny。
10. `RunWithScopeDecision` 闭包外的操作不受 scope 决策影响。

## 3. 测试矩阵（后续迭代完善）

按主体与操作维度建立表驱动测试：

- 主体：System / User / APIKey
- 操作：Query / Mutation
- 作用域：System Scope / Project Scope / Own Scope
- 期望：Allow / Deny / Filtered

## 4. 验收标准

1. 白名单外请求路径零裸 `Allow`。
2. P0/P1 链路通过回归。
3. 审计日志可追踪全部 bypass 动作。
4. 新增权限相关缺陷在不变量测试中可复现并可防回归。

---

## 风险与回滚

### 风险

1. 历史逻辑依赖隐式放行，迁移后可能出现"原本可用路径被拒绝"。
2. 中间件身份收敛可能暴露历史错误调用。
3. `RunWithBypass` 闭包模式会增加部分代码的嵌套层级。

### 缓解

1. 分阶段灰度：先 P0，再扩大。
2. 对关键接口加观测：拒绝率、403 明细、bypass 使用统计。
3. 保留短期回滚开关（仅用于迁移期）。
4. 迁移前先为目标路径补充现有行为的 snapshot 测试，确保改造后行为一致。

---

## 附录：实施优先级清单

### 优先级 P0

1. `internal/authz`（新建：Principal + WithBypassPrivacy + RunWithBypass + WithScopeDecision + RunWithScopeDecision + HasScope + 审计）
2. `internal/server/middleware/auth.go`（接入 Principal，主体唯一性）
3. `internal/server/gql/system.resolvers.go`、`internal/server/gql/me.resolvers.go`（入口层 bypass 收敛）
4. `internal/server/biz/user.go`（7 处，请求链路跨主体风险）

### 优先级 P1

1. `internal/server/biz/system.go`（15 处，最大热点，加入 lint 白名单，使用 `authz` 原语收敛）
2. `internal/server/biz/quota.go`（2 处，配额链路）
3. `internal/server/biz/role.go`（5 处，角色管理）
4. `internal/server/biz/permission_validator.go`（2 处，权限校验辅助）
5. `internal/server/biz/auth.go`（3 处，认证操作收敛）
6. `internal/server/gql/dashboard.resolvers.go`（10 处，Scope-Gated Decision 收敛至 `authz.WithScopeDecision`）

### 优先级 P2

1. 后台作业统一 System 主体：`server/gc/gc.go`（7 处）、`backup/autobackup.go`、`biz/channel_probe.go`、`biz/channel_model_sync.go`、`biz/channel_auto_disable.go`、`biz/provider_quota_service.go`、`biz/channel_metrics.go`
2. 散点 bypass 清理：`biz/data_storage.go`（4 处）、`biz/api_key.go`（3 处）、`biz/channel_llm.go`（2 处）、`biz/prompt.go`（2 处）、`biz/channel_apikey.go`（2 处）、其他各 1 处
