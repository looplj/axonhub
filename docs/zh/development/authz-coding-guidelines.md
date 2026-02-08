# Authz 包使用规范

本文档定义了 `internal/authz` 包的使用规范，用于实施 Ent Privacy 权限治理方案。

## 背景

根据 [Ent Privacy 权限治理方案](./ent-privacy-governance-plan.md)，我们引入了 `internal/authz` 包来提供受控的权限绕过机制，替代直接调用 `privacy.DecisionContext(ctx, privacy.Allow)` 的做法。

## 核心原则

1. **单一主体**：每个请求只能有一个授权主体（Principal）
2. **受控绕过**：所有 bypass 必须通过 `authz` 包提供的 API
3. **作用域隔离**：bypass 应该限制在最小操作范围内
4. **可审计**：所有 bypass 操作都会被记录审计日志

## API 概览

### Principal（授权主体）

```go
// 主体类型
PrincipalTypeSystem  // 系统主体（后台任务）
PrincipalTypeUser    // 用户主体
PrincipalTypeAPIKey  // API Key 主体

// 创建主体上下文
ctx = authz.NewSystemContext(ctx)           // 系统主体
ctx = authz.NewUserContext(ctx, userID)     // 用户主体
ctx = authz.NewAPIKeyContext(ctx, apiKeyID, projectID) // API Key 主体

// 设置主体（带冲突检测）
ctx, err := authz.WithPrincipal(ctx, principal)

// 获取主体
p, ok := authz.GetPrincipal(ctx)
p := authz.MustGetPrincipal(ctx)  // 不存在时 panic
```

### Bypass（受控绕过）

```go
// 方式一：闭包模式（推荐）
// 将 bypass 限制在最小操作范围内
result, err := authz.RunWithBypass(ctx, "quota-check", func(bypassCtx context.Context) (T, error) {
    return client.Request.Query().Where(...).Count(bypassCtx)
})

// 方式二：显式创建 bypass context
// 仅在必要时使用，变量必须命名为 bypassCtx
bypassCtx := authz.WithBypassPrivacy(ctx, "auth-lookup")
result, err := client.User.Get(bypassCtx, id)
```

### Scope 授权决策（Scope-Gated Decision）

```go
// 方式一：闭包模式（推荐）
// 将 scope 决策限制在最小操作范围内
result, err := authz.RunWithScopeDecision(ctx, scopes.ScopeReadDashboard, func(ctx context.Context) (*DashboardOverview, error) {
    return buildDashboardOverview(ctx, client)
})

// 方式二：显式创建 scope decision context
// 仅在必要时使用
scopeCtx := authz.WithScopeDecision(ctx, scopes.ScopeReadDashboard)
result, err := client.Request.Query().All(scopeCtx)

// 纯判断函数（不注入 DecisionContext，用于业务分支逻辑）
if authz.HasScope(ctx, scopes.ScopeAdmin) {
    // 执行管理员操作
}
```

## 编码规范

### 1. 禁止直接使用 privacy.Allow

**禁止**（除非在允许名单中）：
```go
// 错误！禁止在 authz 包外直接使用
ctx = privacy.DecisionContext(ctx, privacy.Allow)
```

**正确**：
```go
// 使用 authz 包提供的受控绕过机制
result, err := authz.RunWithBypass(ctx, "reason", func(bypassCtx context.Context) (T, error) {
    // 操作
})
```

### 2. 优先使用 RunWithBypass 闭包模式

**推荐**：
```go
// bypass 仅覆盖必要的查询
count, err := authz.RunWithBypass(ctx, "quota-request-count", func(ctx context.Context) (int, error) {
    return client.Request.Query().Where(...).Count(ctx)
})
```

**不推荐**：
```go
// bypass context 传递给整个函数，容易扩散
bypassCtx := authz.WithBypassPrivacy(ctx, "quota-check")
result := s.doSomethingComplex(bypassCtx)
```

### 3. 变量命名约定

当必须使用 `WithBypassPrivacy` 时，变量**必须**命名为 `bypassCtx`，禁止赋给 `ctx`：

```go
// 正确
bypassCtx := authz.WithBypassPrivacy(ctx, "reason")

// 错误！会导致 bypass 沿调用链扩散
ctx = authz.WithBypassPrivacy(ctx, "reason")
```

### 4. 提供稳定的 reason

所有 bypass 使用必须携带简洁且稳定的 reason（用于审计聚合）：

```go
// 好的 reason：稳定、可聚合、有语义
authz.RunWithBypass(ctx, "quota-check", ...)
authz.RunWithBypass(ctx, "auth-lookup", ...)
authz.RunWithBypass(ctx, "permission-check", ...)

// 避免的 reason：包含动态值
authz.RunWithBypass(ctx, fmt.Sprintf("quota-check-%d", userID), ...) // 不要这样做
```

### 5. 后台任务必须声明 System 主体

```go
// 之前（直接 bypass，无主体声明）
ctx = ent.NewContext(ctx, svc.db)
ctx = privacy.DecisionContext(ctx, privacy.Allow)

// 之后（显式 System 主体 + 受控 bypass）
ctx = ent.NewContext(ctx, svc.db)
ctx = authz.NewSystemContext(ctx)
ctx = authz.WithBypassPrivacy(ctx, "provider-quota-check")
```

### 6. 主体冲突检测

中间件必须通过 `authz.WithPrincipal` 保证 Principal 唯一：

```go
// 在中间件中设置主体
principal := authz.Principal{
    Type:   authz.PrincipalTypeUser,
    UserID: &user.ID,
}
ctx, err := authz.WithPrincipal(ctx, principal)
if err != nil {
    // 主体冲突，拒绝请求
    return nil, fmt.Errorf("principal conflict: %w", err)
}
```

### 7. 文件命名规范

- 优先在 `*_internal.go` 文件中调用 `authz.WithBypassPrivacy` / `authz.RunWithBypass`
- `*_internal.go` 文件用于承载"系统内部用例/后台作业/内部校验"的编排逻辑
- 对于 `system.go` 等场景复杂、bypass 调用密集的文件，允许直接调用 `authz` 原语，通过 lint 允许名单管理

### 8. Bypass 与 Scope 决策的区别

| 维度 | `WithBypassPrivacy` | `WithScopeDecision` |
|------|-------------------|-------------------|
| 语义 | 绕过权限规则 | 基于 scope 做授权判断 |
| 条件 | 无条件 Allow | 有 scope → Allow，无 scope → Deny |
| 适用场景 | 系统内部操作、内部查询 | 请求链路中的业务授权 |
| 主体要求 | 通常需要 System 主体 | 支持 User / APIKey / System |
| 审计标识 | reason 字符串 | scope slug |
| 作用域隔离 | `RunWithBypass` 闭包 | `RunWithScopeDecision` 闭包 |

## 常见场景示例

### 场景一：配额检查中的内部统计

```go
func (s *QuotaService) CheckQuota(ctx context.Context, apiKeyID int) error {
    // 获取当前请求的主体
    p, ok := authz.GetPrincipal(ctx)
    if !ok {
        return fmt.Errorf("no principal")
    }

    // 验证 apiKeyID 与请求主体一致
    if p.IsAPIKey() && p.APIKeyID != nil && *p.APIKeyID != apiKeyID {
        return fmt.Errorf("api key mismatch")
    }

    // 使用 RunWithBypass 进行内部统计查询
    count, err := authz.RunWithBypass(ctx, "quota-request-count", func(bypassCtx context.Context) (int, error) {
        return s.client.Request.Query().
            Where(request.APIKeyID(apiKeyID)).
            Count(bypassCtx)
    })
    if err != nil {
        return err
    }

    // 继续配额检查逻辑...
}
```

### 场景二：权限校验辅助查询

```go
func (v *PermissionValidator) HasPermission(ctx context.Context, userID int, permission string) (bool, error) {
    // 使用 RunWithBypass 查询权限数据
    roles, err := authz.RunWithBypass(ctx, "permission-check", func(bypassCtx context.Context) ([]*ent.Role, error) {
        return v.client.User.Query().
            Where(user.ID(userID)).
            QueryRoles().
            All(bypassCtx)
    })
    if err != nil {
        return false, err
    }

    // 校验权限...
}
```

### 场景三：后台 GC 任务

```go
func (gc *GC) Run(ctx context.Context) error {
    // 显式声明 System 主体
    ctx = authz.NewSystemContext(ctx)

    // 使用 RunWithBypass 执行清理
    deleted, err := authz.RunWithBypass(ctx, "gc-old-requests", func(bypassCtx context.Context) (int, error) {
        return gc.client.Request.Delete().
            Where(request.CreatedAtLT(cutoff)).
            Exec(bypassCtx)
    })
    if err != nil {
        return err
    }

    log.Info(ctx, "GC completed", log.Int("deleted", deleted))
    return nil
}
```

### 场景四：Dashboard Scope 授权

```go
func (r *dashboardResolver) Overview(ctx context.Context) (*DashboardOverview, error) {
    // 使用 RunWithScopeDecision 限制 scope 决策作用范围
    result, err := authz.RunWithScopeDecision(ctx, scopes.ScopeReadDashboard, func(ctx context.Context) (*DashboardOverview, error) {
        return buildDashboardOverview(ctx, r.client)
    })
    if err != nil {
        return nil, err
    }
    return result, nil
}
```

## 测试

### 测试主体设置

```go
func TestSomething(t *testing.T) {
    ctx := authz.NewSystemContext(context.Background())
    // 或者
    ctx := authz.NewUserContext(context.Background(), 123)

    // 测试代码...
}
```

### 测试主体冲突

```go
func TestPrincipalConflict(t *testing.T) {
    ctx := authz.NewUserContext(context.Background(), 123)

    // 尝试设置不同的主体应该失败
    apiKeyID := 456
    _, err := authz.WithPrincipal(ctx, authz.Principal{
        Type:     authz.PrincipalTypeAPIKey,
        APIKeyID: &apiKeyID,
    })
    if err == nil {
        t.Error("expected conflict error")
    }
}
```

## 迁移检查

运行以下命令检查是否还有非法的 `privacy.DecisionContext(...Allow)` 调用：

```bash
make lint-privacy
```

或在 CI 中自动检查：

```bash
make lint
```

## 允许名单

以下位置允许直接调用 `privacy.DecisionContext(...Allow)`：

1. `internal/authz/*` - 受控绕过机制实现

以下位置允许调用 `authz` bypass API（如 `WithBypassPrivacy`、`RunWithBypass`）：

1. `internal/authz/*` - 框架实现
2. `*_internal.go` - 内部用例文件
3. `internal/server/gql/*.resolvers.go` - GraphQL Resolver 文件
4. `internal/server/biz/system_*.go` - 系统服务文件
5. `*_test.go` - 测试文件
6. `internal/ent/migrate/datamigrate/*` - 数据迁移脚本
7. 以下文件（特殊情况可加入允许名单）：
   - `internal/server/orchestrator/orchestrator.go`
   - `internal/server/biz/auth.go`
   - `internal/server/biz/quota.go`
   - `internal/server/biz/permission_validator.go`
   - `internal/server/biz/prompt.go`
   - `internal/server/biz/system.go`

## 相关文档

- [Ent Privacy 权限治理方案](./ent-privacy-governance-plan.md)
- `internal/authz/principal.go` - Principal 实现
- `internal/authz/bypass.go` - Bypass 机制实现
- `internal/authz/scope.go` - Scope 授权决策实现
