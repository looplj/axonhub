[Claude Platform Docs](/docs/en/home)

Overview

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

Loading...

Messages/Tools

# Tool use with Claude

Connect Claude to external tools and APIs. See where tools execute, when Claude calls them, and which tool fits your task.

Tool use lets Claude call functions that you define or that Anthropic provides. Claude determines when to call a tool based on the user's request and the tool's description. It then returns a structured call that your application executes (client tools) or that Anthropic executes (server tools).

Here's a minimal example using a server tool, the [Web search tool](/docs/en/agents-and-tools/tool-use/web-search-tool), which Anthropic executes for you:

```
client = anthropic.Anthropic() client = anthropic.Anthropic()response = client.messages.create(response = client.messages.create( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=1024,  max_tokens = 1024, tools=[{"type": "web_search_20260209", "name": "web_search"}],  tools =[{"type": "web_search_20260209", "name": "web_search"}], messages=[{"role": "user", "content": "What's the latest on the Mars rover?"}],  messages =[{"role": "user", "content": "What's the latest on the Mars rover?"}],))print(response.content) print(response.content)
```

Claude runs the search on Anthropic's infrastructure and returns the cited results in the same response. To have Claude call a function that you define, pass a tool with an `input_schema`, then execute the call when Claude returns a `tool_use` block. [Define tools](/docs/en/agents-and-tools/tool-use/define-tools) and [Handle tool calls](/docs/en/agents-and-tools/tool-use/handle-tool-calls) cover that round trip.

## How tool use works

Tools differ primarily by where the code executes. **Client tools** (including user-defined tools and tools with Anthropic-defined schemas, such as `bash` and `text_editor`) run in your application. Claude responds with `stop_reason: "tool_use"` and one or more `tool_use` blocks. Your code executes the operation and sends back a `tool_result`. **Server tools** (such as `web_search`, `web_fetch`, `code_execution`, and `tool_search`) run on Anthropic's infrastructure: you see the results directly without handling execution, unless Claude calls the tool in the same group of parallel tool calls as one of your client tools (see [Stop reasons and fallback](/docs/en/build-with-claude/handling-stop-reasons#tool-use)).

For the full conceptual model including the agentic loop and when to choose each approach, see [How tool use works](/docs/en/agents-and-tools/tool-use/how-tool-use-works).

For connecting to Model Context Protocol (MCP) servers, see the [MCP connector](/docs/en/agents-and-tools/mcp-connector). For building your own MCP client, see the Model Context Protocol guide to [building an MCP client](https://modelcontextprotocol.io/docs/develop/build-client).

## When Claude uses tools

With the default `tool_choice` of `{"type": "auto"}`, Claude determines on each turn whether to call a tool or respond directly. It calls a tool when the request maps to that tool's described capability and the answer isn't already in context. It responds directly for stable knowledge, creative tasks, and conversational turns.

This boundary is steerable through your system prompt. If Claude isn't calling tools when you expect, a light instruction such as `"Use the tools to investigate before responding."` increases tool use. A stronger form such as `"Always call a tool first before responding."` pushes further. Conversely, `"Use your judgment about whether to call a tool or respond directly."` keeps triggering behavior conservative.

To require a tool call rather than rely on prompting, set [`tool_choice`](/docs/en/agents-and-tools/tool-use/define-tools#forcing-tool-use).



**Guarantee schema conformance with strict tool use**

Add `strict: true` to your custom tool definitions to ensure Claude's tool calls always match your schema exactly. See [Strict tool use](/docs/en/agents-and-tools/tool-use/strict-tool-use).

Each server tool's page describes its own trigger boundary in more detail.

## Choose a tool

For `type` strings, versions, and beta headers, see [Tool reference](/docs/en/agents-and-tools/tool-use/tool-reference).

### Your own tools

For tools you define, you write the schema and your application executes each call.

[Define tools

Specify tool schemas, write descriptions, and control when Claude calls your tools.](/docs/en/agents-and-tools/tool-use/define-tools)[Handle tool calls

Parse `tool_use` blocks, format `tool_result` responses, and handle errors.](/docs/en/agents-and-tools/tool-use/handle-tool-calls)

### Anthropic-schema client tools

Anthropic publishes the schema and trains Claude on it. Your application still executes each call and returns the `tool_result`.

[Memory tool

Store and retrieve information across conversations in files you control.](/docs/en/agents-and-tools/tool-use/memory-tool)[Bash tool

Run shell commands in a persistent session that maintains state.](/docs/en/agents-and-tools/tool-use/bash-tool)[

Text editor tool

View and modify text files to debug, fix, and improve code.](/docs/en/agents-and-tools/tool-use/text-editor-tool)[

Computer use tool

Take screenshots and control the mouse and keyboard in a desktop environment.](/docs/en/agents-and-tools/tool-use/computer-use-tool)

### Server tools

Server tools run on Anthropic's infrastructure, with no handler code in your application. See [Server tools](/docs/en/agents-and-tools/tool-use/server-tools) for the mechanics they share.

[Web search tool

Search the web for information beyond the knowledge cutoff, with cited sources.](/docs/en/agents-and-tools/tool-use/web-search-tool)[

Web fetch tool

Retrieve the full content of specified web pages and PDF documents.](/docs/en/agents-and-tools/tool-use/web-fetch-tool)[

Code execution tool

Run Python and bash code in a sandboxed container to analyze data and generate files.](/docs/en/agents-and-tools/tool-use/code-execution-tool)[Advisor tool

Let a faster executor model consult a higher-intelligence advisor model mid-generation.](/docs/en/agents-and-tools/tool-use/advisor-tool)[

Tool search tool

Work with thousands of tools by discovering and loading them on demand.](/docs/en/agents-and-tools/tool-use/tool-search-tool)[

MCP connector

Connect to remote MCP servers from the Messages API without a separate MCP client.](/docs/en/agents-and-tools/mcp-connector)



[Claude Managed Agents](/docs/en/managed-agents/overview) provides a built-in toolset that Claude uses autonomously within a session. For that toolset and the Managed Agents way to add custom tools, see its [Tools](/docs/en/managed-agents/tools) page.

## 

Tool use requests are priced based on:

1. The total number of input tokens sent to the model (including in the `tools` parameter)
2. The number of output tokens generated
3. For server-side tools, additional usage-based pricing (e.g., web search charges per search performed)

Client-side tools are priced the same as any other Claude API request, while server-side tools may incur additional charges based on their specific usage.

The additional tokens from tool use come from:

* The `tools` parameter in API requests (tool names, descriptions, and schemas)
* `tool_use` content blocks in API requests and responses
* `tool_result` content blocks in API requests

When you use `tools`, the API also automatically includes a special system prompt for the model which enables tool use. The number of tool use tokens required for each model are listed below (excluding the additional tokens listed above). Note that the table assumes at least 1 tool is provided. If no `tools` are provided, then a tool choice of `none` uses 0 additional system prompt tokens.

| Model | Tool choice | Tool use system prompt token count |
| --- | --- | --- |
| Claude Opus 4.8 | `auto`, `none`  ---  `any`, `tool` | 290 tokens  ---  410 tokens |
| Claude Opus 4.7 | `auto`, `none`  ---  `any`, `tool` | 675 tokens  ---  804 tokens |
| Claude Opus 4.6 | `auto`, `none`  ---  `any`, `tool` | 497 tokens  ---  589 tokens |
| Claude Opus 4.5 | `auto`, `none`  ---  `any`, `tool` | 496 tokens  ---  588 tokens |
| Claude Opus 4.1 ([deprecated](/docs/en/about-claude/model-deprecations)) | `auto`, `none`  ---  `any`, `tool` | 313 tokens  ---  315 tokens |
| Claude Opus 4 ([retired, except on Google Cloud](/docs/en/about-claude/model-deprecations)) | `auto`, `none`  ---  `any`, `tool` | 313 tokens  ---  315 tokens |
| Claude Sonnet 5 | `auto`, `none`  ---  `any`, `tool` | 354 tokens  ---  474 tokens |
| Claude Sonnet 4.6 | `auto`, `none`  ---  `any`, `tool` | 497 tokens  ---  589 tokens |
| Claude Sonnet 4.5 | `auto`, `none`  ---  `any`, `tool` | 496 tokens  ---  588 tokens |
| Claude Sonnet 4 ([retired, except on Bedrock and Google Cloud](/docs/en/about-claude/model-deprecations)) | `auto`, `none`  ---  `any`, `tool` | 313 tokens  ---  315 tokens |
| Claude Haiku 4.5 | `auto`, `none`  ---  `any`, `tool` | 496 tokens  ---  588 tokens |
| Claude Haiku 3.5 ([retired, except on Bedrock and Google Cloud](/docs/en/about-claude/model-deprecations)) | `auto`, `none`  ---  `any`, `tool` | 264 tokens  ---  355 tokens |

These token counts are added to your normal input and output tokens to calculate the total cost of a request.

See the [Models overview](/docs/en/about-claude/models/overview#latest-models-comparison) table for current per-model prices.

When you send a tool use prompt, like any other API request, the response includes both input and output token counts in the reported `usage` metrics.

Some server tools add usage-based charges on top of tokens: see [Web search tool](/docs/en/agents-and-tools/tool-use/web-search-tool#usage-and-pricing) and [Code execution tool](/docs/en/agents-and-tools/tool-use/code-execution-tool#usage-and-pricing) for their rates.

## Next steps

[How tool use works

Understand the tool use loop, where tools execute, and when to use tools instead of prose.](/docs/en/agents-and-tools/tool-use/how-tool-use-works)[Tutorial: Build a tool-using agent

A guided walkthrough from a single tool call to a production-ready agentic loop.](/docs/en/agents-and-tools/tool-use/build-a-tool-using-agent)[Tool reference

Directory of Anthropic-provided tools and reference for optional tool definition properties.](/docs/en/agents-and-tools/tool-use/tool-reference)

Was this page helpful?
