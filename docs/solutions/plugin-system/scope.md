# 作用范围

[插件系统](README.md)的可选能力：Owner 把某个插件的请求钩子收窄到指定的 project、API Key 或模型。不匹配的请求不借 VM、不序列化请求。

本篇不在 [任务表](README.md#9-任务管理) 里。当前交付中，请求钩子对全部请求生效，插件在 `ctx.request` 里自行判断，见 [request.md](request.md#4-钩子)。下面的设计是排期时的实现合同。排期后加入任务表，依赖 T-03 和 T-13。

## 1. 背景

当前交付中，声明了请求钩子的插件对每个请求都会执行。插件可以读 `ctx.request.projectId`、`apiKeyId`、`model`，然后提前返回 `undefined`，但这时宿主已经借出 VM，并把请求序列化给了 JS。100 KiB 的请求体，单次 `onRequest` 的预算是 5 ms（[request.md](request.md#7-验收)）。一个只服务一个 project 的插件，也会让所有 project 的请求都付出这部分开销。

当前交付允许插件自行判断是否处理请求；宿主侧收窄用于省掉不匹配请求的执行开销，作为可选能力单独排期。

三个维度在钩子中间件的 `OnInboundLlmRequest` 里都能拿到，宿主在这里统一匹配，`Process` 不用改：

- project：`contexts.GetProjectID(ctx)`。`middleware/auth.go` 用 API Key 鉴权后写入；playground 在 `api/playground.go` 261 行写入。
- API Key：`contexts.GetAPIKey(ctx)`，与 `Process` 221 行的取法相同。playground 请求没有 API Key。
- 模型：`llmRequest.Model`。与 `checkApiKeyModelAccess`（`orchestrator/model_access.go`）的取法相同，各入口的 inbound transformer 已经填好，Gemini 也从路径解析进来。

## 2. 目标

1. Owner 能给插件设置 `match`：`projectIDs`、`apiKeyIDs`、`models`。不设置就是全部请求。
2. 不匹配的插件由宿主跳过：不借 VM、不序列化请求、不调用 JS。
3. 同一请求的 `onRequest`、`onAttempt` 使用同一个匹配结果；开放 [响应钩子](response.md) 后，响应钩子也沿用。

### 非目标

- 收窄事件。事件订阅者在 `onEvent` 里按 `event.subject` 过滤。
- 按渠道收窄 `onAttempt`。插件读 `ctx.attempt.channel` 自行判断。
- project 管理员自己启用或配置插件。插件仍只由 Owner 管理。
- 按用户、按 API Key 的 profile 或按 tag 匹配。

## 3. User Stories

1. 作为系统 Owner，我希望把一个改写插件只开给某个 project，以便其他 project 的请求既不受影响，也不多花时间。
2. 作为系统 Owner，我希望插件只处理 `claude-*` 模型的请求，以便其他模型的请求不进入 JS。
3. 作为插件作者，我希望不用在每个钩子开头重复写过滤代码，也能在需要时自己判断。

## 4. 决策

| 方案 | 结论 |
|------|------|
| 宿主在钩子中间件的 `OnInboundLlmRequest` 里统一匹配三个维度，借 VM 之前完成 | 采用。和 `checkApiKeyModelAccess` 同一处取 model，`Process` 和中间件组装都不用改 |
| 插件在 JS 里读 `ctx.request` 自己过滤 | 作为当前交付的做法保留，但不能省掉借 VM 和序列化 |
| 用 `gjson` 取 body 的 `model` | 放弃。Gemini 的模型在路径里，并且可能和 inbound transformer 的解析结果不一致 |
| 每个插件按前一个插件改写后的 model 判断 | 放弃。匹配结果会取决于插件顺序。统一用进入钩子中间件时的 model，即客户端请求的 model |
| 自己实现一套 glob | 放弃。复用 `internal/pkg/xregexp.MatchString`，与模型映射的写法一致 |

## 5. 设计

### 字段

`plugins` 增加可空 JSON 列 `match`，`internal/objects/plugin.go` 增加 `PluginMatch`。这个 JSON 落在 `internal/objects`，按该包的惯例用 camelCase：

```go
type PluginMatch struct {
    ProjectIDs []int    `json:"projectIDs,omitempty"`
    APIKeyIDs  []int    `json:"apiKeyIDs,omitempty"`
    Models     []string `json:"models,omitempty"`
}
```

语义：

- 维度之间是 AND，维度内部是 OR。空列表表示该维度不限。`match` 为 null 或三个列表都为空时，等同于当前交付的全局行为。
- `models` 的每一项按 `xregexp.MatchString` 匹配：不含正则字符时精确匹配，`*` 匹配全部，其余按带锚点的正则匹配。注意 `.` 是正则字符，`gpt-4.1` 也会被当作正则，这与模型映射的现有行为一致。
- 请求没有 API Key（playground）时，`apiKeyIDs` 非空就视为不匹配。
- `match` 只影响请求钩子，不影响事件订阅、UI 和宿主 API。

保存校验（`updatePluginSettings`）：

- `projectIDs`、`apiKeyIDs` 引用的行必须存在。
- 同时设置了两者时，每个 API Key 所属的 project 必须在 `projectIDs` 里，否则这个 API Key 永远匹配不上，直接报错。
- `models` 的每项先过 `xregexp.ValidateRegex`。
- 数量限制见第 7 节。

project 或 API Key 后来被删除时不级联。失效的 id 永远匹配不上，管理页标出失效引用。如果某个维度只剩失效 id，插件实际上对任何请求都不生效，管理页用警告显示。

### 匹配流程

```text
1. Process 组装中间件：与当前交付相同，有声明请求钩子的已启用插件时追加钩子中间件
2. 钩子中间件 OnInboundLlmRequest(req)，宿主匹配：
   - project = contexts.GetProjectID(ctx)，apiKey = contexts.GetAPIKey(ctx)，model = req.Model
   - 按 projectIDs、apiKeyIDs、models 过滤 Generation 里的钩子插件，结果存进请求状态
   - 列表为空：直接返回 req，不借 VM
   - 否则按 priority 调用剩下插件的 onRequest
3. onAttempt（以及开放后的响应钩子）只调用第 2 步留下的插件，不重新匹配
```

第 2 步发生在 `enforceQuota`、`checkApiKeyModelAccess`、`applyModelMapping` 之前，用的是客户端请求的 model，不是映射后的 model。某个插件在 `onRequest` 里改了 model，不会让后面的插件重新匹配。

Generation 构建时把 `projectIDs`、`apiKeyIDs` 转成集合，并预编译 `models`。改 `match` 走同一次 `updated_at` 刷新，见 [安装与同步](README.md#安装与同步)。

### 改动清单

#### `internal/ent/schema/plugin.go`
- 增加 `field.JSON("match", &objects.PluginMatch{}).Optional()`，执行 `make generate`。

#### `internal/objects/plugin.go`
- `PluginMatch` 及 `Validate()`：数量上限、`xregexp.ValidateRegex`。

#### `internal/server/plugin/hooks.go`
- `OnInboundLlmRequest` 里取 project、API Key、model 并匹配，匹配通过的插件才借 VM。`Middleware(ctx)` 和 `RequestHookProvider` 接口不变，`Process` 不改。
- 请求状态里保存过滤后的插件列表，后续钩子复用。

#### `internal/server/plugin/manager.go`
- Generation 中预计算匹配集合和编译后的模型模式。

#### `internal/server/gql/plugin.graphql`
- `PluginMatch`、`PluginMatchInput`。`updatePluginSettings` 增加 `match` 参数。`Plugin.matchWarnings` 返回失效引用。
- 手写 resolver：保存校验用 Owner 上下文查 project 和 API Key 是否存在，并核对 API Key 所属的 project。

#### 前端
- `features/plugins/components/plugin-match-form.tsx`：project 多选、API Key 多选（按已选 project 过滤）、模型模式列表。只对声明了请求钩子的插件显示。
- `features/plugins/data/plugins.ts`：query 和 mutation 增加 `match`。
- `locales/en/plugins.json`、`locales/zh-CN/plugins.json`：表单文案、失效引用警告。

### Migration

```text
1. plugin.go 增加可空 JSON 列 match，执行 make generate
2. 无回填。null 表示全部请求，与之前的行为一致
3. make migration-test-all-dbs TAG=<排期时最新的发布版本>
```

回滚：旧二进制的 Ent 查询只选自己知道的列，多出的 `match` 列会被忽略，插件回到全局生效。

## 6. 验收

| 类别 | 标准 | 验证方式 |
|------|------|----------|
| 功能 | `projectIDs=[A]` 时，project B 的请求不借 VM、不调用 JS；project A 的请求调用 | `go test ./internal/server/plugin/ -run TestHookMatch`，统计 VM 借出次数 |
| 功能 | `models=["claude-.*"]` 时，`gpt-5` 请求不借 VM、不调用 JS；`claude-sonnet-4` 请求调用 | 同上 |
| 功能 | Gemini 入口按路径里的模型匹配 | orchestrator 集成测试，Gemini 入口加 httptest 上游 |
| 功能 | 第一个插件在 `onRequest` 里把 model 改成不匹配第二个插件的值，第二个插件仍然执行 | 同上 |
| 功能 | 同一请求的 `onAttempt` 只调用 `onRequest` 阶段留下的插件，重试和切换渠道后也一样 | 同上，流式和非流式各一组 |
| 错误处理 | API Key 不属于已选 project、模式不是合法正则、超过数量上限：保存失败，可读错误 | resolver 单测 |
| 兼容 | `match` 为 null 的插件行为与当前交付完全一致 | 复用 request.md 的兼容测试 |
| 迁移 | 三种数据库升级成功 | `make migration-test-all-dbs` |
| i18n | `en` 与 `zh-CN` 的 key 一致 | 前端既有 i18n key 校验 |

## 7. 限制

排期时并入 [运行限制](README.md#运行限制)，在此之前以本表为准。

| 项 | 限制 |
|----|------|
| `projectIDs` | ≤ 100 |
| `apiKeyIDs` | ≤ 500 |
| `models` | ≤ 50 项，每项 ≤ 128 字符 |

## 8. 风险与妥协

| 风险 | 触发条件 | 影响 | 对策 |
|------|----------|------|------|
| 删除 project 或 API Key 后插件静默失效 | `match` 只剩失效 id | 插件不再处理任何请求 | 管理页显示 `matchWarnings` |
| 模型正则写错范围 | `gpt-4.1` 被当成正则，或忘了锚点之外的匹配规则 | 匹配到预期之外的模型 | 表单里提供"测试模型名"预览，用同一个 `xregexp.MatchString` |

```text
1. 妥协：按客户端请求的 model 匹配，不按映射后的 model
   - 原因：钩子在模型映射之前执行，onRequest 改写 model 后再匹配会让结果取决于插件顺序
   - 影响：想按上游实际模型处理的插件，要在 onAttempt 里读 ctx.attempt 自行判断
   - 升级触发：用户需要按实际模型收窄 onAttempt
   - 后续方向：增加 attemptModels，只在 onAttempt 前按实际模型过滤

2. 妥协：不收窄事件
   - 原因：事件是异步投递，过滤成本低，而且 subject 里已有 project、API Key、model
   - 影响：订阅 request.* 的插件仍会收到全部请求事件
   - 升级触发：request.* 事件的队列丢弃明显增多
   - 后续方向：订阅时由宿主按同一个 `match` 过滤，在入队前丢弃
```
