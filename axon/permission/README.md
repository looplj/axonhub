# Permission

The `permission` package implements a multi-layered permission evaluation system for controlling agent tool execution. It determines whether a tool call should be **allowed**, **denied**, or **require human approval** based on the resources the tool intends to access.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Evaluator                            │
│                                                         │
│  ToolRequest ──► Extractor ──► HardDeny ──► Grant Store │
│                                    │             │      │
│                                    │        (no match)  │
│                                    ▼             ▼      │
│                                  deny      Policy Engine│
│                                              │   │   │  │
│                                           allow deny req│
│                                                      │  │
│                                               Approval  │
│                                               Service   │
└─────────────────────────────────────────────────────────┘
```

The evaluation pipeline processes each tool call through four stages in order:

1. **Extractor** — Parses tool input and extracts resources (paths, directories, commands, URLs, domains, skills)
2. **HardDeny** — Rejects dangerous operations unconditionally (sensitive paths, destructive commands, non-HTTP schemes)
3. **Grant Store** — Checks if a previous approval already covers this operation (resource-subset matching + hierarchical directory matching)
4. **Policy Engine** — Evaluates configurable YAML rules to produce a decision

If the policy engine returns `require_approval`, the **Approval Service** is invoked to request human authorization.

## Package Structure

```
permission/
├── approval/          # Human approval request/response handling
│   ├── service.go     # Service interface
│   ├── local.go       # InProcessService (in-memory pub/sub)
│   └── remote.go      # RemoteApprover (via AxonHub GraphQL API)
├── extractor/         # Resource extraction from tool inputs
│   └── extractor.go   # DefaultExtractor implementation
├── grant/             # Approval memory (remembers past grants)
│   ├── store.go       # Store interface + MemoryStore
│   └── file_store.go  # FileStore for persistent workspace/global grants
├── policy/            # Rule-based policy engine
│   ├── policy.go      # Policy document schema + YAML loading
│   └── engine.go      # Rule evaluation engine
├── evaluator.go       # Top-level Evaluator orchestrator
├── types.go           # Shared types (Effect, RiskLevel, Resource, ToolDecision)
├── hard_deny.go       # Hard-coded deny rules (defense-in-depth)
├── context.go         # Context helpers for workspace propagation
└── errors.go          # Sentinel errors (ErrToolCallBlocked, ErrToolCallDenied)
```

## Core Types

### Effect

Every evaluation produces one of three effects:

| Effect              | Meaning                                             |
|---------------------|-----------------------------------------------------|
| `allow`             | Tool call proceeds without interruption              |
| `deny`              | Tool call is blocked; an error is returned to the agent |
| `require_approval`  | Execution pauses until a human grants or denies      |

### RiskLevel

A UI/audit hint attached to each decision: `low`, `medium`, `high`, `critical`.

### ResourceType

Resources extracted from tool inputs are classified into six types:

| Type      | Fields Used                                         | Extracted From        |
|-----------|-----------------------------------------------------|-----------------------|
| `path`    | `Path`, `WorkspaceRel`, `OutsideWorkspace`          | Read, Write, Edit     |
| `dir`     | `Path`, `WorkspaceRel`, `OutsideWorkspace`          | Read, Write, Edit (parent dir), Glob, Grep, Bash (cwd) |
| `command` | `Command`, `Cwd`                                    | Bash                  |
| `url`     | `URL`, `Domain`, `Scheme`                           | WebFetch              |
| `domain`  | `Domain`                                            | WebFetch, WebSearch   |
| `skill`   | `Skill`                                             | Skill                 |

**Note:** Read, Write, and Edit produce **both** a `path` resource (the specific file) and a `dir` resource (the parent directory). The `path` resource enables fine-grained policy matching (e.g. `path_matches: ["**/*.secret"]`) and hard deny checks on sensitive file paths. The `dir` resource enables hierarchical grant matching so that granting access to a parent directory covers child paths.

### ToolRequest

The input to `Evaluator.Evaluate()`:

```go
type ToolRequest struct {
    ThreadID   string
    Workspace  string            // absolute path of the agent workspace
    ToolCallID string
    ToolName   string            // e.g. "Read", "Bash", "WebFetch"
    ToolInput  json.RawMessage   // raw JSON arguments from the LLM
    StartedAt  time.Time
}
```

### ToolDecision

The output of the evaluation pipeline:

```go
type ToolDecision struct {
    Effect    Effect
    RuleID    string            // which rule matched (e.g. "hard_deny.bash", "allow.tool")
    Reason    string            // human-readable explanation
    RiskLevel RiskLevel
    Display   DecisionDisplay   // summary + details for UI
    Resources []Resource        // extracted resources
}
```

## Components

### Extractor (`extractor/`)

The `Extractor` interface parses raw tool input JSON and returns a list of `Resource` structs. The `DefaultExtractor` handles all built-in tools:

| Tool       | Resources Produced                                                    |
|------------|-----------------------------------------------------------------------|
| Read/Write/Edit | **path** resource for the file + **dir** resource for the parent directory |
| Glob/Grep  | **dir** resource for the search path; defaults to workspace root if empty |
| Bash       | **command** resource + **dir** resource for the cwd                   |
| WebFetch   | **url** resource (redacted) + **domain** resource                     |
| WebSearch  | **domain** resources for `allowed_domains` and `blocked_domains`      |
| Skill      | **skill** resource (validated: lowercase, 1-64 chars, alphanumeric + hyphens) |

Unknown tool names return `nil` resources with no error.

Path and directory resources include:
- **`Path`** — Cleaned absolute path
- **`WorkspaceRel`** — Relative to workspace (empty if outside)
- **`OutsideWorkspace`** — `true` if the path/directory is not under the workspace root

### HardDeny (`hard_deny.go`)

Defense-in-depth layer that **cannot be overridden** by policy rules or grants. It blocks:

**Dangerous commands** (Bash tool):
- `rm -rf`, `rm -f /...`
- `mkfs`, `wipefs`
- `shutdown`, `reboot`
- `sudo`
- Reverse shell patterns (`nc -e`, `/dev/tcp/`)

**Sensitive paths** (any file tool via `path` resources):
- `/etc/`, `/system/`, `/private/etc/`, `/private/var/db/`, `/var/db/`
- `/.ssh/`, `/.gnupg/`

**Non-HTTP URL schemes** (WebFetch):
- Any scheme other than `http` or `https`

### Policy Engine (`policy/`)

The policy engine evaluates a list of YAML-defined rules against extracted resources.

#### Policy Document Schema

```yaml
version: 1

defaults:
  # "require_approval_by_default" (default) | "deny_by_default" | "allow_by_default"
  mode: require_approval_by_default

allow:
  # Low-precedence tool allowlist (no resource constraints)
  - tool: Read
  - tool: Glob

rules:
  - id: unique-rule-id
    effect: allow | deny | require_approval
    risk_level: low | medium | high | critical  # optional, defaults vary by effect
    reason: "human-readable explanation"         # optional
    when:
      tool_in:                                   # optional; matches if empty
        - Read
        - Write
      resource:                                  # optional; all conditions are AND-ed
        outside_workspace: true                  # match paths/dirs outside workspace
        path_matches:                            # glob patterns — matches both path and dir resources
          - "**/*.secret"
          - "/home/user/data/**"
        dir_matches:                             # glob patterns — matches dir resources only
          - ".git/**"
          - "node_modules/**"
        domain_in:                               # exact domain match
          - "example.com"
        scheme_in:                               # URL scheme match
          - "https"
        command_matches:                         # regex patterns
          - "^npm\\s+.*"
        skill_in:                                # exact skill name match (case-insensitive)
          - "deploy"
```

**`path_matches` vs `dir_matches`:**
- `path_matches` matches against both `path` and `dir` resource types. Use it for general file/directory matching (e.g. deny all access under `.secret/`).
- `dir_matches` matches against `dir` resource types only. Use it when you need to constrain rules to directory-level operations specifically.

Both use the same glob syntax and match against `WorkspaceRel` when available, otherwise the absolute `Path`.

#### Evaluation Order

1. Rules are evaluated **top-to-bottom** in the merged document
2. A **deny** rule short-circuits immediately — the tool call is blocked
3. **allow** and **require_approval** matches are recorded; later rules override earlier ones
4. After all rules, the **tool allowlist** (`allow:`) is checked (lowest precedence)
5. If nothing matches, the **defaults.mode** determines the outcome

#### Path Glob Syntax

| Pattern | Meaning                            |
|---------|------------------------------------|
| `*`     | Any characters except `/`          |
| `?`     | Any single character except `/`    |
| `**`    | Any characters including `/`       |

Patterns match against `WorkspaceRel` (for workspace-internal paths) or the absolute `Path` (for external paths).

#### Policy File Locations

Loaded via `policy.DefaultPaths(configDir, workspace)`:

1. `<workspace>/.agent/policy.yml` — workspace-level policy
2. `<configDir>/policy.yml` — user/global-level policy

Multiple files are merged; later files override `defaults.mode`, and rules/allow entries are appended.

### Grant Store (`grant/`)

The grant store remembers previously approved operations so the user is not prompted repeatedly. It supports **hierarchical directory matching**: granting access to a parent directory automatically covers all descendant directories.
It also supports **resource-subset matching**: if the stored grant was recorded for a subset of resources, it can still match future requests that include additional resources.

#### Scopes

| Scope       | Key Derivation            | Persistence | Behavior                                    |
|-------------|---------------------------|-------------|----------------------------------------------|
| `once`      | `ToolCallID`              | Memory only | Consumed on first match (one-time pass)      |
| `thread`    | `(ThreadID, key)`         | Memory only | Valid for the duration of the thread          |
| `workspace` | `(Workspace, key)`        | File-backed | Persists across restarts for the same workspace |
| `global`    | `key`                     | File-backed | Applies to all workspaces                    |

#### Key Construction

The grant key is a SHA-256 hash derived from:

- **Tool name** (lowercased)
- **Directory tokens** (order-insensitive, de-duplicated):
  - `ResourcePath` contributes the **parent directory** as `dir:<workspace-rel-dir>` or `dir_abs:<abs-dir>`
  - `ResourceDir` contributes the **directory itself** as `dir:<workspace-rel-dir>` or `dir_abs:<abs-dir>`
- **Domain resources**: lowercased domain
- **Command resources**: program + optional subcommand token(s)
  - always includes the program name (first word), e.g. `cmd:go`
  - if a subcommand exists (first non-flag argument), also includes `cmd:"go test"`
- **Skill resources**: lowercased skill name

Because tokens are sorted and de-duplicated, the same logical resource set produces the same key regardless of extraction order.

#### Hierarchical Directory Matching

When checking whether a grant covers a request, the store derives a set of candidate keys for the request by:
1. Generating keys for **all non-empty subsets** of the request’s resource tokens (so a previously recorded subset-grant can match), then
2. For any `dir:*` / `dir_abs:*` token, generating keys for all **ancestor directories** up to the root (so parent grants can cover descendants).

This means:

- Granting `Read` on directory `src` covers `Read` on `src/pkg`, `src/pkg/util`, etc.
- Granting `Read` on workspace root (`.`) covers all directories in the workspace.
- A child grant does **not** cover its parent — granting `src/pkg` does not grant `src`.
- Sibling directories are independent — granting `src/pkg` does not grant `src/cmd`.
- Hierarchical matching is scoped per tool — granting `Read` on `src` does not grant `Write` on `src`.

Example walk for a request targeting `src/pkg/util`:

```
1. Check exact key: (Read, dir:src/pkg/util)    → no match
2. Check parent:    (Read, dir:src/pkg)          → no match
3. Check parent:    (Read, dir:src)              → MATCH → allowed
4. (Would check):   (Read, dir:.)               → (not reached)
```

Non-directory resources (domains, commands, skills) do not have a hierarchy, but they still participate in subset matching (e.g. a stored grant for `domain:example.com` can match a request that also includes additional tokens for the same tool).

#### File Storage

The `FileStore` persists grants as JSON files:

```
<baseDir>/
├── workspaces/
│   └── <sha256-hash>.json    # per-workspace grants
└── global.json                # global grants
```

File format:
```json
{
  "version": 1,
  "updated_at": "2024-01-01T00:00:00Z",
  "keys": ["<sha256-hash-1>", "<sha256-hash-2>"]
}
```

Grant files are written atomically (write to `.tmp`, then rename).

### Approval Service (`approval/`)

When the policy engine returns `require_approval`, the evaluator delegates to an `approval.Service` to obtain human authorization.

#### Interface

```go
type Service interface {
    Subscribe(ctx context.Context) <-chan Request
    Request(ctx context.Context, req Request) (Response, error)
    Grant(req Request, scope grant.Scope) error
    Deny(req Request) error
    Active() (Request, bool)
}
```

#### Implementations

| Implementation      | Use Case             | Mechanism                                                  |
|---------------------|----------------------|------------------------------------------------------------|
| `InProcessService`  | Local / embedded UI  | In-memory pub/sub with subscriber channels                 |
| `remoteApprover`    | AxonClaw (headless)  | Sends approval request via AxonHub GraphQL API; polls for result |

The **remote approver** flow:
1. Sends an `approval_request` message to AxonHub via `ReplyMessage`
2. Polls `PullAgentMessages` for an `approval_result` response
3. The request expires after 2 minutes
4. Once resolved, the message is acknowledged via `AckAgentMessages`

## Evaluator (`evaluator.go`)

The `Evaluator` orchestrates the full pipeline:

```go
evaluator := permission.NewEvaluator(permission.EvaluatorOptions{
    Logger:    logger,
    Extractor: extractor.DefaultExtractor{},  // optional, defaults to DefaultExtractor
    Policy:    policyEngine,
    Approver:  approvalService,
    Grants:    grantStore,
})

err := evaluator.Evaluate(ctx, toolRequest)
// err == nil                     → allowed
// errors.Is(err, ErrToolCallBlocked) → hard denied
// errors.Is(err, ErrToolCallDenied)  → denied by policy or user
```

When an approval is granted, the evaluator automatically records it in the grant store. If the scope is `workspace`, it also persists the grant to disk.

## Usage in AxonClaw

AxonClaw initializes the permission system in `cmd/axonclaw/main.go`:

```go
// 1. Create file-backed grant store
grantsStore := grant.NewMemoryStore(
    grant.NewFileStore(filepath.Join(wd, ".axonclaw", "permission")),
)
grantsStore.LoadWorkspace(wd)

// 2. Load policy from .axonclaw/policy.yml (creates default if missing)
pdoc, _ := conf.LoadOrCreatePolicy(wd)
eng, _ := policy.New(pdoc)

// 3. Create remote approver (polls AxonHub for approval decisions)
remoteApprover := approval.NewRemoteApprover(logger, gqlClient, cfg.PollInterval)

// 4. Build evaluator
permEvaluator := permission.NewEvaluator(permission.EvaluatorOptions{
    Logger:   logger,
    Policy:   eng,
    Approver: remoteApprover,
    Grants:   grantsStore,
})

// 5. Wire as agent middleware
middleware := runner.NewPermissionMiddleware(permEvaluator)
```

The middleware calls `Evaluate()` in `BeforeTool` and `LogToolResult` in `AfterTool`.

## Errors

| Error               | Meaning                                                     |
|---------------------|-------------------------------------------------------------|
| `ErrToolCallBlocked` | Hard deny triggered — the operation is fundamentally unsafe  |
| `ErrToolCallDenied`  | Denied by policy rule or rejected by human during approval   |
