![OpenAI Developers](/OpenAI_Developers.svg)

## Search the API docs

### Suggested

### Suggested

### Get started

### Core concepts

### Agents SDK

### Tools

### Run and scale

### Evaluation

### Realtime and audio

### Specialized models

### Going live

### Legacy APIs

### Resources

### Getting Started

### Using Codex

### Configuration

### Administration

### Automation

### Learn

### Releases

### Core Concepts

### Plan

### Build

### Deploy

### Conversion apps

### Guides

### Resources

### Get started

### Guides

### File Upload

### API

### Measurement

### Advertiser API

### API Reference

### Recent

### Topics

### Topics

### Contribute

### Categories

### Topics

### Programs

### Events

### Spaces

### Get started

### Core concepts

### Agents SDK

### Tools

### Run and scale

### Evaluation

### Realtime and audio

### Specialized models

### Going live

### Legacy APIs

### Resources

# Tool search

Load deferred tools at runtime so models only import the definitions they need.

Tool search allows the model to dynamically search for and load tools into the model’s context as needed. This allows you to avoid loading all tool definitions into the model’s context up front and **may help reduce overall token usage and cost**. For optimal cost and latency, tool search is designed to **preserve the model’s cache**. When new tools are discovered by the model, they are injected at the end of the context window.

`gpt-5.4`
`tool_search`

To activate tool search, you must do two things:

`tool_search`
`tools`
`defer_loading: true`
`defer_loading: true`

### Use namespaces where possible

You can use tool search with deferred [functions](/api/docs/guides/function-calling#defining-functions), [namespaces](/api/docs/guides/function-calling#defining-namespaces), or [MCP servers](/api/docs/guides/tools-connectors-mcp), but we recommend using namespaces or MCP servers when possible. Our models have primarily been trained to search those surfaces, and token savings are usually more material there.

For namespaces, `defer_loading` applies to the functions inside the namespace, not to the namespace object itself.

`defer_loading`

At the start of a request, the model still sees the name and description of whatever is searchable. For a namespace or MCP server, that means the model sees only the namespace or server name and description at the beginning, without showing details of the individual functions contained within it until the tool search tool loads them. For an individual deferred function, the model still sees the function name and description, so in practice tool search is mostly deferring the parameter schema.

For maximum token savings, we recommend grouping deferred functions into namespaces or MCP servers with clear, high-level descriptions that give the model a strong overview of what is contained within them, so it can effectively search and load only the relevant functions. As a best practice, aim to keep each namespace to fewer than 10 functions for better token efficiency and model performance.

`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
{
 "tools": [
 {
 "type": "namespace",
 "name": "crm",
 "description": "CRM tools for customer lookup and order management.",
 "tools": [
 {
 "type": "function",
 "name": "list_open_orders",
 "description": "List open orders for a customer ID.",
 "defer_loading": true,
 "parameters": {
 "type": "object",
 "properties": {
 "customer_id": { "type": "string" }
 },
 "required": ["customer_id"],
 "additionalProperties": false
 }
 }
 ]
 },
 {
 "type": "tool_search"
 }
 ]
 }`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28`

Namespaces can have a mix of tools that are deferred and not deferred. Tools without `defer_loading: true` are callable immediately, while deferred tools in the same namespace are loaded through tool search.

`defer_loading: true`

### Tool search types

There are two ways to use tool search:

`tool_search_call`
`tool_search_output`

Start with hosted tool search if the candidate tools are already known when
you create the request. Use client-executed tool search when tool discovery
depends on project state, tenant state, or another system your application
controls.

## Hosted tool search

Hosted tool search is the simplest path when you already know the full inventory of [functions](/api/docs/guides/function-calling#defining-functions), [namespaces](/api/docs/guides/function-calling#defining-namespaces), or [MCP servers](/api/docs/guides/tools-connectors-mcp) you want the model to search. You declare them up front, add `{"type": "tool_search"}`, and let the API decide what to load.

`{"type": "tool_search"}`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
29
30
31
32
33
34
35
36
37
38
39
40
41
42
43
44
45
46
47
48
49
50
from openai import OpenAI
client = OpenAI()
crm_namespace = {
 "type": "namespace",
 "name": "crm",
 "description": "CRM tools for customer lookup and order management.",
 "tools": [
 {
 "type": "function",
 "name": "get_customer_profile",
 "description": "Fetch a customer profile by customer ID.",
 "parameters": {
 "type": "object",
 "properties": {
 "customer_id": {"type": "string"},
 },
 "required": ["customer_id"],
 "additionalProperties": False,
 },
 },
 {
 "type": "function",
 "name": "list_open_orders",
 "description": "List open orders for a customer ID.",
 "defer_loading": True,
 "parameters": {
 "type": "object",
 "properties": {
 "customer_id": {"type": "string"},
 },
 "required": ["customer_id"],
 "additionalProperties": False,
 },
 },
 ],
}
response = client.responses.create(
 model="gpt-5.5",
 input="List open orders for customer CUST-12345.",
 tools=[
 crm_namespace,
 {"type": "tool_search"},
 ],
 parallel_tool_calls=False,
)
print(response.output)`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
29
30
31
32
33
34
35
36
37
38
39
40
41
42
43
44
45
46
47
48
49
50`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
29
30
31
32
33
34
35
36
37
38
39
40
41
42
43
44
45
46
47
import OpenAI from "openai";
const client = new OpenAI();
const crmNamespace = {
 type: "namespace",
 name: "crm",
 description: "CRM tools for customer lookup and order management.",
 tools: [
 {
 type: "function",
 name: "get_customer_profile",
 description: "Fetch a customer profile by customer ID.",
 parameters: {
 type: "object",
 properties: {
 customer_id: { type: "string" },
 },
 required: ["customer_id"],
 additionalProperties: false,
 },
 },
 {
 type: "function",
 name: "list_open_orders",
 description: "List open orders for a customer ID.",
 defer_loading: true,
 parameters: {
 type: "object",
 properties: {
 customer_id: { type: "string" },
 },
 required: ["customer_id"],
 additionalProperties: false,
 },
 },
 ],
};
const response = await client.responses.create({
 model: "gpt-5.5",
 input: "List open orders for customer CUST-12345.",
 tools: [crmNamespace, { type: "tool_search" }],
 parallel_tool_calls: false,
});
console.log(response.output);`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
29
30
31
32
33
34
35
36
37
38
39
40
41
42
43
44
45
46
47`

If the model decides it needs a deferred tool, the response includes two additional output items before the eventual function call:

`tool_search_call`
`tool_search_output`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
29
30
31
32
33
34
35
36
37
38
39
40
41
42
43
44
45
46
47
[
 {
 "type": "tool_search_call",
 "execution": "server",
 "call_id": null,
 "status": "completed",
 "arguments": {
 "paths": ["crm"]
 }
 },
 {
 "type": "tool_search_output",
 "execution": "server",
 "call_id": null,
 "status": "completed",
 "tools": [
 {
 "type": "namespace",
 "name": "crm",
 "description": "CRM tools for customer lookup and order management.",
 "tools": [
 {
 "type": "function",
 "name": "list_open_orders",
 "description": "List open orders for a customer ID.",
 "defer_loading": true,
 "parameters": {
 "type": "object",
 "properties": {
 "customer_id": { "type": "string" }
 },
 "required": ["customer_id"],
 "additionalProperties": false
 }
 }
 ]
 }
 ]
 },
 {
 "type": "function_call",
 "name": "list_open_orders",
 "namespace": "crm",
 "call_id": "call_abc123",
 "arguments": "{\"customer_id\":\"CUST-12345\"}"
 }
]`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
29
30
31
32
33
34
35
36
37
38
39
40
41
42
43
44
45
46
47`

In hosted mode, `execution` is set to `server` and `call_id` is set to `null`.

`execution`
`server`
`call_id`
`null`

For more complex tasks, the model can also load multiple namespaces or MCP servers in the same `tool_search_call`. For example, if it needs functions from different namespaces to complete one task, it may choose to search and load those surfaces together before making the subsequent function calls.

`tool_search_call`

## Client-executed tool search

Client-executed tool search gives your application full control over how tool discovery works. This is useful when the available tools depend on information that is not practical to declare in the initial `tools` list.

`tools`

Configure the `tool_search` tool with `execution: "client"` and a schema for the search arguments your application expects:

`tool_search`
`execution: "client"`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
29
30
31
32
33
34
35
36
37
38
39
40
41
42
43
44
45
46
47
48
49
50
51
52
53
54
55
56
57
58
59
60
61
from openai import OpenAI
client = OpenAI()
first_response = client.responses.create(
 model="gpt-5.5",
 input="Find the shipping ETA tool first, then use it for order_42.",
 tools=[
 {
 "type": "tool_search",
 "execution": "client",
 "description": "Find the project-specific tools needed to continue the task.",
 "parameters": {
 "type": "object",
 "properties": {
 "goal": {"type": "string"},
 },
 "required": ["goal"],
 "additionalProperties": False,
 },
 }
 ],
 parallel_tool_calls=False,
)
search_call = next(
 item for item in first_response.output if item.type == "tool_search_call"
)
loaded_tools = [
 {
 "type": "function",
 "name": "get_shipping_eta",
 "description": "Look up shipping ETA details for an order.",
 "defer_loading": True,
 "parameters": {
 "type": "object",
 "properties": {
 "order_id": {"type": "string"},
 },
 "required": ["order_id"],
 "additionalProperties": False,
 },
 }
]
second_response = client.responses.create(
 model="gpt-5.5",
 input=[
 *first_response.output,
 {
 "type": "tool_search_output",
 "execution": "client",
 "call_id": search_call.call_id,
 "status": "completed",
 "tools": loaded_tools,
 },
 ],
)
print(second_response.output)`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
29
30
31
32
33
34
35
36
37
38
39
40
41
42
43
44
45
46
47
48
49
50
51
52
53
54
55
56
57
58
59
60
61`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
29
30
31
32
33
34
35
36
37
38
39
40
41
42
43
44
45
46
47
48
49
50
51
52
53
54
55
56
57
58
59
60
61
import OpenAI from "openai";
const client = new OpenAI();
const firstResponse = await client.responses.create({
 model: "gpt-5.5",
 input: "Find the shipping ETA tool first, then use it for order_42.",
 tools: [
 {
 type: "tool_search",
 execution: "client",
 description: "Find the project-specific tools needed to continue the task.",
 parameters: {
 type: "object",
 properties: {
 goal: { type: "string" },
 },
 required: ["goal"],
 additionalProperties: false,
 },
 },
 ],
 parallel_tool_calls: false,
});
const searchCall = firstResponse.output.find(
 (item) => item.type === "tool_search_call",
);
const loadedTools = [
 {
 type: "function",
 name: "get_shipping_eta",
 description: "Look up shipping ETA details for an order.",
 defer_loading: true,
 parameters: {
 type: "object",
 properties: {
 order_id: { type: "string" },
 },
 required: ["order_id"],
 additionalProperties: false,
 },
 },
];
const secondResponse = await client.responses.create({
 model: "gpt-5.5",
 input: [
 ...firstResponse.output,
 {
 type: "tool_search_output",
 execution: "client",
 call_id: searchCall.call_id,
 status: "completed",
 tools: loadedTools,
 },
 ],
});
console.log(secondResponse.output);`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
29
30
31
32
33
34
35
36
37
38
39
40
41
42
43
44
45
46
47
48
49
50
51
52
53
54
55
56
57
58
59
60
61`

On the first turn, the model emits a `tool_search_call` and stops there:

`tool_search_call`
`1
2
3
4
5
6
7
8
9
10
11
[
 {
 "type": "tool_search_call",
 "execution": "client",
 "call_id": "call_abc123",
 "status": "completed",
 "arguments": {
 "goal": "Find the shipping ETA tool for order_42."
 }
 }
]`
`1
2
3
4
5
6
7
8
9
10
11`

Your application then performs the search and returns a `tool_search_output` with the tools it wants to load:

`tool_search_output`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
[
 {
 "type": "tool_search_output",
 "execution": "client",
 "call_id": "call_abc123",
 "status": "completed",
 "tools": [
 {
 "type": "function",
 "name": "get_shipping_eta",
 "description": "Look up shipping ETA details for an order.",
 "defer_loading": true,
 "parameters": {
 "type": "object",
 "properties": {
 "order_id": { "type": "string" }
 },
 "required": ["order_id"],
 "additionalProperties": false
 }
 }
 ]
 }
]`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24`

On the next turn, the loaded tool is callable like a normal function:

`1
2
3
4
5
6
7
8
9
[
 {
 "type": "function_call",
 "name": "get_shipping_eta",
 "namespace": "get_shipping_eta",
 "call_id": "call_xyz456",
 "arguments": "{\"order_id\":\"order_42\"}"
 }
]`
`1
2
3
4
5
6
7
8
9`

In client mode, `execution` is set to `client` and `call_id` is defined. Echo the same `call_id` from the `tool_search_call` in your `tool_search_output`.

`execution`
`client`
`call_id`
`call_id`
`tool_search_call`
`tool_search_output`

## Advanced usage

### Keep namespace descriptions clear

Make namespace descriptions clear and descriptive of the use case, because the model relies on this description to decide when to load a subset of functions in that namespace. Avoid overly long descriptions. Instead, put richer detail in the deferred function descriptions that are loaded only when needed.

### Understand what gets loaded

`tool_search_output.tools` contains the list of tools that were dynamically loaded by the model. The model will be able to call any of these tools in future turns, so in client mode you do not need to load the same tool again across turns. Tools that were not listed as part of this array will not be available to the model. If you want to disable a loaded tool, you can remove it from the `tool_search_output` item where you define the loaded tool set, but note that changing the loaded tool set will break the model’s cache from that point forward.

`tool_search_output.tools`
`tool_search_output`

### Advanced injection patterns

Most integrations declare tools in the request’s `tools` parameter. Client-executed tool search also supports more advanced patterns where your application returns tools that were not present in the original request. Treat this as an advanced workflow: validate the returned schemas carefully and only expose trusted tool definitions.

`tools`

### Tool search and caching

All tools are loaded at the end of the model’s context window. This holds true for both hosted tool search and client-executed tool search. This allows the model’s cache to be preserved from one request to another, lowering overall costs and boosting speed.

### Add tools at a specific point in the input

For advanced workflows, you can use an `additional_tools` input item to make tools available at a specific point in the conversation. This is useful when your application loads tools outside the normal tool search flow or needs to preserve the ordering of tools added during a previous response.

`additional_tools`

Set `role` to `developer` and include the tools to add in the item’s `tools` array:

`role`
`developer`
`tools`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
{
 "type": "additional_tools",
 "role": "developer",
 "tools": [
 {
 "type": "function",
 "name": "get_customer",
 "description": "Look up a customer by ID.",
 "parameters": {
 "type": "object",
 "properties": {
 "customer_id": { "type": "string" }
 },
 "required": ["customer_id"],
 "additionalProperties": false
 }
 }
 ]
 }`
`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19`

Tools in an `additional_tools` item become available only after that item appears in the input. When you manually round-trip conversation items, preserve the item’s position so the model sees the same tools at the same point in the conversation.

`additional_tools`

## Related guides

## Docs agent

Loading docs agent...
