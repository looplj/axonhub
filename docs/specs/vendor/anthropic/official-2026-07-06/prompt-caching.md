[Claude Platform Docs](/docs/en/home)

Prompt caching

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Messages/Context management

# Prompt caching

Prompt caching optimizes your API usage by allowing resuming from specific prefixes in your prompts. This significantly reduces processing time and costs for repetitive tasks or prompts with consistent elements.



This feature is eligible for [Zero Data Retention (ZDR)](/docs/en/build-with-claude/api-and-data-retention). When your organization has a ZDR arrangement, data sent through this feature is not stored after the API response is returned.

There are two ways to enable prompt caching:

* **[Automatic caching](#automatic-caching)**: Add a single `cache_control` field at the top level of your request. The system automatically applies the cache breakpoint to the last cacheable block and moves it forward as conversations grow. Best for multi-turn conversations where the growing message history should be cached automatically.
* **[Explicit cache breakpoints](#explicit-cache-breakpoints)**: Place `cache_control` directly on individual content blocks for fine-grained control over exactly what gets cached.

The simplest way to start is with automatic caching:

```
client = anthropic.Anthropic() client = anthropic.Anthropic() response = client.messages.create(response = client.messages.create( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=1024,  max_tokens = 1024, cache_control={"type": "ephemeral"},  cache_control ={"type": "ephemeral"}, system="You are an AI assistant tasked with analyzing literary works. Your goal is to provide insightful commentary on themes, characters, and writing style.",  system ="You are an AI assistant tasked with analyzing literary works. Your goal is to provide insightful commentary on themes, characters, and writing style.", messages=[ messages =[ { { "role": "user",  "role": "user", "content": "Analyze the major themes in 'Pride and Prejudice'.",  "content": "Analyze the major themes in 'Pride and Prejudice'.", } } ], ],))print(response.usage.model_dump_json()) print(response.usage.model_dump_json())
```

With automatic caching, the system caches all content up to and including the last cacheable block. On subsequent requests with the same prefix, cached content is reused automatically.

---

## How prompt caching works

When you send a request with prompt caching enabled:

1. The system checks if a prompt prefix, up to a specified cache breakpoint, is already cached from a recent query.
2. If found, it uses the cached version, reducing processing time and costs.
3. Otherwise, it processes the full prompt and caches the prefix once the response begins.

This is especially useful for:

* Prompts with many examples
* Large amounts of context or background information
* Repetitive tasks with consistent instructions
* Long multi-turn conversations

By default, the cache has a 5-minute lifetime. The cache is refreshed for no additional cost each time the cached content is used.



If you find that 5 minutes is too short, Anthropic also offers a 1-hour cache duration [at additional cost](#pricing).

For more information, see [1-hour cache duration](#1-hour-cache-duration).



**Prompt caching caches the full prefix**

Prompt caching references the entire prompt - `tools`, `system`, and `messages` (in that order) up to and including the block designated with `cache_control`.

---

## Pricing

Prompt caching introduces a new pricing structure. The table below shows the price per million tokens for each supported model:

| Model | Base Input Tokens | 5m Cache Writes | 1h Cache Writes | Cache Hits & Refreshes | Output Tokens |
| --- | --- | --- | --- | --- | --- |
| Claude Fable 5 | $10 / MTok | $12.50 / MTok | $20 / MTok | $1 / MTok | $50 / MTok |
| Claude Mythos 5 ([limited availability](https://anthropic.com/glasswing)) | $10 / MTok | $12.50 / MTok | $20 / MTok | $1 / MTok | $50 / MTok |
| Claude Opus 4.8 | $5 / MTok | $6.25 / MTok | $10 / MTok | $0.50 / MTok | $25 / MTok |
| Claude Opus 4.7 | $5 / MTok | $6.25 / MTok | $10 / MTok | $0.50 / MTok | $25 / MTok |
| Claude Opus 4.6 | $5 / MTok | $6.25 / MTok | $10 / MTok | $0.50 / MTok | $25 / MTok |
| Claude Opus 4.5 | $5 / MTok | $6.25 / MTok | $10 / MTok | $0.50 / MTok | $25 / MTok |
| Claude Opus 4.1 ([deprecated](/docs/en/about-claude/model-deprecations)) | $15 / MTok | $18.75 / MTok | $30 / MTok | $1.50 / MTok | $75 / MTok |
| Claude Opus 4 ([retired, except on Google Cloud](/docs/en/about-claude/model-deprecations)) | $15 / MTok | $18.75 / MTok | $30 / MTok | $1.50 / MTok | $75 / MTok |
| Claude Sonnet 5 [through August 31, 2026](/docs/en/about-claude/pricing#claude-sonnet-5-introductory-pricing) | $2 / MTok | $2.50 / MTok | $4 / MTok | $0.20 / MTok | $10 / MTok |
| Claude Sonnet 5 starting September 1, 2026 | $3 / MTok | $3.75 / MTok | $6 / MTok | $0.30 / MTok | $15 / MTok |
| Claude Sonnet 4.6 | $3 / MTok | $3.75 / MTok | $6 / MTok | $0.30 / MTok | $15 / MTok |
| Claude Sonnet 4.5 | $3 / MTok | $3.75 / MTok | $6 / MTok | $0.30 / MTok | $15 / MTok |
| Claude Sonnet 4 ([retired, except on Bedrock and Google Cloud](/docs/en/about-claude/model-deprecations)) | $3 / MTok | $3.75 / MTok | $6 / MTok | $0.30 / MTok | $15 / MTok |
| Claude Haiku 4.5 | $1 / MTok | $1.25 / MTok | $2 / MTok | $0.10 / MTok | $5 / MTok |
| Claude Haiku 3.5 ([retired, except on Bedrock and Google Cloud](/docs/en/about-claude/model-deprecations)) | $0.80 / MTok | $1 / MTok | $1.60 / MTok | $0.08 / MTok | $4 / MTok |



The table above reflects the following pricing multipliers for prompt caching:

* 5-minute cache write tokens are 1.25 times the base input tokens price
* 1-hour cache write tokens are 2 times the base input tokens price
* Cache read tokens are 0.1 times the base input tokens price

These multipliers stack with other pricing modifiers such as the Batch API discount and data residency. See [pricing](/docs/en/about-claude/pricing) for full details.

---

## Supported models

Prompt caching (both automatic and explicit) is supported on all [active Claude models](/docs/en/about-claude/models/overview).

---

## Automatic caching

Automatic caching is the simplest way to enable prompt caching. Instead of placing `cache_control` on individual content blocks, add a single `cache_control` field at the top level of your request body. The system automatically applies the cache breakpoint to the last cacheable block.

```
client = anthropic.Anthropic() client = anthropic.Anthropic() response = client.messages.create(response = client.messages.create( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=1024,  max_tokens = 1024, cache_control={"type": "ephemeral"},  cache_control ={"type": "ephemeral"}, system="You are a helpful assistant that remembers our conversation.",  system ="You are a helpful assistant that remembers our conversation.", messages=[ messages =[ {"role": "user", "content": "My name is Alex. I work on machine learning."}, {"role": "user", "content": "My name is Alex. I work on machine learning."}, { { "role": "assistant",  "role": "assistant", "content": "Nice to meet you, Alex! How can I help with your ML work today?",  "content": "Nice to meet you, Alex! How can I help with your ML work today?", }, }, {"role": "user", "content": "What did I say I work on?"}, {"role": "user", "content": "What did I say I work on?"}, ], ],))print(response.usage.model_dump_json()) print(response.usage.model_dump_json())
```

### How automatic caching works in multi-turn conversations

With automatic caching, the cache point moves forward automatically as conversations grow. Each new request caches everything up to the last cacheable block, and previous content is read from cache.

| Request | Content | Cache behavior |
| --- | --- | --- |
| Request 1 | System   + User(1) + Asst(1)   + **User(2)** ◀ cache | Everything written to cache |
| Request 2 | System   + User(1) + Asst(1)   + User(2) + Asst(2)   + **User(3)** ◀ cache | System through User(2) read from cache;   Asst(2) + User(3) written to cache |
| Request 3 | System   + User(1) + Asst(1)   + User(2) + Asst(2)   + User(3) + Asst(3)   + **User(4)** ◀ cache | System through User(3) read from cache;   Asst(3) + User(4) written to cache |

The cache breakpoint automatically moves to the last cacheable block in each request, so you don't need to update any `cache_control` markers as the conversation grows.

### TTL support

By default, automatic caching uses a 5-minute TTL. You can specify a 1-hour TTL at 2x the base input token price:

```
{ "cache_control": { "type": "ephemeral", "ttl": "1h" } }{ "cache_control": { "type": "ephemeral", "ttl": "1h" } }
```

### Combining with block-level caching

Automatic caching is compatible with [explicit cache breakpoints](#explicit-cache-breakpoints). When used together, the automatic cache breakpoint uses one of the 4 available breakpoint slots.

This lets you combine both approaches. For example, use an explicit breakpoint to cache your system prompt, while automatic caching handles the conversation:

```
{{ "model": "claude-opus-4-8",  "model": "claude-opus-4-8", "max_tokens": 1024,  "max_tokens": 1024, "cache_control": { "type": "ephemeral" },  "cache_control": { "type": "ephemeral" }, "system": [ "system": [ { { "type": "text",  "type": "text", "text": "You are a helpful assistant.",  "text": "You are a helpful assistant.", "cache_control": { "type": "ephemeral" }  "cache_control": { "type": "ephemeral" } } } ], ], "messages": [{ "role": "user", "content": "What are the key terms?" }]  "messages": [{ "role": "user", "content": "What are the key terms?" }]}}
```

### What stays the same

Automatic caching uses the same underlying caching infrastructure. Pricing, minimum token thresholds, context ordering requirements, and the 20-block lookback window all apply the same as with explicit breakpoints.

### Edge cases

* If the last block already has an explicit `cache_control` with the same TTL, automatic caching is a no-op.
* If the last block has an explicit `cache_control` with a different TTL, the API returns a 400 error.
* If 4 explicit block-level breakpoints already exist, the API returns a 400 error (no slots left for automatic caching).
* If the last block is not eligible as an automatic cache breakpoint target, the system silently walks backwards to find the nearest eligible block. If none is found, caching is skipped.



Automatic caching is available on the Claude API, [Claude Platform on AWS](/docs/en/build-with-claude/claude-platform-on-aws), and [Microsoft Foundry](/docs/en/build-with-claude/claude-in-microsoft-foundry). Bedrock and Google Cloud do not support automatic caching.

---

## Explicit cache breakpoints

For more control over caching, you can place `cache_control` directly on individual content blocks. This is useful when you need to cache different sections that change at different frequencies, or need fine-grained control over exactly what gets cached.

### Structuring your prompt

Place static content (tool definitions, system instructions, context, examples) at the beginning of your prompt. Mark the end of the reusable content for caching using the `cache_control` parameter.

Cache prefixes are created in the following order: `tools`, `system`, then `messages`. This order forms a hierarchy where each level builds upon the previous ones.

#### How automatic prefix checking works

You can use just one cache breakpoint at the end of your static content, and the system will automatically find the longest prefix that a prior request already wrote to the cache. Understanding how this works helps you optimize your caching strategy.

**Three core principles:**

1. **Cache writes happen only at your breakpoint.** Marking a block with `cache_control` writes exactly one cache entry: a hash of the prefix ending at that block. The system does not write entries for any earlier position. Because the hash is cumulative, covering everything up to and including the breakpoint, changing any block at or before the breakpoint produces a different hash on the next request.
2. **Cache reads look backward for entries that prior requests wrote.** On each request the system computes the prefix hash at your breakpoint and checks for a matching cache entry. If none exists, it walks backward one block at a time, checking whether the prefix hash at each earlier position matches something already in the cache. It is looking for prior writes, not for stable content.
3. **The lookback window is 20 blocks.** The system checks at most 20 positions per breakpoint, counting the breakpoint itself as the first. If the system finds no matching entry in that window, checking stops (or resumes from the next explicit breakpoint, if any).

**Example: Lookback in a growing conversation**

You append new blocks each turn and set `cache_control` on the final block of each request:

* **Turn 1:** 10 blocks, breakpoint on block 10. No prior cache entries exist. The system writes an entry at block 10.
* **Turn 2:** 15 blocks, breakpoint on block 15. Block 15 has no entry, so the system walks back to block 10 and finds the turn-1 entry. Cache hit at block 10; the system processes only blocks 11 through 15 fresh and writes a new entry at block 15.
* **Turn 3:** 35 blocks, breakpoint on block 35. The system checks 20 positions (blocks 35 through 16) and finds nothing. The turn-2 entry at block 15 is one position outside the window, so there is no cache hit. Adding a second breakpoint at block 15 starts a second lookback window there, which finds the turn-2 entry.

**Common mistake: Breakpoint on content that changes every request**

Your prompt has a large static system context (blocks 1 through 5) followed by a per-request block containing a timestamp and the user message (block 6). You set `cache_control` on block 6:

* **Request 1:** Cache write at block 6. The hash includes the timestamp.
* **Request 2:** The timestamp differs, so the prefix hash at block 6 differs. The lookback walks through blocks 5, 4, 3, 2, and 1, but the system never wrote an entry at any of those positions. No cache hit. You pay for a fresh cache write on every request and never get a read.

The lookback does not find stable content behind your breakpoint and cache it. It finds entries that prior requests already wrote, and writes happen only at breakpoints. Move `cache_control` to block 5, the last block that stays the same across requests, and every subsequent request reads the cached prefix. [Automatic caching](#automatic-caching) hits the same trap: it places the breakpoint on the last cacheable block, which in this structure is the one that changes every request, so use an explicit breakpoint on block 5 instead.

**Key takeaway:** Place `cache_control` on the last block whose prefix is identical across the requests you want to share a cache. In a growing conversation the final block works as long as each turn adds fewer than 20 blocks: earlier content never changes, so the next request's lookback finds the prior write. For a prompt with a varying suffix (timestamps, per-request context, the incoming message), place the breakpoint at the end of the static prefix, not on the varying block.

#### When to use multiple breakpoints

You can define up to 4 cache breakpoints if you want to:

* Cache different sections that change at different frequencies (for example, tools rarely change, but context updates daily)
* Have more control over exactly what gets cached
* Ensure a cache hit when a growing conversation pushes your breakpoint 20 or more blocks past the last cache write



**Important limitation:** The lookback can only find entries that earlier requests already wrote. If a growing conversation pushes your breakpoint 20 or more blocks past the last write, the lookback window misses it. Add a second breakpoint closer to that position from the start so a write accumulates there before you need it.

### Understanding cache breakpoint costs

**Cache breakpoints themselves don't add any cost.** You are only charged for:

* **Cache writes**: When new content is written to the cache (25% more than base input tokens for 5-minute TTL)
* **Cache reads**: When cached content is used (10% of base input token price)
* **Regular input tokens**: For any uncached content

Adding more `cache_control` breakpoints doesn't increase your costs - you still pay the same amount based on what content is actually cached and read. The breakpoints simply give you control over what sections can be cached independently.

---

## Caching strategies and considerations

### Cache limitations

On the Claude API, [Claude Platform on AWS](/docs/en/build-with-claude/claude-platform-on-aws), [Google Cloud](/docs/en/build-with-claude/claude-on-vertex-ai), and [Microsoft Foundry](/docs/en/build-with-claude/claude-in-microsoft-foundry), the minimum cacheable prompt length is:

* 512 tokens for Claude Fable 5 and [Claude Mythos 5](https://anthropic.com/glasswing)
* 2,048 tokens for [Claude Mythos Preview](https://anthropic.com/glasswing) and Claude Opus 4.7
* 4,096 tokens for Claude Opus 4.6 and Claude Opus 4.5
* 1,024 tokens for Claude Opus 4.8, Claude Sonnet 5, Claude Sonnet 4.6, Claude Sonnet 4.5, Claude Opus 4.1 ([deprecated](/docs/en/about-claude/model-deprecations)), Claude Opus 4 ([retired, except on Google Cloud](/docs/en/about-claude/model-deprecations)), and Claude Sonnet 4 ([retired, except on Bedrock and Google Cloud](/docs/en/about-claude/model-deprecations))
* 4,096 tokens for Claude Haiku 4.5
* 2,048 tokens for Claude Haiku 3.5 ([retired, except on Google Cloud](/docs/en/about-claude/model-deprecations))

Model availability varies by platform, and so can the minimum for newly released models: on [Amazon Bedrock](/docs/en/build-with-claude/claude-in-amazon-bedrock), the minimum cacheable prompt length for Claude Fable 5 and Claude Mythos 5 is 1,024 tokens.

Shorter prompts cannot be cached, even if marked with `cache_control`. Any requests to cache fewer than this number of tokens will be processed without caching, and no error is returned. To verify whether a prompt was cached, check the response usage [fields](/docs/en/build-with-claude/prompt-caching#tracking-cache-performance): if both `cache_creation_input_tokens` and `cache_read_input_tokens` are 0, the prompt was not cached (likely because it did not meet the minimum length requirement).

If your prompt falls just short of the minimum for your model and platform, expanding the cached content to reach the threshold is often worthwhile. Cache reads cost significantly less than uncached input tokens, so reaching the minimum can reduce costs for frequently reused prompts.



[Bedrock](/docs/en/build-with-claude/claude-in-amazon-bedrock) is an AWS-operated platform. On Bedrock, see the [Bedrock prompt caching documentation](https://docs.aws.amazon.com/bedrock/latest/userguide/prompt-caching.html) for the per-model minimums, failure behavior, and usage-field names that apply.

For concurrent requests, note that a cache entry only becomes available after the first response begins. If you need cache hits for parallel requests, wait for the first response before sending subsequent requests.

Currently, "ephemeral" is the only supported cache type, which by default has a 5-minute lifetime.

### What can be cached

Most blocks in the request can be cached. This includes:

* Tools: Tool definitions in the `tools` array
* System messages: Content blocks in the `system` array
* Text messages: Content blocks in the `messages.content` array, for both user and assistant turns
* Images & Documents: Content blocks in the `messages.content` array, in user turns
* Tool use and tool results: Content blocks in the `messages.content` array, in both user and assistant turns

Each of these elements can be cached, either automatically or by marking them with `cache_control`.

### What cannot be cached

While most request blocks can be cached, there are some exceptions:

* Thinking blocks cannot be cached directly with `cache_control`. However, thinking blocks CAN be cached alongside other content when they appear in previous assistant turns. When cached this way, they DO count as input tokens when read from cache.
* Sub-content blocks (like [citations](/docs/en/build-with-claude/citations)) themselves cannot be cached directly. Instead, cache the top-level block.

  In the case of citations, the top-level document content blocks that serve as the source material for citations can be cached. This allows you to use prompt caching with citations effectively by caching the documents that citations will reference.
* Empty text blocks cannot be cached.

### What invalidates the cache

Modifications to cached content can invalidate some or all of the cache.

As described in [Structuring your prompt](#structuring-your-prompt), the cache follows the hierarchy: `tools` → `system` → `messages`. Changes at each level invalidate that level and all subsequent levels.

The following table shows which parts of the cache are invalidated by different types of changes. ✘ indicates that the cache is invalidated, while ✓ indicates that the cache remains valid.

| What changes | Tools cache | System cache | Messages cache | Impact |
| --- | --- | --- | --- | --- |
| **Tool definitions** | ✘ | ✘ | ✘ | Modifying tool definitions (names, descriptions, parameters) invalidates the entire cache |
| **Web search toggle** | ✓ | ✘ | ✘ | Enabling/disabling web search modifies the system prompt |
| **Citations toggle** | ✓ | ✘ | ✘ | Enabling/disabling citations modifies the system prompt |
| **Speed setting** | ✓ | ✘ | ✘ | Switching between [`speed: "fast"` and standard speed](/docs/en/build-with-claude/fast-mode) invalidates system and message caches |
| **Tool choice** | ✓ | ✓ | ✘ | Changes to `tool_choice` parameter only affect message blocks |
| **Images** | ✓ | ✓ | ✘ | Adding/removing images anywhere in the prompt affects message blocks |
| **Thinking parameters** | ✓ | ✓ | ✘ | Changes to extended thinking settings (enable/disable, budget) affect message blocks |
| **Non-tool results passed to extended thinking requests** | ✓ | ✓ | Model-specific | On Opus 4.5+ and Sonnet 4.6+, thinking blocks are preserved by default, so the cache remains valid (✓). On earlier Opus/Sonnet models and all Haiku models, all previously-cached thinking blocks are stripped from context, and any messages that follow those thinking blocks are removed from the cache (✘). For more details, see [Caching with thinking blocks](#caching-with-thinking-blocks). |



On Claude Opus 4.8, you can add a new system instruction partway through a conversation without invalidating the system or message caches. Append a `{"role": "system"}` message to `messages` instead of editing the top-level `system` field, so the cached prefix stays unchanged. See [Mid-conversation system messages](/docs/en/build-with-claude/mid-conversation-system-messages).

### Tracking cache performance

Monitor cache performance using these API response fields, within `usage` in the response (or `message_start` event if [streaming](/docs/en/build-with-claude/streaming)):

* `cache_creation_input_tokens`: Number of tokens written to the cache when creating a new entry.
* `cache_read_input_tokens`: Number of tokens retrieved from the cache for this request.
* `input_tokens`: Number of input tokens which were not read from or used to create a cache (that is, tokens after the last cache breakpoint).



**Understanding the token breakdown**

The `input_tokens` field represents only the tokens that come **after the last cache breakpoint** in your request - not all the input tokens you sent.

To calculate total input tokens:

```
total_input_tokens = cache_read_input_tokens + cache_creation_input_tokens + input_tokens
```

**Spatial explanation:**

* `cache_read_input_tokens` = tokens before breakpoint already cached (reads)
* `cache_creation_input_tokens` = tokens before breakpoint being cached now (writes)
* `input_tokens` = tokens after your last breakpoint (not eligible for cache)

**Example:** If you have a request with 100,000 tokens of cached content (read from cache), 0 tokens of new content being cached, and 50 tokens in your user message (after the cache breakpoint):

* `cache_read_input_tokens`: 100,000
* `cache_creation_input_tokens`: 0
* `input_tokens`: 50
* **Total input tokens processed**: 100,050 tokens

This is important for understanding both costs and rate limits, as `input_tokens` will typically be much smaller than your total input when using caching effectively.

### Caching with thinking blocks

When using [extended thinking](/docs/en/build-with-claude/extended-thinking) with prompt caching, thinking blocks have special behavior:

**Automatic caching alongside other content**: While thinking blocks cannot be explicitly marked with `cache_control`, they get cached as part of the request content when you make subsequent API calls with tool results. This commonly happens during tool use when you pass thinking blocks back to continue the conversation.

**Input token counting**: When thinking blocks are read from cache, they count as input tokens in your usage metrics. This is important for cost calculation and token budgeting.

**Cache invalidation patterns**:

* Cache remains valid when only tool results are provided as user messages
* On Opus 4.5+ and Sonnet 4.6+, thinking blocks are preserved by default even when non-tool-result user content is added, so the cache remains valid
* On earlier Opus/Sonnet models and all Haiku models, cache gets invalidated when non-tool-result user content is added, causing all previous thinking blocks to be stripped from context
* This caching behavior occurs even without explicit `cache_control` markers

For more details on cache invalidation, see [What invalidates the cache](#what-invalidates-the-cache).

**Example with tool use**:

```
Request 1: User: "What's the weather in Paris?"Request 1: User: "What's the weather in Paris?"Response: [thinking_block_1] + [tool_use block 1]Response: [thinking_block_1] + [tool_use block 1] Request 2:Request 2:User: ["What's the weather in Paris?"],User: ["What's the weather in Paris?"],Assistant: [thinking_block_1] + [tool_use block 1],Assistant: [thinking_block_1] + [tool_use block 1],User: [tool_result_1, cache=True]User: [tool_result_1, cache=True]Response: [thinking_block_2] + [text block 2]Response: [thinking_block_2] + [text block 2]# Request 2 caches its request content (not the response)# Request 2 caches its request content (not the response)# The cache includes: user message, thinking_block_1, tool_use block 1, and tool_result_1# The cache includes: user message, thinking_block_1, tool_use block 1, and tool_result_1 Request 3:Request 3:User: ["What's the weather in Paris?"],User: ["What's the weather in Paris?"],Assistant: [thinking_block_1] + [tool_use block 1],Assistant: [thinking_block_1] + [tool_use block 1],User: [tool_result_1, cache=True],User: [tool_result_1, cache=True],Assistant: [thinking_block_2] + [text block 2],Assistant: [thinking_block_2] + [text block 2],User: [Text response, cache=True]User: [Text response, cache=True]# On earlier Opus/Sonnet and all Haiku models, non-tool-result user block causes prior thinking blocks to be stripped; on Opus 4.5+/Sonnet 4.6+ they are kept# On earlier Opus/Sonnet and all Haiku models, non-tool-result user block causes prior thinking blocks to be stripped; on Opus 4.5+/Sonnet 4.6+ they are keptnon-tool-result
```

On earlier Opus/Sonnet models and all Haiku models, all previous thinking blocks are removed from context at this point. On Opus 4.5+ and Sonnet 4.6+, prior thinking blocks are kept by default and remain part of the cached prefix.

For more detailed information, see the [extended thinking documentation](/docs/en/build-with-claude/extended-thinking#understanding-thinking-block-caching-behavior).

### Cache storage and sharing



As of February 5, 2026, prompt caching uses [workspace](/docs/en/manage-claude/workspaces)-level isolation instead of organization-level isolation. Caches are isolated per workspace, ensuring data separation between workspaces within the same organization. This applies to the Claude API, Claude Platform on AWS, and Microsoft Foundry; Bedrock and Google Cloud maintain organization-level cache isolation. If you use multiple workspaces, review your caching strategy to account for this difference.

* **Organization and workspace isolation:** Caches are isolated between organizations. Different organizations never share caches, even if they use identical prompts. As of February 5, 2026, caches are also isolated per workspace within an organization on the Claude API, Claude Platform on AWS, and Microsoft Foundry; Bedrock and Google Cloud continue to use organization-level isolation only.
* **Exact matching:** Cache hits require 100% identical prompt segments, including all text and images up to and including the block marked with cache control.
* **Output token generation:** Prompt caching has no effect on output token generation. The response you receive is identical to what you would get if prompt caching were not used.

### Best practices for effective caching

To optimize prompt caching performance:

* Start with [automatic caching](#automatic-caching) for multi-turn conversations. It handles breakpoint management automatically.
* Use [explicit block-level breakpoints](#explicit-cache-breakpoints) when you need to cache different sections with different change frequencies.
* Cache stable, reusable content like system instructions, background information, large contexts, or frequent tool definitions.
* Place cached content at the prompt's beginning for best performance.
* Use cache breakpoints strategically to separate different cacheable prefix sections.
* Place the breakpoint on the last block that stays identical across requests. For a prompt with a static prefix and a varying suffix (timestamps, per-request context, the incoming message), that is the end of the prefix, not the varying block.
* Regularly analyze cache hit rates and adjust your strategy as needed.

### Optimizing for different use cases

Tailor your prompt caching strategy to your scenario:

* Conversational agents: Reduce cost and latency for extended conversations, especially those with long instructions or uploaded documents.
* Coding assistants: Improve autocomplete and codebase Q&A by keeping relevant sections or a summarized version of the codebase in the prompt.
* Large document processing: Incorporate complete long-form material including images in your prompt without increasing response latency.
* Detailed instruction sets: Share extensive lists of instructions, procedures, and examples to fine-tune Claude's responses. Developers often include an example or two in the prompt, but with prompt caching you can get even better performance by including 20+ diverse examples of high quality answers.
* Agentic tool use: Enhance performance for scenarios involving multiple tool calls and iterative code changes, where each step typically requires a new API call.
* Talk to books, papers, documentation, podcast transcripts, and other longform content: Bring any knowledge base alive by embedding the entire document(s) into the prompt, and letting users ask it questions.

### Troubleshooting common issues

If experiencing unexpected behavior:



[Cache diagnostics](/docs/en/build-with-claude/cache-diagnostics) (beta) has the API compare consecutive requests and report exactly where the prompt prefix diverged, which automatically handles many of the steps in this list.

* Ensure cached sections are identical across calls. For explicit breakpoints, verify that `cache_control` markers are in the same locations
* Check that calls are made within the cache lifetime (5 minutes by default)
* Verify that `tool_choice` and image usage remain consistent between calls
* Validate that you are caching at least the minimum number of tokens for your model and platform (see [Cache limitations](#cache-limitations))
* Confirm your breakpoint is on a block that stays identical across requests. Cache writes happen only at the breakpoint, and if that block changes (timestamps, per-request context, the incoming message), the prefix hash never matches. The lookback does not find stable content behind the breakpoint; it only finds entries that earlier requests wrote at their own breakpoints
* Verify that the keys in your `tool_use` content blocks have stable ordering as some languages (for example, Swift, Go) randomize key order during JSON conversion, breaking caches
* Use [cache diagnostics](/docs/en/build-with-claude/cache-diagnostics) to have the API compare consecutive requests and report which part of the prompt diverged



Changes to `tool_choice` or the presence/absence of images anywhere in the prompt will invalidate the cache, requiring a new cache entry to be created. For more details on cache invalidation, see [What invalidates the cache](#what-invalidates-the-cache).

---

## 1-hour cache duration

If you find that 5 minutes is too short, Anthropic also offers a 1-hour cache duration [at additional cost](#pricing).



The 1-hour cache duration is available on the Claude API, [Claude Platform on AWS](/docs/en/build-with-claude/claude-platform-on-aws), [Amazon Bedrock](/docs/en/build-with-claude/claude-in-amazon-bedrock), [Amazon Bedrock (legacy)](/docs/en/build-with-claude/claude-on-amazon-bedrock-legacy), [Google Cloud](/docs/en/build-with-claude/claude-on-vertex-ai), and [Microsoft Foundry](/docs/en/build-with-claude/claude-in-microsoft-foundry).

To use the extended cache, include `ttl` in the `cache_control` definition like this:

```
"cache_control": {"cache_control": { "type": "ephemeral",  "type": "ephemeral", "ttl": "1h"  "ttl": "1h"}}
```

The response will include detailed cache information like the following:

Output

```
{{ "usage": { "usage": { "input_tokens": 2048,  "input_tokens": 2048, "cache_read_input_tokens": 1800,  "cache_read_input_tokens": 1800, "cache_creation_input_tokens": 248,  "cache_creation_input_tokens": 248, "output_tokens": 503,  "output_tokens": 503,  "cache_creation": { "cache_creation": { "ephemeral_5m_input_tokens": 148,  "ephemeral_5m_input_tokens": 148, "ephemeral_1h_input_tokens": 100  "ephemeral_1h_input_tokens": 100 } } } }}}
```

Note that the current `cache_creation_input_tokens` field equals the sum of the values in the `cache_creation` object.

If you see `ephemeral_5m_input_tokens` writes you didn't request while using server tools such as web search, see [this guide on prompt caching and tool use](/docs/en/agents-and-tools/tool-use/tool-use-with-prompt-caching#server-tool-results-are-cached-automatically).

### When to use the 1-hour cache

If you have prompts that are used at a regular cadence (that is, system prompts that are used more frequently than every 5 minutes), continue to use the 5-minute cache, since this will continue to be refreshed at no additional charge.

The 1-hour cache is best used in the following scenarios:

* When you have prompts that are likely used less frequently than 5 minutes, but more frequently than every hour. For example, when an agentic side-agent will take longer than 5 minutes, or when storing a long chat conversation with a user and you generally expect that user may not respond in the next 5 minutes.
* When latency is important and your follow up prompts may be sent beyond 5 minutes.
* When you want to improve your rate limit utilization, since cache hits are not deducted against your rate limit.



The 5-minute and 1-hour cache behave the same with respect to latency. You will generally see improved time-to-first-token for long documents.

### Mixing different TTLs

You can use both 1-hour and 5-minute cache controls in the same request, but with an important constraint: Cache entries with longer TTL must appear before shorter TTLs (that is, a 1-hour cache entry must appear before any 5-minute cache entries).

When mixing TTLs, the API determines three billing locations in your prompt:

1. Position `A`: The token count at the highest cache hit (or 0 if no hits).
2. Position `B`: The token count at the highest 1-hour `cache_control` block after `A` (or equals `A` if none exist).
3. Position `C`: The token count at the last `cache_control` block.



If `B` and/or `C` are larger than `A`, they will necessarily be cache misses, because `A` is the highest cache hit.

You'll be charged for:

1. Cache read tokens for `A`.
2. 1-hour cache write tokens for `(B - A)`.
3. 5-minute cache write tokens for `(C - B)`.

Here are 3 examples. This depicts the input tokens of 3 requests, each of which has different cache hits and cache misses. Each has a different calculated pricing, shown in the colored boxes, as a result. ![Mixing TTLs Diagram](/docs/images/prompt-cache-mixed-ttl.svg)

---

## Pre-warming the cache

Cache pre-warming lets you load your system prompt or tool definitions into the prompt cache before a user triggers a real request. This eliminates the cache-miss latency penalty on the first user interaction, reducing time-to-first-token (TTFT) for latency-sensitive applications.

### How it works

Set `max_tokens: 0` in your request. The API reads your prompt into the model and writes the cache at any `cache_control` breakpoint, then returns immediately without generating any output. The response has an empty `content` array, `stop_reason: "max_tokens"`, and a fully populated `usage` block.

Place the `cache_control` breakpoint on the last block that is shared with the follow-up request (typically your system prompt or tool definitions), not on the placeholder user message. Otherwise the cache entry is keyed to the placeholder and the follow-up request won't hit it. This means using an [explicit cache breakpoint](#explicit-cache-breakpoints) rather than [automatic caching](#automatic-caching), since automatic caching places the breakpoint on the last block, which here is the placeholder. The placeholder user message can be any string with non-whitespace content (the examples here use `"warmup"`); its content is read into the model but never answered.



A pre-warm request incurs a **cache write** charge if the prefix is not already cached, the same as any other request. Check `usage.cache_creation_input_tokens` in the response to confirm a write occurred. Zero output tokens are billed.

```
client = anthropic.Anthropic() client = anthropic.Anthropic() # Fire this before users arrive to warm the shared system-prompt cache.# Fire this before users arrive to warm the shared system-prompt cache.prewarm = client.messages.create(prewarm = client.messages.create( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=0,  max_tokens = 0, system=[ system =[ { { "type": "text",  "type": "text", "text": "You are an expert software engineer with deep knowledge of distributed systems...",  "text": "You are an expert software engineer with deep knowledge of distributed systems...", "cache_control": {"type": "ephemeral"},  "cache_control": {"type": "ephemeral"}, } } ], ], messages=[{"role": "user", "content": "warmup"}],  messages =[{"role": "user", "content": "warmup"}],))print(prewarm.stop_reason) # "max_tokens" print(prewarm.stop_reason) # "max_tokens"print(prewarm.content) # [] print(prewarm.content) # []print(prewarm.usage) print(prewarm.usage)
```

The API returns an empty `content` array:

Output

```
{{ "id": "msg_01XFDUDYJgAACzvnptvVoYEL",  "id": "msg_01XFDUDYJgAACzvnptvVoYEL", "type": "message",  "type": "message", "role": "assistant",  "role": "assistant", "content": [],  "content": [], "model": "claude-opus-4-8",  "model": "claude-opus-4-8", "stop_reason": "max_tokens",  "stop_reason": "max_tokens", "stop_sequence": null,  "stop_sequence": null, "usage": { "usage": { "input_tokens": 8,  "input_tokens": 8, "cache_creation_input_tokens": 5120,  "cache_creation_input_tokens": 5120, "cache_read_input_tokens": 0,  "cache_read_input_tokens": 0, "cache_creation": { "cache_creation": { "ephemeral_5m_input_tokens": 5120,  "ephemeral_5m_input_tokens": 5120, "ephemeral_1h_input_tokens": 0  "ephemeral_1h_input_tokens": 0 }, }, "iterations": [ "iterations": [ { { "input_tokens": 8,  "input_tokens": 8, "output_tokens": 0,  "output_tokens": 0, "cache_read_input_tokens": 0,  "cache_read_input_tokens": 0, "cache_creation_input_tokens": 5120,  "cache_creation_input_tokens": 5120, "cache_creation": { "cache_creation": { "ephemeral_5m_input_tokens": 5120,  "ephemeral_5m_input_tokens": 5120, "ephemeral_1h_input_tokens": 0  "ephemeral_1h_input_tokens": 0 }, }, "type": "message"  "type": "message" } } ], ], "output_tokens": 0,  "output_tokens": 0, "service_tier": "standard",  "service_tier": "standard", "inference_geo": "global"  "inference_geo": "global" } }}}
```

### Typical usage pattern

Fire a pre-warm request when your application starts (or on a scheduled interval), then send real user requests after the pre-warm completes:

```
client = anthropic.Anthropic() client = anthropic.Anthropic() SYSTEM_PROMPT = [SYSTEM_PROMPT = [ { { "type": "text",  "type": "text", "text": "You are an expert software engineer with deep knowledge of distributed systems...",  "text": "You are an expert software engineer with deep knowledge of distributed systems...", "cache_control": {"type": "ephemeral"},  "cache_control": {"type": "ephemeral"}, } }]] def prewarm_cache() -> None: def  prewarm_cache() -> None: """Call this at application startup or on a scheduled interval.""" """Call this at application startup or on a scheduled interval.""" client.messages.create( client.messages.create( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=0,  max_tokens = 0, system=SYSTEM_PROMPT,  system = SYSTEM_PROMPT, messages=[{"role": "user", "content": "warmup"}],  messages =[{"role": "user", "content": "warmup"}], ) ) def respond(user_message: str) -> anthropic.types.Message: def  respond(user_message: str) -> anthropic.types.Message: """The real user request; benefits from a warm cache.""" """The real user request; benefits from a warm cache.""" return client.messages.create( return client.messages.create( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=1024,  max_tokens = 1024, system=SYSTEM_PROMPT,  system = SYSTEM_PROMPT, messages=[{"role": "user", "content": user_message}],  messages =[{"role": "user", "content": user_message}], ) ) # Warm the cache before any user traffic arrives.# Warm the cache before any user traffic arrives.prewarm_cache()prewarm_cache() # Later, when the user submits a message, the system-prompt prefix is already cached.# Later, when the user submits a message, the system-prompt prefix is already cached.response = respond("How do I implement a binary search tree?") response = respond("How do I implement a binary search tree?")print(response.content[0].text) print(response.content[0].text)
```

Keep in mind that the cache TTL still applies. For the default 5-minute cache, send a new pre-warm request at least every 5 minutes to keep the cache warm. For longer gaps between user requests, use the [1-hour cache duration](#1-hour-cache-duration) instead.

### 

A `max_tokens: 0` request is rejected with an `invalid_request_error` if any of the following are set, since each implies output that a zero-token budget cannot produce:

* `stream: true`
* [Extended thinking](/docs/en/build-with-claude/extended-thinking) (`thinking.type: "enabled"`)
* [Structured outputs](/docs/en/build-with-claude/structured-outputs) (`output_config.format`)
* `tool_choice` of `{"type": "tool", ...}` or `{"type": "any"}`

`max_tokens: 0` is also rejected inside a [Message Batches](/docs/en/build-with-claude/batch-processing) request. Pre-warming targets time-to-first-token, which does not apply to batch processing, and a cache entry written during batch processing would likely expire before the follow-up request runs.

### Replacing the max\_tokens=1 workaround

Before `max_tokens: 0` was available, some applications used `max_tokens: 1` warm-up calls to achieve the same effect. The `max_tokens: 0` approach is preferred: no output is produced, so there is no single-token reply to discard, no output tokens are billed, and the intent of the request is unambiguous.

---

## Prompt caching examples

To help you get started with prompt caching, the [prompt caching cookbook](https://platform.claude.com/cookbook/misc-prompt-caching) provides detailed examples and best practices.

The following code snippets showcase various prompt caching patterns. These examples demonstrate how to implement caching in different scenarios, helping you understand the practical applications of this feature:

## Data retention

Prompt caching (both automatic and explicit) is ZDR eligible. Anthropic does not store the raw text of your prompts or Claude's responses.

KV (key-value) cache representations and cryptographic hashes of cached content are held in memory only and are not stored at rest. Cached entries have a minimum lifetime of 5 minutes (standard) or 1 hour (extended), after which they are promptly, though not immediately, deleted. Cache entries are isolated between organizations and, on the Claude API, Claude Platform on AWS, and Microsoft Foundry, between workspaces within an organization.

For ZDR eligibility across all features, see [API and data retention](/docs/en/manage-claude/api-and-data-retention).

---

## 

Was this page helpful?
