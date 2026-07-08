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

# Migrate to the Responses API

The [Responses API](/api/docs/api-reference/responses) is our new API primitive, an evolution of [Chat Completions](/api/docs/api-reference/chat) which brings added simplicity and powerful agentic primitives to your integrations.

**While Chat Completions remains supported, Responses is recommended for all new projects.**

## About the Responses API

The Responses API is a unified interface for building powerful, agent-like applications. It contains:

## Responses benefits

The Responses API contains several benefits over Chat Completions:

`web_search`
`image_generation`
`file_search`
`code_interpreter`
`store: true`

| Capabilities | Chat Completions API | Responses API |
| --- | --- | --- |
| Text generation |  |  |
| Audio |  | Coming soon |
| Vision |  |  |
| Structured Outputs |  |  |
| Function calling |  |  |
| Web search |  |  |
| File search |  |  |
| Computer use |  |  |
| Code interpreter |  |  |
| MCP |  |  |
| Image generation |  |  |
| Reasoning summaries |  |  |

### Examples

See how the Responses API compares to the Chat Completions API in specific scenarios.

#### Messages vs. Items

Both APIs make it easy to generate output from our models. The input to, and result of, a call to Chat completions is an array of *Messages*, while
the Responses API uses *Items*. An Item is a union of many types, representing the range of possibilities
of model actions. A `message` is a type of Item, as is a `function_call` or `function_call_output`. Unlike a Chat Completions Message, where
many concerns are glued together into one object, Items are distinct from one another and better represent the basic unit of model context.

`message`
`function_call`
`function_call_output`

Additionally, Chat Completions can return multiple parallel generations as `choices`, using the `n` param. In Responses, we’ve removed this param, leaving only one generation.

`choices`
`n`
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
from openai import OpenAI
client = OpenAI()
completion = client.chat.completions.create(
model="gpt-5.5",
messages=[
{
"role": "user",
"content": "Write a one-sentence bedtime story about a unicorn."
}
]
)
print(completion.choices[0].message.content)`
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
14`
`1
2
3
4
5
6
7
8
9
from openai import OpenAI
client = OpenAI()
response = client.responses.create(
model="gpt-5.5",
input="Write a one-sentence bedtime story about a unicorn."
)
print(response.output_text)`
`1
2
3
4
5
6
7
8
9`

When you get a response back from the Responses API, the fields differ slightly.
Instead of a `message`, you receive a typed `response` object with its own `id`.
Responses are stored by default. Chat completions are stored by default for new accounts.
To disable storage when using either API, set `store: false`.

`message`
`response`
`id`
`store: false`

The objects you receive back from these APIs will differ slightly. In Chat Completions, you receive an array of
`choices`, each containing a `message`. In Responses, you receive an array of Items labeled `output`.

`choices`
`message`
`output`
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
 "id": "chatcmpl-C9EDpkjH60VPPIB86j2zIhiR8kWiC",
 "object": "chat.completion",
 "created": 1756315657,
 "model": "gpt-5.5",
 "choices": [
 {
 "index": 0,
 "message": {
 "role": "assistant",
 "content": "Under a blanket of starlight, a sleepy unicorn tiptoed through moonlit meadows, gathering dreams like dew to tuck beneath its silver mane until morning.",
 "refusal": null,
 "annotations": []
 },
 "finish_reason": "stop"
 }
 ],
 ...
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
{
 "id": "resp_68af4030592c81938ec0a5fbab4a3e9f05438e46b5f69a3b",
 "object": "response",
 "created_at": 1756315696,
 "model": "gpt-5.5",
 "output": [
 {
 "id": "rs_68af4030baa48193b0b43b4c2a176a1a05438e46b5f69a3b",
 "type": "reasoning",
 "content": [],
 "summary": []
 },
 {
 "id": "msg_68af40337e58819392e935fb404414d005438e46b5f69a3b",
 "type": "message",
 "status": "completed",
 "content": [
 {
 "type": "output_text",
 "annotations": [],
 "logprobs": [],
 "text": "Under a quilt of moonlight, a drowsy unicorn wandered through quiet meadows, brushing blossoms with her glowing horn so they sighed soft lullabies that carried every dreamer gently to sleep."
 }
 ],
 "role": "assistant"
 }
 ],
 ...
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
28
29`

### Additional differences

`store: false`
`reasoning: none`
`response_format`
`text.format`
`output_text`
`previous_response_id`

## Migrating from Chat Completions

Treat migration as three related changes: send requests to `/v1/responses`, read output from a typed `output` array, and choose how your application will carry state between turns.

`/v1/responses`
`output`

### 1. Update generation endpoints

Start by updating your generation endpoints from `post /v1/chat/completions` to `post /v1/responses`.

`post /v1/chat/completions`
`post /v1/responses`

If you are not using functions or multimodal inputs, simple message inputs are compatible from one API to the other:

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
INPUT='[
 { "role": "system", "content": "You are a helpful assistant." },
 { "role": "user", "content": "Hello!" }
]'
curl -s https://api.openai.com/v1/chat/completions \
 -H "Content-Type: application/json" \
 -H "Authorization: Bearer $OPENAI_API_KEY" \
 -d "{
 \"model\": \"gpt-5.5\",
 \"messages\": $INPUT
 }"
curl -s https://api.openai.com/v1/responses \
 -H "Content-Type: application/json" \
 -H "Authorization: Bearer $OPENAI_API_KEY" \
 -d "{
 \"model\": \"gpt-5.5\",
 \"input\": $INPUT
 }"`
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
20`
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
const context = [
 { role: 'system', content: 'You are a helpful assistant.' },
 { role: 'user', content: 'Hello!' }
];
const completion = await client.chat.completions.create({
 model: 'gpt-5.5',
 messages: context
});
const response = await client.responses.create({
 model: "gpt-5.5",
 input: context
});`
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
14`
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
context = [
 { "role": "system", "content": "You are a helpful assistant." },
 { "role": "user", "content": "Hello!" }
]
completion = client.chat.completions.create(
 model="gpt-5.5",
 messages=context
)
response = client.responses.create(
 model="gpt-5.5",
 input=context
)`
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
14`

Chat Completions

With Chat Completions, you create a `messages` array and read the model text
from `completion.choices[0].message.content`.

`messages`
`completion.choices[0].message.content`
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
import OpenAI from 'openai';
const client = new OpenAI({ apiKey: process.env.OPENAI_API_KEY });
const completion = await client.chat.completions.create({
 model: 'gpt-5.5',
 messages: [
 { 'role': 'system', 'content': 'You are a helpful assistant.' },
 { 'role': 'user', 'content': 'Hello!' }
 ]
});
console.log(completion.choices[0].message.content);`
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
from openai import OpenAI
client = OpenAI()
completion = client.chat.completions.create(
 model="gpt-5.5",
 messages=[
 {"role": "system", "content": "You are a helpful assistant."},
 {"role": "user", "content": "Hello!"}
 ]
)
print(completion.choices[0].message.content)`
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
curl https://api.openai.com/v1/chat/completions \
 -H "Content-Type: application/json" \
 -H "Authorization: Bearer $OPENAI_API_KEY" \
 -d '{
 "model": "gpt-5.5",
 "messages": [
 {"role": "system", "content": "You are a helpful assistant."},
 {"role": "user", "content": "Hello!"}
 ]
 }'`
`1
2
3
4
5
6
7
8
9
10`

Responses

With Responses, you can separate `instructions` and `input` at the top level
and read generated text from `response.output_text`.

`instructions`
`input`
`response.output_text`
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
import OpenAI from 'openai';
const client = new OpenAI({ apiKey: process.env.OPENAI_API_KEY });
const response = await client.responses.create({
 model: 'gpt-5.5',
 instructions: 'You are a helpful assistant.',
 input: 'Hello!'
});
console.log(response.output_text);`
`1
2
3
4
5
6
7
8
9
10`
`1
2
3
4
5
6
7
8
9
from openai import OpenAI
client = OpenAI()
response = client.responses.create(
 model="gpt-5.5",
 instructions="You are a helpful assistant.",
 input="Hello!"
)
print(response.output_text)`
`1
2
3
4
5
6
7
8
9`
`1
2
3
4
5
6
7
8
curl https://api.openai.com/v1/responses \
 -H "Content-Type: application/json" \
 -H "Authorization: Bearer $OPENAI_API_KEY" \
 -d '{
 "model": "gpt-5.5",
 "instructions": "You are a helpful assistant.",
 "input": "Hello!"
 }'`
`1
2
3
4
5
6
7
8`

### 2. Map Messages to Items

Chat Completions uses `messages` as both input and output. Responses uses `input` and `output` arrays of typed Items. A `message` is one Item type, alongside Items such as `reasoning`, `function_call`, and `function_call_output`.

`messages`
`input`
`output`
`message`
`reasoning`
`function_call`
`function_call_output`

| Chat Completions concept | Responses mapping |
| --- | --- |
| `messages[]` | `input`, as a string or an array of input Items |
| System or developer guidance | Top-level `instructions`, or compatible message Items when you need to preserve an existing transcript |
| User message | An input message Item with `role: "user"` |
| Assistant message | An output message Item in `response.output`; pass it back in `input` if you manually manage state |
| Tool or function call | A `function_call` output Item |
| Tool or function result | A `function_call_output` input Item linked to the call with `call_id` |
| Multiple generations with `n` | Not available in Responses; make separate requests if you need multiple candidate outputs |

`messages[]`
`input`
`instructions`
`role: "user"`
`response.output`
`input`
`function_call`
`function_call_output`
`call_id`
`n`

When you only need the final text, use the SDK `output_text` helper. When your flow uses reasoning, tools, or multimodal output, iterate over `response.output` and handle each Item by its `type`.

`output_text`
`response.output`
`type`

### 3. Update multi-turn conversations

If you have multi-turn conversations in your application, update your context logic. Responses gives you three common state-management options:

`previous_response_id`
`instructions`
`previous_response_id`
`instructions`
`output`

Chat Completions

In Chat Completions, you store the transcript and send the accumulated
`messages` array on each request.

`messages`
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
let messages = [
 { 'role': 'system', 'content': 'You are a helpful assistant.' },
 { 'role': 'user', 'content': 'What is the capital of France?' }
 ];
const res1 = await client.chat.completions.create({
 model: 'gpt-5.5',
 messages
});
messages = messages.concat([res1.choices[0].message]);
messages.push({ 'role': 'user', 'content': 'And its population?' });
const res2 = await client.chat.completions.create({
 model: 'gpt-5.5',
 messages
});`
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
16`
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
messages = [
 {"role": "system", "content": "You are a helpful assistant."},
 {"role": "user", "content": "What is the capital of France?"}
]
res1 = client.chat.completions.create(model="gpt-5.5", messages=messages)
messages += [res1.choices[0].message]
messages += [{"role": "user", "content": "And its population?"}]
res2 = client.chat.completions.create(model="gpt-5.5", messages=messages)`
`1
2
3
4
5
6
7
8
9
10`

Responses

With Responses, you can manually pass outputs from one response into the
input of another.

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
context = [
 { "role": "user", "content": "What is the capital of France?" }
]
res1 = client.responses.create(
 model="gpt-5.5",
 input=context,
)
# Append the first response's output to context
context += res1.output
# Add the next user message
context += [
 { "role": "user", "content": "And its population?" }
]
res2 = client.responses.create(
 model="gpt-5.5",
 input=context,
)`
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
20`
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
let context = [
 { role: "user", content: "What is the capital of France?" }
];
const res1 = await client.responses.create({
 model: "gpt-5.5",
 input: context,
});
// Append the first response’s output to context
context = context.concat(res1.output);
// Add the next user message
context.push({ role: "user", content: "And its population?" });
const res2 = await client.responses.create({
 model: "gpt-5.5",
 input: context,
});`
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

You can also use `previous_response_id` to reference the previous response
and create response chains or forks.

`previous_response_id`
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
const res1 = await client.responses.create({
 model: 'gpt-5.5',
 input: 'What is the capital of France?',
 store: true
});
const res2 = await client.responses.create({
 model: 'gpt-5.5',
 input: 'And its population?',
 previous_response_id: res1.id,
 store: true
});`
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
12`
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
res1 = client.responses.create(
 model="gpt-5.5",
 input="What is the capital of France?",
 store=True
)
res2 = client.responses.create(
 model="gpt-5.5",
 input="And its population?",
 previous_response_id=res1.id,
 store=True
)`
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
12`

Even when using `previous_response_id`, all previous input tokens for responses in the chain are billed as input tokens in the API.

`previous_response_id`

### 4. Decide when to use statefulness

Responses are stored by default. Chat Completions are stored by default for new accounts. To disable storage in either API, set `store: false`.

`store: false`

Some organizations, such as those with Zero Data Retention (ZDR) requirements, cannot use the Responses API in a stateful way due to compliance or data retention policies. To support these cases, OpenAI offers encrypted reasoning items, allowing you to keep your workflow stateless while still benefiting from reasoning items.

To disable statefulness but still take advantage of reasoning:

`store: false`
`["reasoning.encrypted_content"]`

The API will then return an encrypted version of the reasoning tokens, which you can pass back in future requests just like regular reasoning items.
For ZDR organizations, OpenAI enforces `store: false` automatically. When a request includes `encrypted_content`, it is decrypted in memory, used for generating the next response, and then securely discarded. Any new reasoning tokens are immediately encrypted and returned to you, ensuring no intermediate state is persisted.

`store: false`
`encrypted_content`

### 5. Update function definitions and outputs

There are two minor, but notable, differences in how functions are defined between Chat Completions and Responses.

`strict`
`strict: false`
`strict: false`

The Responses API function example on the right is functionally equivalent to the Chat Completions example on the left.

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
{
 "type": "function",
 "function": {
 "name": "get_weather",
 "description": "Determine weather in my location",
 "strict": true,
 "parameters": {
 "type": "object",
 "properties": {
 "location": {
 "type": "string",
 },
 },
 "additionalProperties": false,
 "required": [
 "location"
 ]
 }
 }
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
20`
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
{
 "type": "function",
 "name": "get_weather",
 "description": "Determine weather in my location",
 "parameters": {
 "type": "object",
 "properties": {
 "location": {
 "type": "string",
 },
 },
 "additionalProperties": false,
 "required": [
 "location"
 ]
 }
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
17`

#### Follow function-calling best practices

In Responses, tool calls and their outputs are two distinct types of Items that are correlated using a `call_id`. See
the [function calling docs](/api/docs/guides/function-calling#function-tool-example) for more detail on how function calling works in Responses.

`call_id`

### 6. Update Structured Outputs definitions

In the Responses API, Structured Outputs definitions have moved from `response_format` to `text.format`:

`response_format`
`text.format`
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
curl https://api.openai.com/v1/chat/completions \
 -H "Content-Type: application/json" \
 -H "Authorization: Bearer $OPENAI_API_KEY" \
 -d '{
 "model": "gpt-5.5",
 "messages": [
 {
 "role": "user",
 "content": "Jane, 54 years old"
 }
 ],
 "response_format": {
 "type": "json_schema",
 "json_schema": {
 "name": "person",
 "strict": true,
 "schema": {
 "type": "object",
 "properties": {
 "name": {
 "type": "string",
 "minLength": 1
 },
 "age": {
 "type": "number",
 "minimum": 0,
 "maximum": 130
 }
 },
 "required": [
 "name",
 "age"
 ],
 "additionalProperties": false
 }
 }
 },
 "reasoning_effort": "medium"
}'`
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
39`
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
from openai import OpenAI
client = OpenAI()
response = client.chat.completions.create(
 model="gpt-5.5",
 messages=[
 {
 "role": "user",
 "content": "Jane, 54 years old",
 }
 ],
 response_format={
 "type": "json_schema",
 "json_schema": {
 "name": "person",
 "strict": True,
 "schema": {
 "type": "object",
 "properties": {
 "name": {
 "type": "string",
 "minLength": 1
 },
 "age": {
 "type": "number",
 "minimum": 0,
 "maximum": 130
 }
 },
 "required": [
 "name",
 "age"
 ],
 "additionalProperties": False
 }
 }
 },
 reasoning_effort="medium"
)`
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
39`
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
const completion = await openai.chat.completions.create({
 model: "gpt-5.5",
 messages: [
 {
 "role": "user",
 "content": "Jane, 54 years old",
 }
 ],
 response_format: {
 type: "json_schema",
 json_schema: {
 name: "person",
 strict: true,
 schema: {
 type: "object",
 properties: {
 name: {
 type: "string",
 minLength: 1
 },
 age: {
 type: "number",
 minimum: 0,
 maximum: 130
 }
 },
 required: [
 "name",
 "age"
 ],
 additionalProperties: false
 }
 }
 },
 reasoning_effort: "medium"
});`
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
36`
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
curl https://api.openai.com/v1/responses \
 -H "Content-Type: application/json" \
 -H "Authorization: Bearer $OPENAI_API_KEY" \
 -d '{
 "model": "gpt-5.5",
 "input": "Jane, 54 years old",
 "text": {
 "format": {
 "type": "json_schema",
 "name": "person",
 "strict": true,
 "schema": {
 "type": "object",
 "properties": {
 "name": {
 "type": "string",
 "minLength": 1
 },
 "age": {
 "type": "number",
 "minimum": 0,
 "maximum": 130
 }
 },
 "required": [
 "name",
 "age"
 ],
 "additionalProperties": false
 }
 }
 }
}'`
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
33`
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
response = client.responses.create(
 model="gpt-5.5",
 input="Jane, 54 years old",
 text={
 "format": {
 "type": "json_schema",
 "name": "person",
 "strict": True,
 "schema": {
 "type": "object",
 "properties": {
 "name": {
 "type": "string",
 "minLength": 1
 },
 "age": {
 "type": "number",
 "minimum": 0,
 "maximum": 130
 }
 },
 "required": [
 "name",
 "age"
 ],
 "additionalProperties": False
 }
 }
 }
)`
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
30`
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
const response = await openai.responses.create({
 model: "gpt-5.5",
 input: "Jane, 54 years old",
 text: {
 format: {
 type: "json_schema",
 name: "person",
 strict: true,
 schema: {
 type: "object",
 properties: {
 name: {
 type: "string",
 minLength: 1
 },
 age: {
 type: "number",
 minimum: 0,
 maximum: 130
 }
 },
 required: [
 "name",
 "age"
 ],
 additionalProperties: false
 }
 },
 }
});`
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
30`

### 7. Update streaming consumers

Chat Completions streaming returns incremental chunks with a `delta` field. Responses streaming uses typed server-sent events. Update stream consumers to branch on each event’s `type` and handle the events your UI or orchestration layer needs.

`delta`
`type`

For text streaming, listen for events such as:

`response.created`
`response.output_text.delta`
`response.completed`
`error`

Function-calling streams can also emit events such as `response.function_call_arguments.delta` and `response.function_call_arguments.done`. See the [streaming Responses guide](/api/docs/guides/streaming-responses?api-mode=responses) and [Responses streaming events reference](/api/docs/api-reference/responses-streaming).

`response.function_call_arguments.delta`
`response.function_call_arguments.done`

### 8. Upgrade to native tools

If your application has use cases that would benefit from OpenAI’s native [tools](/api/docs/guides/tools), you can update your tool calls to use OpenAI’s tools out of the box.

Chat Completions

With Chat Completions, you cannot use OpenAI-hosted tools natively and have
to write your own tool integration.

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
async function web_search(query) {
 const fetch = (await import('node-fetch')).default;
 const res = await fetch(`https://api.example.com/search?q=${query}`);
 const data = await res.json();
 return data.results;
}
const completion = await client.chat.completions.create({
 model: 'gpt-5.5',
 messages: [
 { role: 'system', content: 'You are a helpful assistant.' },
 { role: 'user', content: 'Who is the current president of France?' }
 ],
 functions: [
 {
 name: 'web_search',
 description: 'Search the web for information',
 parameters: {
 type: 'object',
 properties: { query: { type: 'string' } },
 required: ['query']
 }
 }
 ]
});`
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
25`
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
import requests
def web_search(query):
 r = requests.get(f"https://api.example.com/search?q={query}")
 return r.json().get("results", [])
completion = client.chat.completions.create(
 model="gpt-5.5",
 messages=[
 {"role": "system", "content": "You are a helpful assistant."},
 {"role": "user", "content": "Who is the current president of France?"}
 ],
 functions=[
 {
 "name": "web_search",
 "description": "Search the web for information",
 "parameters": {
 "type": "object",
 "properties": {"query": {"type": "string"}},
 "required": ["query"]
 }
 }
 ]
)`
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
`1
2
3
4
curl https://api.example.com/search \
 -G \
 --data-urlencode "q=your+search+term" \
 --data-urlencode "key=$SEARCH_API_KEY"`
`1
2
3
4`

Responses

With Responses, you can specify the tools that you want the model to use.

`1
2
3
4
5
6
7
const answer = await client.responses.create({
 model: 'gpt-5.5',
 input: 'Who is the current president of France?',
 tools: [{ type: 'web_search' }]
});
console.log(answer.output_text);`
`1
2
3
4
5
6
7`
`1
2
3
4
5
6
7
answer = client.responses.create(
 model="gpt-5.5",
 input="Who is the current president of France?",
 tools=[{"type": "web_search"}]
)
print(answer.output_text)`
`1
2
3
4
5
6
7`
`1
2
3
4
5
6
7
8
curl https://api.openai.com/v1/responses \
 -H "Content-Type: application/json" \
 -H "Authorization: Bearer $OPENAI_API_KEY" \
 -d '{
 "model": "gpt-5.5",
 "input": "Who is the current president of France?",
 "tools": [{"type": "web_search"}]
 }'`
`1
2
3
4
5
6
7
8`

### 9. Check common migration errors

Watch for these issues when moving code from Chat Completions to Responses:

`choices[0].message.content`
`response.output_text`
`response.output`
`output`
`call_id`
`response_format`
`text.format`
`previous_response_id`

## Incremental rollout checklist

Chat Completions remains supported, so you can migrate one user flow at a time.

`previous_response_id`
`store: false`
`call_id`
`response_format`
`text.format`

We recommend migrating all flows to the Responses API over time to take advantage of the latest OpenAI features and improvements.

## Assistants API

Based on developer feedback from the [Assistants API](/api/docs/api-reference/assistants) beta, we’ve incorporated key improvements into the Responses API to make it more flexible, faster, and easier to use. The Responses API represents the future direction for building agents on OpenAI.

We now have Assistant-like and Thread-like objects in the Responses API. Learn more in the [migration guide](/api/docs/guides/assistants/migration). As of August 26, 2025, we’re deprecating the Assistants API, with a sunset date of August 26, 2026.

## Docs agent

Loading docs agent...
