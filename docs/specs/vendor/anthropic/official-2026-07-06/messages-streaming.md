### Cookie settings

We use cookies to deliver and improve our services, analyze site usage, and if you agree, to customize or personalize your experience and market our services to you. You can read our Cookie Policy [here](https://www.anthropic.com/legal/cookies).

[Claude Platform Docs](/docs/en/home)

Streaming Messages

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

Messages/Model capabilities

# Streaming messages

Stream Messages API responses incrementally with server-sent events, including text, tool use, and extended thinking deltas.

When creating a Message, you can set `"stream": true` to incrementally stream the response using [server-sent events](https://developer.mozilla.org/en-US/Web/API/Server-sent%5Fevents/Using%5Fserver-sent%5Fevents) (SSE).

## Streaming with SDKs

The [Python](https://github.com/anthropics/anthropic-sdk-python) and [TypeScript](https://github.com/anthropics/anthropic-sdk-typescript) SDKs offer multiple ways of streaming. The [PHP](https://github.com/anthropics/anthropic-sdk-php) SDK provides streaming via `createStream()`. The Python SDK allows both sync and async streams. See the documentation in each SDK for details.

```
client = anthropic.Anthropic() client = anthropic.Anthropic() with client.messages.stream(with client.messages.stream( max_tokens=1024,  max_tokens = 1024, messages=[{"role": "user", "content": "Hello"}],  messages =[{"role": "user", "content": "Hello"}], model="claude-opus-4-8",  model ="claude-opus-4-8",) as stream:) as stream: for text in stream.text_stream:  for  text in stream.text_stream: print(text, end="", flush=True)  print(text, end = "", flush = True)
```

## Get the final message without handling events

If you don't need to process text as it arrives, the SDKs provide a way to use streaming under the hood while returning the complete `Message` object, identical to what `.create()` returns. This is especially useful for requests with large `max_tokens` values, where the SDKs require streaming to avoid HTTP timeouts.

```
client = anthropic.Anthropic() client = anthropic.Anthropic() with client.messages.stream(with client.messages.stream( max_tokens=128000,  max_tokens = 128000, messages=[{"role": "user", "content": "Write a detailed analysis..."}],  messages =[{"role": "user", "content": "Write a detailed analysis..."}], model="claude-opus-4-8",  model ="claude-opus-4-8",) as stream:) as stream: message = stream.get_final_message()  message = stream.get_final_message() print(message.content[0].text) print(message.content[0].text)
```

The `.stream()` call keeps the HTTP connection alive with server-sent events, then `.get_final_message()` (Python) or `.finalMessage()` (TypeScript) accumulates all events and returns the complete `Message` object. In Go, you call `message.Accumulate(event)` inside the stream loop to build the same complete `Message`. In Java, use `MessageAccumulator.create()` and call `accumulator.accumulate(event)` on each event. In C#, await the stream's `.Aggregate()` extension method to get the complete `Message`, or pass a `MessageContentAggregator` to `.CollectAsync()` to aggregate while handling events. In Ruby, call `.accumulated_message` on the stream. In the PHP SDK, you iterate over stream events manually to accumulate the response.

## Event types

Each server-sent event includes a named event type and associated JSON data. Each event uses an SSE event name (for example, `event: message_stop`), and includes the matching event `type` in its data.

Each stream uses the following event flow:

1. `message_start`: contains a `Message` object with empty `content`.
2. A series of content blocks, each of which has a `content_block_start`, one or more `content_block_delta` events, and a `content_block_stop` event. Each content block has an `index` that corresponds to its index in the final Message `content` array. One exception: during [server-side fallback](/docs/en/build-with-claude/refusals-and-fallback#server-side-fallback) responses, a `fallback` content block arrives at each model boundary as a `content_block_start` and `content_block_stop` pair with no deltas in between.
3. One or more `message_delta` events, indicating top-level changes to the final `Message` object.
4. A final `message_stop` event.



The token counts shown in the `usage` field of the `message_delta` event are *cumulative*.

### Ping events

Event streams may also include any number of `ping` events.

### Error events

The API may occasionally send [errors](/docs/en/api/errors) in the event stream. For example, during periods of high usage, you may receive an `overloaded_error`, which would normally correspond to an HTTP 529 in a non-streaming context:

Example error

```
event: errorevent: errordata: {"type": "error", "error": {"type": "overloaded_error", "message": "Overloaded"}}data: {"type": "error", "error": {"type": "overloaded_error", "message": "Overloaded"}}
```

### Other events

In accordance with the [versioning policy](/docs/en/api/versioning), new event types may be added, and your code should handle unknown event types gracefully.

## Content block delta types

Each `content_block_delta` event contains a `delta` of a type that updates the `content` block at a given `index`.

### Text delta

A `text` content block delta looks like:

Text delta

```
event: content_block_deltaevent: content_block_deltadata: {"type": "content_block_delta","index": 0,"delta": {"type": "text_delta", "text": "ello frien"}}data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "ello frien"}}
```

### Input JSON delta

The deltas for `tool_use` content blocks correspond to updates for the `input` field of the block. To support maximum granularity, the deltas are *partial JSON strings*, whereas the final `tool_use.input` is always an *object*.

You can accumulate the string deltas and parse the JSON once you receive a `content_block_stop` event, by using a library like [Pydantic](https://docs.pydantic.dev/latest/concepts/json/#partial-json-parsing) to do partial JSON parsing, or by using the [SDKs](/docs/en/cli-sdks-libraries/overview), which provide helpers to access parsed incremental values.

A `tool_use` content block delta looks like:

Input JSON delta

```
event: content_block_deltaevent: content_block_deltadata: {"type": "content_block_delta","index": 1,"delta": {"type": "input_json_delta","partial_json": "{\"location\": \"San Fra"}}}data: {"type": "content_block_delta", "index": 1, "delta": {"type": "input_json_delta", "partial_json": "{\" location \": \" San Fra"}}}
```

Note: Current models only support emitting one complete key and value property from `input` at a time. As such, when using tools, there may be delays between streaming events while the model is working. Once an `input` key and value are accumulated, they are emitted as multiple `content_block_delta` events with chunked partial json so that the format can automatically support finer granularity in future models.

### Thinking delta

When using [extended thinking](/docs/en/build-with-claude/extended-thinking#streaming-thinking) with streaming enabled, you'll receive thinking content via `thinking_delta` events. These deltas correspond to the `thinking` field of the `thinking` content blocks.

For thinking content, a special `signature_delta` event is sent just before the `content_block_stop` event. This signature is used to verify the integrity of the thinking block.

When `display: "omitted"` is set on the thinking configuration, no `thinking_delta` events are sent. The thinking block opens, receives a single `signature_delta`, and closes. See [Controlling thinking display](/docs/en/build-with-claude/extended-thinking#controlling-thinking-display).

A typical thinking delta looks like:

Thinking delta

```
event: content_block_deltaevent: content_block_deltadata: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "I need to find the GCD of 1071 and 462 using the Euclidean algorithm.\n\n1071 = 2 × 462 + 147"}}data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "I need to find the GCD of 1071 and 462 using the Euclidean algorithm. \n\n1071 = 2 × 462 + 147"}}
```

The signature delta looks like:

Signature delta

```
event: content_block_deltaevent: content_block_deltadata: {"type": "content_block_delta", "index": 0, "delta": {"type": "signature_delta", "signature": "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGgxBdjrkzLoky3dl1pkiMOYds..."}}data: {"type": "content_block_delta", "index": 0, "delta": {"type": "signature_delta", "signature": "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGgxBdjrkzLoky3dl1pkiMOYds..."}}
```

## Full HTTP stream response

Use the [client SDKs](/docs/en/cli-sdks-libraries/overview) when using streaming mode. However, if you are building a direct API integration, you need to handle these events yourself.

A stream response consists of:

1. A `message_start` event
2. Potentially multiple content blocks, each of which contains:
   * A `content_block_start` event
   * Potentially multiple `content_block_delta` events
   * A `content_block_stop` event
3. One or more `message_delta` events
4. A `message_stop` event

There may be `ping` events dispersed throughout the response as well. See [Event types](#event-types) for more details on the format.

### Basic streaming request

```
client = anthropic.Anthropic() client = anthropic.Anthropic() with client.messages.stream(with client.messages.stream( model="claude-opus-4-8",  model ="claude-opus-4-8", messages=[{"role": "user", "content": "Hello"}],  messages =[{"role": "user", "content": "Hello"}], max_tokens=256,  max_tokens = 256,) as stream:) as stream: for text in stream.text_stream:  for  text in stream.text_stream: print(text, end="", flush=True)  print(text, end = "", flush = True)
```

Response

```
event: message_startevent: message_startdata: {"type": "message_start", "message": {"id": "msg_1nZdL29xx5MUA1yADyHTEsnR8uuvGzszyY", "type": "message", "role": "assistant", "content": [], "model": "claude-opus-4-8", "stop_reason": null, "stop_sequence": null, "usage": {"input_tokens": 25, "output_tokens": 1}}}data: {"type": "message_start", "message": {"id": "msg_1nZdL29xx5MUA1yADyHTEsnR8uuvGzszyY", "type": "message", "role": "assistant", "content": [], "model": "claude-opus-4-8", "stop_reason": null, "stop_sequence": null, "usage": {"input_tokens": 25, "output_tokens": 1}}} event: content_block_startevent: content_block_startdata: {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}data: {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}} event: pingevent: pingdata: {"type": "ping"}data: {"type": "ping"} event: content_block_deltaevent: content_block_deltadata: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hello"}}data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hello"}} event: content_block_deltaevent: content_block_deltadata: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "!"}}data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "!"}} event: content_block_stopevent: content_block_stopdata: {"type": "content_block_stop", "index": 0}data: {"type": "content_block_stop", "index": 0} event: message_deltaevent: message_deltadata: {"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence":null}, "usage": {"output_tokens": 15}}data: {"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": null}, "usage": {"output_tokens": 15}} event: message_stopevent: message_stopdata: {"type": "message_stop"}data: {"type": "message_stop"} 
```

### Streaming request with tool use



Tool use supports [fine-grained streaming](/docs/en/agents-and-tools/tool-use/fine-grained-tool-streaming) for parameter values. Enable it per tool with `eager_input_streaming`.

This request asks Claude to use a tool to report the weather.

```
client = anthropic.Anthropic() client = anthropic.Anthropic() tools = [tools = [ { { "name": "get_weather",  "name": "get_weather", "description": "Get the current weather in a given location",  "description": "Get the current weather in a given location", "input_schema": { "input_schema": { "type": "object",  "type": "object", "properties": { "properties": { "location": { "location": { "type": "string",  "type": "string", "description": "The city and state, e.g. San Francisco, CA",  "description": "The city and state, e.g. San Francisco, CA", } } }, }, "required": ["location"],  "required": ["location"], }, }, } }]] with client.messages.stream(with client.messages.stream( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=1024,  max_tokens = 1024, tools=tools,  tools =tools, tool_choice={"type": "any"},  tool_choice ={"type": "any"}, messages=[ messages =[ {"role": "user", "content": "What is the weather like in San Francisco?"} {"role": "user", "content": "What is the weather like in San Francisco?"} ], ],) as stream:) as stream: for text in stream.text_stream:  for  text in stream.text_stream: print(text, end="", flush=True)  print(text, end = "", flush = True)
```

Response

```
event: message_startevent: message_startdata: {"type":"message_start","message":{"id":"msg_014p7gG3wDgGV9EUtLvnow3U","type":"message","role":"assistant","model":"claude-opus-4-8","stop_sequence":null,"usage":{"input_tokens":472,"output_tokens":2},"content":[],"stop_reason":null}}data: {"type": "message_start", "message":{"id": "msg_014p7gG3wDgGV9EUtLvnow3U", "type": "message", "role": "assistant", "model":"claude-opus-4-8", "stop_sequence": null, "usage":{"input_tokens": 472, "output_tokens": 2}, "content":[], "stop_reason": null}} event: content_block_startevent: content_block_startdata: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}data: {"type": "content_block_start", "index": 0, "content_block":{"type": "text", "text": ""}} event: pingevent: pingdata: {"type": "ping"}data: {"type": "ping"} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Okay"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text": "Okay"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":","}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text":","}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" let"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text": " let"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"'s"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text": "'s"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" check"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text": " check"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" the"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text": " the"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" weather"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text": " weather"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" for"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text": " for"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" San"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text": " San"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" Francisco"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text": " Francisco"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":","}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text":","}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" CA"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text": " CA"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":":"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text":":"}} event: content_block_stopevent: content_block_stopdata: {"type":"content_block_stop","index":0}data: {"type": "content_block_stop", "index": 0} event: content_block_startevent: content_block_startdata: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_01T1x1fJ34qAmk2tNTrN7Up6","name":"get_weather","input":{}}}data: {"type": "content_block_start", "index": 1, "content_block":{"type": "tool_use", "id": "toolu_01T1x1fJ34qAmk2tNTrN7Up6", "name": "get_weather", "input":{}}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":""}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json": ""}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"location\":"}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json":"{\" location \":"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":" \"San"}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json": " \" San"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":" Francisc"}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json": " Francisc"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"o,"}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json":"o,"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":" CA\"}"}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json": " CA \"}"}} event: content_block_stopevent: content_block_stopdata: {"type":"content_block_stop","index":1}data: {"type": "content_block_stop", "index": 1} event: message_deltaevent: message_deltadata: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":89}}data: {"type": "message_delta", "delta":{"stop_reason": "tool_use", "stop_sequence": null}, "usage":{"output_tokens": 89}} event: message_stopevent: message_stopdata: {"type":"message_stop"}data: {"type": "message_stop"}
```

### Streaming request with extended thinking

This request enables extended thinking with streaming. The `display: "summarized"` setting streams a condensed summary of Claude's reasoning rather than the full chain of thought.

```
client = anthropic.Anthropic() client = anthropic.Anthropic() with client.messages.stream(with client.messages.stream( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=20000,  max_tokens = 20000, thinking={"type": "adaptive", "display": "summarized"},  thinking ={"type": "adaptive", "display": "summarized"}, messages=[ messages =[ { { "role": "user",  "role": "user", "content": "What is the greatest common divisor of 1071 and 462?",  "content": "What is the greatest common divisor of 1071 and 462?", } } ], ],) as stream:) as stream: for event in stream:  for  event in stream: if event.type == "content_block_delta":  if event.type ==  "content_block_delta": if event.delta.type == "thinking_delta":  if event.delta.type ==  "thinking_delta": print(event.delta.thinking, end="", flush=True)  print(event.delta.thinking, end = "", flush = True) elif event.delta.type == "text_delta":  elif event.delta.type ==  "text_delta": print(event.delta.text, end="", flush=True)  print(event.delta.text, end = "", flush = True)
```

Response

```
event: message_startevent: message_startdata: {"type": "message_start", "message": {"id": "msg_01...", "type": "message", "role": "assistant", "content": [], "model": "claude-opus-4-8", "stop_reason": null, "stop_sequence": null}}data: {"type": "message_start", "message": {"id": "msg_01...", "type": "message", "role": "assistant", "content": [], "model": "claude-opus-4-8", "stop_reason": null, "stop_sequence": null}} event: content_block_startevent: content_block_startdata: {"type": "content_block_start", "index": 0, "content_block": {"type": "thinking", "thinking": "", "signature": ""}}data: {"type": "content_block_start", "index": 0, "content_block": {"type": "thinking", "thinking": "", "signature": ""}} event: content_block_deltaevent: content_block_deltadata: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "I need to find the GCD of 1071 and 462 using the Euclidean algorithm.\n\n1071 = 2 × 462 + 147"}}data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "I need to find the GCD of 1071 and 462 using the Euclidean algorithm. \n\n1071 = 2 × 462 + 147"}} event: content_block_deltaevent: content_block_deltadata: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "\n462 = 3 × 147 + 21"}}data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": " \n462 = 3 × 147 + 21"}} event: content_block_deltaevent: content_block_deltadata: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "\n147 = 7 × 21 + 0"}}data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": " \n147 = 7 × 21 + 0"}} event: content_block_deltaevent: content_block_deltadata: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "\nThe remainder is 0, so GCD(1071, 462) = 21."}}data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": " \nThe remainder is 0, so GCD(1071, 462) = 21."}} event: content_block_deltaevent: content_block_deltadata: {"type": "content_block_delta", "index": 0, "delta": {"type": "signature_delta", "signature": "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGgxBdjrkzLoky3dl1pkiMOYds..."}}data: {"type": "content_block_delta", "index": 0, "delta": {"type": "signature_delta", "signature": "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGgxBdjrkzLoky3dl1pkiMOYds..."}} event: content_block_stopevent: content_block_stopdata: {"type": "content_block_stop", "index": 0}data: {"type": "content_block_stop", "index": 0} event: content_block_startevent: content_block_startdata: {"type": "content_block_start", "index": 1, "content_block": {"type": "text", "text": ""}}data: {"type": "content_block_start", "index": 1, "content_block": {"type": "text", "text": ""}} event: content_block_deltaevent: content_block_deltadata: {"type": "content_block_delta", "index": 1, "delta": {"type": "text_delta", "text": "The greatest common divisor of 1071 and 462 is **21**."}}data: {"type": "content_block_delta", "index": 1, "delta": {"type": "text_delta", "text": "The greatest common divisor of 1071 and 462 is **21**."}} event: content_block_stopevent: content_block_stopdata: {"type": "content_block_stop", "index": 1}data: {"type": "content_block_stop", "index": 1} event: message_deltaevent: message_deltadata: {"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": null}}data: {"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": null}} event: message_stopevent: message_stopdata: {"type": "message_stop"}data: {"type": "message_stop"}
```

### Streaming request with web search tool use

This request asks Claude to search the web for current weather information.

```
client = anthropic.Anthropic() client = anthropic.Anthropic() with client.messages.stream(with client.messages.stream( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=1024,  max_tokens = 1024, tools=[{"type": "web_search_20250305", "name": "web_search", "max_uses": 5}],  tools =[{"type": "web_search_20250305", "name": "web_search", "max_uses": 5}], messages=[ messages =[ {"role": "user", "content": "What is the weather like in New York City today?"} {"role": "user", "content": "What is the weather like in New York City today?"} ], ],) as stream:) as stream: for text in stream.text_stream:  for  text in stream.text_stream: print(text, end="", flush=True)  print(text, end = "", flush = True)
```

Response

```
event: message_startevent: message_startdata: {"type":"message_start","message":{"id":"msg_01G...","type":"message","role":"assistant","model":"claude-opus-4-8","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":2679,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":3}}}data: {"type": "message_start", "message":{"id":"msg_01G...", "type": "message", "role": "assistant", "model":"claude-opus-4-8", "content":[], "stop_reason": null, "stop_sequence": null, "usage":{"input_tokens": 2679, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": 3}}} event: content_block_startevent: content_block_startdata: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}data: {"type": "content_block_start", "index": 0, "content_block":{"type": "text", "text": ""}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"I'll check"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text": "I'll check"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" the current weather in New York City for you"}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text": " the current weather in New York City for you"}} event: pingevent: pingdata: {"type": "ping"}data: {"type": "ping"} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"."}}data: {"type": "content_block_delta", "index": 0, "delta":{"type": "text_delta", "text":"."}} event: content_block_stopevent: content_block_stopdata: {"type":"content_block_stop","index":0}data: {"type": "content_block_stop", "index": 0} event: content_block_startevent: content_block_startdata: {"type":"content_block_start","index":1,"content_block":{"type":"server_tool_use","id":"srvtoolu_014hJH82Qum7Td6UV8gDXThB","name":"web_search","input":{}}}data: {"type": "content_block_start", "index": 1, "content_block":{"type": "server_tool_use", "id": "srvtoolu_014hJH82Qum7Td6UV8gDXThB", "name": "web_search", "input":{}}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":""}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json": ""}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"query"}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json":"{\" query"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\":"}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json": " \":"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":" \"weather"}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json": " \" weather"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":" NY"}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json": " NY"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"C to"}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json": "C to"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"day\"}"}}data: {"type": "content_block_delta", "index": 1, "delta":{"type": "input_json_delta", "partial_json": "day \"}"}} event: content_block_stopevent: content_block_stopdata: {"type":"content_block_stop","index":1 }data: {"type": "content_block_stop", "index": 1 } event: content_block_startevent: content_block_startdata: {"type":"content_block_start","index":2,"content_block":{"type":"web_search_tool_result","tool_use_id":"srvtoolu_014hJH82Qum7Td6UV8gDXThB","content":[{"type":"web_search_result","title":"Weather in New York City in May 2025 (New York) - detailed Weather Forecast for a month","url":"https://world-weather.info/forecast/usa/new_york/may-2025/","encrypted_content":"Ev0DCioIAxgCIiQ3NmU4ZmI4OC1k...","page_age":null},...]}}data: {"type": "content_block_start", "index": 2, "content_block":{"type": "web_search_tool_result", "tool_use_id": "srvtoolu_014hJH82Qum7Td6UV8gDXThB", "content":[{"type": "web_search_result", "title":"Weather in New York City in May 2025 (New York) - detailed Weather Forecast for a month", "url":"https://world-weather.info/forecast/usa/new_york/may-2025/", "encrypted_content":"Ev0DCioIAxgCIiQ3NmU4ZmI4OC1k...", "page_age": null},...]}} event: content_block_stopevent: content_block_stopdata: {"type":"content_block_stop","index":2}data: {"type": "content_block_stop", "index": 2} event: content_block_startevent: content_block_startdata: {"type":"content_block_start","index":3,"content_block":{"type":"text","text":""}}data: {"type": "content_block_start", "index": 3, "content_block":{"type": "text", "text": ""}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":"Here's the current weather information for New York"}}data: {"type": "content_block_delta", "index": 3, "delta":{"type": "text_delta", "text": "Here's the current weather information for New York"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":" City:\n\n# Weather"}}data: {"type": "content_block_delta", "index": 3, "delta":{"type": "text_delta", "text":" City: \n\n # Weather"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":" in New York City"}}data: {"type": "content_block_delta", "index": 3, "delta":{"type": "text_delta", "text": " in New York City"}} event: content_block_deltaevent: content_block_deltadata: {"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":"\n\n"}}data: {"type": "content_block_delta", "index": 3, "delta":{"type": "text_delta", "text": " \n\n "}} ...... event: content_block_stopevent: content_block_stopdata: {"type":"content_block_stop","index":17}data: {"type": "content_block_stop", "index": 17} event: message_deltaevent: message_deltadata: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"input_tokens":10682,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":510,"server_tool_use":{"web_search_requests":1}}}data: {"type": "message_delta", "delta":{"stop_reason": "end_turn", "stop_sequence": null}, "usage":{"input_tokens": 10682, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": 510, "server_tool_use":{"web_search_requests": 1}}} event: message_stopevent: message_stopdata: {"type":"message_stop"}data: {"type": "message_stop"}
```

## Error recovery

### Claude 4.5 and earlier

For Claude 4.5 models and earlier, you can recover a streaming request that was interrupted due to network issues, timeouts, or other errors by resuming from where the stream was interrupted. This approach saves you from re-processing the entire response.

The basic recovery strategy involves:

1. **Capture the partial response:** Save all content that was successfully received before the error occurred
2. **Construct a continuation request:** Create a new API request that includes the partial assistant response as the beginning of a new assistant message
3. **Resume streaming:** Continue receiving the rest of the response from where it was interrupted

### Claude 4.6 and later

For Claude 4.6 and later models, the same capture-and-resume strategy applies, but step 2 changes: instead of placing the partial response in an assistant message, add a user message that instructs the model to continue from where it left off.

1. **Capture the partial response:** Save all content that was successfully received before the error occurred
2. **Construct a continuation request:** Create a new API request with a user message containing the partial response and an instruction to continue, for example:

   Sample prompt

   ```
   Your previous response was interrupted and ended with [previous_response]. Continue from where you left off.
   ```
3. **Resume streaming:** Continue receiving the rest of the response from where it was interrupted

### Error recovery best practices

1. **Use SDK features:** Leverage the SDK's built-in message accumulation and error handling capabilities
2. **Handle content types:** Be aware that messages can contain multiple content blocks (`text`, `tool_use`, `thinking`). Tool use and extended thinking blocks cannot be partially recovered. You can resume streaming from the most recent text block.

## Next steps

[Stop reasons and fallback

Handle each `stop_reason` value once a stream completes.](/docs/en/build-with-claude/handling-stop-reasons)[Fine-grained tool streaming

Stream tool input JSON without server-side buffering for lower latency.](/docs/en/agents-and-tools/tool-use/fine-grained-tool-streaming)[Extended thinking

Stream extended thinking output with `thinking_delta` and `signature_delta` events.](/docs/en/build-with-claude/extended-thinking)[Client SDKs

Use the official SDKs, which handle streaming, accumulation, and reconnection for you.](/docs/en/cli-sdks-libraries/overview)[Batch processing

Process large volumes of requests asynchronously when you don't need real-time responses.](/docs/en/build-with-claude/batch-processing)

Was this page helpful?
