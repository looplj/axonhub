# Axon / AxonCli Permission & Approval Framework

> **Status**: Implemented (MVP)
> **Date**: 2026-02-22
> **Author**: AI Assistant

## Overview

为 `axon`（Agent/Tool 执行内核）与 `axoncli`（TUI + 本地工具集成端）设计一套统一的权限控制与审批（human-in-the-loop）框架，覆盖：

- 本地工具调用的授权（AuthZ）与资源约束（workspace、路径、命令、域名等）
- 高风险操作的审批（Approval）与临时放行（Grant：once/thread/workspace）
- 审计（Audit）与可观测性（日志/事件）

该框架不替代已有“工具内部安全校验”（如 Bash denylist），而是在“工具真正执行前”增加统一治理层，做到可配置、可扩展、默认安全。

## Goals

- 默认安全：无显式授权或策略匹配时倾向拒绝或要求审批
- 可控可审：可解释的决策（命中哪条规则/原因），可追溯（审计日志）
- 可插拔：策略来源、审批渠道、审计落地方式可替换
- 与现有代码结构匹配：在最小侵入点接入（工具执行前一刻）

## Non-Goals

- 不讨论 AxonHub 服务端 RBAC/Ent privacy（本方案仅针对 `axon`/`axoncli` 本地工具执行）
- 不设计 LLM 提示注入与内容安全（仅管“能不能做/需不需要人确认”）
- 不要求一次性实现所有模式（方案按阶段可逐步落地）

## Terminology

- **Capability**：能力维度的权限（如 `fs.read`、`proc.exec`）
- **Resource**：能力作用对象（path/command/domain 等）
- **Policy**：规则集合，输出 Decision
- **Approval**：人类确认（允许一次/允许 thread/允许 workspace/拒绝）
- **Grant**：审批结果的临时放行记录（once/thread/workspace）
- **Audit**：决策与执行结果记录（当前为日志）

## Integration Points

### Axon: 统一拦截点（强推荐）

在工具真正执行前统一做：参数资源提取 → 权限判断 → 必要时审批 → 审计记录。

可接入点：

- `axon/agent/agent.go:527` `(*Agent).executeTool`
- `axon/agent/agent.go:822` `(*Agent).executeToolStream`

理由：

- 已在这里做了 schema 校验（ValidateArguments）
- 这里是唯一能保证“工具不会先跑”的中心化位置
- 与工具实现解耦，避免每个工具各做一套权限逻辑

### AxonCli: 审批 UI 与策略加载

`axoncli` 负责：

- 加载策略文件（用户级 + 项目级 + 默认）
- 在 TUI 展示审批对话框与审计提示
- 为 `axon` 注入 Thread/Workspace 信息（context value）

## Architecture

框架按四层组织：

1) **Context**：Thread/Workspace 等上下文
2) **Policy Engine (AuthZ)**：capability + resource → decision
3) **Approver (HITL)**：decision=require_approval 时的人机交互
4) **Audit**：记录决策与执行结果（当前为 slog 日志）

建议的执行时序：

1. LLM 发起 tool_call（已有）
2. 参数 schema 校验通过（已有）
3. ExtractResources：从 tool input 提取 path/command/url 等
4. PolicyEngine.Evaluate：ALLOW / DENY / REQUIRE_APPROVAL
5. 若 REQUIRE_APPROVAL：Approver.Request → 得到批准结果（allow once / allow thread / allow workspace / deny）
6. 执行 tool.Execute
7. Audit：记录决策、审批、执行结果（成功/失败）

## Data Model

### Capability

建议从“工具名”抽象为“能力”，便于策略稳定与扩展：

- `fs.read` / `fs.write` / `fs.edit`
- `proc.exec`
- `net.fetch` / `net.search`
- `memory.read` / `memory.write`
- `skill.run`

### Resource

统一资源表达用于规则匹配与审计展示：

- File：`path`、`workspace_rel`、`outside_workspace`
- Process：`command`、`cwd`
- Network：`url`（脱敏）、`domain`、`scheme`

### Decision

Decision 需能表达“必须审批”，并携带可解释信息：

- `ALLOW`
- `DENY`
- `REQUIRE_APPROVAL`

Decision 输出建议包含：

- `rule_id`：命中规则
- `reason`：面向用户的简短解释
- `risk_level`：low/medium/high/critical
- `display`：可展示给 TUI 的摘要（已脱敏）

## Detailed Design（可落地详细设计）

本节给出 axon/axoncli 可直接按模块拆分的详细设计，并保证未来其他 agent 框架也能复用同一套 permission 组件（不依赖 axon 的具体 Tool 实现）。

### Package Layout（建议）

- `axon/permission/`：核心抽象与默认实现（与 agent/、tools/ 解耦）
  - `extractor/`：资源抽取与归一（按 tool name）
  - `policy/`：PolicyEngine + YAML loader + matcher
  - `approval/`：阻塞式审批服务（In-process）
  - `grant/`：GrantStore（once/thread/workspace）与 key normalization
  - `hard_deny.go`：强规则（危险命令、敏感路径、URL scheme）
  - `evaluator.go`：统一决策管线（extract → hard deny → grant → policy → approval）

如果暂时不想引入新目录，也可以先放在 `axon/internal/permission`，但建议最终形成可复用的公共包。

### Core Interfaces（Go 形态，与当前实现一致）

**Agent Middleware**（axon 已有）：

```go
type Middleware interface {
	BeforeTool(ctx context.Context, req ToolRequest) error
	AfterTool(ctx context.Context, req ToolRequest, toolErr error) error
}
```

**Permission Evaluator**（permission 包提供的执行前评估器）：

```go
type Evaluator struct{ /* ... */ }

func (e *Evaluator) Evaluate(ctx context.Context, req ToolRequest) error
func (e *Evaluator) LogToolResult(req ToolRequest, toolErr error)
```

**ToolRequest**（permission 侧最小请求体）：

```go
type ToolRequest struct {
	ThreadID   string
	Workspace  string
	ToolCallID string
	ToolName   string
	ToolInput  json.RawMessage
	StartedAt  time.Time
}
```

**Policy Engine**（YAML → Engine）：

```go
doc, err := policy.LoadFiles(paths...)
eng, err := policy.New(doc)
dec := eng.Evaluate(capabilities, resources)
```

**Approver Service**（阻塞式审批服务，供 TUI 订阅并回包）：

```go
type Service interface {
	Subscribe(ctx context.Context) <-chan Request
	Request(ctx context.Context, req Request) (Response, error)
	Grant(req Request, scope grant.Scope) error
	Deny(req Request) error
	Active() (Request, bool)
}
```

### Resource Extraction（资源抽取与归一）

核心目标：把 tool input 里的“真实作用对象”提取出来，用于策略匹配、审批展示、审计脱敏。

当前实现为 `axon/permission/extractor.DefaultExtractor`，按 tool name 解析 input，输出 `[]extractor.Resource`，并做最小化归一：

- `Read` / `Write` / `Edit`：提取 `path`，归一为绝对路径 + workspace 相对路径 + `outside_workspace`
- `Glob` / `Grep`：提取 `path`（为空则使用 workspace）
- `Bash`：提取 `command` + `cwd`（为空则使用 workspace）；同时产出一条 `command` 资源与一条 `path(cwd)` 资源
- `WebFetch`：提取 URL，归一为 `scheme` + `domain`，并脱敏（清空 query/fragment/userinfo）
- `WebSearch`：提取 `allowed_domains` / `blocked_domains` 为 domain 资源（不记录 query）

**资源归一（影响 grant key 与策略命中）**：

- `fs.*`：优先归一为“目录级”或“文件级”，并记录 `workspace_relative_path` + `outside_workspace`
- `proc.exec`：拆解为 `argv[0]`（可执行文件）+ `args`（可选脱敏）+ `cwd`；至少保留 “命令摘要/危险模式命中”
- `net.fetch/search`：归一为 `scheme` + `domain`；query 一律脱敏

### Decision Pipeline（完整决策链路）

当前实现的执行顺序（见 `axon/permission/evaluator.go`）：

1) `ValidateArguments`（axon 已有）  
2) `ExtractResources`  
   - 若抽取失败：直接生成 `EffectRequireApproval`（`rule_id=extract.failed`）并进入审批流程  
3) `HardDeny`（强规则，永不放行）  
4) `GrantStore.Match`（once/thread/workspace）  
5) `PolicyEngine.Evaluate`（规则匹配，输出 allow/deny/require_approval）  
6) 若 require_approval：`Approver.Request` 阻塞等待  
7) 允许后写入 `GrantStore`（按 scope；workspace scope 会落盘）  
8) `Audit`：以 slog 记录 `permission: decision`  

执行完成后 `AfterTool` 当前只做 `LogToolResult`（slog 记录成功/失败），未实现结构化 JSONL/OTel sink。

### Grant Model（一次/会话/持久）

当前实现的 scope（见 `axon/permission/grant/store.go`）：

- `once`：按 `ToolCallID` 一次性放行（命中后会被消费）
- `thread`：按 `ThreadID` + key 放行
- `workspace`：按 `Workspace` + key 放行，并可落盘

Key 由 `(capability, tool_name, resources...)` hash 得到：

- `fs.read`：默认按目录级授权（`filepath.Dir(workspace_rel)` 或绝对目录）
- `fs.write/fs.edit`：按路径授权（workspace 相对路径优先）
- `net.*`：按 `domain` 授权
- `proc.exec`：按命令首 token 授权（`commandSummary`）

workspace scope 的落盘路径为：`~/.axoncli/permission/workspaces/<workspace-hash>.json`（见 `axon/permission/grant/file_store.go`）。

### Concurrency & UX（顺序化审批）

默认 **同一进程同一时刻只弹一个审批**：

- 方便用户理解“当前在批准什么”
- 简化 TUI 状态机
- 避免多个 tool_call 并发时 UI 交互错位

实现上可采用 `requestMu` 保序；如需并发审批，再引入 `queue + request_id`。

### Eventing（axon/axoncli 通用事件协议）

当前实现未引入独立的 permission event 协议；审批请求通过 `approval.Service.Subscribe()` 提供 channel 给 TUI 订阅，TUI 调用 `Grant/Deny` 回包。

### Policy Engine（匹配能力 + 资源）

当前实现的 YAML 结构（见 `axon/permission/policy/policy.go`）：

```yaml
version: 1

defaults:
  mode: require_approval_by_default # 或 deny_by_default / allow_by_default

allow:
  - capability: fs.read

rules:
  - id: allow_workspace_read
    effect: allow
    risk_level: low
    reason: allowed by policy rule
    when:
      capability_in: ["fs.read"]
      resource:
        outside_workspace: false

  - id: require_exec
    effect: require_approval
    when:
      capability_in: ["proc.exec"]
```

匹配维度：

- `when.capability_in`: string[]
- `when.resource`:
  - `outside_workspace`: bool
  - `path_matches`: string[]（glob，支持 `*`/`?`/`**`）
  - `domain_in`: string[]
  - `scheme_in`: string[]
  - `command_matches`: string[]（regexp）

默认行为：当没有任何 allow/deny/require_approval 命中时，按 `defaults.mode` 返回：

- `require_approval_by_default`（默认）：`require_approval`（`rule_id=default.require_approval`）
- `deny_by_default`：`deny`（`rule_id=default.deny`）
- `allow_by_default`：`allow`（`rule_id=default.allow`）

### Approval（axoncli 的实现建议）

当前实现采用 **同进程 TUI**：

- Agent middleware 调用 `permission.Evaluator.Evaluate()`，当 decision=require_approval 时通过 `approval.InProcessService.Request()` 阻塞等待
- TUI 通过 `Subscribe()` 接收请求，弹出 modal，用户选择 scope（once/thread/workspace）或 deny
- TUI 调用 `Grant()` / `Deny()` 回包，解除阻塞

### Tool Mapping（axon 内置工具到 capability/resource）

为确保策略稳定，建议 **不要** 直接按 `ToolName` 写策略（工具名可能变、action 可能扩展），而是把 axon 内置工具映射到 capability + resource：

- `Read` → `fs.read`（resource: `path`）
- `Glob` → `fs.read`（resource: `path` + `pattern` 可选不进匹配）
- `Grep` → `fs.read`（resource: `path` + `query`）
- `Write` → `fs.write`（resource: `path`）
- `Edit` → `fs.edit`（resource: `path`）
- `Bash` → `proc.exec`（resource: `command` + `cwd`）
- `WebSearch` → `net.search`（resource: `domain` 来自 allow/blocked_domains 可选）
- `WebFetch` → `net.fetch`（resource: `url/domain/scheme`）
- `Skill` → `skill.run`（resource: `skill_name` + `skill_source`）
- `Memory` → `memory.read`/`memory.write`（resource: `namespace` 可选）

**Action 字段**：

- 能力足够时优先用 capability（更稳定）
- 若需要进一步细分（如 `edit` 的 create/replace、`web_fetch` 的 GET/POST），再引入 `action`，并允许 `tool:action` allowlist

### Policy Examples（axoncli 可直接使用的例子）

示例：常见“工作区内读/写允许；编辑与命令执行需要审批；工作区外拒绝”：

```yaml
version: 1

rules:
  - id: deny_outside_workspace
    effect: deny
    when:
      capability_in: ["fs.read", "fs.write", "fs.edit"]
      resource:
        outside_workspace: true

  - id: allow_workspace_read_write
    effect: allow
    when:
      capability_in: ["fs.read", "fs.write"]
      resource:
        outside_workspace: false

  - id: require_edit
    effect: require_approval
    when:
      capability_in: ["fs.edit"]

  - id: require_exec
    effect: require_approval
    when:
      capability_in: ["proc.exec"]
```

示例：仅允许 `example.com` 抓取（其余 require_approval 或 deny）：

```yaml
version: 1

rules:
  - id: allow_fetch_example
    effect: allow
    when:
      capability_in: ["net.fetch"]
      resource:
        domain_in: ["example.com"]

  - id: require_approval_fetch_other
    effect: require_approval
    when:
      capability_in: ["net.fetch"]
```

## Policy Engine

### Evaluation Order（从高到低）

当前实现的优先级（见 `axon/permission/evaluator.go`）：

1. Resource extraction failure → `require_approval`（`rule_id=extract.failed`）
2. Hard Deny（永远拒绝）：危险命令 / 敏感路径 / 非 http(s) scheme
3. GrantStore（once/thread/workspace）
4. Policy rules（deny 优先；否则记录 allow/require_approval 候选）
5. Capability allowlist（`allow:`，低优先级）
6. 默认：`require_approval`（`rule_id=default.require_approval`）

## Approval Framework

审批需要满足两点：

1) 能在工具执行前停住（或等价地确保工具不会先执行）  
2) 能把批准结果纳入审计与范围控制（once/thread/workspace）

### Implemented：阻塞式审批握手（In-process）

在 `axon` 的执行拦截点中：

- Policy 返回 `REQUIRE_APPROVAL`
- `approval.Service.Request()` 阻塞等待 TUI 回包
- 用户选择（允许一次/允许 thread/允许 workspace/拒绝）
- 允许后继续执行工具

优点：语义严谨、体验一致、不会发生“先执行后补审批”。

## Default Risk Model（建议默认值）

以当前默认 policy + 默认 hard deny 为基线：

- `fs.read`：默认允许（workspace 内）
- `fs.write`：默认允许（workspace 内；来自默认 policy）
- `fs.edit`：默认需要审批（默认 policy 未放行）
- `proc.exec`：默认需要审批；但 `Bash` 执行 `axoncli ...` 默认允许（默认 policy）
- `net.search` / `net.fetch`：默认需要审批（默认 policy 未放行）

备注：现有 Bash denylist 仍保留为 Hard Deny 的一部分；策略层做更细的 allowlist/require_approval。

## Workspace & Path Policy（强建议优先落地）

工具实现已具备路径限制能力（`restrict`/workspace 校验），但需要由框架与策略统一控制：

- 默认禁止 workspace 外的读写执行
- 支持策略放行特定目录（如 monorepo 上层、cache、临时目录）
- 审批时展示“将访问的绝对路径/相对路径/是否越界”

## Web Fetch Policy（域名与协议约束）

对 `web_fetch` 的建议约束：

- 默认仅允许 `http/https`（hard deny 非 http(s) scheme）
- 可选 domain allowlist / blocklist
- 记录 `domain` 而非完整 URL query（避免泄露 token）
- 对可能下载二进制/大文件的场景提升风险等级（require_approval）

## Audit & Observability

### Audit Event

当前实现的审计为 slog 日志：

- decision：`permission: decision`（包含 tool_call_id/tool/thread_id/workspace/effect/rule_id/risk/reason）
- execution：`permission: tool execution ok|error`

### Redaction（脱敏）

绝不记录明文 secret：

- Authorization / Cookie / API keys / token / password
- 任何可能包含密钥的 URL query

审计中仅保留必要信息（如域名、路径、命令模式、env key 列表）。

## Implementation Phases（建议路线）

当前代码已包含（MVP）：

1. Policy Engine（YAML + matcher）
2. Resource extractor（覆盖 axoncli 常用工具）
3. Hard deny（命令/敏感路径/scheme）
4. Blocking approval（In-process TUI）
5. Grants（once/thread/workspace；workspace 可落盘）

后续可选增强（未实现）：

- 结构化审计 sink（JSONL/OTel）
- 更丰富的策略语法（any/all、优先级、action 细分）
- 远程审批（HTTP/WebSocket）

## Appendix: Suggested Policy File Locations

当前实现的 policy 搜索路径（见 `axon/permission/policy/policy.go` 与 `cmd/axoncli/conf/policy.go`）：

- 项目级：`$WORKSPACE/.agent/policy.yml`
- 用户级：`~/.axoncli/policy.yml`（不存在时会自动创建默认文件）

## Appendix: Minimal Adoption Guide（落地步骤）

按最小改动接入 axon（不要求一次性做完所有模块）：

1) 在 `tool.Execute()` 前调用 `mw.BeforeTool()`，内部委托给 `permission.Evaluator.Evaluate()`  
2) `Evaluate()` 返回 error 则阻止执行（error 可区分 blocked/denied）  
3) tool 执行后调用 `mw.AfterTool()`，当前只记录执行结果日志  
4) `axoncli` 提供 `approval.InProcessService` + TUI modal + grant store 持久化 + policy 文件加载/创建

## Appendix: AxonCli Wiring（axoncli 接入示例）

axoncli 目前在 `cmd/axoncli/main.go` 注册工具，且所有工具都以 `restrict=false` 创建（即工具自身并不限制 workspace 越界）。本方案建议将 “是否允许越界/是否需要审批” 收敛到统一 permission layer，工具层继续保留最小安全校验（如 Bash denylist、web_fetch 协议校验）。

推荐的 wiring：

- `axoncli` 启动时 `LoadOrCreatePolicy(configDir, workspaceDir)`，优先使用 `$WORKSPACE/.agent/policy.yml`，否则落到 `~/.axoncli/policy.yml`（不存在时创建默认）
- 构建 `permission.Evaluator` 并用 `agent.WithMiddlewares(permMiddleware)` 注入
- `Approver` 使用 `approval.NewInProcessService()`，TUI 通过 `Subscribe()` 展示审批弹窗并调用 `Grant/Deny` 回包
- `GrantStore` 使用 `grant.NewMemoryStore(grant.FileStore{BaseDir: "~/.axoncli/permission"})`，workspace scope 自动落盘

## Appendix: Reuse by Other Agent Frameworks（供后续框架复用）

为让后续其他 agent 框架（例如不同 provider、不同 tool runtime、甚至 server-side agent）复用本方案，建议遵守以下边界：

- **不依赖 Tool 实现细节**：permission 只看 `ToolRequest{ToolName, ToolInput}` 与 extractor 抽取出的 `Resource`；执行器只需要在“执行前/执行后”调用 middleware
- **不依赖 UI**：Approver 通过 `approval.Service` 抽象；当前实现为 in-process channel，可扩展为 HTTP/Remote
- **策略与审计独立**：PolicyEngine 与 GrantStore 独立于 axoncli；审计当前为 slog，可替换为结构化 sink
