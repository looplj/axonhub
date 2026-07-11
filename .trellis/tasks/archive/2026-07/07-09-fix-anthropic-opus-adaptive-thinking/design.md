# Design — Anthropic Thinking Capability Policy

## Problem Boundary

这是协议转换问题，但 Anthropic `thinking` wire shape 的合法性还取决于**目标模型能力**。因此模型名不能散落在协议转换分支中，也不能把 `reasoning.effort` 直接等同于 `budget_tokens`。

Anthropic 官方资料证明：manual `thinking:{type:"enabled",budget_tokens:N}` 与 adaptive `thinking:{type:"adaptive"}` + `output_config.effort` 并不是对所有目标模型通用的可互换 shape。

## Capability Policy Boundary

```text
OpenAI/LLM reasoning intent
  -> Anthropic thinking capability policy
  -> Anthropic outbound wire shape
```

定义集中 capability enum：

```text
adaptive_only
adaptive_preferred
manual_supported
unknown
```

要求：

1. `buildBaseRequest` 不得直接对具体模型名做 `if strings.Contains(...)` 分支。
2. 模型 family / alias 匹配只能存在于集中 capability resolver 中；它是 provider data policy，不是协议字段转换逻辑。
3. `claude-opus-4-8` 仅作为：
   - official capability policy 数据项；
   - #23429/#23430 的 regression fixture。
4. capability resolver 必须能由 `anthropic.Config` 覆盖，便于 channel/provider 对非官方 Anthropic-compatible 上游声明实际能力；默认 resolver 使用官方 Anthropic policy。
5. unknown capability 不能自动生成 manual `budget_tokens`。当 source 只有 OpenAI/LLM `ReasoningEffort` 时，必须走明确的 unsupported/lossy policy，而不是猜测。

## Wire Decision Table

| Source intent | Target capability | Output |
|---|---|---|
| `none` | any capability that supports explicit disabled | `thinking:{type:"disabled"}`；无 `output_config.effort`。 |
| effort (`minimal/low/medium/high/xhigh`) | `adaptive_only` / `adaptive_preferred` | `thinking:{type:"adaptive"}` + legal `output_config.effort`。 |
| effort | `manual_supported` | `thinking:{type:"enabled",budget_tokens:N}`，但仅当 `N >= 1024 && N < max_tokens`。 |
| effort | `unknown` | 不生成 guessed manual/adaptive thinking；记录 explicit loss/unsupported。 |
| explicit native Anthropic thinking metadata | same Anthropic family | native same-protocol replay优先。 |

## Effort Mapping

Anthropic public effort values are `low`, `medium`, `high`, `xhigh`, `max`。OpenAI 有额外 `minimal`，因此 mapping 是受控降级：

```text
none    -> disabled
minimal -> low
low     -> low
medium  -> medium
high    -> max       # 本地明确要求
xhigh   -> max       # 本地明确要求
max     -> max       # 内部/Anthropic native round-trip
unknown -> unsupported/lossy, not max
```

## Manual Budget Safety

Manual thinking 仅在 `manual_supported` policy 下生成。官方约束：

```text
budget_tokens >= 1024
budget_tokens < max_tokens
```

如果 `max_tokens <= 1024`，不存在合法 manual-thinking budget；必须返回明确 transformer validation error，不能生成零、负数或省略的 `budget_tokens`。

## DeepSeek Boundary

DeepSeek Anthropic-format 是独立 adapter platform policy：可使用它自己的 `output_config.effort` 语义，但不能共享 Anthropic Claude capability mapping，也不发送 Anthropic `thinking.type="adaptive"`。
