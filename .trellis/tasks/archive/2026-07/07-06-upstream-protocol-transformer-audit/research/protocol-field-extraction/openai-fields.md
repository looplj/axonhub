# OpenAI official schema fields

Source: `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/protocol-field-extraction/openai-openapi.github.yaml`

## OpenAI Responses request: CreateResponse

| Field | Required | Type | Meaning | Enum |
|---|---:|---|---|---|
| `metadata` | no | `Metadata` |  |  |
| `top_logprobs` | no | `anyOf(integer | null)` |  |  |
| `temperature` | no | `anyOf(number | null)` |  |  |
| `top_p` | no | `anyOf(number | null)` |  |  |
| `user` | no | `string` | This field is being replaced by `safety_identifier` and `prompt_cache_key`. Use `prompt_cache_key` instead to maintain caching optimizations. A stable identifier for your end-users. Used to boost cache hit rates by better bucketing similar requests and to help OpenAI detect and prevent abuse. [Learn more](/docs/guides/safety-best-practices#safety-identifiers). |  |
| `safety_identifier` | no | `string` | A stable identifier used to help detect users of your application that may be violating OpenAI's usage policies. The IDs should be a string that uniquely identifies each user, with a maximum length of 64 characters. We recommend hashing their username or email address, in order to avoid sending us any identifying information. [Learn more](/docs/guides/safety-best-practices#safety-identifiers). |  |
| `prompt_cache_key` | no | `string` | Used by OpenAI to cache responses for similar requests to optimize your cache hit rates. Replaces the `user` field. [Learn more](/docs/guides/prompt-caching). |  |
| `service_tier` | no | `ServiceTier` |  | auto, default, flex, scale, priority |
| `prompt_cache_retention` | no | `anyOf(string | null)` |  | in_memory, 24h |
| `previous_response_id` | no | `anyOf(string | null)` |  |  |
| `model` | no | `ModelIdsResponses` | Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a wide range of models with different capabilities, performance characteristics, and price points. Refer to the [model guide](/docs/models) to browse and compare available models. | gpt-5.4, gpt-5.4-mini, gpt-5.4-nano, gpt-5.4-mini-2026-03-17, gpt-5.4-nano-2026-03-17, gpt-5.3-chat-latest, gpt-5.2, gpt-5.2-2025-12-11, gpt-5.2-chat-latest, gpt-5.2-pro, gpt-5.2-pro-2025-12-11, gpt-5.1, gpt-5.1-2025-11-13, gpt-5.1-codex, gpt-5.1-mini, gpt-5.1-chat-latest, gpt-5, gpt-5-mini, gpt-5-nano, gpt-5-2025-08-07 |
| `reasoning` | no | `anyOf(Reasoning | null)` |  |  |
| `background` | no | `anyOf(boolean | null)` |  |  |
| `max_tool_calls` | no | `anyOf(integer | null)` |  |  |
| `text` | no | `ResponseTextParam` | Configuration options for a text response from the model. Can be plain text or structured JSON data. Learn more: - [Text inputs and outputs](/docs/guides/text) - [Structured Outputs](/docs/guides/structured-outputs) |  |
| `tools` | no | `ToolsArray` | An array of tools the model may call while generating a response. You can specify which tool to use by setting the `tool_choice` parameter. We support the following categories of tools: - **Built-in tools**: Tools that are provided by OpenAI that extend the model's capabilities, like [web search](/docs/guides/tools-web-search) or [file search](/docs/guides/tools-file-search). Learn more about [built-in tools](/docs/guides/tools). - **MCP Tools**: Integrations with third-party systems via custom MCP servers or predefined connectors such as Google Drive and SharePoint. Learn more about [MCP Tools](/docs/guides/tools-connectors-mcp). - **Function calls (custom tools)**: Functions that are defined by you, enabling the model to call your own code with strongly typed arguments and outputs. Learn more about [function calling](/docs/guides/function-calling). You can also use custom tools to call your own code. |  |
| `tool_choice` | no | `ToolChoiceParam` | How the model should select which tool (or tools) to use when generating a response. See the `tools` parameter to see how to specify which tools the model can call. | none, auto, required |
| `prompt` | no | `Prompt` |  |  |
| `truncation` | no | `anyOf(string | null)` |  | auto, disabled |
| `input` | no | `InputParam` | Text, image, or file inputs to the model, used to generate a response. Learn more: - [Text inputs and outputs](/docs/guides/text) - [Image inputs](/docs/guides/images) - [File inputs](/docs/guides/pdf-files) - [Conversation state](/docs/guides/conversation-state) - [Function calling](/docs/guides/function-calling) |  |
| `include` | no | `anyOf(array[IncludeEnum] | null)` |  |  |
| `parallel_tool_calls` | no | `anyOf(boolean | null)` |  |  |
| `store` | no | `anyOf(boolean | null)` |  |  |
| `instructions` | no | `anyOf(string | null)` |  |  |
| `stream` | no | `anyOf(boolean | null)` |  |  |
| `stream_options` | no | `ResponseStreamOptions` |  |  |
| `conversation` | no | `anyOf(ConversationParam | null)` |  |  |
| `context_management` | no | `anyOf(array[ContextManagementParam] | null)` |  |  |
| `max_output_tokens` | no | `anyOf(integer | null)` |  |  |

## OpenAI Responses response: Response

| Field | Required | Type | Meaning | Enum |
|---|---:|---|---|---|
| `metadata` | yes | `Metadata` |  |  |
| `top_logprobs` | no | `anyOf(integer | null)` |  |  |
| `temperature` | yes | `anyOf(number | null)` |  |  |
| `top_p` | yes | `anyOf(number | null)` |  |  |
| `user` | no | `string` | This field is being replaced by `safety_identifier` and `prompt_cache_key`. Use `prompt_cache_key` instead to maintain caching optimizations. A stable identifier for your end-users. Used to boost cache hit rates by better bucketing similar requests and to help OpenAI detect and prevent abuse. [Learn more](/docs/guides/safety-best-practices#safety-identifiers). |  |
| `safety_identifier` | no | `string` | A stable identifier used to help detect users of your application that may be violating OpenAI's usage policies. The IDs should be a string that uniquely identifies each user, with a maximum length of 64 characters. We recommend hashing their username or email address, in order to avoid sending us any identifying information. [Learn more](/docs/guides/safety-best-practices#safety-identifiers). |  |
| `prompt_cache_key` | no | `string` | Used by OpenAI to cache responses for similar requests to optimize your cache hit rates. Replaces the `user` field. [Learn more](/docs/guides/prompt-caching). |  |
| `service_tier` | no | `ServiceTier` |  | auto, default, flex, scale, priority |
| `prompt_cache_retention` | no | `anyOf(string | null)` |  | in_memory, 24h |
| `previous_response_id` | no | `anyOf(string | null)` |  |  |
| `model` | yes | `ModelIdsResponses` | Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a wide range of models with different capabilities, performance characteristics, and price points. Refer to the [model guide](/docs/models) to browse and compare available models. | gpt-5.4, gpt-5.4-mini, gpt-5.4-nano, gpt-5.4-mini-2026-03-17, gpt-5.4-nano-2026-03-17, gpt-5.3-chat-latest, gpt-5.2, gpt-5.2-2025-12-11, gpt-5.2-chat-latest, gpt-5.2-pro, gpt-5.2-pro-2025-12-11, gpt-5.1, gpt-5.1-2025-11-13, gpt-5.1-codex, gpt-5.1-mini, gpt-5.1-chat-latest, gpt-5, gpt-5-mini, gpt-5-nano, gpt-5-2025-08-07 |
| `reasoning` | no | `anyOf(Reasoning | null)` |  |  |
| `background` | no | `anyOf(boolean | null)` |  |  |
| `max_tool_calls` | no | `anyOf(integer | null)` |  |  |
| `text` | no | `ResponseTextParam` | Configuration options for a text response from the model. Can be plain text or structured JSON data. Learn more: - [Text inputs and outputs](/docs/guides/text) - [Structured Outputs](/docs/guides/structured-outputs) |  |
| `tools` | yes | `ToolsArray` | An array of tools the model may call while generating a response. You can specify which tool to use by setting the `tool_choice` parameter. We support the following categories of tools: - **Built-in tools**: Tools that are provided by OpenAI that extend the model's capabilities, like [web search](/docs/guides/tools-web-search) or [file search](/docs/guides/tools-file-search). Learn more about [built-in tools](/docs/guides/tools). - **MCP Tools**: Integrations with third-party systems via custom MCP servers or predefined connectors such as Google Drive and SharePoint. Learn more about [MCP Tools](/docs/guides/tools-connectors-mcp). - **Function calls (custom tools)**: Functions that are defined by you, enabling the model to call your own code with strongly typed arguments and outputs. Learn more about [function calling](/docs/guides/function-calling). You can also use custom tools to call your own code. |  |
| `tool_choice` | yes | `ToolChoiceParam` | How the model should select which tool (or tools) to use when generating a response. See the `tools` parameter to see how to specify which tools the model can call. | none, auto, required |
| `prompt` | no | `Prompt` |  |  |
| `truncation` | no | `anyOf(string | null)` |  | auto, disabled |
| `id` | yes | `string` | Unique identifier for this Response. |  |
| `object` | yes | `string` | The object type of this resource - always set to `response`. | response |
| `status` | no | `string` | The status of the response generation. One of `completed`, `failed`, `in_progress`, `cancelled`, `queued`, or `incomplete`. | completed, failed, in_progress, cancelled, queued, incomplete |
| `created_at` | yes | `number` | Unix timestamp (in seconds) of when this Response was created. |  |
| `completed_at` | no | `anyOf(number | null)` |  |  |
| `error` | yes | `ResponseError` |  |  |
| `incomplete_details` | yes | `anyOf(object | null)` |  |  |
| `output` | yes | `array[OutputItem]` | An array of content items generated by the model. - The length and order of items in the `output` array is dependent on the model's response. - Rather than accessing the first item in the `output` array and assuming it's an `assistant` message with the content generated by the model, you might consider using the `output_text` property where supported in SDKs. |  |
| `instructions` | yes | `anyOf(oneOf(string | array[InputItem]) | null)` |  |  |
| `output_text` | no | `anyOf(string | null)` |  |  |
| `usage` | no | `ResponseUsage` | Represents token usage details including input tokens, output tokens, a breakdown of output tokens, and the total tokens used. |  |
| `parallel_tool_calls` | yes | `boolean` | Whether to allow the model to run tool calls in parallel. |  |
| `conversation` | no | `anyOf(Conversation-2 | null)` |  |  |
| `max_output_tokens` | no | `anyOf(integer | null)` |  |  |

## OpenAI Chat request: CreateChatCompletionRequest

| Field | Required | Type | Meaning | Enum |
|---|---:|---|---|---|
| `metadata` | no | `Metadata` |  |  |
| `top_logprobs` | no | `anyOf(integer | null)` |  |  |
| `temperature` | no | `anyOf(number | null)` |  |  |
| `top_p` | no | `anyOf(number | null)` |  |  |
| `user` | no | `string` | This field is being replaced by `safety_identifier` and `prompt_cache_key`. Use `prompt_cache_key` instead to maintain caching optimizations. A stable identifier for your end-users. Used to boost cache hit rates by better bucketing similar requests and to help OpenAI detect and prevent abuse. [Learn more](/docs/guides/safety-best-practices#safety-identifiers). |  |
| `safety_identifier` | no | `string` | A stable identifier used to help detect users of your application that may be violating OpenAI's usage policies. The IDs should be a string that uniquely identifies each user, with a maximum length of 64 characters. We recommend hashing their username or email address, in order to avoid sending us any identifying information. [Learn more](/docs/guides/safety-best-practices#safety-identifiers). |  |
| `prompt_cache_key` | no | `string` | Used by OpenAI to cache responses for similar requests to optimize your cache hit rates. Replaces the `user` field. [Learn more](/docs/guides/prompt-caching). |  |
| `service_tier` | no | `ServiceTier` |  | auto, default, flex, scale, priority |
| `prompt_cache_retention` | no | `anyOf(string | null)` |  | in_memory, 24h |
| `messages` | yes | `array[ChatCompletionRequestMessage]` | A list of messages comprising the conversation so far. Depending on the [model](/docs/models) you use, different message types (modalities) are supported, like [text](/docs/guides/text-generation), [images](/docs/guides/vision), and [audio](/docs/guides/audio). |  |
| `model` | yes | `ModelIdsShared` | Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a wide range of models with different capabilities, performance characteristics, and price points. Refer to the [model guide](/docs/models) to browse and compare available models. | gpt-5.4, gpt-5.4-mini, gpt-5.4-nano, gpt-5.4-mini-2026-03-17, gpt-5.4-nano-2026-03-17, gpt-5.3-chat-latest, gpt-5.2, gpt-5.2-2025-12-11, gpt-5.2-chat-latest, gpt-5.2-pro, gpt-5.2-pro-2025-12-11, gpt-5.1, gpt-5.1-2025-11-13, gpt-5.1-codex, gpt-5.1-mini, gpt-5.1-chat-latest, gpt-5, gpt-5-mini, gpt-5-nano, gpt-5-2025-08-07 |
| `modalities` | no | `ResponseModalities` |  |  |
| `verbosity` | no | `Verbosity` |  | low, medium, high |
| `reasoning_effort` | no | `ReasoningEffort` |  | none, minimal, low, medium, high, xhigh |
| `max_completion_tokens` | no | `integer` | An upper bound for the number of tokens that can be generated for a completion, including visible output tokens and [reasoning tokens](/docs/guides/reasoning). |  |
| `frequency_penalty` | no | `number` | Number between -2.0 and 2.0. Positive values penalize new tokens based on their existing frequency in the text so far, decreasing the model's likelihood to repeat the same line verbatim. |  |
| `presence_penalty` | no | `number` | Number between -2.0 and 2.0. Positive values penalize new tokens based on whether they appear in the text so far, increasing the model's likelihood to talk about new topics. |  |
| `web_search_options` | no | `object` | This tool searches the web for relevant results to use in a response. Learn more about the [web search tool](/docs/guides/tools-web-search?api-mode=chat). |  |
| `response_format` | no | `oneOf(ResponseFormatText | ResponseFormatJsonSchema | ResponseFormatJsonObject)` | An object specifying the format that the model must output. Setting to `{ "type": "json_schema", "json_schema": {...} }` enables Structured Outputs which ensures the model will match your supplied JSON schema. Learn more in the [Structured Outputs guide](/docs/guides/structured-outputs). Setting to `{ "type": "json_object" }` enables the older JSON mode, which ensures the message the model generates is valid JSON. Using `json_schema` is preferred for models that support it. |  |
| `audio` | no | `object` | Parameters for audio output. Required when audio output is requested with `modalities: ["audio"]`. [Learn more](/docs/guides/audio). |  |
| `store` | no | `boolean` | Whether or not to store the output of this chat completion request for use in our [model distillation](/docs/guides/distillation) or [evals](/docs/guides/evals) products. Supports text and image inputs. Note: image inputs over 8MB will be dropped. |  |
| `stream` | no | `boolean` | If set to true, the model response data will be streamed to the client as it is generated using [server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Event_stream_format). See the [Streaming section below](/docs/api-reference/chat/streaming) for more information, along with the [streaming responses](/docs/guides/streaming-responses) guide for more information on how to handle the streaming events. |  |
| `stop` | no | `StopConfiguration` | Not supported with latest reasoning models `o3` and `o4-mini`. Up to 4 sequences where the API will stop generating further tokens. The returned text will not contain the stop sequence. |  |
| `logit_bias` | no | `object` | Modify the likelihood of specified tokens appearing in the completion. Accepts a JSON object that maps tokens (specified by their token ID in the tokenizer) to an associated bias value from -100 to 100. Mathematically, the bias is added to the logits generated by the model prior to sampling. The exact effect will vary per model, but values between -1 and 1 should decrease or increase likelihood of selection; values like -100 or 100 should result in a ban or exclusive selection of the relevant token. |  |
| `logprobs` | no | `boolean` | Whether to return log probabilities of the output tokens or not. If true, returns the log probabilities of each output token returned in the `content` of `message`. |  |
| `max_tokens` | no | `integer` | The maximum number of [tokens](/tokenizer) that can be generated in the chat completion. This value can be used to control [costs](https://openai.com/api/pricing/) for text generated via API. This value is now deprecated in favor of `max_completion_tokens`, and is not compatible with [o-series models](/docs/guides/reasoning). |  |
| `n` | no | `integer` | How many chat completion choices to generate for each input message. Note that you will be charged based on the number of generated tokens across all of the choices. Keep `n` as `1` to minimize costs. |  |
| `prediction` | no | `oneOf(PredictionContent)` | Configuration for a [Predicted Output](/docs/guides/predicted-outputs), which can greatly improve response times when large parts of the model response are known ahead of time. This is most common when you are regenerating a file with only minor changes to most of the content. |  |
| `seed` | no | `integer` | This feature is in Beta. If specified, our system will make a best effort to sample deterministically, such that repeated requests with the same `seed` and parameters should return the same result. Determinism is not guaranteed, and you should refer to the `system_fingerprint` response parameter to monitor changes in the backend. |  |
| `stream_options` | no | `ChatCompletionStreamOptions` |  |  |
| `tools` | no | `array[oneOf(ChatCompletionTool | CustomToolChatCompletions)]` | A list of tools the model may call. You can provide either [custom tools](/docs/guides/function-calling#custom-tools) or [function tools](/docs/guides/function-calling). |  |
| `tool_choice` | no | `ChatCompletionToolChoiceOption` | Controls which (if any) tool is called by the model. `none` means the model will not call any tool and instead generates a message. `auto` means the model can pick between generating a message or calling one or more tools. `required` means the model must call one or more tools. Specifying a particular tool via `{"type": "function", "function": {"name": "my_function"}}` forces the model to call that tool. `none` is the default when no tools are present. `auto` is the default if tools are present. | none, auto, required |
| `parallel_tool_calls` | no | `ParallelToolCalls` | Whether to enable [parallel function calling](/docs/guides/function-calling#configuring-parallel-function-calling) during tool use. |  |
| `function_call` | no | `oneOf(string | ChatCompletionFunctionCallOption)` | Deprecated in favor of `tool_choice`. Controls which (if any) function is called by the model. `none` means the model will not call a function and instead generates a message. `auto` means the model can pick between generating a message or calling a function. Specifying a particular function via `{"name": "my_function"}` forces the model to call that function. `none` is the default when no functions are present. `auto` is the default if functions are present. | none, auto |
| `functions` | no | `array[ChatCompletionFunctions]` | Deprecated in favor of `tools`. A list of functions the model may generate JSON inputs for. |  |

## OpenAI Chat response: CreateChatCompletionResponse

| Field | Required | Type | Meaning | Enum |
|---|---:|---|---|---|
| `id` | yes | `string` | A unique identifier for the chat completion. |  |
| `choices` | yes | `array[object]` | A list of chat completion choices. Can be more than one if `n` is greater than 1. |  |
| `created` | yes | `integer` | The Unix timestamp (in seconds) of when the chat completion was created. |  |
| `model` | yes | `string` | The model used for the chat completion. |  |
| `service_tier` | no | `ServiceTier` |  | auto, default, flex, scale, priority |
| `system_fingerprint` | no | `string` | This fingerprint represents the backend configuration that the model runs with. Can be used in conjunction with the `seed` request parameter to understand when backend changes have been made that might impact determinism. |  |
| `object` | yes | `string` | The object type, which is always `chat.completion`. | chat.completion |
| `usage` | no | `CompletionUsage` | Usage statistics for the completion request. |  |

## OpenAI Responses stream/event schema names
- `ResponseAudioDeltaEvent`
- `ResponseAudioDoneEvent`
- `ResponseAudioTranscriptDeltaEvent`
- `ResponseAudioTranscriptDoneEvent`
- `ResponseCodeInterpreterCallCodeDeltaEvent`
- `ResponseCodeInterpreterCallCodeDoneEvent`
- `ResponseCodeInterpreterCallCompletedEvent`
- `ResponseCodeInterpreterCallInProgressEvent`
- `ResponseCodeInterpreterCallInterpretingEvent`
- `ResponseCompletedEvent`
- `ResponseContentPartAddedEvent`
- `ResponseContentPartDoneEvent`
- `ResponseCreatedEvent`
- `ResponseCustomToolCallInputDeltaEvent`
- `ResponseCustomToolCallInputDoneEvent`
- `ResponseErrorEvent`
- `ResponseFailedEvent`
- `ResponseFileSearchCallCompletedEvent`
- `ResponseFileSearchCallInProgressEvent`
- `ResponseFileSearchCallSearchingEvent`
- `ResponseFunctionCallArgumentsDeltaEvent`
- `ResponseFunctionCallArgumentsDoneEvent`
- `ResponseImageGenCallCompletedEvent`
- `ResponseImageGenCallGeneratingEvent`
- `ResponseImageGenCallInProgressEvent`
- `ResponseImageGenCallPartialImageEvent`
- `ResponseInProgressEvent`
- `ResponseIncompleteEvent`
- `ResponseMCPCallArgumentsDeltaEvent`
- `ResponseMCPCallArgumentsDoneEvent`
- `ResponseMCPCallCompletedEvent`
- `ResponseMCPCallFailedEvent`
- `ResponseMCPCallInProgressEvent`
- `ResponseMCPListToolsCompletedEvent`
- `ResponseMCPListToolsFailedEvent`
- `ResponseMCPListToolsInProgressEvent`
- `ResponseOutputItemAddedEvent`
- `ResponseOutputItemDoneEvent`
- `ResponseOutputTextAnnotationAddedEvent`
- `ResponseQueuedEvent`
- `ResponseReasoningSummaryPartAddedEvent`
- `ResponseReasoningSummaryPartDoneEvent`
- `ResponseReasoningSummaryTextDeltaEvent`
- `ResponseReasoningSummaryTextDoneEvent`
- `ResponseReasoningTextDeltaEvent`
- `ResponseReasoningTextDoneEvent`
- `ResponseRefusalDeltaEvent`
- `ResponseRefusalDoneEvent`
- `ResponseStreamEvent`
- `ResponseTextDeltaEvent`
- `ResponseTextDoneEvent`
- `ResponseWebSearchCallCompletedEvent`
- `ResponseWebSearchCallInProgressEvent`
- `ResponseWebSearchCallSearchingEvent`
- `ResponsesClientEvent`
- `ResponsesClientEventResponseCreate`
- `ResponsesServerEvent`

## OpenAI Chat stream schema names
- `ChatCompletionMessageToolCallChunk`
- `ChatCompletionStreamOptions`
- `ChatCompletionStreamResponseDelta`
- `CreateChatCompletionStreamResponse`

## OpenAI related nested schemas
- `ChatCompletionAllowedTools`: `mode`, `tools`
- `ChatCompletionAllowedToolsChoice`: `type`, `allowed_tools`
- `ChatCompletionDeleted`: `object`, `id`, `deleted`
- `ChatCompletionFunctionCallOption`: `name`
- `ChatCompletionFunctions`: `description`, `name`, `parameters`
- `ChatCompletionList`: `object`, `data`, `first_id`, `last_id`, `has_more`
- `ChatCompletionMessageCustomToolCall`: `id`, `type`, `custom`
- `ChatCompletionMessageList`: `object`, `data`, `first_id`, `last_id`, `has_more`
- `ChatCompletionMessageToolCall`: `id`, `type`, `function`
- `ChatCompletionMessageToolCallChunk`: `index`, `id`, `type`, `function`
- `ChatCompletionNamedToolChoice`: `type`, `function`
- `ChatCompletionNamedToolChoiceCustom`: `type`, `custom`
- `ChatCompletionRequestAssistantMessage`: `content`, `refusal`, `role`, `name`, `audio`, `tool_calls`, `function_call`
- `ChatCompletionRequestAssistantMessageContentPart`: `type`, `text`, `refusal`
- `ChatCompletionRequestDeveloperMessage`: `content`, `role`, `name`
- `ChatCompletionRequestFunctionMessage`: `role`, `content`, `name`
- `ChatCompletionRequestMessage`: `content`, `role`, `name`, `refusal`, `audio`, `tool_calls`, `function_call`, `tool_call_id`
- `ChatCompletionRequestMessageContentPartAudio`: `type`, `input_audio`
- `ChatCompletionRequestMessageContentPartFile`: `type`, `file`
- `ChatCompletionRequestMessageContentPartImage`: `type`, `image_url`
- `ChatCompletionRequestMessageContentPartRefusal`: `type`, `refusal`
- `ChatCompletionRequestMessageContentPartText`: `type`, `text`
- `ChatCompletionRequestSystemMessage`: `content`, `role`, `name`
- `ChatCompletionRequestSystemMessageContentPart`: `type`, `text`
- `ChatCompletionRequestToolMessage`: `role`, `content`, `tool_call_id`
- `ChatCompletionRequestToolMessageContentPart`: `type`, `text`
- `ChatCompletionRequestUserMessage`: `content`, `role`, `name`
- `ChatCompletionRequestUserMessageContentPart`: `type`, `text`, `image_url`, `input_audio`, `file`
- `ChatCompletionResponseMessage`: `content`, `refusal`, `tool_calls`, `annotations`, `role`, `function_call`, `audio`
- `ChatCompletionStreamOptions`: `include_usage`, `include_obfuscation`
- `ChatCompletionStreamResponseDelta`: `content`, `function_call`, `tool_calls`, `role`, `refusal`
- `ChatCompletionTokenLogprob`: `token`, `logprob`, `bytes`, `top_logprobs`
- `ChatCompletionTool`: `type`, `function`
- `ChatCompletionToolChoiceOption`: `type`, `allowed_tools`, `function`, `custom`
- `ChatSessionAutomaticThreadTitling`: `enabled`
- `ChatSessionChatkitConfiguration`: `automatic_thread_titling`, `file_upload`, `history`
- `ChatSessionFileUpload`: `enabled`, `max_file_size`, `max_files`
- `ChatSessionHistory`: `enabled`, `recent_threads`
- `ChatSessionRateLimits`: `max_requests_per_1_minute`
- `ChatSessionResource`: `id`, `object`, `expires_at`, `client_secret`, `workflow`, `user`, `rate_limits`, `max_requests_per_1_minute`, `status`, `chatkit_configuration`
- `ChatkitConfigurationParam`: `automatic_thread_titling`, `file_upload`, `history`
- `ChatkitWorkflow`: `id`, `version`, `state_variables`, `tracing`
- `ChatkitWorkflowTracing`: `enabled`
- `CreateChatCompletionStreamResponse`: `id`, `choices`, `created`, `model`, `service_tier`, `system_fingerprint`, `object`, `usage`
- `ResponseAudioDeltaEvent`: `type`, `sequence_number`, `delta`
- `ResponseAudioDoneEvent`: `type`, `sequence_number`
- `ResponseAudioTranscriptDeltaEvent`: `type`, `delta`, `sequence_number`
- `ResponseAudioTranscriptDoneEvent`: `type`, `sequence_number`
- `ResponseCodeInterpreterCallCodeDeltaEvent`: `type`, `output_index`, `item_id`, `delta`, `sequence_number`
- `ResponseCodeInterpreterCallCodeDoneEvent`: `type`, `output_index`, `item_id`, `code`, `sequence_number`
- `ResponseCodeInterpreterCallCompletedEvent`: `type`, `output_index`, `item_id`, `sequence_number`
- `ResponseCodeInterpreterCallInProgressEvent`: `type`, `output_index`, `item_id`, `sequence_number`
- `ResponseCodeInterpreterCallInterpretingEvent`: `type`, `output_index`, `item_id`, `sequence_number`
- `ResponseCompletedEvent`: `type`, `response`, `sequence_number`
- `ResponseContentPartAddedEvent`: `type`, `item_id`, `output_index`, `content_index`, `part`, `sequence_number`
- `ResponseContentPartDoneEvent`: `type`, `item_id`, `output_index`, `content_index`, `sequence_number`, `part`
- `ResponseCreatedEvent`: `type`, `response`, `sequence_number`
- `ResponseCustomToolCallInputDeltaEvent`: `type`, `sequence_number`, `output_index`, `item_id`, `delta`
- `ResponseCustomToolCallInputDoneEvent`: `type`, `sequence_number`, `output_index`, `item_id`, `input`
- `ResponseError`: `code`, `message`
- `ResponseErrorEvent`: `type`, `code`, `message`, `param`, `sequence_number`
- `ResponseFailedEvent`: `type`, `sequence_number`, `response`
- `ResponseFileSearchCallCompletedEvent`: `type`, `output_index`, `item_id`, `sequence_number`
- `ResponseFileSearchCallInProgressEvent`: `type`, `output_index`, `item_id`, `sequence_number`
- `ResponseFileSearchCallSearchingEvent`: `type`, `output_index`, `item_id`, `sequence_number`
- `ResponseFormatJsonObject`: `type`
- `ResponseFormatJsonSchema`: `type`, `json_schema`
- `ResponseFormatText`: `type`
- `ResponseFormatTextGrammar`: `type`, `grammar`
- `ResponseFormatTextPython`: `type`
- `ResponseFunctionCallArgumentsDeltaEvent`: `type`, `item_id`, `output_index`, `sequence_number`, `delta`
- `ResponseFunctionCallArgumentsDoneEvent`: `type`, `item_id`, `name`, `output_index`, `sequence_number`, `arguments`
- `ResponseImageGenCallCompletedEvent`: `type`, `output_index`, `sequence_number`, `item_id`
- `ResponseImageGenCallGeneratingEvent`: `type`, `output_index`, `item_id`, `sequence_number`
- `ResponseImageGenCallInProgressEvent`: `type`, `output_index`, `item_id`, `sequence_number`
- `ResponseImageGenCallPartialImageEvent`: `type`, `output_index`, `item_id`, `sequence_number`, `partial_image_index`, `partial_image_b64`
- `ResponseInProgressEvent`: `type`, `response`, `sequence_number`
- `ResponseIncompleteEvent`: `type`, `response`, `sequence_number`
- `ResponseItemList`: `object`, `data`, `has_more`, `first_id`, `last_id`
- `ResponseLogProb`: `token`, `logprob`, `top_logprobs`
- `ResponseMCPCallArgumentsDeltaEvent`: `type`, `output_index`, `item_id`, `delta`, `sequence_number`
- `ResponseMCPCallArgumentsDoneEvent`: `type`, `output_index`, `item_id`, `arguments`, `sequence_number`
- `ResponseMCPCallCompletedEvent`: `type`, `item_id`, `output_index`, `sequence_number`
- `ResponseMCPCallFailedEvent`: `type`, `item_id`, `output_index`, `sequence_number`
- `ResponseMCPCallInProgressEvent`: `type`, `sequence_number`, `output_index`, `item_id`
- `ResponseMCPListToolsCompletedEvent`: `type`, `item_id`, `output_index`, `sequence_number`
- `ResponseMCPListToolsFailedEvent`: `type`, `item_id`, `output_index`, `sequence_number`
- `ResponseMCPListToolsInProgressEvent`: `type`, `item_id`, `output_index`, `sequence_number`
- `ResponseOutputItemAddedEvent`: `type`, `output_index`, `sequence_number`, `item`
- `ResponseOutputItemDoneEvent`: `type`, `output_index`, `sequence_number`, `item`
- `ResponseOutputText`: `type`, `text`, `annotations`
- `ResponseOutputTextAnnotationAddedEvent`: `type`, `item_id`, `output_index`, `content_index`, `annotation_index`, `sequence_number`, `annotation`
- `ResponseProperties`: `previous_response_id`, `model`, `reasoning`, `background`, `max_tool_calls`, `text`, `tools`, `tool_choice`, `prompt`, `truncation`
- `ResponseQueuedEvent`: `type`, `response`, `sequence_number`
- `ResponseReasoningSummaryPartAddedEvent`: `type`, `item_id`, `output_index`, `summary_index`, `sequence_number`, `part`
- `ResponseReasoningSummaryPartDoneEvent`: `type`, `item_id`, `output_index`, `summary_index`, `sequence_number`, `part`
- `ResponseReasoningSummaryTextDeltaEvent`: `type`, `item_id`, `output_index`, `summary_index`, `delta`, `sequence_number`
- `ResponseReasoningSummaryTextDoneEvent`: `type`, `item_id`, `output_index`, `summary_index`, `text`, `sequence_number`
- `ResponseReasoningTextDeltaEvent`: `type`, `item_id`, `output_index`, `content_index`, `delta`, `sequence_number`
- `ResponseReasoningTextDoneEvent`: `type`, `item_id`, `output_index`, `content_index`, `text`, `sequence_number`
- `ResponseRefusalDeltaEvent`: `type`, `item_id`, `output_index`, `content_index`, `delta`, `sequence_number`
- `ResponseRefusalDoneEvent`: `type`, `item_id`, `output_index`, `content_index`, `refusal`, `sequence_number`
- `ResponseStreamEvent`: `type`, `sequence_number`, `delta`, `output_index`, `item_id`, `code`, `response`, `content_index`, `part`, `message`, `param`, `name`, `arguments`, `item`, `summary_index`, `text`, `refusal`, `logprobs`, `partial_image_index`, `partial_image_b64`, `annotation_index`, `annotation`, `input`
- `ResponseStreamOptions`: `include_obfuscation`
- `ResponseTextDeltaEvent`: `type`, `item_id`, `output_index`, `content_index`, `delta`, `sequence_number`, `logprobs`
- `ResponseTextDoneEvent`: `type`, `item_id`, `output_index`, `content_index`, `text`, `sequence_number`, `logprobs`
- `ResponseTextParam`: `format`, `verbosity`
- `ResponseUsage`: `input_tokens`, `input_tokens_details`, `output_tokens`, `output_tokens_details`, `total_tokens`
- `ResponseWebSearchCallCompletedEvent`: `type`, `output_index`, `item_id`, `sequence_number`
- `ResponseWebSearchCallInProgressEvent`: `type`, `output_index`, `item_id`, `sequence_number`
- `ResponseWebSearchCallSearchingEvent`: `type`, `output_index`, `item_id`, `sequence_number`
- `ResponsesClientEvent`: `type`, `metadata`, `top_logprobs`, `temperature`, `top_p`, `user`, `safety_identifier`, `prompt_cache_key`, `service_tier`, `prompt_cache_retention`, `previous_response_id`, `model`, `reasoning`, `background`, `max_tool_calls`, `text`, `tools`, `tool_choice`, `prompt`, `truncation`, `input`, `include`, `parallel_tool_calls`, `store`, `instructions`, `stream`, `stream_options`, `conversation`, `context_management`, `max_output_tokens`
- `ResponsesClientEventResponseCreate`: `type`, `metadata`, `top_logprobs`, `temperature`, `top_p`, `user`, `safety_identifier`, `prompt_cache_key`, `service_tier`, `prompt_cache_retention`, `previous_response_id`, `model`, `reasoning`, `background`, `max_tool_calls`, `text`, `tools`, `tool_choice`, `prompt`, `truncation`, `input`, `include`, `parallel_tool_calls`, `store`, `instructions`, `stream`, `stream_options`, `conversation`, `context_management`, `max_output_tokens`
- `ResponsesServerEvent`: `type`, `sequence_number`, `delta`, `output_index`, `item_id`, `code`, `response`, `content_index`, `part`, `message`, `param`, `name`, `arguments`, `item`, `summary_index`, `text`, `refusal`, `logprobs`, `partial_image_index`, `partial_image_b64`, `annotation_index`, `annotation`, `input`
