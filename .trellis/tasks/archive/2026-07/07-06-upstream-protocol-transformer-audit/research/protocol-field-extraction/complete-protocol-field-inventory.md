# Complete protocol field inventory against AxonHub code

Sources: OpenAI OpenAPI YAML, Anthropic official raw docs + MCP connector companion docs, upstream/current Go structs.

## OpenAI Chat request

Code target: `llm/transformer/openai/model.go` struct `Request`

| Field | Required | Official type | Upstream top-level? | Current top-level? | Upstream any json tag? | Author handling |
|---|---:|---|---:|---:|---:|---|
| `metadata` | no | `Metadata` | yes | yes | yes | native top-level struct |
| `top_logprobs` | no | `anyOf(integer | null)` | yes | yes | yes | native top-level struct |
| `temperature` | no | `anyOf(number | null)` | yes | yes | yes | native top-level struct |
| `top_p` | no | `anyOf(number | null)` | yes | yes | yes | native top-level struct |
| `user` | no | `string` | yes | yes | yes | native top-level struct |
| `safety_identifier` | no | `string` | yes | yes | yes | native top-level struct |
| `prompt_cache_key` | no | `string` | yes | yes | yes | native top-level struct |
| `service_tier` | no | `ServiceTier` | yes | yes | yes | native top-level struct |
| `prompt_cache_retention` | no | `anyOf(string | null)` | no | no | yes | nested/response/helper struct only |
| `messages` | yes | `array[ChatCompletionRequestMessage]` | yes | yes | yes | native top-level struct |
| `model` | yes | `ModelIdsShared` | yes | yes | yes | native top-level struct |
| `modalities` | no | `ResponseModalities` | yes | yes | yes | native top-level struct |
| `verbosity` | no | `Verbosity` | yes | yes | yes | native top-level struct |
| `reasoning_effort` | no | `ReasoningEffort` | yes | yes | yes | native top-level struct |
| `max_completion_tokens` | no | `integer` | yes | yes | yes | native top-level struct |
| `frequency_penalty` | no | `number` | yes | yes | yes | native top-level struct |
| `presence_penalty` | no | `number` | yes | yes | yes | native top-level struct |
| `web_search_options` | no | `object` | no | no | no | missing in upstream request; modern Chat native field candidate |
| `response_format` | no | `oneOf(ResponseFormatText | ResponseFormatJsonSchema | ResponseFormatJsonObject)` | yes | yes | yes | native top-level struct |
| `audio` | no | `object` | no | no | yes | nested/response/helper struct only |
| `store` | no | `boolean` | yes | yes | yes | native top-level struct |
| `stream` | no | `boolean` | yes | yes | yes | native top-level struct |
| `stop` | no | `StopConfiguration` | yes | yes | yes | native top-level struct |
| `logit_bias` | no | `object` | yes | yes | yes | native top-level struct |
| `logprobs` | no | `boolean` | yes | yes | yes | native top-level struct |
| `max_tokens` | no | `integer` | yes | yes | yes | native top-level struct |
| `n` | no | `integer` | no | no | no | missing/not modeled |
| `prediction` | no | `oneOf(PredictionContent)` | no | no | no | missing in upstream request; modern Chat native field candidate |
| `seed` | no | `integer` | yes | yes | yes | native top-level struct |
| `stream_options` | no | `ChatCompletionStreamOptions` | yes | yes | yes | native top-level struct |
| `tools` | no | `array[oneOf(ChatCompletionTool | CustomToolChatCompletions)]` | yes | yes | yes | native top-level struct |
| `tool_choice` | no | `ChatCompletionToolChoiceOption` | yes | yes | yes | native top-level struct |
| `parallel_tool_calls` | no | `ParallelToolCalls` | yes | yes | yes | native top-level struct |
| `function_call` | no | `oneOf(string | ChatCompletionFunctionCallOption)` | no | no | no | missing/not modeled |
| `functions` | no | `array[ChatCompletionFunctions]` | no | no | no | missing/not modeled |

## OpenAI Chat response

Code target: `llm/transformer/openai/model.go` struct `Response`

| Field | Required | Official type | Upstream top-level? | Current top-level? | Upstream any json tag? | Author handling |
|---|---:|---|---:|---:|---:|---|
| `id` | yes | `string` | yes | yes | yes | native top-level struct |
| `choices` | yes | `array[object]` | yes | yes | yes | native top-level struct |
| `created` | yes | `integer` | yes | yes | yes | native top-level struct |
| `model` | yes | `string` | yes | yes | yes | native top-level struct |
| `service_tier` | no | `ServiceTier` | yes | yes | yes | native top-level struct |
| `system_fingerprint` | no | `string` | yes | yes | yes | native top-level struct |
| `object` | yes | `string` | yes | yes | yes | native top-level struct |
| `usage` | no | `CompletionUsage` | yes | yes | yes | native top-level struct |

## OpenAI Responses request

Code target: `llm/transformer/openai/responses/model.go` struct `Request`

| Field | Required | Official type | Upstream top-level? | Current top-level? | Upstream any json tag? | Author handling |
|---|---:|---|---:|---:|---:|---|
| `metadata` | no | `Metadata` | yes | yes | yes | native top-level struct |
| `top_logprobs` | no | `anyOf(integer | null)` | yes | yes | yes | native top-level struct |
| `temperature` | no | `anyOf(number | null)` | yes | yes | yes | native top-level struct |
| `top_p` | no | `anyOf(number | null)` | yes | yes | yes | native top-level struct |
| `user` | no | `string` | yes | yes | yes | native top-level struct |
| `safety_identifier` | no | `string` | yes | yes | yes | native top-level struct |
| `prompt_cache_key` | no | `string` | yes | yes | yes | native top-level struct |
| `service_tier` | no | `ServiceTier` | yes | yes | yes | native top-level struct |
| `prompt_cache_retention` | no | `anyOf(string | null)` | yes | yes | yes | native top-level struct |
| `previous_response_id` | no | `anyOf(string | null)` | yes | yes | yes | native top-level struct |
| `model` | no | `ModelIdsResponses` | yes | yes | yes | native top-level struct |
| `reasoning` | no | `anyOf(Reasoning | null)` | yes | yes | yes | native top-level struct |
| `background` | no | `anyOf(boolean | null)` | yes | yes | yes | native top-level struct |
| `max_tool_calls` | no | `anyOf(integer | null)` | yes | yes | yes | native top-level struct |
| `text` | no | `ResponseTextParam` | yes | yes | yes | native top-level struct |
| `tools` | no | `ToolsArray` | yes | yes | yes | native top-level struct |
| `tool_choice` | no | `ToolChoiceParam` | yes | yes | yes | native top-level struct |
| `prompt` | no | `Prompt` | no | yes | yes | nested/response/helper struct only |
| `truncation` | no | `anyOf(string | null)` | yes | yes | yes | native top-level struct |
| `input` | no | `InputParam` | yes | yes | yes | native top-level struct |
| `include` | no | `anyOf(array[IncludeEnum] | null)` | yes | yes | yes | native top-level struct |
| `parallel_tool_calls` | no | `anyOf(boolean | null)` | yes | yes | yes | native top-level struct |
| `store` | no | `anyOf(boolean | null)` | yes | yes | yes | native top-level struct |
| `instructions` | no | `anyOf(string | null)` | yes | yes | yes | native top-level struct |
| `stream` | no | `anyOf(boolean | null)` | yes | yes | yes | native top-level struct |
| `stream_options` | no | `ResponseStreamOptions` | yes | yes | yes | native top-level struct |
| `conversation` | no | `anyOf(ConversationParam | null)` | no | no | yes | nested/response/helper struct only |
| `context_management` | no | `anyOf(array[ContextManagementParam] | null)` | no | no | no | missing in upstream request; should be native/opaque request field |
| `max_output_tokens` | no | `anyOf(integer | null)` | yes | yes | yes | native top-level struct |

## OpenAI Responses response

Code target: `llm/transformer/openai/responses/model.go` struct `Response`

| Field | Required | Official type | Upstream top-level? | Current top-level? | Upstream any json tag? | Author handling |
|---|---:|---|---:|---:|---:|---|
| `metadata` | yes | `Metadata` | yes | yes | yes | native top-level struct |
| `top_logprobs` | no | `anyOf(integer | null)` | yes | yes | yes | native top-level struct |
| `temperature` | yes | `anyOf(number | null)` | yes | yes | yes | native top-level struct |
| `top_p` | yes | `anyOf(number | null)` | yes | yes | yes | native top-level struct |
| `user` | no | `string` | yes | yes | yes | native top-level struct |
| `safety_identifier` | no | `string` | yes | yes | yes | native top-level struct |
| `prompt_cache_key` | no | `string` | yes | yes | yes | native top-level struct |
| `service_tier` | no | `ServiceTier` | yes | yes | yes | native top-level struct |
| `prompt_cache_retention` | no | `anyOf(string | null)` | yes | yes | yes | native top-level struct |
| `previous_response_id` | no | `anyOf(string | null)` | yes | yes | yes | native top-level struct |
| `model` | yes | `ModelIdsResponses` | yes | yes | yes | native top-level struct |
| `reasoning` | no | `anyOf(Reasoning | null)` | yes | yes | yes | native top-level struct |
| `background` | no | `anyOf(boolean | null)` | yes | yes | yes | native top-level struct |
| `max_tool_calls` | no | `anyOf(integer | null)` | yes | yes | yes | native top-level struct |
| `text` | no | `ResponseTextParam` | yes | yes | yes | native top-level struct |
| `tools` | yes | `ToolsArray` | yes | yes | yes | native top-level struct |
| `tool_choice` | yes | `ToolChoiceParam` | yes | yes | yes | native top-level struct |
| `prompt` | no | `Prompt` | yes | yes | yes | native top-level struct |
| `truncation` | no | `anyOf(string | null)` | yes | yes | yes | native top-level struct |
| `id` | yes | `string` | yes | yes | yes | native top-level struct |
| `object` | yes | `string` | yes | yes | yes | native top-level struct |
| `status` | no | `string` | yes | yes | yes | native top-level struct |
| `created_at` | yes | `number` | yes | yes | yes | native top-level struct |
| `completed_at` | no | `anyOf(number | null)` | no | no | no | missing/not modeled |
| `error` | yes | `ResponseError` | yes | yes | yes | native top-level struct |
| `incomplete_details` | yes | `anyOf(object | null)` | yes | yes | yes | native top-level struct |
| `output` | yes | `array[OutputItem]` | yes | yes | yes | native top-level struct |
| `instructions` | yes | `anyOf(oneOf(string | array[InputItem]) | null)` | yes | yes | yes | native top-level struct |
| `output_text` | no | `anyOf(string | null)` | no | no | no | missing/not modeled |
| `usage` | no | `ResponseUsage` | yes | yes | yes | native top-level struct |
| `parallel_tool_calls` | yes | `boolean` | yes | yes | yes | native top-level struct |
| `conversation` | no | `anyOf(Conversation-2 | null)` | yes | yes | yes | native top-level struct |
| `max_output_tokens` | no | `anyOf(integer | null)` | yes | yes | yes | native top-level struct |

## Anthropic Messages request

Code target: `llm/transformer/anthropic/model.go` struct `MessageRequest`

| Field | Required | Official type | Upstream top-level? | Current top-level? | Upstream any json tag? | Author handling |
|---|---:|---|---:|---:|---:|---|
| `max_tokens` | yes | `number` | yes | yes | yes | native top-level struct |
| `messages` | yes | `array[MessageParam]` | yes | yes | yes | native top-level struct |
| `model` | yes | `Model` | yes | yes | yes | native top-level struct |
| `container` | no | `string` | no | no | no | missing or companion-native field candidate |
| `inference_geo` | no | `string` | no | no | no | missing or companion-native field candidate |
| `metadata` | no | `Metadata` | yes | yes | yes | native top-level struct |
| `output_config` | no | `OutputConfig` | yes | yes | yes | native top-level struct |
| `service_tier` | no | `auto | standard_only` | yes | yes | yes | native top-level struct |
| `stop_sequences` | no | `array[string]` | yes | yes | yes | native top-level struct |
| `stream` | no | `boolean` | yes | yes | yes | native top-level struct |
| `system` | no | `string | array[TextBlockParam]` | yes | yes | yes | native top-level struct |
| `temperature` | no | `number` | yes | yes | yes | native top-level struct |
| `thinking` | no | `ThinkingConfigParam` | yes | yes | yes | native top-level struct |
| `tool_choice` | no | `ToolChoice` | yes | yes | yes | native top-level struct |
| `tools` | no | `array[ToolUnion]` | yes | yes | yes | native top-level struct |
| `top_k` | no | `number` | yes | yes | yes | native top-level struct |
| `top_p` | no | `number` | yes | yes | yes | native top-level struct |

## Anthropic Message response

Code target: `llm/transformer/anthropic/model.go` struct `Message`

| Field | Required | Official type | Upstream top-level? | Current top-level? | Upstream any json tag? | Author handling |
|---|---:|---|---:|---:|---:|---|
| `id` | yes | `string` | yes | yes | yes | native top-level struct |
| `container` | no | `Container` | no | no | no | missing or companion-native field candidate |
| `content` | yes | `array[ContentBlock]` | yes | yes | yes | native top-level struct |
| `model` | yes | `Model` | yes | yes | yes | native top-level struct |
| `role` | yes | `assistant` | yes | yes | yes | native top-level struct |
| `stop_details` | no | `StopDetails` | no | no | no | missing/not modeled |
| `stop_reason` | yes | `string` | yes | yes | yes | native top-level struct |
| `stop_sequence` | no | `string|null` | yes | yes | yes | native top-level struct |
| `type` | yes | `message` | yes | yes | yes | native top-level struct |
| `usage` | yes | `Usage` | yes | yes | yes | native top-level struct |

## Stream/event schema coverage

| Protocol | Event/schema | Upstream present by name/string? | Current present by name/string? |
|---|---|---:|---:|
| `openai_responses` | `ResponseAudioDeltaEvent` | no | no |
| `openai_responses` | `ResponseAudioDoneEvent` | no | no |
| `openai_responses` | `ResponseAudioTranscriptDeltaEvent` | no | no |
| `openai_responses` | `ResponseAudioTranscriptDoneEvent` | no | no |
| `openai_responses` | `ResponseCodeInterpreterCallCodeDeltaEvent` | no | no |
| `openai_responses` | `ResponseCodeInterpreterCallCodeDoneEvent` | no | no |
| `openai_responses` | `ResponseCodeInterpreterCallCompletedEvent` | no | no |
| `openai_responses` | `ResponseCodeInterpreterCallInProgressEvent` | no | no |
| `openai_responses` | `ResponseCodeInterpreterCallInterpretingEvent` | no | no |
| `openai_responses` | `ResponseCompletedEvent` | no | no |
| `openai_responses` | `ResponseContentPartAddedEvent` | no | no |
| `openai_responses` | `ResponseContentPartDoneEvent` | no | no |
| `openai_responses` | `ResponseCreatedEvent` | no | no |
| `openai_responses` | `ResponseCustomToolCallInputDeltaEvent` | no | no |
| `openai_responses` | `ResponseCustomToolCallInputDoneEvent` | no | no |
| `openai_responses` | `ResponseErrorEvent` | no | no |
| `openai_responses` | `ResponseFailedEvent` | no | no |
| `openai_responses` | `ResponseFileSearchCallCompletedEvent` | no | no |
| `openai_responses` | `ResponseFileSearchCallInProgressEvent` | no | no |
| `openai_responses` | `ResponseFileSearchCallSearchingEvent` | no | no |
| `openai_responses` | `ResponseFunctionCallArgumentsDeltaEvent` | no | no |
| `openai_responses` | `ResponseFunctionCallArgumentsDoneEvent` | no | no |
| `openai_responses` | `ResponseImageGenCallCompletedEvent` | no | no |
| `openai_responses` | `ResponseImageGenCallGeneratingEvent` | no | no |
| `openai_responses` | `ResponseImageGenCallInProgressEvent` | no | no |
| `openai_responses` | `ResponseImageGenCallPartialImageEvent` | no | no |
| `openai_responses` | `ResponseInProgressEvent` | no | no |
| `openai_responses` | `ResponseIncompleteEvent` | no | no |
| `openai_responses` | `ResponseMCPCallArgumentsDeltaEvent` | no | no |
| `openai_responses` | `ResponseMCPCallArgumentsDoneEvent` | no | no |
| `openai_responses` | `ResponseMCPCallCompletedEvent` | no | no |
| `openai_responses` | `ResponseMCPCallFailedEvent` | no | no |
| `openai_responses` | `ResponseMCPCallInProgressEvent` | no | no |
| `openai_responses` | `ResponseMCPListToolsCompletedEvent` | no | no |
| `openai_responses` | `ResponseMCPListToolsFailedEvent` | no | no |
| `openai_responses` | `ResponseMCPListToolsInProgressEvent` | no | no |
| `openai_responses` | `ResponseOutputItemAddedEvent` | no | no |
| `openai_responses` | `ResponseOutputItemDoneEvent` | no | no |
| `openai_responses` | `ResponseOutputTextAnnotationAddedEvent` | no | no |
| `openai_responses` | `ResponseQueuedEvent` | no | no |
| `openai_responses` | `ResponseReasoningSummaryPartAddedEvent` | no | no |
| `openai_responses` | `ResponseReasoningSummaryPartDoneEvent` | no | no |
| `openai_responses` | `ResponseReasoningSummaryTextDeltaEvent` | no | no |
| `openai_responses` | `ResponseReasoningSummaryTextDoneEvent` | no | no |
| `openai_responses` | `ResponseReasoningTextDeltaEvent` | no | no |
| `openai_responses` | `ResponseReasoningTextDoneEvent` | no | no |
| `openai_responses` | `ResponseRefusalDeltaEvent` | no | no |
| `openai_responses` | `ResponseRefusalDoneEvent` | no | no |
| `openai_responses` | `ResponseStreamEvent` | no | no |
| `openai_responses` | `ResponseTextDeltaEvent` | no | no |
| `openai_responses` | `ResponseTextDoneEvent` | no | no |
| `openai_responses` | `ResponseWebSearchCallCompletedEvent` | no | no |
| `openai_responses` | `ResponseWebSearchCallInProgressEvent` | no | no |
| `openai_responses` | `ResponseWebSearchCallSearchingEvent` | no | no |
| `openai_responses` | `ResponsesClientEvent` | no | no |
| `openai_responses` | `ResponsesClientEventResponseCreate` | no | no |
| `openai_responses` | `ResponsesServerEvent` | no | no |
| `openai_chat` | `ChatCompletionMessageToolCallChunk` | no | no |
| `openai_chat` | `ChatCompletionStreamOptions` | no | no |
| `openai_chat` | `ChatCompletionStreamResponseDelta` | no | no |
| `openai_chat` | `CreateChatCompletionStreamResponse` | no | no |
| `anthropic` | `message_start` | yes | yes |
| `anthropic` | `content_block_start` | yes | yes |
| `anthropic` | `content_block_delta` | yes | yes |
| `anthropic` | `content_block_stop` | yes | yes |
| `anthropic` | `message_delta` | yes | yes |
| `anthropic` | `message_stop` | yes | yes |
| `anthropic` | `ping` | yes | yes |
| `anthropic` | `error` | yes | yes |
| `anthropic` | `text_delta` | yes | yes |
| `anthropic` | `input_json_delta` | yes | yes |
| `anthropic` | `thinking_delta` | yes | yes |
| `anthropic` | `signature_delta` | yes | yes |

## Anthropic MCP connector companion fields

| Field | Required | Type |
|---|---:|---|
| `mcp_servers` | no | `array[MCPServer]` |
| `tools[].type=mcp_toolset` | no | `ToolUnion variant` |
| `mcp_servers[].name` | yes | `string` |
| `mcp_servers[].url` | yes | `string` |
| `mcp_servers[].authorization_token` | no | `string` |
| `mcp_servers[].tool_configuration` | no | `object` |

## OpenAI nested schema field lists

See `openai-fields.md` and `openai-fields.json` for full nested schema field lists. This matrix compares top-level request/response and stream/event coverage first.
