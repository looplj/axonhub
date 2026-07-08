# Messages

## Create a Message

**post** `/v1/messages`

Send a structured list of input messages with text and/or image content, and the model will generate the next message in the conversation.

The Messages API can be used for either single queries or stateless multi-turn conversations.

Learn more about the Messages API in our [user guide](https://platform.claude.com/docs/en/get-started)

### Header Parameters

- `"anthropic-user-profile-id": optional string`

  The user profile ID to attribute this request to. Use when acting on behalf of a party other than your organization. Requires the `user-profiles` beta header.

### Body Parameters

- `max_tokens: number`

  The maximum number of tokens to generate before stopping.

  Note that our models may stop _before_ reaching this maximum. This parameter only specifies the absolute maximum number of tokens to generate.

  Set to `0` to populate the [prompt cache](https://platform.claude.com/docs/en/build-with-claude/prompt-caching#pre-warming-the-cache) without generating a response.

  Different models have different maximum values for this parameter.  See [models](https://platform.claude.com/docs/en/about-claude/models/overview) for details.

- `messages: array of MessageParam`

  Input messages.

  Our models are trained to operate on alternating `user` and `assistant` conversational turns. When creating a new `Message`, you specify the prior conversational turns with the `messages` parameter, and the model then generates the next `Message` in the conversation. Consecutive `user` or `assistant` turns in your request will be combined into a single turn.

  Each input message must be an object with a `role` and `content`. You can specify a single `user`-role message, or you can include multiple `user` and `assistant` messages.

  If the final message uses the `assistant` role, the response content will continue immediately from the content in that message. This can be used to constrain part of the model's response.

  Example with a single `user` message:

  ```json
  [{"role": "user", "content": "Hello, Claude"}]
  ```

  Example with multiple conversational turns:

  ```json
  [
    {"role": "user", "content": "Hello there."},
    {"role": "assistant", "content": "Hi, I'm Claude. How can I help you?"},
    {"role": "user", "content": "Can you explain LLMs in plain English?"},
  ]
  ```

  Example with a partially-filled response from Claude:

  ```json
  [
    {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
    {"role": "assistant", "content": "The best answer is ("},
  ]
  ```

  Each input message `content` may be either a single `string` or an array of content blocks, where each block has a specific `type`. Using a `string` for `content` is shorthand for an array of one content block of type `"text"`. The following input messages are equivalent:

  ```json
  {"role": "user", "content": "Hello, Claude"}
  ```

  ```json
  {"role": "user", "content": [{"type": "text", "text": "Hello, Claude"}]}
  ```

  See [input examples](https://platform.claude.com/docs/en/build-with-claude/working-with-messages).

  Note that if you want to include a [system prompt](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#give-claude-a-role), you can use the top-level `system` parameter — there is no `"system"` role for input messages in the Messages API.

  There is a limit of 100,000 messages in a single request.

  - `content: string or array of ContentBlockParam`

    - `string`

    - `array of ContentBlockParam`

      - `TextBlockParam object { text, type, cache_control, citations }`

        - `text: string`

        - `type: "text"`

          - `"text"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

          - `type: "ephemeral"`

            - `"ephemeral"`

          - `ttl: optional "5m" or "1h"`

            The time-to-live for the cache control breakpoint.

            This may be one the following values:

            - `5m`: 5 minutes
            - `1h`: 1 hour

            Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

            - `"5m"`

            - `"1h"`

        - `citations: optional array of TextCitationParam`

          - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_char_index: number`

            - `start_char_index: number`

            - `type: "char_location"`

              - `"char_location"`

          - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_page_number: number`

            - `start_page_number: number`

            - `type: "page_location"`

              - `"page_location"`

          - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `document_index: number`

            - `document_title: string`

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `type: "content_block_location"`

              - `"content_block_location"`

          - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

            - `cited_text: string`

            - `encrypted_index: string`

            - `title: string`

            - `type: "web_search_result_location"`

              - `"web_search_result_location"`

            - `url: string`

          - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `search_result_index: number`

              0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

              Counted separately from `document_index`; server-side web search results are not included in this count.

            - `source: string`

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `title: string`

            - `type: "search_result_location"`

              - `"search_result_location"`

      - `ImageBlockParam object { source, type, cache_control }`

        - `source: Base64ImageSource or URLImageSource`

          - `Base64ImageSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

              - `"image/jpeg"`

              - `"image/png"`

              - `"image/gif"`

              - `"image/webp"`

            - `type: "base64"`

              - `"base64"`

          - `URLImageSource object { type, url }`

            - `type: "url"`

              - `"url"`

            - `url: string`

        - `type: "image"`

          - `"image"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `DocumentBlockParam object { source, type, cache_control, 3 more }`

        - `source: Base64PDFSource or PlainTextSource or ContentBlockSource or URLPDFSource`

          - `Base64PDFSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "application/pdf"`

              - `"application/pdf"`

            - `type: "base64"`

              - `"base64"`

          - `PlainTextSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "text/plain"`

              - `"text/plain"`

            - `type: "text"`

              - `"text"`

          - `ContentBlockSource object { content, type }`

            - `content: string or array of ContentBlockSourceContent`

              - `string`

              - `ContentBlockSourceContent = array of ContentBlockSourceContent`

                - `TextBlockParam object { text, type, cache_control, citations }`

                - `ImageBlockParam object { source, type, cache_control }`

            - `type: "content"`

              - `"content"`

          - `URLPDFSource object { type, url }`

            - `type: "url"`

              - `"url"`

            - `url: string`

        - `type: "document"`

          - `"document"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

          - `enabled: optional boolean`

        - `context: optional string`

        - `title: optional string`

      - `SearchResultBlockParam object { content, source, title, 3 more }`

        - `content: array of TextBlockParam`

          - `text: string`

          - `type: "text"`

          - `cache_control: optional CacheControlEphemeral`

            Create a cache control breakpoint at this content block.

          - `citations: optional array of TextCitationParam`

        - `source: string`

        - `title: string`

        - `type: "search_result"`

          - `"search_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

      - `ThinkingBlockParam object { signature, thinking, type }`

        - `signature: string`

        - `thinking: string`

        - `type: "thinking"`

          - `"thinking"`

      - `RedactedThinkingBlockParam object { data, type }`

        - `data: string`

        - `type: "redacted_thinking"`

          - `"redacted_thinking"`

      - `ToolUseBlockParam object { id, input, name, 3 more }`

        - `id: string`

        - `input: map[unknown]`

        - `name: string`

        - `type: "tool_use"`

          - `"tool_use"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

            - `type: "direct"`

              - `"direct"`

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

            - `tool_id: string`

            - `type: "code_execution_20250825"`

              - `"code_execution_20250825"`

          - `ServerToolCaller20260120 object { tool_id, type }`

            - `tool_id: string`

            - `type: "code_execution_20260120"`

              - `"code_execution_20260120"`

      - `ToolResultBlockParam object { tool_use_id, type, cache_control, 2 more }`

        - `tool_use_id: string`

        - `type: "tool_result"`

          - `"tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `content: optional string or array of TextBlockParam or ImageBlockParam or SearchResultBlockParam or 2 more`

          - `string`

          - `array of TextBlockParam or ImageBlockParam or SearchResultBlockParam or 2 more`

            - `TextBlockParam object { text, type, cache_control, citations }`

            - `ImageBlockParam object { source, type, cache_control }`

            - `SearchResultBlockParam object { content, source, title, 3 more }`

            - `DocumentBlockParam object { source, type, cache_control, 3 more }`

            - `ToolReferenceBlockParam object { tool_name, type, cache_control }`

              Tool reference block that can be included in tool_result content.

              - `tool_name: string`

              - `type: "tool_reference"`

                - `"tool_reference"`

              - `cache_control: optional CacheControlEphemeral`

                Create a cache control breakpoint at this content block.

        - `is_error: optional boolean`

      - `ServerToolUseBlockParam object { id, input, name, 3 more }`

        - `id: string`

        - `input: map[unknown]`

        - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

          - `"web_search"`

          - `"web_fetch"`

          - `"code_execution"`

          - `"bash_code_execution"`

          - `"text_editor_code_execution"`

          - `"tool_search_tool_regex"`

          - `"tool_search_tool_bm25"`

        - `type: "server_tool_use"`

          - `"server_tool_use"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

      - `WebSearchToolResultBlockParam object { content, tool_use_id, type, 2 more }`

        - `content: WebSearchToolResultBlockParamContent`

          - `WebSearchToolResultBlockItem = array of WebSearchResultBlockParam`

            - `encrypted_content: string`

            - `title: string`

            - `type: "web_search_result"`

              - `"web_search_result"`

            - `url: string`

            - `page_age: optional string`

          - `WebSearchToolRequestError object { error_code, type }`

            - `error_code: WebSearchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"max_uses_exceeded"`

              - `"too_many_requests"`

              - `"query_too_long"`

              - `"request_too_large"`

            - `type: "web_search_tool_result_error"`

              - `"web_search_tool_result_error"`

        - `tool_use_id: string`

        - `type: "web_search_tool_result"`

          - `"web_search_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

      - `WebFetchToolResultBlockParam object { content, tool_use_id, type, 2 more }`

        - `content: WebFetchToolResultErrorBlockParam or WebFetchBlockParam`

          - `WebFetchToolResultErrorBlockParam object { error_code, type }`

            - `error_code: WebFetchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"url_too_long"`

              - `"url_not_allowed"`

              - `"url_not_in_prior_context"`

              - `"url_not_accessible"`

              - `"unsupported_content_type"`

              - `"too_many_requests"`

              - `"max_uses_exceeded"`

              - `"unavailable"`

            - `type: "web_fetch_tool_result_error"`

              - `"web_fetch_tool_result_error"`

          - `WebFetchBlockParam object { content, type, url, retrieved_at }`

            - `content: DocumentBlockParam`

            - `type: "web_fetch_result"`

              - `"web_fetch_result"`

            - `url: string`

              Fetched content URL

            - `retrieved_at: optional string`

              ISO 8601 timestamp when the content was retrieved

        - `tool_use_id: string`

        - `type: "web_fetch_tool_result"`

          - `"web_fetch_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

      - `CodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

        - `content: CodeExecutionToolResultBlockParamContent`

          Code execution result with encrypted stdout for PFC + web_search results.

          - `CodeExecutionToolResultErrorParam object { error_code, type }`

            - `error_code: CodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

            - `type: "code_execution_tool_result_error"`

              - `"code_execution_tool_result_error"`

          - `CodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

            - `content: array of CodeExecutionOutputBlockParam`

              - `file_id: string`

              - `type: "code_execution_output"`

                - `"code_execution_output"`

            - `return_code: number`

            - `stderr: string`

            - `stdout: string`

            - `type: "code_execution_result"`

              - `"code_execution_result"`

          - `EncryptedCodeExecutionResultBlockParam object { content, encrypted_stdout, return_code, 2 more }`

            Code execution result with encrypted stdout for PFC + web_search results.

            - `content: array of CodeExecutionOutputBlockParam`

              - `file_id: string`

              - `type: "code_execution_output"`

            - `encrypted_stdout: string`

            - `return_code: number`

            - `stderr: string`

            - `type: "encrypted_code_execution_result"`

              - `"encrypted_code_execution_result"`

        - `tool_use_id: string`

        - `type: "code_execution_tool_result"`

          - `"code_execution_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `BashCodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

        - `content: BashCodeExecutionToolResultErrorParam or BashCodeExecutionResultBlockParam`

          - `BashCodeExecutionToolResultErrorParam object { error_code, type }`

            - `error_code: BashCodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

              - `"output_file_too_large"`

            - `type: "bash_code_execution_tool_result_error"`

              - `"bash_code_execution_tool_result_error"`

          - `BashCodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

            - `content: array of BashCodeExecutionOutputBlockParam`

              - `file_id: string`

              - `type: "bash_code_execution_output"`

                - `"bash_code_execution_output"`

            - `return_code: number`

            - `stderr: string`

            - `stdout: string`

            - `type: "bash_code_execution_result"`

              - `"bash_code_execution_result"`

        - `tool_use_id: string`

        - `type: "bash_code_execution_tool_result"`

          - `"bash_code_execution_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `TextEditorCodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

        - `content: TextEditorCodeExecutionToolResultErrorParam or TextEditorCodeExecutionViewResultBlockParam or TextEditorCodeExecutionCreateResultBlockParam or TextEditorCodeExecutionStrReplaceResultBlockParam`

          - `TextEditorCodeExecutionToolResultErrorParam object { error_code, type, error_message }`

            - `error_code: TextEditorCodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

              - `"file_not_found"`

            - `type: "text_editor_code_execution_tool_result_error"`

              - `"text_editor_code_execution_tool_result_error"`

            - `error_message: optional string`

          - `TextEditorCodeExecutionViewResultBlockParam object { content, file_type, type, 3 more }`

            - `content: string`

            - `file_type: "text" or "image" or "pdf"`

              - `"text"`

              - `"image"`

              - `"pdf"`

            - `type: "text_editor_code_execution_view_result"`

              - `"text_editor_code_execution_view_result"`

            - `num_lines: optional number`

            - `start_line: optional number`

            - `total_lines: optional number`

          - `TextEditorCodeExecutionCreateResultBlockParam object { is_file_update, type }`

            - `is_file_update: boolean`

            - `type: "text_editor_code_execution_create_result"`

              - `"text_editor_code_execution_create_result"`

          - `TextEditorCodeExecutionStrReplaceResultBlockParam object { type, lines, new_lines, 3 more }`

            - `type: "text_editor_code_execution_str_replace_result"`

              - `"text_editor_code_execution_str_replace_result"`

            - `lines: optional array of string`

            - `new_lines: optional number`

            - `new_start: optional number`

            - `old_lines: optional number`

            - `old_start: optional number`

        - `tool_use_id: string`

        - `type: "text_editor_code_execution_tool_result"`

          - `"text_editor_code_execution_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `ToolSearchToolResultBlockParam object { content, tool_use_id, type, cache_control }`

        - `content: ToolSearchToolResultErrorParam or ToolSearchToolSearchResultBlockParam`

          - `ToolSearchToolResultErrorParam object { error_code, type, error_message }`

            - `error_code: ToolSearchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

            - `type: "tool_search_tool_result_error"`

              - `"tool_search_tool_result_error"`

            - `error_message: optional string`

          - `ToolSearchToolSearchResultBlockParam object { tool_references, type }`

            - `tool_references: array of ToolReferenceBlockParam`

              - `tool_name: string`

              - `type: "tool_reference"`

              - `cache_control: optional CacheControlEphemeral`

                Create a cache control breakpoint at this content block.

            - `type: "tool_search_tool_search_result"`

              - `"tool_search_tool_search_result"`

        - `tool_use_id: string`

        - `type: "tool_search_tool_result"`

          - `"tool_search_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `ContainerUploadBlockParam object { file_id, type, cache_control }`

        A content block that represents a file to be uploaded to the container
        Files uploaded via this block will be available in the container's input directory.

        - `file_id: string`

        - `type: "container_upload"`

          - `"container_upload"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `MidConversationSystemBlockParam object { content, type, cache_control }`

        System instructions that appear mid-conversation.

        Use this block to provide or update system-level instructions at a specific
        point in the conversation, rather than only via the top-level `system` parameter.

        - `content: array of TextBlockParam`

          System instruction text blocks.

          - `text: string`

          - `type: "text"`

          - `cache_control: optional CacheControlEphemeral`

            Create a cache control breakpoint at this content block.

          - `citations: optional array of TextCitationParam`

        - `type: "mid_conv_system"`

          - `"mid_conv_system"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

  - `role: "user" or "assistant" or "system"`

    - `"user"`

    - `"assistant"`

    - `"system"`

- `model: Model`

  The model that will complete your prompt.

  See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

  - `"claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more`

    The model that will complete your prompt.

    See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

    - `"claude-sonnet-5"`

      High-performance model for coding and agents

    - `"claude-fable-5"`

      Next generation of intelligence for the hardest knowledge work and coding problems

    - `"claude-mythos-5"`

      Most capable model for cybersecurity and biology research

    - `"claude-opus-4-8"`

      Frontier intelligence for long-running agents and coding

    - `"claude-opus-4-7"`

      Frontier intelligence for long-running agents and coding

    - `"claude-mythos-preview"`

      New class of intelligence, strongest in coding and cybersecurity

    - `"claude-opus-4-6"`

      Frontier intelligence for long-running agents and coding

    - `"claude-sonnet-4-6"`

      Best combination of speed and intelligence

    - `"claude-haiku-4-5"`

      Fastest model with near-frontier intelligence

    - `"claude-haiku-4-5-20251001"`

      Fastest model with near-frontier intelligence

    - `"claude-opus-4-5"`

      Premium model combining maximum intelligence with practical performance

    - `"claude-opus-4-5-20251101"`

      Premium model combining maximum intelligence with practical performance

    - `"claude-sonnet-4-5"`

      High-performance model for agents and coding

    - `"claude-sonnet-4-5-20250929"`

      High-performance model for agents and coding

    - `"claude-opus-4-1"`

      Exceptional model for specialized complex tasks

    - `"claude-opus-4-1-20250805"`

      Exceptional model for specialized complex tasks

  - `string`

- `cache_control: optional CacheControlEphemeral`

  Top-level cache control automatically applies a cache_control marker to the last cacheable block in the request.

- `container: optional string`

  Container identifier for reuse across requests.

- `inference_geo: optional string`

  Specifies the geographic region for inference processing. If not specified, the workspace's `default_inference_geo` is used.

- `metadata: optional Metadata`

  An object describing metadata about the request.

  - `user_id: optional string`

    An external identifier for the user who is associated with the request.

    This should be a uuid, hash value, or other opaque identifier. Anthropic may use this id to help detect abuse. Do not include any identifying information such as name, email address, or phone number.

- `output_config: optional OutputConfig`

  Configuration options for the model's output, such as the output format.

  - `effort: optional "low" or "medium" or "high" or 2 more`

    All possible effort levels.

    - `"low"`

    - `"medium"`

    - `"high"`

    - `"xhigh"`

    - `"max"`

  - `format: optional JSONOutputFormat`

    A schema to specify Claude's output format in responses. See [structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)

    - `schema: map[unknown]`

      The JSON schema of the format

    - `type: "json_schema"`

      - `"json_schema"`

- `service_tier: optional "auto" or "standard_only"`

  Determines whether to use priority capacity (if available) or standard capacity for this request.

  Anthropic offers different levels of service for your API requests. See [service-tiers](https://platform.claude.com/docs/en/api/service-tiers) for details.

  - `"auto"`

  - `"standard_only"`

- `stop_sequences: optional array of string`

  Custom text sequences that will cause the model to stop generating.

  Our models will normally stop when they have naturally completed their turn, which will result in a response `stop_reason` of `"end_turn"`.

  If you want the model to stop generating when it encounters custom strings of text, you can use the `stop_sequences` parameter. If the model encounters one of the custom sequences, the response `stop_reason` value will be `"stop_sequence"` and the response `stop_sequence` value will contain the matched stop sequence.

- `stream: optional boolean`

  Whether to incrementally stream the response using server-sent events.

  See [streaming](https://platform.claude.com/docs/en/build-with-claude/streaming) for details.

- `system: optional string or array of TextBlockParam`

  System prompt.

  A system prompt is a way of providing context and instructions to Claude, such as specifying a particular goal or role. See our [guide to system prompts](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#give-claude-a-role).

  - `string`

  - `array of TextBlockParam`

    - `text: string`

    - `type: "text"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional array of TextCitationParam`

- `temperature: optional number`

  Amount of randomness injected into the response.

  Defaults to `1.0`. Ranges from `0.0` to `1.0`. Use `temperature` closer to `0.0` for analytical / multiple choice, and closer to `1.0` for creative and generative tasks.

  Note that even with `temperature` of `0.0`, the results will not be fully deterministic.

- `thinking: optional ThinkingConfigParam`

  Configuration for enabling Claude's extended thinking.

  When enabled, responses include `thinking` content blocks showing Claude's thinking process before the final answer. Requires a minimum budget of 1,024 tokens and counts towards your `max_tokens` limit.

  See [extended thinking](https://platform.claude.com/docs/en/build-with-claude/extended-thinking) for details.

  - `ThinkingConfigEnabled object { budget_tokens, type, display }`

    - `budget_tokens: number`

      Determines how many tokens Claude can use for its internal reasoning process. Larger budgets can enable more thorough analysis for complex problems, improving response quality.

      Must be ≥1024 and less than `max_tokens`.

      See [extended thinking](https://platform.claude.com/docs/en/build-with-claude/extended-thinking) for details.

    - `type: "enabled"`

      - `"enabled"`

    - `display: optional "summarized" or "omitted"`

      Controls how thinking content appears in the response. When set to `summarized`, thinking is returned normally. When set to `omitted`, thinking content is redacted but a signature is returned for multi-turn continuity. Defaults to `summarized`.

      - `"summarized"`

      - `"omitted"`

  - `ThinkingConfigDisabled object { type }`

    - `type: "disabled"`

      - `"disabled"`

  - `ThinkingConfigAdaptive object { type, display }`

    - `type: "adaptive"`

      - `"adaptive"`

    - `display: optional "summarized" or "omitted"`

      Controls how thinking content appears in the response. When set to `summarized`, thinking is returned normally. When set to `omitted`, thinking content is redacted but a signature is returned for multi-turn continuity. Defaults to `summarized`.

      - `"summarized"`

      - `"omitted"`

- `tool_choice: optional ToolChoice`

  How the model should use the provided tools. The model can use a specific tool, any available tool, decide by itself, or not use tools at all.

  - `ToolChoiceAuto object { type, disable_parallel_tool_use }`

    The model will automatically decide whether to use tools.

    - `type: "auto"`

      - `"auto"`

    - `disable_parallel_tool_use: optional boolean`

      Whether to disable parallel tool use.

      Defaults to `false`. If set to `true`, the model will output at most one tool use.

  - `ToolChoiceAny object { type, disable_parallel_tool_use }`

    The model will use any available tools.

    - `type: "any"`

      - `"any"`

    - `disable_parallel_tool_use: optional boolean`

      Whether to disable parallel tool use.

      Defaults to `false`. If set to `true`, the model will output exactly one tool use.

  - `ToolChoiceTool object { name, type, disable_parallel_tool_use }`

    The model will use the specified tool with `tool_choice.name`.

    - `name: string`

      The name of the tool to use.

    - `type: "tool"`

      - `"tool"`

    - `disable_parallel_tool_use: optional boolean`

      Whether to disable parallel tool use.

      Defaults to `false`. If set to `true`, the model will output exactly one tool use.

  - `ToolChoiceNone object { type }`

    The model will not be allowed to use tools.

    - `type: "none"`

      - `"none"`

- `tools: optional array of ToolUnion`

  Definitions of tools that the model may use.

  If you include `tools` in your API request, the model may return `tool_use` content blocks that represent the model's use of those tools. You can then run those tools using the tool input generated by the model and then optionally return results back to the model using `tool_result` content blocks.

  There are two types of tools: **client tools** and **server tools**. The behavior described below applies to client tools. For [server tools](https://platform.claude.com/docs/en/agents-and-tools/tool-use/server-tools), see their individual documentation as each has its own behavior (e.g., the [web search tool](https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-search-tool)).

  Each tool definition includes:

  * `name`: Name of the tool.
  * `description`: Optional, but strongly-recommended description of the tool.
  * `input_schema`: [JSON schema](https://json-schema.org/draft/2020-12) for the tool `input` shape that the model will produce in `tool_use` output content blocks.

  For example, if you defined `tools` as:

  ```json
  [
    {
      "name": "get_stock_price",
      "description": "Get the current stock price for a given ticker symbol.",
      "input_schema": {
        "type": "object",
        "properties": {
          "ticker": {
            "type": "string",
            "description": "The stock ticker symbol, e.g. AAPL for Apple Inc."
          }
        },
        "required": ["ticker"]
      }
    }
  ]
  ```

  And then asked the model "What's the S&P 500 at today?", the model might produce `tool_use` content blocks in the response like this:

  ```json
  [
    {
      "type": "tool_use",
      "id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
      "name": "get_stock_price",
      "input": { "ticker": "^GSPC" }
    }
  ]
  ```

  You might then run your `get_stock_price` tool with `{"ticker": "^GSPC"}` as an input, and return the following back to the model in a subsequent `user` message:

  ```json
  [
    {
      "type": "tool_result",
      "tool_use_id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
      "content": "259.75 USD"
    }
  ]
  ```

  Tools can be used for workflows that include running client-side tools and functions, or more generally whenever you want the model to produce a particular JSON structure of output.

  See our [guide](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview) for more details.

  - `Tool object { input_schema, name, allowed_callers, 7 more }`

    - `input_schema: object { type, properties, required }`

      [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.

      This defines the shape of the `input` that your tool accepts and that the model will produce.

      - `type: "object"`

        - `"object"`

      - `properties: optional map[unknown]`

      - `required: optional array of string`

    - `name: string`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `description: optional string`

      Description of what this tool does.

      Tool descriptions should be as detailed as possible. The more information that the model has about what the tool is and how to use it, the better it will perform. You can use natural language descriptions to reinforce important aspects of the tool input JSON schema.

    - `eager_input_streaming: optional boolean`

      Enable eager input streaming for this tool. When true, tool input parameters will be streamed incrementally as they are generated, and types will be inferred on-the-fly rather than buffering the full JSON output. When false, streaming is disabled for this tool even if the fine-grained-tool-streaming beta is active. When null (default), uses the default behavior based on beta headers.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `type: optional "custom"`

      - `"custom"`

  - `ToolBash20250124 object { name, type, allowed_callers, 4 more }`

    - `name: "bash"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"bash"`

    - `type: "bash_20250124"`

      - `"bash_20250124"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20250522 object { name, type, allowed_callers, 3 more }`

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20250522"`

      - `"code_execution_20250522"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20250825 object { name, type, allowed_callers, 3 more }`

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20250825"`

      - `"code_execution_20250825"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20260120 object { name, type, allowed_callers, 3 more }`

    Code execution tool with REPL state persistence (daemon mode + gVisor checkpoint).

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20260120"`

      - `"code_execution_20260120"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20260521 object { name, type, allowed_callers, 3 more }`

    Code execution tool with REPL state persistence.

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20260521"`

      - `"code_execution_20260521"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `MemoryTool20250818 object { name, type, allowed_callers, 4 more }`

    - `name: "memory"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"memory"`

    - `type: "memory_20250818"`

      - `"memory_20250818"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolTextEditor20250124 object { name, type, allowed_callers, 4 more }`

    - `name: "str_replace_editor"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"str_replace_editor"`

    - `type: "text_editor_20250124"`

      - `"text_editor_20250124"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolTextEditor20250429 object { name, type, allowed_callers, 4 more }`

    - `name: "str_replace_based_edit_tool"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"str_replace_based_edit_tool"`

    - `type: "text_editor_20250429"`

      - `"text_editor_20250429"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolTextEditor20250728 object { name, type, allowed_callers, 5 more }`

    - `name: "str_replace_based_edit_tool"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"str_replace_based_edit_tool"`

    - `type: "text_editor_20250728"`

      - `"text_editor_20250728"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `max_characters: optional number`

      Maximum number of characters to display when viewing a file. If not specified, defaults to displaying the full file.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `WebSearchTool20250305 object { name, type, allowed_callers, 7 more }`

    - `name: "web_search"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_search"`

    - `type: "web_search_20250305"`

      - `"web_search_20250305"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

    - `blocked_domains: optional array of string`

      If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `user_location: optional UserLocation`

      Parameters for the user's location. Used to provide more relevant search results.

      - `type: "approximate"`

        - `"approximate"`

      - `city: optional string`

        The city of the user.

      - `country: optional string`

        The two letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the user.

      - `region: optional string`

        The region of the user.

      - `timezone: optional string`

        The [IANA timezone](https://nodatime.org/TimeZones) of the user.

  - `WebFetchTool20250910 object { name, type, allowed_callers, 8 more }`

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20250910"`

      - `"web_fetch_20250910"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `WebSearchTool20260209 object { name, type, allowed_callers, 7 more }`

    - `name: "web_search"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_search"`

    - `type: "web_search_20260209"`

      - `"web_search_20260209"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

    - `blocked_domains: optional array of string`

      If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `user_location: optional UserLocation`

      Parameters for the user's location. Used to provide more relevant search results.

  - `WebFetchTool20260209 object { name, type, allowed_callers, 8 more }`

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20260209"`

      - `"web_fetch_20260209"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `WebFetchTool20260309 object { name, type, allowed_callers, 9 more }`

    Web fetch tool with use_cache parameter for bypassing cached content.

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20260309"`

      - `"web_fetch_20260309"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `use_cache: optional boolean`

      Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.

  - `WebSearchTool20260318 object { name, type, allowed_callers, 8 more }`

    - `name: "web_search"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_search"`

    - `type: "web_search_20260318"`

      - `"web_search_20260318"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

    - `blocked_domains: optional array of string`

      If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `response_inclusion: optional "full" or "excluded"`

      How this tool's result blocks appear in the API response when the result was consumed by a completed code_execution call in the same turn. 'full' returns the complete content (default). 'excluded' drops the nested server_tool_use and result block pair entirely. Results from direct calls, or from code_execution calls that paused before completing, are always returned in full so they can be sent back on the next turn.

      - `"full"`

      - `"excluded"`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `user_location: optional UserLocation`

      Parameters for the user's location. Used to provide more relevant search results.

  - `WebFetchTool20260318 object { name, type, allowed_callers, 10 more }`

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20260318"`

      - `"web_fetch_20260318"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `response_inclusion: optional "full" or "excluded"`

      How this tool's result blocks appear in the API response when the result was consumed by a completed code_execution call in the same turn. 'full' returns the complete content (default). 'excluded' drops the nested server_tool_use and result block pair entirely. Results from direct calls, or from code_execution calls that paused before completing, are always returned in full so they can be sent back on the next turn.

      - `"full"`

      - `"excluded"`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `use_cache: optional boolean`

      Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.

  - `ToolSearchToolBm25_20251119 object { name, type, allowed_callers, 3 more }`

    - `name: "tool_search_tool_bm25"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"tool_search_tool_bm25"`

    - `type: "tool_search_tool_bm25_20251119" or "tool_search_tool_bm25"`

      - `"tool_search_tool_bm25_20251119"`

      - `"tool_search_tool_bm25"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolSearchToolRegex20251119 object { name, type, allowed_callers, 3 more }`

    - `name: "tool_search_tool_regex"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"tool_search_tool_regex"`

    - `type: "tool_search_tool_regex_20251119" or "tool_search_tool_regex"`

      - `"tool_search_tool_regex_20251119"`

      - `"tool_search_tool_regex"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

- `top_k: optional number`

  Only sample from the top K options for each subsequent token.

  Used to remove "long tail" low probability responses. [Learn more technical details here](https://towardsdatascience.com/how-to-sample-from-language-models-682bceb97277).

  Recommended for advanced use cases only.

- `top_p: optional number`

  Use nucleus sampling.

  In nucleus sampling, we compute the cumulative distribution over all the options for each subsequent token in decreasing probability order and cut it off once it reaches a particular probability specified by `top_p`.

  Recommended for advanced use cases only.

### Returns

- `Message object { id, container, content, 7 more }`

  - `id: string`

    Unique object identifier.

    The format and length of IDs may change over time.

  - `container: Container`

    Information about the container used in the request (for the code execution tool)

    - `id: string`

      Identifier for the container used in this request

    - `expires_at: string`

      The time at which the container will expire.

  - `content: array of ContentBlock`

    Content generated by the model.

    This is an array of content blocks, each of which has a `type` that determines its shape.

    Example:

    ```json
    [{"type": "text", "text": "Hi, I'm Claude."}]
    ```

    If the request input `messages` ended with an `assistant` turn, then the response `content` will continue directly from that last turn. You can use this to constrain the model's output.

    For example, if the input `messages` were:

    ```json
    [
      {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
      {"role": "assistant", "content": "The best answer is ("}
    ]
    ```

    Then the response `content` might be:

    ```json
    [{"type": "text", "text": "B)"}]
    ```

    - `TextBlock object { citations, text, type }`

      - `citations: array of TextCitation`

        Citations supporting the text block.

        The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.

        - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

          - `cited_text: string`

          - `document_index: number`

          - `document_title: string`

          - `end_char_index: number`

          - `file_id: string`

          - `start_char_index: number`

          - `type: "char_location"`

            - `"char_location"`

        - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

          - `cited_text: string`

          - `document_index: number`

          - `document_title: string`

          - `end_page_number: number`

          - `file_id: string`

          - `start_page_number: number`

          - `type: "page_location"`

            - `"page_location"`

        - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

          - `cited_text: string`

            The full text of the cited block range, concatenated.

            Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

          - `document_index: number`

          - `document_title: string`

          - `end_block_index: number`

            Exclusive 0-based end index of the cited block range in the source's `content` array.

            Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

          - `file_id: string`

          - `start_block_index: number`

            0-based index of the first cited block in the source's `content` array.

          - `type: "content_block_location"`

            - `"content_block_location"`

        - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

          - `cited_text: string`

          - `encrypted_index: string`

          - `title: string`

          - `type: "web_search_result_location"`

            - `"web_search_result_location"`

          - `url: string`

        - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

          - `cited_text: string`

            The full text of the cited block range, concatenated.

            Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

          - `end_block_index: number`

            Exclusive 0-based end index of the cited block range in the source's `content` array.

            Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

          - `search_result_index: number`

            0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

            Counted separately from `document_index`; server-side web search results are not included in this count.

          - `source: string`

          - `start_block_index: number`

            0-based index of the first cited block in the source's `content` array.

          - `title: string`

          - `type: "search_result_location"`

            - `"search_result_location"`

      - `text: string`

      - `type: "text"`

        - `"text"`

    - `ThinkingBlock object { signature, thinking, type }`

      - `signature: string`

      - `thinking: string`

      - `type: "thinking"`

        - `"thinking"`

    - `RedactedThinkingBlock object { data, type }`

      - `data: string`

      - `type: "redacted_thinking"`

        - `"redacted_thinking"`

    - `ToolUseBlock object { id, caller, input, 2 more }`

      - `id: string`

      - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

        Tool invocation directly from the model.

        - `DirectCaller object { type }`

          Tool invocation directly from the model.

          - `type: "direct"`

            - `"direct"`

        - `ServerToolCaller object { tool_id, type }`

          Tool invocation generated by a server-side tool.

          - `tool_id: string`

          - `type: "code_execution_20250825"`

            - `"code_execution_20250825"`

        - `ServerToolCaller20260120 object { tool_id, type }`

          - `tool_id: string`

          - `type: "code_execution_20260120"`

            - `"code_execution_20260120"`

      - `input: map[unknown]`

      - `name: string`

      - `type: "tool_use"`

        - `"tool_use"`

    - `ServerToolUseBlock object { id, caller, input, 2 more }`

      - `id: string`

      - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

        Tool invocation directly from the model.

        - `DirectCaller object { type }`

          Tool invocation directly from the model.

        - `ServerToolCaller object { tool_id, type }`

          Tool invocation generated by a server-side tool.

        - `ServerToolCaller20260120 object { tool_id, type }`

      - `input: map[unknown]`

      - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

        - `"web_search"`

        - `"web_fetch"`

        - `"code_execution"`

        - `"bash_code_execution"`

        - `"text_editor_code_execution"`

        - `"tool_search_tool_regex"`

        - `"tool_search_tool_bm25"`

      - `type: "server_tool_use"`

        - `"server_tool_use"`

    - `WebSearchToolResultBlock object { caller, content, tool_use_id, type }`

      - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

        Tool invocation directly from the model.

        - `DirectCaller object { type }`

          Tool invocation directly from the model.

        - `ServerToolCaller object { tool_id, type }`

          Tool invocation generated by a server-side tool.

        - `ServerToolCaller20260120 object { tool_id, type }`

      - `content: WebSearchToolResultBlockContent`

        - `WebSearchToolResultError object { error_code, type }`

          - `error_code: WebSearchToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"max_uses_exceeded"`

            - `"too_many_requests"`

            - `"query_too_long"`

            - `"request_too_large"`

          - `type: "web_search_tool_result_error"`

            - `"web_search_tool_result_error"`

        - `array of WebSearchResultBlock`

          - `encrypted_content: string`

          - `page_age: string`

          - `title: string`

          - `type: "web_search_result"`

            - `"web_search_result"`

          - `url: string`

      - `tool_use_id: string`

      - `type: "web_search_tool_result"`

        - `"web_search_tool_result"`

    - `WebFetchToolResultBlock object { caller, content, tool_use_id, type }`

      - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

        Tool invocation directly from the model.

        - `DirectCaller object { type }`

          Tool invocation directly from the model.

        - `ServerToolCaller object { tool_id, type }`

          Tool invocation generated by a server-side tool.

        - `ServerToolCaller20260120 object { tool_id, type }`

      - `content: WebFetchToolResultErrorBlock or WebFetchBlock`

        - `WebFetchToolResultErrorBlock object { error_code, type }`

          - `error_code: WebFetchToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"url_too_long"`

            - `"url_not_allowed"`

            - `"url_not_in_prior_context"`

            - `"url_not_accessible"`

            - `"unsupported_content_type"`

            - `"too_many_requests"`

            - `"max_uses_exceeded"`

            - `"unavailable"`

          - `type: "web_fetch_tool_result_error"`

            - `"web_fetch_tool_result_error"`

        - `WebFetchBlock object { content, retrieved_at, type, url }`

          - `content: DocumentBlock`

            - `citations: CitationsConfig`

              Citation configuration for the document

              - `enabled: boolean`

            - `source: Base64PDFSource or PlainTextSource`

              - `Base64PDFSource object { data, media_type, type }`

                - `data: string`

                - `media_type: "application/pdf"`

                  - `"application/pdf"`

                - `type: "base64"`

                  - `"base64"`

              - `PlainTextSource object { data, media_type, type }`

                - `data: string`

                - `media_type: "text/plain"`

                  - `"text/plain"`

                - `type: "text"`

                  - `"text"`

            - `title: string`

              The title of the document

            - `type: "document"`

              - `"document"`

          - `retrieved_at: string`

            ISO 8601 timestamp when the content was retrieved

          - `type: "web_fetch_result"`

            - `"web_fetch_result"`

          - `url: string`

            Fetched content URL

      - `tool_use_id: string`

      - `type: "web_fetch_tool_result"`

        - `"web_fetch_tool_result"`

    - `CodeExecutionToolResultBlock object { content, tool_use_id, type }`

      - `content: CodeExecutionToolResultBlockContent`

        Code execution result with encrypted stdout for PFC + web_search results.

        - `CodeExecutionToolResultError object { error_code, type }`

          - `error_code: CodeExecutionToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"too_many_requests"`

            - `"execution_time_exceeded"`

          - `type: "code_execution_tool_result_error"`

            - `"code_execution_tool_result_error"`

        - `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

          - `content: array of CodeExecutionOutputBlock`

            - `file_id: string`

            - `type: "code_execution_output"`

              - `"code_execution_output"`

          - `return_code: number`

          - `stderr: string`

          - `stdout: string`

          - `type: "code_execution_result"`

            - `"code_execution_result"`

        - `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

          Code execution result with encrypted stdout for PFC + web_search results.

          - `content: array of CodeExecutionOutputBlock`

            - `file_id: string`

            - `type: "code_execution_output"`

          - `encrypted_stdout: string`

          - `return_code: number`

          - `stderr: string`

          - `type: "encrypted_code_execution_result"`

            - `"encrypted_code_execution_result"`

      - `tool_use_id: string`

      - `type: "code_execution_tool_result"`

        - `"code_execution_tool_result"`

    - `BashCodeExecutionToolResultBlock object { content, tool_use_id, type }`

      - `content: BashCodeExecutionToolResultError or BashCodeExecutionResultBlock`

        - `BashCodeExecutionToolResultError object { error_code, type }`

          - `error_code: BashCodeExecutionToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"too_many_requests"`

            - `"execution_time_exceeded"`

            - `"output_file_too_large"`

          - `type: "bash_code_execution_tool_result_error"`

            - `"bash_code_execution_tool_result_error"`

        - `BashCodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

          - `content: array of BashCodeExecutionOutputBlock`

            - `file_id: string`

            - `type: "bash_code_execution_output"`

              - `"bash_code_execution_output"`

          - `return_code: number`

          - `stderr: string`

          - `stdout: string`

          - `type: "bash_code_execution_result"`

            - `"bash_code_execution_result"`

      - `tool_use_id: string`

      - `type: "bash_code_execution_tool_result"`

        - `"bash_code_execution_tool_result"`

    - `TextEditorCodeExecutionToolResultBlock object { content, tool_use_id, type }`

      - `content: TextEditorCodeExecutionToolResultError or TextEditorCodeExecutionViewResultBlock or TextEditorCodeExecutionCreateResultBlock or TextEditorCodeExecutionStrReplaceResultBlock`

        - `TextEditorCodeExecutionToolResultError object { error_code, error_message, type }`

          - `error_code: TextEditorCodeExecutionToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"too_many_requests"`

            - `"execution_time_exceeded"`

            - `"file_not_found"`

          - `error_message: string`

          - `type: "text_editor_code_execution_tool_result_error"`

            - `"text_editor_code_execution_tool_result_error"`

        - `TextEditorCodeExecutionViewResultBlock object { content, file_type, num_lines, 3 more }`

          - `content: string`

          - `file_type: "text" or "image" or "pdf"`

            - `"text"`

            - `"image"`

            - `"pdf"`

          - `num_lines: number`

          - `start_line: number`

          - `total_lines: number`

          - `type: "text_editor_code_execution_view_result"`

            - `"text_editor_code_execution_view_result"`

        - `TextEditorCodeExecutionCreateResultBlock object { is_file_update, type }`

          - `is_file_update: boolean`

          - `type: "text_editor_code_execution_create_result"`

            - `"text_editor_code_execution_create_result"`

        - `TextEditorCodeExecutionStrReplaceResultBlock object { lines, new_lines, new_start, 3 more }`

          - `lines: array of string`

          - `new_lines: number`

          - `new_start: number`

          - `old_lines: number`

          - `old_start: number`

          - `type: "text_editor_code_execution_str_replace_result"`

            - `"text_editor_code_execution_str_replace_result"`

      - `tool_use_id: string`

      - `type: "text_editor_code_execution_tool_result"`

        - `"text_editor_code_execution_tool_result"`

    - `ToolSearchToolResultBlock object { content, tool_use_id, type }`

      - `content: ToolSearchToolResultError or ToolSearchToolSearchResultBlock`

        - `ToolSearchToolResultError object { error_code, error_message, type }`

          - `error_code: ToolSearchToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"too_many_requests"`

            - `"execution_time_exceeded"`

          - `error_message: string`

          - `type: "tool_search_tool_result_error"`

            - `"tool_search_tool_result_error"`

        - `ToolSearchToolSearchResultBlock object { tool_references, type }`

          - `tool_references: array of ToolReferenceBlock`

            - `tool_name: string`

            - `type: "tool_reference"`

              - `"tool_reference"`

          - `type: "tool_search_tool_search_result"`

            - `"tool_search_tool_search_result"`

      - `tool_use_id: string`

      - `type: "tool_search_tool_result"`

        - `"tool_search_tool_result"`

    - `ContainerUploadBlock object { file_id, type }`

      Response model for a file uploaded to the container.

      - `file_id: string`

      - `type: "container_upload"`

        - `"container_upload"`

  - `model: Model`

    The model that will complete your prompt.

    See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

    - `"claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more`

      The model that will complete your prompt.

      See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

      - `"claude-sonnet-5"`

        High-performance model for coding and agents

      - `"claude-fable-5"`

        Next generation of intelligence for the hardest knowledge work and coding problems

      - `"claude-mythos-5"`

        Most capable model for cybersecurity and biology research

      - `"claude-opus-4-8"`

        Frontier intelligence for long-running agents and coding

      - `"claude-opus-4-7"`

        Frontier intelligence for long-running agents and coding

      - `"claude-mythos-preview"`

        New class of intelligence, strongest in coding and cybersecurity

      - `"claude-opus-4-6"`

        Frontier intelligence for long-running agents and coding

      - `"claude-sonnet-4-6"`

        Best combination of speed and intelligence

      - `"claude-haiku-4-5"`

        Fastest model with near-frontier intelligence

      - `"claude-haiku-4-5-20251001"`

        Fastest model with near-frontier intelligence

      - `"claude-opus-4-5"`

        Premium model combining maximum intelligence with practical performance

      - `"claude-opus-4-5-20251101"`

        Premium model combining maximum intelligence with practical performance

      - `"claude-sonnet-4-5"`

        High-performance model for agents and coding

      - `"claude-sonnet-4-5-20250929"`

        High-performance model for agents and coding

      - `"claude-opus-4-1"`

        Exceptional model for specialized complex tasks

      - `"claude-opus-4-1-20250805"`

        Exceptional model for specialized complex tasks

    - `string`

  - `role: "assistant"`

    Conversational role of the generated message.

    This will always be `"assistant"`.

    - `"assistant"`

  - `stop_details: RefusalStopDetails`

    Structured information about a refusal.

    - `category: "cyber" or "bio" or "frontier_llm" or "reasoning_extraction"`

      The policy category that triggered a refusal.

      - `"cyber"`

      - `"bio"`

      - `"frontier_llm"`

      - `"reasoning_extraction"`

    - `explanation: string`

      Human-readable explanation of the refusal.

      This text is not guaranteed to be stable. `null` when no explanation is available for the category.

    - `type: "refusal"`

      - `"refusal"`

  - `stop_reason: StopReason`

    The reason that we stopped.

    This may be one the following values:

    * `"end_turn"`: the model reached a natural stopping point
    * `"max_tokens"`: we exceeded the requested `max_tokens` or the model's maximum
    * `"stop_sequence"`: one of your provided custom `stop_sequences` was generated
    * `"tool_use"`: the model invoked one or more tools
    * `"pause_turn"`: we paused a long-running turn. You may provide the response back as-is in a subsequent request to let the model continue.
    * `"refusal"`: when streaming classifiers intervene to handle potential policy violations

    In non-streaming mode this value is always non-null. In streaming mode, it is null in the `message_start` event and non-null otherwise.

    - `"end_turn"`

    - `"max_tokens"`

    - `"stop_sequence"`

    - `"tool_use"`

    - `"pause_turn"`

    - `"refusal"`

  - `stop_sequence: string`

    Which custom stop sequence was generated, if any.

    This value will be a non-null string if one of your custom stop sequences was generated.

  - `type: "message"`

    Object type.

    For Messages, this is always `"message"`.

    - `"message"`

  - `usage: Usage`

    Billing and rate-limit usage.

    Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems.

    Under the hood, the API transforms requests into a format suitable for the model. The model's output then goes through a parsing stage before becoming an API response. As a result, the token counts in `usage` will not match one-to-one with the exact visible content of an API request or response.

    For example, `output_tokens` will be non-zero, even for an empty string response from Claude.

    Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.

    - `cache_creation: CacheCreation`

      Breakdown of cached tokens by TTL

      - `ephemeral_1h_input_tokens: number`

        The number of input tokens used to create the 1 hour cache entry.

      - `ephemeral_5m_input_tokens: number`

        The number of input tokens used to create the 5 minute cache entry.

    - `cache_creation_input_tokens: number`

      The number of input tokens used to create the cache entry.

    - `cache_read_input_tokens: number`

      The number of input tokens read from the cache.

    - `inference_geo: string`

      The geographic region where inference was performed for this request.

    - `input_tokens: number`

      The number of input tokens which were used.

    - `output_tokens: number`

      The number of output tokens which were used.

    - `output_tokens_details: OutputTokensDetails`

      Breakdown of output tokens by category.

      `output_tokens` remains the inclusive, authoritative total used for billing.
      This object provides a read-only decomposition for observability — for example,
      how many of the billed output tokens were spent on internal reasoning that may
      have been summarized before being returned to you.

      - `thinking_tokens: number`

        Number of output tokens the model generated as internal reasoning, including
        the thinking-block delimiter tokens.

        Reflects the raw reasoning the model produced, not the (possibly shorter)
        summarized thinking text returned in the response body. Computed by
        re-tokenizing the raw reasoning text, so it may differ from the model's exact
        generation count by a small number of tokens. Always ≤ `output_tokens`;
        `output_tokens - thinking_tokens` approximates the non-reasoning output.

    - `server_tool_use: ServerToolUsage`

      The number of server tool requests.

      - `web_fetch_requests: number`

        The number of web fetch tool requests.

      - `web_search_requests: number`

        The number of web search tool requests.

    - `service_tier: "standard" or "priority" or "batch"`

      If the request used the priority, standard, or batch tier.

      - `"standard"`

      - `"priority"`

      - `"batch"`

### Example

```http
curl https://api.anthropic.com/v1/messages \
    -H 'Content-Type: application/json' \
    -H 'anthropic-version: 2023-06-01' \
    -H "X-Api-Key: $ANTHROPIC_API_KEY" \
    --max-time 600 \
    -d "{
          \"max_tokens\": 1024,
          \"messages\": [
            {
              \"content\": \"Hello, world\",
              \"role\": \"user\"
            }
          ],
          \"model\": \"claude-opus-4-6\",
          \"stream\": false,
          \"system\": [
            {
              \"text\": \"Today's date is 2024-06-01.\",
              \"type\": \"text\"
            }
          ],
          \"temperature\": 1,
          \"thinking\": {
            \"type\": \"adaptive\"
          },
          \"tools\": [
            {
              \"input_schema\": {
                \"type\": \"object\",
                \"properties\": {
                  \"location\": \"bar\",
                  \"unit\": \"bar\"
                },
                \"required\": [
                  \"location\"
                ]
              },
              \"name\": \"name\"
            }
          ],
          \"top_k\": 5,
          \"top_p\": 0.7
        }"
```

#### Response

```json
{
  "id": "msg_013Zva2CMHLNnXjNJJKqJ2EF",
  "container": {
    "id": "id",
    "expires_at": "2019-12-27T18:11:19.117Z"
  },
  "content": [
    {
      "citations": [
        {
          "cited_text": "cited_text",
          "document_index": 0,
          "document_title": "document_title",
          "end_char_index": 0,
          "file_id": "file_id",
          "start_char_index": 0,
          "type": "char_location"
        }
      ],
      "text": "Hi! My name is Claude.",
      "type": "text"
    }
  ],
  "model": "claude-opus-4-6",
  "role": "assistant",
  "stop_details": {
    "category": "cyber",
    "explanation": "explanation",
    "type": "refusal"
  },
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "type": "message",
  "usage": {
    "cache_creation": {
      "ephemeral_1h_input_tokens": 0,
      "ephemeral_5m_input_tokens": 0
    },
    "cache_creation_input_tokens": 2051,
    "cache_read_input_tokens": 2051,
    "inference_geo": "inference_geo",
    "input_tokens": 2095,
    "output_tokens": 503,
    "output_tokens_details": {
      "thinking_tokens": 0
    },
    "server_tool_use": {
      "web_fetch_requests": 2,
      "web_search_requests": 0
    },
    "service_tier": "standard"
  }
}
```

## Count tokens in a Message

**post** `/v1/messages/count_tokens`

Count the number of tokens in a Message.

The Token Count API can be used to count the number of tokens in a Message, including tools, images, and documents, without creating it.

Learn more about token counting in our [user guide](https://platform.claude.com/docs/en/build-with-claude/token-counting)

### Header Parameters

- `"anthropic-user-profile-id": optional string`

  The user profile ID to attribute this request to. Use when acting on behalf of a party other than your organization. Requires the `user-profiles` beta header.

### Body Parameters

- `messages: array of MessageParam`

  Input messages.

  Our models are trained to operate on alternating `user` and `assistant` conversational turns. When creating a new `Message`, you specify the prior conversational turns with the `messages` parameter, and the model then generates the next `Message` in the conversation. Consecutive `user` or `assistant` turns in your request will be combined into a single turn.

  Each input message must be an object with a `role` and `content`. You can specify a single `user`-role message, or you can include multiple `user` and `assistant` messages.

  If the final message uses the `assistant` role, the response content will continue immediately from the content in that message. This can be used to constrain part of the model's response.

  Example with a single `user` message:

  ```json
  [{"role": "user", "content": "Hello, Claude"}]
  ```

  Example with multiple conversational turns:

  ```json
  [
    {"role": "user", "content": "Hello there."},
    {"role": "assistant", "content": "Hi, I'm Claude. How can I help you?"},
    {"role": "user", "content": "Can you explain LLMs in plain English?"},
  ]
  ```

  Example with a partially-filled response from Claude:

  ```json
  [
    {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
    {"role": "assistant", "content": "The best answer is ("},
  ]
  ```

  Each input message `content` may be either a single `string` or an array of content blocks, where each block has a specific `type`. Using a `string` for `content` is shorthand for an array of one content block of type `"text"`. The following input messages are equivalent:

  ```json
  {"role": "user", "content": "Hello, Claude"}
  ```

  ```json
  {"role": "user", "content": [{"type": "text", "text": "Hello, Claude"}]}
  ```

  See [input examples](https://platform.claude.com/docs/en/build-with-claude/working-with-messages).

  Note that if you want to include a [system prompt](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#give-claude-a-role), you can use the top-level `system` parameter — there is no `"system"` role for input messages in the Messages API.

  There is a limit of 100,000 messages in a single request.

  - `content: string or array of ContentBlockParam`

    - `string`

    - `array of ContentBlockParam`

      - `TextBlockParam object { text, type, cache_control, citations }`

        - `text: string`

        - `type: "text"`

          - `"text"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

          - `type: "ephemeral"`

            - `"ephemeral"`

          - `ttl: optional "5m" or "1h"`

            The time-to-live for the cache control breakpoint.

            This may be one the following values:

            - `5m`: 5 minutes
            - `1h`: 1 hour

            Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

            - `"5m"`

            - `"1h"`

        - `citations: optional array of TextCitationParam`

          - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_char_index: number`

            - `start_char_index: number`

            - `type: "char_location"`

              - `"char_location"`

          - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_page_number: number`

            - `start_page_number: number`

            - `type: "page_location"`

              - `"page_location"`

          - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `document_index: number`

            - `document_title: string`

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `type: "content_block_location"`

              - `"content_block_location"`

          - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

            - `cited_text: string`

            - `encrypted_index: string`

            - `title: string`

            - `type: "web_search_result_location"`

              - `"web_search_result_location"`

            - `url: string`

          - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `search_result_index: number`

              0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

              Counted separately from `document_index`; server-side web search results are not included in this count.

            - `source: string`

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `title: string`

            - `type: "search_result_location"`

              - `"search_result_location"`

      - `ImageBlockParam object { source, type, cache_control }`

        - `source: Base64ImageSource or URLImageSource`

          - `Base64ImageSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

              - `"image/jpeg"`

              - `"image/png"`

              - `"image/gif"`

              - `"image/webp"`

            - `type: "base64"`

              - `"base64"`

          - `URLImageSource object { type, url }`

            - `type: "url"`

              - `"url"`

            - `url: string`

        - `type: "image"`

          - `"image"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `DocumentBlockParam object { source, type, cache_control, 3 more }`

        - `source: Base64PDFSource or PlainTextSource or ContentBlockSource or URLPDFSource`

          - `Base64PDFSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "application/pdf"`

              - `"application/pdf"`

            - `type: "base64"`

              - `"base64"`

          - `PlainTextSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "text/plain"`

              - `"text/plain"`

            - `type: "text"`

              - `"text"`

          - `ContentBlockSource object { content, type }`

            - `content: string or array of ContentBlockSourceContent`

              - `string`

              - `ContentBlockSourceContent = array of ContentBlockSourceContent`

                - `TextBlockParam object { text, type, cache_control, citations }`

                - `ImageBlockParam object { source, type, cache_control }`

            - `type: "content"`

              - `"content"`

          - `URLPDFSource object { type, url }`

            - `type: "url"`

              - `"url"`

            - `url: string`

        - `type: "document"`

          - `"document"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

          - `enabled: optional boolean`

        - `context: optional string`

        - `title: optional string`

      - `SearchResultBlockParam object { content, source, title, 3 more }`

        - `content: array of TextBlockParam`

          - `text: string`

          - `type: "text"`

          - `cache_control: optional CacheControlEphemeral`

            Create a cache control breakpoint at this content block.

          - `citations: optional array of TextCitationParam`

        - `source: string`

        - `title: string`

        - `type: "search_result"`

          - `"search_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

      - `ThinkingBlockParam object { signature, thinking, type }`

        - `signature: string`

        - `thinking: string`

        - `type: "thinking"`

          - `"thinking"`

      - `RedactedThinkingBlockParam object { data, type }`

        - `data: string`

        - `type: "redacted_thinking"`

          - `"redacted_thinking"`

      - `ToolUseBlockParam object { id, input, name, 3 more }`

        - `id: string`

        - `input: map[unknown]`

        - `name: string`

        - `type: "tool_use"`

          - `"tool_use"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

            - `type: "direct"`

              - `"direct"`

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

            - `tool_id: string`

            - `type: "code_execution_20250825"`

              - `"code_execution_20250825"`

          - `ServerToolCaller20260120 object { tool_id, type }`

            - `tool_id: string`

            - `type: "code_execution_20260120"`

              - `"code_execution_20260120"`

      - `ToolResultBlockParam object { tool_use_id, type, cache_control, 2 more }`

        - `tool_use_id: string`

        - `type: "tool_result"`

          - `"tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `content: optional string or array of TextBlockParam or ImageBlockParam or SearchResultBlockParam or 2 more`

          - `string`

          - `array of TextBlockParam or ImageBlockParam or SearchResultBlockParam or 2 more`

            - `TextBlockParam object { text, type, cache_control, citations }`

            - `ImageBlockParam object { source, type, cache_control }`

            - `SearchResultBlockParam object { content, source, title, 3 more }`

            - `DocumentBlockParam object { source, type, cache_control, 3 more }`

            - `ToolReferenceBlockParam object { tool_name, type, cache_control }`

              Tool reference block that can be included in tool_result content.

              - `tool_name: string`

              - `type: "tool_reference"`

                - `"tool_reference"`

              - `cache_control: optional CacheControlEphemeral`

                Create a cache control breakpoint at this content block.

        - `is_error: optional boolean`

      - `ServerToolUseBlockParam object { id, input, name, 3 more }`

        - `id: string`

        - `input: map[unknown]`

        - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

          - `"web_search"`

          - `"web_fetch"`

          - `"code_execution"`

          - `"bash_code_execution"`

          - `"text_editor_code_execution"`

          - `"tool_search_tool_regex"`

          - `"tool_search_tool_bm25"`

        - `type: "server_tool_use"`

          - `"server_tool_use"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

      - `WebSearchToolResultBlockParam object { content, tool_use_id, type, 2 more }`

        - `content: WebSearchToolResultBlockParamContent`

          - `WebSearchToolResultBlockItem = array of WebSearchResultBlockParam`

            - `encrypted_content: string`

            - `title: string`

            - `type: "web_search_result"`

              - `"web_search_result"`

            - `url: string`

            - `page_age: optional string`

          - `WebSearchToolRequestError object { error_code, type }`

            - `error_code: WebSearchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"max_uses_exceeded"`

              - `"too_many_requests"`

              - `"query_too_long"`

              - `"request_too_large"`

            - `type: "web_search_tool_result_error"`

              - `"web_search_tool_result_error"`

        - `tool_use_id: string`

        - `type: "web_search_tool_result"`

          - `"web_search_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

      - `WebFetchToolResultBlockParam object { content, tool_use_id, type, 2 more }`

        - `content: WebFetchToolResultErrorBlockParam or WebFetchBlockParam`

          - `WebFetchToolResultErrorBlockParam object { error_code, type }`

            - `error_code: WebFetchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"url_too_long"`

              - `"url_not_allowed"`

              - `"url_not_in_prior_context"`

              - `"url_not_accessible"`

              - `"unsupported_content_type"`

              - `"too_many_requests"`

              - `"max_uses_exceeded"`

              - `"unavailable"`

            - `type: "web_fetch_tool_result_error"`

              - `"web_fetch_tool_result_error"`

          - `WebFetchBlockParam object { content, type, url, retrieved_at }`

            - `content: DocumentBlockParam`

            - `type: "web_fetch_result"`

              - `"web_fetch_result"`

            - `url: string`

              Fetched content URL

            - `retrieved_at: optional string`

              ISO 8601 timestamp when the content was retrieved

        - `tool_use_id: string`

        - `type: "web_fetch_tool_result"`

          - `"web_fetch_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

      - `CodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

        - `content: CodeExecutionToolResultBlockParamContent`

          Code execution result with encrypted stdout for PFC + web_search results.

          - `CodeExecutionToolResultErrorParam object { error_code, type }`

            - `error_code: CodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

            - `type: "code_execution_tool_result_error"`

              - `"code_execution_tool_result_error"`

          - `CodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

            - `content: array of CodeExecutionOutputBlockParam`

              - `file_id: string`

              - `type: "code_execution_output"`

                - `"code_execution_output"`

            - `return_code: number`

            - `stderr: string`

            - `stdout: string`

            - `type: "code_execution_result"`

              - `"code_execution_result"`

          - `EncryptedCodeExecutionResultBlockParam object { content, encrypted_stdout, return_code, 2 more }`

            Code execution result with encrypted stdout for PFC + web_search results.

            - `content: array of CodeExecutionOutputBlockParam`

              - `file_id: string`

              - `type: "code_execution_output"`

            - `encrypted_stdout: string`

            - `return_code: number`

            - `stderr: string`

            - `type: "encrypted_code_execution_result"`

              - `"encrypted_code_execution_result"`

        - `tool_use_id: string`

        - `type: "code_execution_tool_result"`

          - `"code_execution_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `BashCodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

        - `content: BashCodeExecutionToolResultErrorParam or BashCodeExecutionResultBlockParam`

          - `BashCodeExecutionToolResultErrorParam object { error_code, type }`

            - `error_code: BashCodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

              - `"output_file_too_large"`

            - `type: "bash_code_execution_tool_result_error"`

              - `"bash_code_execution_tool_result_error"`

          - `BashCodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

            - `content: array of BashCodeExecutionOutputBlockParam`

              - `file_id: string`

              - `type: "bash_code_execution_output"`

                - `"bash_code_execution_output"`

            - `return_code: number`

            - `stderr: string`

            - `stdout: string`

            - `type: "bash_code_execution_result"`

              - `"bash_code_execution_result"`

        - `tool_use_id: string`

        - `type: "bash_code_execution_tool_result"`

          - `"bash_code_execution_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `TextEditorCodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

        - `content: TextEditorCodeExecutionToolResultErrorParam or TextEditorCodeExecutionViewResultBlockParam or TextEditorCodeExecutionCreateResultBlockParam or TextEditorCodeExecutionStrReplaceResultBlockParam`

          - `TextEditorCodeExecutionToolResultErrorParam object { error_code, type, error_message }`

            - `error_code: TextEditorCodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

              - `"file_not_found"`

            - `type: "text_editor_code_execution_tool_result_error"`

              - `"text_editor_code_execution_tool_result_error"`

            - `error_message: optional string`

          - `TextEditorCodeExecutionViewResultBlockParam object { content, file_type, type, 3 more }`

            - `content: string`

            - `file_type: "text" or "image" or "pdf"`

              - `"text"`

              - `"image"`

              - `"pdf"`

            - `type: "text_editor_code_execution_view_result"`

              - `"text_editor_code_execution_view_result"`

            - `num_lines: optional number`

            - `start_line: optional number`

            - `total_lines: optional number`

          - `TextEditorCodeExecutionCreateResultBlockParam object { is_file_update, type }`

            - `is_file_update: boolean`

            - `type: "text_editor_code_execution_create_result"`

              - `"text_editor_code_execution_create_result"`

          - `TextEditorCodeExecutionStrReplaceResultBlockParam object { type, lines, new_lines, 3 more }`

            - `type: "text_editor_code_execution_str_replace_result"`

              - `"text_editor_code_execution_str_replace_result"`

            - `lines: optional array of string`

            - `new_lines: optional number`

            - `new_start: optional number`

            - `old_lines: optional number`

            - `old_start: optional number`

        - `tool_use_id: string`

        - `type: "text_editor_code_execution_tool_result"`

          - `"text_editor_code_execution_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `ToolSearchToolResultBlockParam object { content, tool_use_id, type, cache_control }`

        - `content: ToolSearchToolResultErrorParam or ToolSearchToolSearchResultBlockParam`

          - `ToolSearchToolResultErrorParam object { error_code, type, error_message }`

            - `error_code: ToolSearchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

            - `type: "tool_search_tool_result_error"`

              - `"tool_search_tool_result_error"`

            - `error_message: optional string`

          - `ToolSearchToolSearchResultBlockParam object { tool_references, type }`

            - `tool_references: array of ToolReferenceBlockParam`

              - `tool_name: string`

              - `type: "tool_reference"`

              - `cache_control: optional CacheControlEphemeral`

                Create a cache control breakpoint at this content block.

            - `type: "tool_search_tool_search_result"`

              - `"tool_search_tool_search_result"`

        - `tool_use_id: string`

        - `type: "tool_search_tool_result"`

          - `"tool_search_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `ContainerUploadBlockParam object { file_id, type, cache_control }`

        A content block that represents a file to be uploaded to the container
        Files uploaded via this block will be available in the container's input directory.

        - `file_id: string`

        - `type: "container_upload"`

          - `"container_upload"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `MidConversationSystemBlockParam object { content, type, cache_control }`

        System instructions that appear mid-conversation.

        Use this block to provide or update system-level instructions at a specific
        point in the conversation, rather than only via the top-level `system` parameter.

        - `content: array of TextBlockParam`

          System instruction text blocks.

          - `text: string`

          - `type: "text"`

          - `cache_control: optional CacheControlEphemeral`

            Create a cache control breakpoint at this content block.

          - `citations: optional array of TextCitationParam`

        - `type: "mid_conv_system"`

          - `"mid_conv_system"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

  - `role: "user" or "assistant" or "system"`

    - `"user"`

    - `"assistant"`

    - `"system"`

- `model: Model`

  The model that will complete your prompt.

  See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

  - `"claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more`

    The model that will complete your prompt.

    See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

    - `"claude-sonnet-5"`

      High-performance model for coding and agents

    - `"claude-fable-5"`

      Next generation of intelligence for the hardest knowledge work and coding problems

    - `"claude-mythos-5"`

      Most capable model for cybersecurity and biology research

    - `"claude-opus-4-8"`

      Frontier intelligence for long-running agents and coding

    - `"claude-opus-4-7"`

      Frontier intelligence for long-running agents and coding

    - `"claude-mythos-preview"`

      New class of intelligence, strongest in coding and cybersecurity

    - `"claude-opus-4-6"`

      Frontier intelligence for long-running agents and coding

    - `"claude-sonnet-4-6"`

      Best combination of speed and intelligence

    - `"claude-haiku-4-5"`

      Fastest model with near-frontier intelligence

    - `"claude-haiku-4-5-20251001"`

      Fastest model with near-frontier intelligence

    - `"claude-opus-4-5"`

      Premium model combining maximum intelligence with practical performance

    - `"claude-opus-4-5-20251101"`

      Premium model combining maximum intelligence with practical performance

    - `"claude-sonnet-4-5"`

      High-performance model for agents and coding

    - `"claude-sonnet-4-5-20250929"`

      High-performance model for agents and coding

    - `"claude-opus-4-1"`

      Exceptional model for specialized complex tasks

    - `"claude-opus-4-1-20250805"`

      Exceptional model for specialized complex tasks

  - `string`

- `cache_control: optional CacheControlEphemeral`

  Top-level cache control automatically applies a cache_control marker to the last cacheable block in the request.

- `output_config: optional OutputConfig`

  Configuration options for the model's output, such as the output format.

  - `effort: optional "low" or "medium" or "high" or 2 more`

    All possible effort levels.

    - `"low"`

    - `"medium"`

    - `"high"`

    - `"xhigh"`

    - `"max"`

  - `format: optional JSONOutputFormat`

    A schema to specify Claude's output format in responses. See [structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)

    - `schema: map[unknown]`

      The JSON schema of the format

    - `type: "json_schema"`

      - `"json_schema"`

- `system: optional string or array of TextBlockParam`

  System prompt.

  A system prompt is a way of providing context and instructions to Claude, such as specifying a particular goal or role. See our [guide to system prompts](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#give-claude-a-role).

  - `string`

  - `array of TextBlockParam`

    - `text: string`

    - `type: "text"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional array of TextCitationParam`

- `thinking: optional ThinkingConfigParam`

  Configuration for enabling Claude's extended thinking.

  When enabled, responses include `thinking` content blocks showing Claude's thinking process before the final answer. Requires a minimum budget of 1,024 tokens and counts towards your `max_tokens` limit.

  See [extended thinking](https://platform.claude.com/docs/en/build-with-claude/extended-thinking) for details.

  - `ThinkingConfigEnabled object { budget_tokens, type, display }`

    - `budget_tokens: number`

      Determines how many tokens Claude can use for its internal reasoning process. Larger budgets can enable more thorough analysis for complex problems, improving response quality.

      Must be ≥1024 and less than `max_tokens`.

      See [extended thinking](https://platform.claude.com/docs/en/build-with-claude/extended-thinking) for details.

    - `type: "enabled"`

      - `"enabled"`

    - `display: optional "summarized" or "omitted"`

      Controls how thinking content appears in the response. When set to `summarized`, thinking is returned normally. When set to `omitted`, thinking content is redacted but a signature is returned for multi-turn continuity. Defaults to `summarized`.

      - `"summarized"`

      - `"omitted"`

  - `ThinkingConfigDisabled object { type }`

    - `type: "disabled"`

      - `"disabled"`

  - `ThinkingConfigAdaptive object { type, display }`

    - `type: "adaptive"`

      - `"adaptive"`

    - `display: optional "summarized" or "omitted"`

      Controls how thinking content appears in the response. When set to `summarized`, thinking is returned normally. When set to `omitted`, thinking content is redacted but a signature is returned for multi-turn continuity. Defaults to `summarized`.

      - `"summarized"`

      - `"omitted"`

- `tool_choice: optional ToolChoice`

  How the model should use the provided tools. The model can use a specific tool, any available tool, decide by itself, or not use tools at all.

  - `ToolChoiceAuto object { type, disable_parallel_tool_use }`

    The model will automatically decide whether to use tools.

    - `type: "auto"`

      - `"auto"`

    - `disable_parallel_tool_use: optional boolean`

      Whether to disable parallel tool use.

      Defaults to `false`. If set to `true`, the model will output at most one tool use.

  - `ToolChoiceAny object { type, disable_parallel_tool_use }`

    The model will use any available tools.

    - `type: "any"`

      - `"any"`

    - `disable_parallel_tool_use: optional boolean`

      Whether to disable parallel tool use.

      Defaults to `false`. If set to `true`, the model will output exactly one tool use.

  - `ToolChoiceTool object { name, type, disable_parallel_tool_use }`

    The model will use the specified tool with `tool_choice.name`.

    - `name: string`

      The name of the tool to use.

    - `type: "tool"`

      - `"tool"`

    - `disable_parallel_tool_use: optional boolean`

      Whether to disable parallel tool use.

      Defaults to `false`. If set to `true`, the model will output exactly one tool use.

  - `ToolChoiceNone object { type }`

    The model will not be allowed to use tools.

    - `type: "none"`

      - `"none"`

- `tools: optional array of MessageCountTokensTool`

  Definitions of tools that the model may use.

  If you include `tools` in your API request, the model may return `tool_use` content blocks that represent the model's use of those tools. You can then run those tools using the tool input generated by the model and then optionally return results back to the model using `tool_result` content blocks.

  There are two types of tools: **client tools** and **server tools**. The behavior described below applies to client tools. For [server tools](https://platform.claude.com/docs/en/agents-and-tools/tool-use/server-tools), see their individual documentation as each has its own behavior (e.g., the [web search tool](https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-search-tool)).

  Each tool definition includes:

  * `name`: Name of the tool.
  * `description`: Optional, but strongly-recommended description of the tool.
  * `input_schema`: [JSON schema](https://json-schema.org/draft/2020-12) for the tool `input` shape that the model will produce in `tool_use` output content blocks.

  For example, if you defined `tools` as:

  ```json
  [
    {
      "name": "get_stock_price",
      "description": "Get the current stock price for a given ticker symbol.",
      "input_schema": {
        "type": "object",
        "properties": {
          "ticker": {
            "type": "string",
            "description": "The stock ticker symbol, e.g. AAPL for Apple Inc."
          }
        },
        "required": ["ticker"]
      }
    }
  ]
  ```

  And then asked the model "What's the S&P 500 at today?", the model might produce `tool_use` content blocks in the response like this:

  ```json
  [
    {
      "type": "tool_use",
      "id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
      "name": "get_stock_price",
      "input": { "ticker": "^GSPC" }
    }
  ]
  ```

  You might then run your `get_stock_price` tool with `{"ticker": "^GSPC"}` as an input, and return the following back to the model in a subsequent `user` message:

  ```json
  [
    {
      "type": "tool_result",
      "tool_use_id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
      "content": "259.75 USD"
    }
  ]
  ```

  Tools can be used for workflows that include running client-side tools and functions, or more generally whenever you want the model to produce a particular JSON structure of output.

  See our [guide](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview) for more details.

  - `Tool object { input_schema, name, allowed_callers, 7 more }`

    - `input_schema: object { type, properties, required }`

      [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.

      This defines the shape of the `input` that your tool accepts and that the model will produce.

      - `type: "object"`

        - `"object"`

      - `properties: optional map[unknown]`

      - `required: optional array of string`

    - `name: string`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `description: optional string`

      Description of what this tool does.

      Tool descriptions should be as detailed as possible. The more information that the model has about what the tool is and how to use it, the better it will perform. You can use natural language descriptions to reinforce important aspects of the tool input JSON schema.

    - `eager_input_streaming: optional boolean`

      Enable eager input streaming for this tool. When true, tool input parameters will be streamed incrementally as they are generated, and types will be inferred on-the-fly rather than buffering the full JSON output. When false, streaming is disabled for this tool even if the fine-grained-tool-streaming beta is active. When null (default), uses the default behavior based on beta headers.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `type: optional "custom"`

      - `"custom"`

  - `ToolBash20250124 object { name, type, allowed_callers, 4 more }`

    - `name: "bash"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"bash"`

    - `type: "bash_20250124"`

      - `"bash_20250124"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20250522 object { name, type, allowed_callers, 3 more }`

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20250522"`

      - `"code_execution_20250522"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20250825 object { name, type, allowed_callers, 3 more }`

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20250825"`

      - `"code_execution_20250825"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20260120 object { name, type, allowed_callers, 3 more }`

    Code execution tool with REPL state persistence (daemon mode + gVisor checkpoint).

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20260120"`

      - `"code_execution_20260120"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20260521 object { name, type, allowed_callers, 3 more }`

    Code execution tool with REPL state persistence.

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20260521"`

      - `"code_execution_20260521"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `MemoryTool20250818 object { name, type, allowed_callers, 4 more }`

    - `name: "memory"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"memory"`

    - `type: "memory_20250818"`

      - `"memory_20250818"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolTextEditor20250124 object { name, type, allowed_callers, 4 more }`

    - `name: "str_replace_editor"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"str_replace_editor"`

    - `type: "text_editor_20250124"`

      - `"text_editor_20250124"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolTextEditor20250429 object { name, type, allowed_callers, 4 more }`

    - `name: "str_replace_based_edit_tool"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"str_replace_based_edit_tool"`

    - `type: "text_editor_20250429"`

      - `"text_editor_20250429"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolTextEditor20250728 object { name, type, allowed_callers, 5 more }`

    - `name: "str_replace_based_edit_tool"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"str_replace_based_edit_tool"`

    - `type: "text_editor_20250728"`

      - `"text_editor_20250728"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `max_characters: optional number`

      Maximum number of characters to display when viewing a file. If not specified, defaults to displaying the full file.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `WebSearchTool20250305 object { name, type, allowed_callers, 7 more }`

    - `name: "web_search"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_search"`

    - `type: "web_search_20250305"`

      - `"web_search_20250305"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

    - `blocked_domains: optional array of string`

      If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `user_location: optional UserLocation`

      Parameters for the user's location. Used to provide more relevant search results.

      - `type: "approximate"`

        - `"approximate"`

      - `city: optional string`

        The city of the user.

      - `country: optional string`

        The two letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the user.

      - `region: optional string`

        The region of the user.

      - `timezone: optional string`

        The [IANA timezone](https://nodatime.org/TimeZones) of the user.

  - `WebFetchTool20250910 object { name, type, allowed_callers, 8 more }`

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20250910"`

      - `"web_fetch_20250910"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `WebSearchTool20260209 object { name, type, allowed_callers, 7 more }`

    - `name: "web_search"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_search"`

    - `type: "web_search_20260209"`

      - `"web_search_20260209"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

    - `blocked_domains: optional array of string`

      If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `user_location: optional UserLocation`

      Parameters for the user's location. Used to provide more relevant search results.

  - `WebFetchTool20260209 object { name, type, allowed_callers, 8 more }`

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20260209"`

      - `"web_fetch_20260209"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `WebFetchTool20260309 object { name, type, allowed_callers, 9 more }`

    Web fetch tool with use_cache parameter for bypassing cached content.

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20260309"`

      - `"web_fetch_20260309"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `use_cache: optional boolean`

      Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.

  - `WebSearchTool20260318 object { name, type, allowed_callers, 8 more }`

    - `name: "web_search"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_search"`

    - `type: "web_search_20260318"`

      - `"web_search_20260318"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

    - `blocked_domains: optional array of string`

      If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `response_inclusion: optional "full" or "excluded"`

      How this tool's result blocks appear in the API response when the result was consumed by a completed code_execution call in the same turn. 'full' returns the complete content (default). 'excluded' drops the nested server_tool_use and result block pair entirely. Results from direct calls, or from code_execution calls that paused before completing, are always returned in full so they can be sent back on the next turn.

      - `"full"`

      - `"excluded"`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `user_location: optional UserLocation`

      Parameters for the user's location. Used to provide more relevant search results.

  - `WebFetchTool20260318 object { name, type, allowed_callers, 10 more }`

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20260318"`

      - `"web_fetch_20260318"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `response_inclusion: optional "full" or "excluded"`

      How this tool's result blocks appear in the API response when the result was consumed by a completed code_execution call in the same turn. 'full' returns the complete content (default). 'excluded' drops the nested server_tool_use and result block pair entirely. Results from direct calls, or from code_execution calls that paused before completing, are always returned in full so they can be sent back on the next turn.

      - `"full"`

      - `"excluded"`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `use_cache: optional boolean`

      Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.

  - `ToolSearchToolBm25_20251119 object { name, type, allowed_callers, 3 more }`

    - `name: "tool_search_tool_bm25"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"tool_search_tool_bm25"`

    - `type: "tool_search_tool_bm25_20251119" or "tool_search_tool_bm25"`

      - `"tool_search_tool_bm25_20251119"`

      - `"tool_search_tool_bm25"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolSearchToolRegex20251119 object { name, type, allowed_callers, 3 more }`

    - `name: "tool_search_tool_regex"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"tool_search_tool_regex"`

    - `type: "tool_search_tool_regex_20251119" or "tool_search_tool_regex"`

      - `"tool_search_tool_regex_20251119"`

      - `"tool_search_tool_regex"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

### Returns

- `MessageTokensCount object { input_tokens }`

  - `input_tokens: number`

    The total number of tokens across the provided list of messages, system prompt, and tools.

### Example

```http
curl https://api.anthropic.com/v1/messages/count_tokens \
    -H 'Content-Type: application/json' \
    -H 'anthropic-version: 2023-06-01' \
    -H "X-Api-Key: $ANTHROPIC_API_KEY" \
    -d "{
          \"messages\": [
            {
              \"content\": \"Hello, world\",
              \"role\": \"user\"
            }
          ],
          \"model\": \"claude-opus-4-6\",
          \"system\": [
            {
              \"text\": \"Today's date is 2024-06-01.\",
              \"type\": \"text\"
            }
          ],
          \"thinking\": {
            \"type\": \"adaptive\"
          },
          \"tools\": [
            {
              \"input_schema\": {
                \"type\": \"object\",
                \"properties\": {
                  \"location\": \"bar\",
                  \"unit\": \"bar\"
                },
                \"required\": [
                  \"location\"
                ]
              },
              \"name\": \"name\"
            }
          ]
        }"
```

#### Response

```json
{
  "input_tokens": 2095
}
```

## Domain Types

### Base64 Image Source

- `Base64ImageSource object { data, media_type, type }`

  - `data: string`

  - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

    - `"image/jpeg"`

    - `"image/png"`

    - `"image/gif"`

    - `"image/webp"`

  - `type: "base64"`

    - `"base64"`

### Base64 PDF Source

- `Base64PDFSource object { data, media_type, type }`

  - `data: string`

  - `media_type: "application/pdf"`

    - `"application/pdf"`

  - `type: "base64"`

    - `"base64"`

### Bash Code Execution Output Block

- `BashCodeExecutionOutputBlock object { file_id, type }`

  - `file_id: string`

  - `type: "bash_code_execution_output"`

    - `"bash_code_execution_output"`

### Bash Code Execution Output Block Param

- `BashCodeExecutionOutputBlockParam object { file_id, type }`

  - `file_id: string`

  - `type: "bash_code_execution_output"`

    - `"bash_code_execution_output"`

### Bash Code Execution Result Block

- `BashCodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

  - `content: array of BashCodeExecutionOutputBlock`

    - `file_id: string`

    - `type: "bash_code_execution_output"`

      - `"bash_code_execution_output"`

  - `return_code: number`

  - `stderr: string`

  - `stdout: string`

  - `type: "bash_code_execution_result"`

    - `"bash_code_execution_result"`

### Bash Code Execution Result Block Param

- `BashCodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

  - `content: array of BashCodeExecutionOutputBlockParam`

    - `file_id: string`

    - `type: "bash_code_execution_output"`

      - `"bash_code_execution_output"`

  - `return_code: number`

  - `stderr: string`

  - `stdout: string`

  - `type: "bash_code_execution_result"`

    - `"bash_code_execution_result"`

### Bash Code Execution Tool Result Block

- `BashCodeExecutionToolResultBlock object { content, tool_use_id, type }`

  - `content: BashCodeExecutionToolResultError or BashCodeExecutionResultBlock`

    - `BashCodeExecutionToolResultError object { error_code, type }`

      - `error_code: BashCodeExecutionToolResultErrorCode`

        - `"invalid_tool_input"`

        - `"unavailable"`

        - `"too_many_requests"`

        - `"execution_time_exceeded"`

        - `"output_file_too_large"`

      - `type: "bash_code_execution_tool_result_error"`

        - `"bash_code_execution_tool_result_error"`

    - `BashCodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

      - `content: array of BashCodeExecutionOutputBlock`

        - `file_id: string`

        - `type: "bash_code_execution_output"`

          - `"bash_code_execution_output"`

      - `return_code: number`

      - `stderr: string`

      - `stdout: string`

      - `type: "bash_code_execution_result"`

        - `"bash_code_execution_result"`

  - `tool_use_id: string`

  - `type: "bash_code_execution_tool_result"`

    - `"bash_code_execution_tool_result"`

### Bash Code Execution Tool Result Block Param

- `BashCodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

  - `content: BashCodeExecutionToolResultErrorParam or BashCodeExecutionResultBlockParam`

    - `BashCodeExecutionToolResultErrorParam object { error_code, type }`

      - `error_code: BashCodeExecutionToolResultErrorCode`

        - `"invalid_tool_input"`

        - `"unavailable"`

        - `"too_many_requests"`

        - `"execution_time_exceeded"`

        - `"output_file_too_large"`

      - `type: "bash_code_execution_tool_result_error"`

        - `"bash_code_execution_tool_result_error"`

    - `BashCodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

      - `content: array of BashCodeExecutionOutputBlockParam`

        - `file_id: string`

        - `type: "bash_code_execution_output"`

          - `"bash_code_execution_output"`

      - `return_code: number`

      - `stderr: string`

      - `stdout: string`

      - `type: "bash_code_execution_result"`

        - `"bash_code_execution_result"`

  - `tool_use_id: string`

  - `type: "bash_code_execution_tool_result"`

    - `"bash_code_execution_tool_result"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

### Bash Code Execution Tool Result Error

- `BashCodeExecutionToolResultError object { error_code, type }`

  - `error_code: BashCodeExecutionToolResultErrorCode`

    - `"invalid_tool_input"`

    - `"unavailable"`

    - `"too_many_requests"`

    - `"execution_time_exceeded"`

    - `"output_file_too_large"`

  - `type: "bash_code_execution_tool_result_error"`

    - `"bash_code_execution_tool_result_error"`

### Bash Code Execution Tool Result Error Code

- `BashCodeExecutionToolResultErrorCode = "invalid_tool_input" or "unavailable" or "too_many_requests" or 2 more`

  - `"invalid_tool_input"`

  - `"unavailable"`

  - `"too_many_requests"`

  - `"execution_time_exceeded"`

  - `"output_file_too_large"`

### Bash Code Execution Tool Result Error Param

- `BashCodeExecutionToolResultErrorParam object { error_code, type }`

  - `error_code: BashCodeExecutionToolResultErrorCode`

    - `"invalid_tool_input"`

    - `"unavailable"`

    - `"too_many_requests"`

    - `"execution_time_exceeded"`

    - `"output_file_too_large"`

  - `type: "bash_code_execution_tool_result_error"`

    - `"bash_code_execution_tool_result_error"`

### Cache Control Ephemeral

- `CacheControlEphemeral object { type, ttl }`

  - `type: "ephemeral"`

    - `"ephemeral"`

  - `ttl: optional "5m" or "1h"`

    The time-to-live for the cache control breakpoint.

    This may be one the following values:

    - `5m`: 5 minutes
    - `1h`: 1 hour

    Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

    - `"5m"`

    - `"1h"`

### Cache Creation

- `CacheCreation object { ephemeral_1h_input_tokens, ephemeral_5m_input_tokens }`

  - `ephemeral_1h_input_tokens: number`

    The number of input tokens used to create the 1 hour cache entry.

  - `ephemeral_5m_input_tokens: number`

    The number of input tokens used to create the 5 minute cache entry.

### Citation Char Location

- `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

  - `cited_text: string`

  - `document_index: number`

  - `document_title: string`

  - `end_char_index: number`

  - `file_id: string`

  - `start_char_index: number`

  - `type: "char_location"`

    - `"char_location"`

### Citation Char Location Param

- `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

  - `cited_text: string`

  - `document_index: number`

  - `document_title: string`

  - `end_char_index: number`

  - `start_char_index: number`

  - `type: "char_location"`

    - `"char_location"`

### Citation Content Block Location

- `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

  - `cited_text: string`

    The full text of the cited block range, concatenated.

    Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

  - `document_index: number`

  - `document_title: string`

  - `end_block_index: number`

    Exclusive 0-based end index of the cited block range in the source's `content` array.

    Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

  - `file_id: string`

  - `start_block_index: number`

    0-based index of the first cited block in the source's `content` array.

  - `type: "content_block_location"`

    - `"content_block_location"`

### Citation Content Block Location Param

- `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

  - `cited_text: string`

    The full text of the cited block range, concatenated.

    Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

  - `document_index: number`

  - `document_title: string`

  - `end_block_index: number`

    Exclusive 0-based end index of the cited block range in the source's `content` array.

    Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

  - `start_block_index: number`

    0-based index of the first cited block in the source's `content` array.

  - `type: "content_block_location"`

    - `"content_block_location"`

### Citation Page Location

- `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

  - `cited_text: string`

  - `document_index: number`

  - `document_title: string`

  - `end_page_number: number`

  - `file_id: string`

  - `start_page_number: number`

  - `type: "page_location"`

    - `"page_location"`

### Citation Page Location Param

- `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

  - `cited_text: string`

  - `document_index: number`

  - `document_title: string`

  - `end_page_number: number`

  - `start_page_number: number`

  - `type: "page_location"`

    - `"page_location"`

### Citation Search Result Location Param

- `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

  - `cited_text: string`

    The full text of the cited block range, concatenated.

    Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

  - `end_block_index: number`

    Exclusive 0-based end index of the cited block range in the source's `content` array.

    Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

  - `search_result_index: number`

    0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

    Counted separately from `document_index`; server-side web search results are not included in this count.

  - `source: string`

  - `start_block_index: number`

    0-based index of the first cited block in the source's `content` array.

  - `title: string`

  - `type: "search_result_location"`

    - `"search_result_location"`

### Citation Web Search Result Location Param

- `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

  - `cited_text: string`

  - `encrypted_index: string`

  - `title: string`

  - `type: "web_search_result_location"`

    - `"web_search_result_location"`

  - `url: string`

### Citations Config

- `CitationsConfig object { enabled }`

  - `enabled: boolean`

### Citations Config Param

- `CitationsConfigParam object { enabled }`

  - `enabled: optional boolean`

### Citations Delta

- `CitationsDelta object { citation, type }`

  - `citation: CitationCharLocation or CitationPageLocation or CitationContentBlockLocation or 2 more`

    - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

      - `cited_text: string`

      - `document_index: number`

      - `document_title: string`

      - `end_char_index: number`

      - `file_id: string`

      - `start_char_index: number`

      - `type: "char_location"`

        - `"char_location"`

    - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

      - `cited_text: string`

      - `document_index: number`

      - `document_title: string`

      - `end_page_number: number`

      - `file_id: string`

      - `start_page_number: number`

      - `type: "page_location"`

        - `"page_location"`

    - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

      - `cited_text: string`

        The full text of the cited block range, concatenated.

        Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

      - `document_index: number`

      - `document_title: string`

      - `end_block_index: number`

        Exclusive 0-based end index of the cited block range in the source's `content` array.

        Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

      - `file_id: string`

      - `start_block_index: number`

        0-based index of the first cited block in the source's `content` array.

      - `type: "content_block_location"`

        - `"content_block_location"`

    - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

      - `cited_text: string`

      - `encrypted_index: string`

      - `title: string`

      - `type: "web_search_result_location"`

        - `"web_search_result_location"`

      - `url: string`

    - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

      - `cited_text: string`

        The full text of the cited block range, concatenated.

        Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

      - `end_block_index: number`

        Exclusive 0-based end index of the cited block range in the source's `content` array.

        Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

      - `search_result_index: number`

        0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

        Counted separately from `document_index`; server-side web search results are not included in this count.

      - `source: string`

      - `start_block_index: number`

        0-based index of the first cited block in the source's `content` array.

      - `title: string`

      - `type: "search_result_location"`

        - `"search_result_location"`

  - `type: "citations_delta"`

    - `"citations_delta"`

### Citations Search Result Location

- `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

  - `cited_text: string`

    The full text of the cited block range, concatenated.

    Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

  - `end_block_index: number`

    Exclusive 0-based end index of the cited block range in the source's `content` array.

    Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

  - `search_result_index: number`

    0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

    Counted separately from `document_index`; server-side web search results are not included in this count.

  - `source: string`

  - `start_block_index: number`

    0-based index of the first cited block in the source's `content` array.

  - `title: string`

  - `type: "search_result_location"`

    - `"search_result_location"`

### Citations Web Search Result Location

- `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

  - `cited_text: string`

  - `encrypted_index: string`

  - `title: string`

  - `type: "web_search_result_location"`

    - `"web_search_result_location"`

  - `url: string`

### Code Execution Output Block

- `CodeExecutionOutputBlock object { file_id, type }`

  - `file_id: string`

  - `type: "code_execution_output"`

    - `"code_execution_output"`

### Code Execution Output Block Param

- `CodeExecutionOutputBlockParam object { file_id, type }`

  - `file_id: string`

  - `type: "code_execution_output"`

    - `"code_execution_output"`

### Code Execution Result Block

- `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

  - `content: array of CodeExecutionOutputBlock`

    - `file_id: string`

    - `type: "code_execution_output"`

      - `"code_execution_output"`

  - `return_code: number`

  - `stderr: string`

  - `stdout: string`

  - `type: "code_execution_result"`

    - `"code_execution_result"`

### Code Execution Result Block Param

- `CodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

  - `content: array of CodeExecutionOutputBlockParam`

    - `file_id: string`

    - `type: "code_execution_output"`

      - `"code_execution_output"`

  - `return_code: number`

  - `stderr: string`

  - `stdout: string`

  - `type: "code_execution_result"`

    - `"code_execution_result"`

### Code Execution Tool 20250522

- `CodeExecutionTool20250522 object { name, type, allowed_callers, 3 more }`

  - `name: "code_execution"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"code_execution"`

  - `type: "code_execution_20250522"`

    - `"code_execution_20250522"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Code Execution Tool 20250825

- `CodeExecutionTool20250825 object { name, type, allowed_callers, 3 more }`

  - `name: "code_execution"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"code_execution"`

  - `type: "code_execution_20250825"`

    - `"code_execution_20250825"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Code Execution Tool 20260120

- `CodeExecutionTool20260120 object { name, type, allowed_callers, 3 more }`

  Code execution tool with REPL state persistence (daemon mode + gVisor checkpoint).

  - `name: "code_execution"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"code_execution"`

  - `type: "code_execution_20260120"`

    - `"code_execution_20260120"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Code Execution Tool 20260521

- `CodeExecutionTool20260521 object { name, type, allowed_callers, 3 more }`

  Code execution tool with REPL state persistence.

  - `name: "code_execution"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"code_execution"`

  - `type: "code_execution_20260521"`

    - `"code_execution_20260521"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Code Execution Tool Result Block

- `CodeExecutionToolResultBlock object { content, tool_use_id, type }`

  - `content: CodeExecutionToolResultBlockContent`

    Code execution result with encrypted stdout for PFC + web_search results.

    - `CodeExecutionToolResultError object { error_code, type }`

      - `error_code: CodeExecutionToolResultErrorCode`

        - `"invalid_tool_input"`

        - `"unavailable"`

        - `"too_many_requests"`

        - `"execution_time_exceeded"`

      - `type: "code_execution_tool_result_error"`

        - `"code_execution_tool_result_error"`

    - `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

      - `content: array of CodeExecutionOutputBlock`

        - `file_id: string`

        - `type: "code_execution_output"`

          - `"code_execution_output"`

      - `return_code: number`

      - `stderr: string`

      - `stdout: string`

      - `type: "code_execution_result"`

        - `"code_execution_result"`

    - `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

      Code execution result with encrypted stdout for PFC + web_search results.

      - `content: array of CodeExecutionOutputBlock`

        - `file_id: string`

        - `type: "code_execution_output"`

      - `encrypted_stdout: string`

      - `return_code: number`

      - `stderr: string`

      - `type: "encrypted_code_execution_result"`

        - `"encrypted_code_execution_result"`

  - `tool_use_id: string`

  - `type: "code_execution_tool_result"`

    - `"code_execution_tool_result"`

### Code Execution Tool Result Block Content

- `CodeExecutionToolResultBlockContent = CodeExecutionToolResultError or CodeExecutionResultBlock or EncryptedCodeExecutionResultBlock`

  Code execution result with encrypted stdout for PFC + web_search results.

  - `CodeExecutionToolResultError object { error_code, type }`

    - `error_code: CodeExecutionToolResultErrorCode`

      - `"invalid_tool_input"`

      - `"unavailable"`

      - `"too_many_requests"`

      - `"execution_time_exceeded"`

    - `type: "code_execution_tool_result_error"`

      - `"code_execution_tool_result_error"`

  - `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

    - `content: array of CodeExecutionOutputBlock`

      - `file_id: string`

      - `type: "code_execution_output"`

        - `"code_execution_output"`

    - `return_code: number`

    - `stderr: string`

    - `stdout: string`

    - `type: "code_execution_result"`

      - `"code_execution_result"`

  - `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

    Code execution result with encrypted stdout for PFC + web_search results.

    - `content: array of CodeExecutionOutputBlock`

      - `file_id: string`

      - `type: "code_execution_output"`

    - `encrypted_stdout: string`

    - `return_code: number`

    - `stderr: string`

    - `type: "encrypted_code_execution_result"`

      - `"encrypted_code_execution_result"`

### Code Execution Tool Result Block Param

- `CodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

  - `content: CodeExecutionToolResultBlockParamContent`

    Code execution result with encrypted stdout for PFC + web_search results.

    - `CodeExecutionToolResultErrorParam object { error_code, type }`

      - `error_code: CodeExecutionToolResultErrorCode`

        - `"invalid_tool_input"`

        - `"unavailable"`

        - `"too_many_requests"`

        - `"execution_time_exceeded"`

      - `type: "code_execution_tool_result_error"`

        - `"code_execution_tool_result_error"`

    - `CodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

      - `content: array of CodeExecutionOutputBlockParam`

        - `file_id: string`

        - `type: "code_execution_output"`

          - `"code_execution_output"`

      - `return_code: number`

      - `stderr: string`

      - `stdout: string`

      - `type: "code_execution_result"`

        - `"code_execution_result"`

    - `EncryptedCodeExecutionResultBlockParam object { content, encrypted_stdout, return_code, 2 more }`

      Code execution result with encrypted stdout for PFC + web_search results.

      - `content: array of CodeExecutionOutputBlockParam`

        - `file_id: string`

        - `type: "code_execution_output"`

      - `encrypted_stdout: string`

      - `return_code: number`

      - `stderr: string`

      - `type: "encrypted_code_execution_result"`

        - `"encrypted_code_execution_result"`

  - `tool_use_id: string`

  - `type: "code_execution_tool_result"`

    - `"code_execution_tool_result"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

### Code Execution Tool Result Block Param Content

- `CodeExecutionToolResultBlockParamContent = CodeExecutionToolResultErrorParam or CodeExecutionResultBlockParam or EncryptedCodeExecutionResultBlockParam`

  Code execution result with encrypted stdout for PFC + web_search results.

  - `CodeExecutionToolResultErrorParam object { error_code, type }`

    - `error_code: CodeExecutionToolResultErrorCode`

      - `"invalid_tool_input"`

      - `"unavailable"`

      - `"too_many_requests"`

      - `"execution_time_exceeded"`

    - `type: "code_execution_tool_result_error"`

      - `"code_execution_tool_result_error"`

  - `CodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

    - `content: array of CodeExecutionOutputBlockParam`

      - `file_id: string`

      - `type: "code_execution_output"`

        - `"code_execution_output"`

    - `return_code: number`

    - `stderr: string`

    - `stdout: string`

    - `type: "code_execution_result"`

      - `"code_execution_result"`

  - `EncryptedCodeExecutionResultBlockParam object { content, encrypted_stdout, return_code, 2 more }`

    Code execution result with encrypted stdout for PFC + web_search results.

    - `content: array of CodeExecutionOutputBlockParam`

      - `file_id: string`

      - `type: "code_execution_output"`

    - `encrypted_stdout: string`

    - `return_code: number`

    - `stderr: string`

    - `type: "encrypted_code_execution_result"`

      - `"encrypted_code_execution_result"`

### Code Execution Tool Result Error

- `CodeExecutionToolResultError object { error_code, type }`

  - `error_code: CodeExecutionToolResultErrorCode`

    - `"invalid_tool_input"`

    - `"unavailable"`

    - `"too_many_requests"`

    - `"execution_time_exceeded"`

  - `type: "code_execution_tool_result_error"`

    - `"code_execution_tool_result_error"`

### Code Execution Tool Result Error Code

- `CodeExecutionToolResultErrorCode = "invalid_tool_input" or "unavailable" or "too_many_requests" or "execution_time_exceeded"`

  - `"invalid_tool_input"`

  - `"unavailable"`

  - `"too_many_requests"`

  - `"execution_time_exceeded"`

### Code Execution Tool Result Error Param

- `CodeExecutionToolResultErrorParam object { error_code, type }`

  - `error_code: CodeExecutionToolResultErrorCode`

    - `"invalid_tool_input"`

    - `"unavailable"`

    - `"too_many_requests"`

    - `"execution_time_exceeded"`

  - `type: "code_execution_tool_result_error"`

    - `"code_execution_tool_result_error"`

### Container

- `Container object { id, expires_at }`

  Information about the container used in the request (for the code execution tool)

  - `id: string`

    Identifier for the container used in this request

  - `expires_at: string`

    The time at which the container will expire.

### Container Upload Block

- `ContainerUploadBlock object { file_id, type }`

  Response model for a file uploaded to the container.

  - `file_id: string`

  - `type: "container_upload"`

    - `"container_upload"`

### Container Upload Block Param

- `ContainerUploadBlockParam object { file_id, type, cache_control }`

  A content block that represents a file to be uploaded to the container
  Files uploaded via this block will be available in the container's input directory.

  - `file_id: string`

  - `type: "container_upload"`

    - `"container_upload"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

### Content Block

- `ContentBlock = TextBlock or ThinkingBlock or RedactedThinkingBlock or 9 more`

  Response model for a file uploaded to the container.

  - `TextBlock object { citations, text, type }`

    - `citations: array of TextCitation`

      Citations supporting the text block.

      The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.

      - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

        - `cited_text: string`

        - `document_index: number`

        - `document_title: string`

        - `end_char_index: number`

        - `file_id: string`

        - `start_char_index: number`

        - `type: "char_location"`

          - `"char_location"`

      - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

        - `cited_text: string`

        - `document_index: number`

        - `document_title: string`

        - `end_page_number: number`

        - `file_id: string`

        - `start_page_number: number`

        - `type: "page_location"`

          - `"page_location"`

      - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

        - `cited_text: string`

          The full text of the cited block range, concatenated.

          Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

        - `document_index: number`

        - `document_title: string`

        - `end_block_index: number`

          Exclusive 0-based end index of the cited block range in the source's `content` array.

          Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

        - `file_id: string`

        - `start_block_index: number`

          0-based index of the first cited block in the source's `content` array.

        - `type: "content_block_location"`

          - `"content_block_location"`

      - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

        - `cited_text: string`

        - `encrypted_index: string`

        - `title: string`

        - `type: "web_search_result_location"`

          - `"web_search_result_location"`

        - `url: string`

      - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

        - `cited_text: string`

          The full text of the cited block range, concatenated.

          Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

        - `end_block_index: number`

          Exclusive 0-based end index of the cited block range in the source's `content` array.

          Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

        - `search_result_index: number`

          0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

          Counted separately from `document_index`; server-side web search results are not included in this count.

        - `source: string`

        - `start_block_index: number`

          0-based index of the first cited block in the source's `content` array.

        - `title: string`

        - `type: "search_result_location"`

          - `"search_result_location"`

    - `text: string`

    - `type: "text"`

      - `"text"`

  - `ThinkingBlock object { signature, thinking, type }`

    - `signature: string`

    - `thinking: string`

    - `type: "thinking"`

      - `"thinking"`

  - `RedactedThinkingBlock object { data, type }`

    - `data: string`

    - `type: "redacted_thinking"`

      - `"redacted_thinking"`

  - `ToolUseBlock object { id, caller, input, 2 more }`

    - `id: string`

    - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

      Tool invocation directly from the model.

      - `DirectCaller object { type }`

        Tool invocation directly from the model.

        - `type: "direct"`

          - `"direct"`

      - `ServerToolCaller object { tool_id, type }`

        Tool invocation generated by a server-side tool.

        - `tool_id: string`

        - `type: "code_execution_20250825"`

          - `"code_execution_20250825"`

      - `ServerToolCaller20260120 object { tool_id, type }`

        - `tool_id: string`

        - `type: "code_execution_20260120"`

          - `"code_execution_20260120"`

    - `input: map[unknown]`

    - `name: string`

    - `type: "tool_use"`

      - `"tool_use"`

  - `ServerToolUseBlock object { id, caller, input, 2 more }`

    - `id: string`

    - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

      Tool invocation directly from the model.

      - `DirectCaller object { type }`

        Tool invocation directly from the model.

      - `ServerToolCaller object { tool_id, type }`

        Tool invocation generated by a server-side tool.

      - `ServerToolCaller20260120 object { tool_id, type }`

    - `input: map[unknown]`

    - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

      - `"web_search"`

      - `"web_fetch"`

      - `"code_execution"`

      - `"bash_code_execution"`

      - `"text_editor_code_execution"`

      - `"tool_search_tool_regex"`

      - `"tool_search_tool_bm25"`

    - `type: "server_tool_use"`

      - `"server_tool_use"`

  - `WebSearchToolResultBlock object { caller, content, tool_use_id, type }`

    - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

      Tool invocation directly from the model.

      - `DirectCaller object { type }`

        Tool invocation directly from the model.

      - `ServerToolCaller object { tool_id, type }`

        Tool invocation generated by a server-side tool.

      - `ServerToolCaller20260120 object { tool_id, type }`

    - `content: WebSearchToolResultBlockContent`

      - `WebSearchToolResultError object { error_code, type }`

        - `error_code: WebSearchToolResultErrorCode`

          - `"invalid_tool_input"`

          - `"unavailable"`

          - `"max_uses_exceeded"`

          - `"too_many_requests"`

          - `"query_too_long"`

          - `"request_too_large"`

        - `type: "web_search_tool_result_error"`

          - `"web_search_tool_result_error"`

      - `array of WebSearchResultBlock`

        - `encrypted_content: string`

        - `page_age: string`

        - `title: string`

        - `type: "web_search_result"`

          - `"web_search_result"`

        - `url: string`

    - `tool_use_id: string`

    - `type: "web_search_tool_result"`

      - `"web_search_tool_result"`

  - `WebFetchToolResultBlock object { caller, content, tool_use_id, type }`

    - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

      Tool invocation directly from the model.

      - `DirectCaller object { type }`

        Tool invocation directly from the model.

      - `ServerToolCaller object { tool_id, type }`

        Tool invocation generated by a server-side tool.

      - `ServerToolCaller20260120 object { tool_id, type }`

    - `content: WebFetchToolResultErrorBlock or WebFetchBlock`

      - `WebFetchToolResultErrorBlock object { error_code, type }`

        - `error_code: WebFetchToolResultErrorCode`

          - `"invalid_tool_input"`

          - `"url_too_long"`

          - `"url_not_allowed"`

          - `"url_not_in_prior_context"`

          - `"url_not_accessible"`

          - `"unsupported_content_type"`

          - `"too_many_requests"`

          - `"max_uses_exceeded"`

          - `"unavailable"`

        - `type: "web_fetch_tool_result_error"`

          - `"web_fetch_tool_result_error"`

      - `WebFetchBlock object { content, retrieved_at, type, url }`

        - `content: DocumentBlock`

          - `citations: CitationsConfig`

            Citation configuration for the document

            - `enabled: boolean`

          - `source: Base64PDFSource or PlainTextSource`

            - `Base64PDFSource object { data, media_type, type }`

              - `data: string`

              - `media_type: "application/pdf"`

                - `"application/pdf"`

              - `type: "base64"`

                - `"base64"`

            - `PlainTextSource object { data, media_type, type }`

              - `data: string`

              - `media_type: "text/plain"`

                - `"text/plain"`

              - `type: "text"`

                - `"text"`

          - `title: string`

            The title of the document

          - `type: "document"`

            - `"document"`

        - `retrieved_at: string`

          ISO 8601 timestamp when the content was retrieved

        - `type: "web_fetch_result"`

          - `"web_fetch_result"`

        - `url: string`

          Fetched content URL

    - `tool_use_id: string`

    - `type: "web_fetch_tool_result"`

      - `"web_fetch_tool_result"`

  - `CodeExecutionToolResultBlock object { content, tool_use_id, type }`

    - `content: CodeExecutionToolResultBlockContent`

      Code execution result with encrypted stdout for PFC + web_search results.

      - `CodeExecutionToolResultError object { error_code, type }`

        - `error_code: CodeExecutionToolResultErrorCode`

          - `"invalid_tool_input"`

          - `"unavailable"`

          - `"too_many_requests"`

          - `"execution_time_exceeded"`

        - `type: "code_execution_tool_result_error"`

          - `"code_execution_tool_result_error"`

      - `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

        - `content: array of CodeExecutionOutputBlock`

          - `file_id: string`

          - `type: "code_execution_output"`

            - `"code_execution_output"`

        - `return_code: number`

        - `stderr: string`

        - `stdout: string`

        - `type: "code_execution_result"`

          - `"code_execution_result"`

      - `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

        Code execution result with encrypted stdout for PFC + web_search results.

        - `content: array of CodeExecutionOutputBlock`

          - `file_id: string`

          - `type: "code_execution_output"`

        - `encrypted_stdout: string`

        - `return_code: number`

        - `stderr: string`

        - `type: "encrypted_code_execution_result"`

          - `"encrypted_code_execution_result"`

    - `tool_use_id: string`

    - `type: "code_execution_tool_result"`

      - `"code_execution_tool_result"`

  - `BashCodeExecutionToolResultBlock object { content, tool_use_id, type }`

    - `content: BashCodeExecutionToolResultError or BashCodeExecutionResultBlock`

      - `BashCodeExecutionToolResultError object { error_code, type }`

        - `error_code: BashCodeExecutionToolResultErrorCode`

          - `"invalid_tool_input"`

          - `"unavailable"`

          - `"too_many_requests"`

          - `"execution_time_exceeded"`

          - `"output_file_too_large"`

        - `type: "bash_code_execution_tool_result_error"`

          - `"bash_code_execution_tool_result_error"`

      - `BashCodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

        - `content: array of BashCodeExecutionOutputBlock`

          - `file_id: string`

          - `type: "bash_code_execution_output"`

            - `"bash_code_execution_output"`

        - `return_code: number`

        - `stderr: string`

        - `stdout: string`

        - `type: "bash_code_execution_result"`

          - `"bash_code_execution_result"`

    - `tool_use_id: string`

    - `type: "bash_code_execution_tool_result"`

      - `"bash_code_execution_tool_result"`

  - `TextEditorCodeExecutionToolResultBlock object { content, tool_use_id, type }`

    - `content: TextEditorCodeExecutionToolResultError or TextEditorCodeExecutionViewResultBlock or TextEditorCodeExecutionCreateResultBlock or TextEditorCodeExecutionStrReplaceResultBlock`

      - `TextEditorCodeExecutionToolResultError object { error_code, error_message, type }`

        - `error_code: TextEditorCodeExecutionToolResultErrorCode`

          - `"invalid_tool_input"`

          - `"unavailable"`

          - `"too_many_requests"`

          - `"execution_time_exceeded"`

          - `"file_not_found"`

        - `error_message: string`

        - `type: "text_editor_code_execution_tool_result_error"`

          - `"text_editor_code_execution_tool_result_error"`

      - `TextEditorCodeExecutionViewResultBlock object { content, file_type, num_lines, 3 more }`

        - `content: string`

        - `file_type: "text" or "image" or "pdf"`

          - `"text"`

          - `"image"`

          - `"pdf"`

        - `num_lines: number`

        - `start_line: number`

        - `total_lines: number`

        - `type: "text_editor_code_execution_view_result"`

          - `"text_editor_code_execution_view_result"`

      - `TextEditorCodeExecutionCreateResultBlock object { is_file_update, type }`

        - `is_file_update: boolean`

        - `type: "text_editor_code_execution_create_result"`

          - `"text_editor_code_execution_create_result"`

      - `TextEditorCodeExecutionStrReplaceResultBlock object { lines, new_lines, new_start, 3 more }`

        - `lines: array of string`

        - `new_lines: number`

        - `new_start: number`

        - `old_lines: number`

        - `old_start: number`

        - `type: "text_editor_code_execution_str_replace_result"`

          - `"text_editor_code_execution_str_replace_result"`

    - `tool_use_id: string`

    - `type: "text_editor_code_execution_tool_result"`

      - `"text_editor_code_execution_tool_result"`

  - `ToolSearchToolResultBlock object { content, tool_use_id, type }`

    - `content: ToolSearchToolResultError or ToolSearchToolSearchResultBlock`

      - `ToolSearchToolResultError object { error_code, error_message, type }`

        - `error_code: ToolSearchToolResultErrorCode`

          - `"invalid_tool_input"`

          - `"unavailable"`

          - `"too_many_requests"`

          - `"execution_time_exceeded"`

        - `error_message: string`

        - `type: "tool_search_tool_result_error"`

          - `"tool_search_tool_result_error"`

      - `ToolSearchToolSearchResultBlock object { tool_references, type }`

        - `tool_references: array of ToolReferenceBlock`

          - `tool_name: string`

          - `type: "tool_reference"`

            - `"tool_reference"`

        - `type: "tool_search_tool_search_result"`

          - `"tool_search_tool_search_result"`

    - `tool_use_id: string`

    - `type: "tool_search_tool_result"`

      - `"tool_search_tool_result"`

  - `ContainerUploadBlock object { file_id, type }`

    Response model for a file uploaded to the container.

    - `file_id: string`

    - `type: "container_upload"`

      - `"container_upload"`

### Content Block Param

- `ContentBlockParam = TextBlockParam or ImageBlockParam or DocumentBlockParam or 14 more`

  Regular text content.

  - `TextBlockParam object { text, type, cache_control, citations }`

    - `text: string`

    - `type: "text"`

      - `"text"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

      - `type: "ephemeral"`

        - `"ephemeral"`

      - `ttl: optional "5m" or "1h"`

        The time-to-live for the cache control breakpoint.

        This may be one the following values:

        - `5m`: 5 minutes
        - `1h`: 1 hour

        Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

        - `"5m"`

        - `"1h"`

    - `citations: optional array of TextCitationParam`

      - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

        - `cited_text: string`

        - `document_index: number`

        - `document_title: string`

        - `end_char_index: number`

        - `start_char_index: number`

        - `type: "char_location"`

          - `"char_location"`

      - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

        - `cited_text: string`

        - `document_index: number`

        - `document_title: string`

        - `end_page_number: number`

        - `start_page_number: number`

        - `type: "page_location"`

          - `"page_location"`

      - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

        - `cited_text: string`

          The full text of the cited block range, concatenated.

          Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

        - `document_index: number`

        - `document_title: string`

        - `end_block_index: number`

          Exclusive 0-based end index of the cited block range in the source's `content` array.

          Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

        - `start_block_index: number`

          0-based index of the first cited block in the source's `content` array.

        - `type: "content_block_location"`

          - `"content_block_location"`

      - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

        - `cited_text: string`

        - `encrypted_index: string`

        - `title: string`

        - `type: "web_search_result_location"`

          - `"web_search_result_location"`

        - `url: string`

      - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

        - `cited_text: string`

          The full text of the cited block range, concatenated.

          Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

        - `end_block_index: number`

          Exclusive 0-based end index of the cited block range in the source's `content` array.

          Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

        - `search_result_index: number`

          0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

          Counted separately from `document_index`; server-side web search results are not included in this count.

        - `source: string`

        - `start_block_index: number`

          0-based index of the first cited block in the source's `content` array.

        - `title: string`

        - `type: "search_result_location"`

          - `"search_result_location"`

  - `ImageBlockParam object { source, type, cache_control }`

    - `source: Base64ImageSource or URLImageSource`

      - `Base64ImageSource object { data, media_type, type }`

        - `data: string`

        - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

          - `"image/jpeg"`

          - `"image/png"`

          - `"image/gif"`

          - `"image/webp"`

        - `type: "base64"`

          - `"base64"`

      - `URLImageSource object { type, url }`

        - `type: "url"`

          - `"url"`

        - `url: string`

    - `type: "image"`

      - `"image"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

  - `DocumentBlockParam object { source, type, cache_control, 3 more }`

    - `source: Base64PDFSource or PlainTextSource or ContentBlockSource or URLPDFSource`

      - `Base64PDFSource object { data, media_type, type }`

        - `data: string`

        - `media_type: "application/pdf"`

          - `"application/pdf"`

        - `type: "base64"`

          - `"base64"`

      - `PlainTextSource object { data, media_type, type }`

        - `data: string`

        - `media_type: "text/plain"`

          - `"text/plain"`

        - `type: "text"`

          - `"text"`

      - `ContentBlockSource object { content, type }`

        - `content: string or array of ContentBlockSourceContent`

          - `string`

          - `ContentBlockSourceContent = array of ContentBlockSourceContent`

            - `TextBlockParam object { text, type, cache_control, citations }`

            - `ImageBlockParam object { source, type, cache_control }`

        - `type: "content"`

          - `"content"`

      - `URLPDFSource object { type, url }`

        - `type: "url"`

          - `"url"`

        - `url: string`

    - `type: "document"`

      - `"document"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      - `enabled: optional boolean`

    - `context: optional string`

    - `title: optional string`

  - `SearchResultBlockParam object { content, source, title, 3 more }`

    - `content: array of TextBlockParam`

      - `text: string`

      - `type: "text"`

      - `cache_control: optional CacheControlEphemeral`

        Create a cache control breakpoint at this content block.

      - `citations: optional array of TextCitationParam`

    - `source: string`

    - `title: string`

    - `type: "search_result"`

      - `"search_result"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

  - `ThinkingBlockParam object { signature, thinking, type }`

    - `signature: string`

    - `thinking: string`

    - `type: "thinking"`

      - `"thinking"`

  - `RedactedThinkingBlockParam object { data, type }`

    - `data: string`

    - `type: "redacted_thinking"`

      - `"redacted_thinking"`

  - `ToolUseBlockParam object { id, input, name, 3 more }`

    - `id: string`

    - `input: map[unknown]`

    - `name: string`

    - `type: "tool_use"`

      - `"tool_use"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

      Tool invocation directly from the model.

      - `DirectCaller object { type }`

        Tool invocation directly from the model.

        - `type: "direct"`

          - `"direct"`

      - `ServerToolCaller object { tool_id, type }`

        Tool invocation generated by a server-side tool.

        - `tool_id: string`

        - `type: "code_execution_20250825"`

          - `"code_execution_20250825"`

      - `ServerToolCaller20260120 object { tool_id, type }`

        - `tool_id: string`

        - `type: "code_execution_20260120"`

          - `"code_execution_20260120"`

  - `ToolResultBlockParam object { tool_use_id, type, cache_control, 2 more }`

    - `tool_use_id: string`

    - `type: "tool_result"`

      - `"tool_result"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `content: optional string or array of TextBlockParam or ImageBlockParam or SearchResultBlockParam or 2 more`

      - `string`

      - `array of TextBlockParam or ImageBlockParam or SearchResultBlockParam or 2 more`

        - `TextBlockParam object { text, type, cache_control, citations }`

        - `ImageBlockParam object { source, type, cache_control }`

        - `SearchResultBlockParam object { content, source, title, 3 more }`

        - `DocumentBlockParam object { source, type, cache_control, 3 more }`

        - `ToolReferenceBlockParam object { tool_name, type, cache_control }`

          Tool reference block that can be included in tool_result content.

          - `tool_name: string`

          - `type: "tool_reference"`

            - `"tool_reference"`

          - `cache_control: optional CacheControlEphemeral`

            Create a cache control breakpoint at this content block.

    - `is_error: optional boolean`

  - `ServerToolUseBlockParam object { id, input, name, 3 more }`

    - `id: string`

    - `input: map[unknown]`

    - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

      - `"web_search"`

      - `"web_fetch"`

      - `"code_execution"`

      - `"bash_code_execution"`

      - `"text_editor_code_execution"`

      - `"tool_search_tool_regex"`

      - `"tool_search_tool_bm25"`

    - `type: "server_tool_use"`

      - `"server_tool_use"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

      Tool invocation directly from the model.

      - `DirectCaller object { type }`

        Tool invocation directly from the model.

      - `ServerToolCaller object { tool_id, type }`

        Tool invocation generated by a server-side tool.

      - `ServerToolCaller20260120 object { tool_id, type }`

  - `WebSearchToolResultBlockParam object { content, tool_use_id, type, 2 more }`

    - `content: WebSearchToolResultBlockParamContent`

      - `WebSearchToolResultBlockItem = array of WebSearchResultBlockParam`

        - `encrypted_content: string`

        - `title: string`

        - `type: "web_search_result"`

          - `"web_search_result"`

        - `url: string`

        - `page_age: optional string`

      - `WebSearchToolRequestError object { error_code, type }`

        - `error_code: WebSearchToolResultErrorCode`

          - `"invalid_tool_input"`

          - `"unavailable"`

          - `"max_uses_exceeded"`

          - `"too_many_requests"`

          - `"query_too_long"`

          - `"request_too_large"`

        - `type: "web_search_tool_result_error"`

          - `"web_search_tool_result_error"`

    - `tool_use_id: string`

    - `type: "web_search_tool_result"`

      - `"web_search_tool_result"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

      Tool invocation directly from the model.

      - `DirectCaller object { type }`

        Tool invocation directly from the model.

      - `ServerToolCaller object { tool_id, type }`

        Tool invocation generated by a server-side tool.

      - `ServerToolCaller20260120 object { tool_id, type }`

  - `WebFetchToolResultBlockParam object { content, tool_use_id, type, 2 more }`

    - `content: WebFetchToolResultErrorBlockParam or WebFetchBlockParam`

      - `WebFetchToolResultErrorBlockParam object { error_code, type }`

        - `error_code: WebFetchToolResultErrorCode`

          - `"invalid_tool_input"`

          - `"url_too_long"`

          - `"url_not_allowed"`

          - `"url_not_in_prior_context"`

          - `"url_not_accessible"`

          - `"unsupported_content_type"`

          - `"too_many_requests"`

          - `"max_uses_exceeded"`

          - `"unavailable"`

        - `type: "web_fetch_tool_result_error"`

          - `"web_fetch_tool_result_error"`

      - `WebFetchBlockParam object { content, type, url, retrieved_at }`

        - `content: DocumentBlockParam`

        - `type: "web_fetch_result"`

          - `"web_fetch_result"`

        - `url: string`

          Fetched content URL

        - `retrieved_at: optional string`

          ISO 8601 timestamp when the content was retrieved

    - `tool_use_id: string`

    - `type: "web_fetch_tool_result"`

      - `"web_fetch_tool_result"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

      Tool invocation directly from the model.

      - `DirectCaller object { type }`

        Tool invocation directly from the model.

      - `ServerToolCaller object { tool_id, type }`

        Tool invocation generated by a server-side tool.

      - `ServerToolCaller20260120 object { tool_id, type }`

  - `CodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

    - `content: CodeExecutionToolResultBlockParamContent`

      Code execution result with encrypted stdout for PFC + web_search results.

      - `CodeExecutionToolResultErrorParam object { error_code, type }`

        - `error_code: CodeExecutionToolResultErrorCode`

          - `"invalid_tool_input"`

          - `"unavailable"`

          - `"too_many_requests"`

          - `"execution_time_exceeded"`

        - `type: "code_execution_tool_result_error"`

          - `"code_execution_tool_result_error"`

      - `CodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

        - `content: array of CodeExecutionOutputBlockParam`

          - `file_id: string`

          - `type: "code_execution_output"`

            - `"code_execution_output"`

        - `return_code: number`

        - `stderr: string`

        - `stdout: string`

        - `type: "code_execution_result"`

          - `"code_execution_result"`

      - `EncryptedCodeExecutionResultBlockParam object { content, encrypted_stdout, return_code, 2 more }`

        Code execution result with encrypted stdout for PFC + web_search results.

        - `content: array of CodeExecutionOutputBlockParam`

          - `file_id: string`

          - `type: "code_execution_output"`

        - `encrypted_stdout: string`

        - `return_code: number`

        - `stderr: string`

        - `type: "encrypted_code_execution_result"`

          - `"encrypted_code_execution_result"`

    - `tool_use_id: string`

    - `type: "code_execution_tool_result"`

      - `"code_execution_tool_result"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

  - `BashCodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

    - `content: BashCodeExecutionToolResultErrorParam or BashCodeExecutionResultBlockParam`

      - `BashCodeExecutionToolResultErrorParam object { error_code, type }`

        - `error_code: BashCodeExecutionToolResultErrorCode`

          - `"invalid_tool_input"`

          - `"unavailable"`

          - `"too_many_requests"`

          - `"execution_time_exceeded"`

          - `"output_file_too_large"`

        - `type: "bash_code_execution_tool_result_error"`

          - `"bash_code_execution_tool_result_error"`

      - `BashCodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

        - `content: array of BashCodeExecutionOutputBlockParam`

          - `file_id: string`

          - `type: "bash_code_execution_output"`

            - `"bash_code_execution_output"`

        - `return_code: number`

        - `stderr: string`

        - `stdout: string`

        - `type: "bash_code_execution_result"`

          - `"bash_code_execution_result"`

    - `tool_use_id: string`

    - `type: "bash_code_execution_tool_result"`

      - `"bash_code_execution_tool_result"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

  - `TextEditorCodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

    - `content: TextEditorCodeExecutionToolResultErrorParam or TextEditorCodeExecutionViewResultBlockParam or TextEditorCodeExecutionCreateResultBlockParam or TextEditorCodeExecutionStrReplaceResultBlockParam`

      - `TextEditorCodeExecutionToolResultErrorParam object { error_code, type, error_message }`

        - `error_code: TextEditorCodeExecutionToolResultErrorCode`

          - `"invalid_tool_input"`

          - `"unavailable"`

          - `"too_many_requests"`

          - `"execution_time_exceeded"`

          - `"file_not_found"`

        - `type: "text_editor_code_execution_tool_result_error"`

          - `"text_editor_code_execution_tool_result_error"`

        - `error_message: optional string`

      - `TextEditorCodeExecutionViewResultBlockParam object { content, file_type, type, 3 more }`

        - `content: string`

        - `file_type: "text" or "image" or "pdf"`

          - `"text"`

          - `"image"`

          - `"pdf"`

        - `type: "text_editor_code_execution_view_result"`

          - `"text_editor_code_execution_view_result"`

        - `num_lines: optional number`

        - `start_line: optional number`

        - `total_lines: optional number`

      - `TextEditorCodeExecutionCreateResultBlockParam object { is_file_update, type }`

        - `is_file_update: boolean`

        - `type: "text_editor_code_execution_create_result"`

          - `"text_editor_code_execution_create_result"`

      - `TextEditorCodeExecutionStrReplaceResultBlockParam object { type, lines, new_lines, 3 more }`

        - `type: "text_editor_code_execution_str_replace_result"`

          - `"text_editor_code_execution_str_replace_result"`

        - `lines: optional array of string`

        - `new_lines: optional number`

        - `new_start: optional number`

        - `old_lines: optional number`

        - `old_start: optional number`

    - `tool_use_id: string`

    - `type: "text_editor_code_execution_tool_result"`

      - `"text_editor_code_execution_tool_result"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

  - `ToolSearchToolResultBlockParam object { content, tool_use_id, type, cache_control }`

    - `content: ToolSearchToolResultErrorParam or ToolSearchToolSearchResultBlockParam`

      - `ToolSearchToolResultErrorParam object { error_code, type, error_message }`

        - `error_code: ToolSearchToolResultErrorCode`

          - `"invalid_tool_input"`

          - `"unavailable"`

          - `"too_many_requests"`

          - `"execution_time_exceeded"`

        - `type: "tool_search_tool_result_error"`

          - `"tool_search_tool_result_error"`

        - `error_message: optional string`

      - `ToolSearchToolSearchResultBlockParam object { tool_references, type }`

        - `tool_references: array of ToolReferenceBlockParam`

          - `tool_name: string`

          - `type: "tool_reference"`

          - `cache_control: optional CacheControlEphemeral`

            Create a cache control breakpoint at this content block.

        - `type: "tool_search_tool_search_result"`

          - `"tool_search_tool_search_result"`

    - `tool_use_id: string`

    - `type: "tool_search_tool_result"`

      - `"tool_search_tool_result"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

  - `ContainerUploadBlockParam object { file_id, type, cache_control }`

    A content block that represents a file to be uploaded to the container
    Files uploaded via this block will be available in the container's input directory.

    - `file_id: string`

    - `type: "container_upload"`

      - `"container_upload"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

  - `MidConversationSystemBlockParam object { content, type, cache_control }`

    System instructions that appear mid-conversation.

    Use this block to provide or update system-level instructions at a specific
    point in the conversation, rather than only via the top-level `system` parameter.

    - `content: array of TextBlockParam`

      System instruction text blocks.

      - `text: string`

      - `type: "text"`

      - `cache_control: optional CacheControlEphemeral`

        Create a cache control breakpoint at this content block.

      - `citations: optional array of TextCitationParam`

    - `type: "mid_conv_system"`

      - `"mid_conv_system"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

### Content Block Source

- `ContentBlockSource object { content, type }`

  - `content: string or array of ContentBlockSourceContent`

    - `string`

    - `ContentBlockSourceContent = array of ContentBlockSourceContent`

      - `TextBlockParam object { text, type, cache_control, citations }`

        - `text: string`

        - `type: "text"`

          - `"text"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

          - `type: "ephemeral"`

            - `"ephemeral"`

          - `ttl: optional "5m" or "1h"`

            The time-to-live for the cache control breakpoint.

            This may be one the following values:

            - `5m`: 5 minutes
            - `1h`: 1 hour

            Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

            - `"5m"`

            - `"1h"`

        - `citations: optional array of TextCitationParam`

          - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_char_index: number`

            - `start_char_index: number`

            - `type: "char_location"`

              - `"char_location"`

          - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_page_number: number`

            - `start_page_number: number`

            - `type: "page_location"`

              - `"page_location"`

          - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `document_index: number`

            - `document_title: string`

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `type: "content_block_location"`

              - `"content_block_location"`

          - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

            - `cited_text: string`

            - `encrypted_index: string`

            - `title: string`

            - `type: "web_search_result_location"`

              - `"web_search_result_location"`

            - `url: string`

          - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `search_result_index: number`

              0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

              Counted separately from `document_index`; server-side web search results are not included in this count.

            - `source: string`

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `title: string`

            - `type: "search_result_location"`

              - `"search_result_location"`

      - `ImageBlockParam object { source, type, cache_control }`

        - `source: Base64ImageSource or URLImageSource`

          - `Base64ImageSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

              - `"image/jpeg"`

              - `"image/png"`

              - `"image/gif"`

              - `"image/webp"`

            - `type: "base64"`

              - `"base64"`

          - `URLImageSource object { type, url }`

            - `type: "url"`

              - `"url"`

            - `url: string`

        - `type: "image"`

          - `"image"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

  - `type: "content"`

    - `"content"`

### Content Block Source Content

- `ContentBlockSourceContent = TextBlockParam or ImageBlockParam`

  - `TextBlockParam object { text, type, cache_control, citations }`

    - `text: string`

    - `type: "text"`

      - `"text"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

      - `type: "ephemeral"`

        - `"ephemeral"`

      - `ttl: optional "5m" or "1h"`

        The time-to-live for the cache control breakpoint.

        This may be one the following values:

        - `5m`: 5 minutes
        - `1h`: 1 hour

        Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

        - `"5m"`

        - `"1h"`

    - `citations: optional array of TextCitationParam`

      - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

        - `cited_text: string`

        - `document_index: number`

        - `document_title: string`

        - `end_char_index: number`

        - `start_char_index: number`

        - `type: "char_location"`

          - `"char_location"`

      - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

        - `cited_text: string`

        - `document_index: number`

        - `document_title: string`

        - `end_page_number: number`

        - `start_page_number: number`

        - `type: "page_location"`

          - `"page_location"`

      - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

        - `cited_text: string`

          The full text of the cited block range, concatenated.

          Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

        - `document_index: number`

        - `document_title: string`

        - `end_block_index: number`

          Exclusive 0-based end index of the cited block range in the source's `content` array.

          Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

        - `start_block_index: number`

          0-based index of the first cited block in the source's `content` array.

        - `type: "content_block_location"`

          - `"content_block_location"`

      - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

        - `cited_text: string`

        - `encrypted_index: string`

        - `title: string`

        - `type: "web_search_result_location"`

          - `"web_search_result_location"`

        - `url: string`

      - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

        - `cited_text: string`

          The full text of the cited block range, concatenated.

          Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

        - `end_block_index: number`

          Exclusive 0-based end index of the cited block range in the source's `content` array.

          Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

        - `search_result_index: number`

          0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

          Counted separately from `document_index`; server-side web search results are not included in this count.

        - `source: string`

        - `start_block_index: number`

          0-based index of the first cited block in the source's `content` array.

        - `title: string`

        - `type: "search_result_location"`

          - `"search_result_location"`

  - `ImageBlockParam object { source, type, cache_control }`

    - `source: Base64ImageSource or URLImageSource`

      - `Base64ImageSource object { data, media_type, type }`

        - `data: string`

        - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

          - `"image/jpeg"`

          - `"image/png"`

          - `"image/gif"`

          - `"image/webp"`

        - `type: "base64"`

          - `"base64"`

      - `URLImageSource object { type, url }`

        - `type: "url"`

          - `"url"`

        - `url: string`

    - `type: "image"`

      - `"image"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

### Direct Caller

- `DirectCaller object { type }`

  Tool invocation directly from the model.

  - `type: "direct"`

    - `"direct"`

### Document Block

- `DocumentBlock object { citations, source, title, type }`

  - `citations: CitationsConfig`

    Citation configuration for the document

    - `enabled: boolean`

  - `source: Base64PDFSource or PlainTextSource`

    - `Base64PDFSource object { data, media_type, type }`

      - `data: string`

      - `media_type: "application/pdf"`

        - `"application/pdf"`

      - `type: "base64"`

        - `"base64"`

    - `PlainTextSource object { data, media_type, type }`

      - `data: string`

      - `media_type: "text/plain"`

        - `"text/plain"`

      - `type: "text"`

        - `"text"`

  - `title: string`

    The title of the document

  - `type: "document"`

    - `"document"`

### Document Block Param

- `DocumentBlockParam object { source, type, cache_control, 3 more }`

  - `source: Base64PDFSource or PlainTextSource or ContentBlockSource or URLPDFSource`

    - `Base64PDFSource object { data, media_type, type }`

      - `data: string`

      - `media_type: "application/pdf"`

        - `"application/pdf"`

      - `type: "base64"`

        - `"base64"`

    - `PlainTextSource object { data, media_type, type }`

      - `data: string`

      - `media_type: "text/plain"`

        - `"text/plain"`

      - `type: "text"`

        - `"text"`

    - `ContentBlockSource object { content, type }`

      - `content: string or array of ContentBlockSourceContent`

        - `string`

        - `ContentBlockSourceContent = array of ContentBlockSourceContent`

          - `TextBlockParam object { text, type, cache_control, citations }`

            - `text: string`

            - `type: "text"`

              - `"text"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

              - `type: "ephemeral"`

                - `"ephemeral"`

              - `ttl: optional "5m" or "1h"`

                The time-to-live for the cache control breakpoint.

                This may be one the following values:

                - `5m`: 5 minutes
                - `1h`: 1 hour

                Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

                - `"5m"`

                - `"1h"`

            - `citations: optional array of TextCitationParam`

              - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

                - `cited_text: string`

                - `document_index: number`

                - `document_title: string`

                - `end_char_index: number`

                - `start_char_index: number`

                - `type: "char_location"`

                  - `"char_location"`

              - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

                - `cited_text: string`

                - `document_index: number`

                - `document_title: string`

                - `end_page_number: number`

                - `start_page_number: number`

                - `type: "page_location"`

                  - `"page_location"`

              - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

                - `cited_text: string`

                  The full text of the cited block range, concatenated.

                  Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

                - `document_index: number`

                - `document_title: string`

                - `end_block_index: number`

                  Exclusive 0-based end index of the cited block range in the source's `content` array.

                  Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

                - `start_block_index: number`

                  0-based index of the first cited block in the source's `content` array.

                - `type: "content_block_location"`

                  - `"content_block_location"`

              - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

                - `cited_text: string`

                - `encrypted_index: string`

                - `title: string`

                - `type: "web_search_result_location"`

                  - `"web_search_result_location"`

                - `url: string`

              - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

                - `cited_text: string`

                  The full text of the cited block range, concatenated.

                  Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

                - `end_block_index: number`

                  Exclusive 0-based end index of the cited block range in the source's `content` array.

                  Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

                - `search_result_index: number`

                  0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

                  Counted separately from `document_index`; server-side web search results are not included in this count.

                - `source: string`

                - `start_block_index: number`

                  0-based index of the first cited block in the source's `content` array.

                - `title: string`

                - `type: "search_result_location"`

                  - `"search_result_location"`

          - `ImageBlockParam object { source, type, cache_control }`

            - `source: Base64ImageSource or URLImageSource`

              - `Base64ImageSource object { data, media_type, type }`

                - `data: string`

                - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

                  - `"image/jpeg"`

                  - `"image/png"`

                  - `"image/gif"`

                  - `"image/webp"`

                - `type: "base64"`

                  - `"base64"`

              - `URLImageSource object { type, url }`

                - `type: "url"`

                  - `"url"`

                - `url: string`

            - `type: "image"`

              - `"image"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

      - `type: "content"`

        - `"content"`

    - `URLPDFSource object { type, url }`

      - `type: "url"`

        - `"url"`

      - `url: string`

  - `type: "document"`

    - `"document"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

  - `citations: optional CitationsConfigParam`

    - `enabled: optional boolean`

  - `context: optional string`

  - `title: optional string`

### Encrypted Code Execution Result Block

- `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

  Code execution result with encrypted stdout for PFC + web_search results.

  - `content: array of CodeExecutionOutputBlock`

    - `file_id: string`

    - `type: "code_execution_output"`

      - `"code_execution_output"`

  - `encrypted_stdout: string`

  - `return_code: number`

  - `stderr: string`

  - `type: "encrypted_code_execution_result"`

    - `"encrypted_code_execution_result"`

### Encrypted Code Execution Result Block Param

- `EncryptedCodeExecutionResultBlockParam object { content, encrypted_stdout, return_code, 2 more }`

  Code execution result with encrypted stdout for PFC + web_search results.

  - `content: array of CodeExecutionOutputBlockParam`

    - `file_id: string`

    - `type: "code_execution_output"`

      - `"code_execution_output"`

  - `encrypted_stdout: string`

  - `return_code: number`

  - `stderr: string`

  - `type: "encrypted_code_execution_result"`

    - `"encrypted_code_execution_result"`

### Image Block Param

- `ImageBlockParam object { source, type, cache_control }`

  - `source: Base64ImageSource or URLImageSource`

    - `Base64ImageSource object { data, media_type, type }`

      - `data: string`

      - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

        - `"image/jpeg"`

        - `"image/png"`

        - `"image/gif"`

        - `"image/webp"`

      - `type: "base64"`

        - `"base64"`

    - `URLImageSource object { type, url }`

      - `type: "url"`

        - `"url"`

      - `url: string`

  - `type: "image"`

    - `"image"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

### Input JSON Delta

- `InputJSONDelta object { partial_json, type }`

  - `partial_json: string`

  - `type: "input_json_delta"`

    - `"input_json_delta"`

### JSON Output Format

- `JSONOutputFormat object { schema, type }`

  - `schema: map[unknown]`

    The JSON schema of the format

  - `type: "json_schema"`

    - `"json_schema"`

### Memory Tool 20250818

- `MemoryTool20250818 object { name, type, allowed_callers, 4 more }`

  - `name: "memory"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"memory"`

  - `type: "memory_20250818"`

    - `"memory_20250818"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `input_examples: optional array of map[unknown]`

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Message

- `Message object { id, container, content, 7 more }`

  - `id: string`

    Unique object identifier.

    The format and length of IDs may change over time.

  - `container: Container`

    Information about the container used in the request (for the code execution tool)

    - `id: string`

      Identifier for the container used in this request

    - `expires_at: string`

      The time at which the container will expire.

  - `content: array of ContentBlock`

    Content generated by the model.

    This is an array of content blocks, each of which has a `type` that determines its shape.

    Example:

    ```json
    [{"type": "text", "text": "Hi, I'm Claude."}]
    ```

    If the request input `messages` ended with an `assistant` turn, then the response `content` will continue directly from that last turn. You can use this to constrain the model's output.

    For example, if the input `messages` were:

    ```json
    [
      {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
      {"role": "assistant", "content": "The best answer is ("}
    ]
    ```

    Then the response `content` might be:

    ```json
    [{"type": "text", "text": "B)"}]
    ```

    - `TextBlock object { citations, text, type }`

      - `citations: array of TextCitation`

        Citations supporting the text block.

        The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.

        - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

          - `cited_text: string`

          - `document_index: number`

          - `document_title: string`

          - `end_char_index: number`

          - `file_id: string`

          - `start_char_index: number`

          - `type: "char_location"`

            - `"char_location"`

        - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

          - `cited_text: string`

          - `document_index: number`

          - `document_title: string`

          - `end_page_number: number`

          - `file_id: string`

          - `start_page_number: number`

          - `type: "page_location"`

            - `"page_location"`

        - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

          - `cited_text: string`

            The full text of the cited block range, concatenated.

            Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

          - `document_index: number`

          - `document_title: string`

          - `end_block_index: number`

            Exclusive 0-based end index of the cited block range in the source's `content` array.

            Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

          - `file_id: string`

          - `start_block_index: number`

            0-based index of the first cited block in the source's `content` array.

          - `type: "content_block_location"`

            - `"content_block_location"`

        - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

          - `cited_text: string`

          - `encrypted_index: string`

          - `title: string`

          - `type: "web_search_result_location"`

            - `"web_search_result_location"`

          - `url: string`

        - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

          - `cited_text: string`

            The full text of the cited block range, concatenated.

            Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

          - `end_block_index: number`

            Exclusive 0-based end index of the cited block range in the source's `content` array.

            Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

          - `search_result_index: number`

            0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

            Counted separately from `document_index`; server-side web search results are not included in this count.

          - `source: string`

          - `start_block_index: number`

            0-based index of the first cited block in the source's `content` array.

          - `title: string`

          - `type: "search_result_location"`

            - `"search_result_location"`

      - `text: string`

      - `type: "text"`

        - `"text"`

    - `ThinkingBlock object { signature, thinking, type }`

      - `signature: string`

      - `thinking: string`

      - `type: "thinking"`

        - `"thinking"`

    - `RedactedThinkingBlock object { data, type }`

      - `data: string`

      - `type: "redacted_thinking"`

        - `"redacted_thinking"`

    - `ToolUseBlock object { id, caller, input, 2 more }`

      - `id: string`

      - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

        Tool invocation directly from the model.

        - `DirectCaller object { type }`

          Tool invocation directly from the model.

          - `type: "direct"`

            - `"direct"`

        - `ServerToolCaller object { tool_id, type }`

          Tool invocation generated by a server-side tool.

          - `tool_id: string`

          - `type: "code_execution_20250825"`

            - `"code_execution_20250825"`

        - `ServerToolCaller20260120 object { tool_id, type }`

          - `tool_id: string`

          - `type: "code_execution_20260120"`

            - `"code_execution_20260120"`

      - `input: map[unknown]`

      - `name: string`

      - `type: "tool_use"`

        - `"tool_use"`

    - `ServerToolUseBlock object { id, caller, input, 2 more }`

      - `id: string`

      - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

        Tool invocation directly from the model.

        - `DirectCaller object { type }`

          Tool invocation directly from the model.

        - `ServerToolCaller object { tool_id, type }`

          Tool invocation generated by a server-side tool.

        - `ServerToolCaller20260120 object { tool_id, type }`

      - `input: map[unknown]`

      - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

        - `"web_search"`

        - `"web_fetch"`

        - `"code_execution"`

        - `"bash_code_execution"`

        - `"text_editor_code_execution"`

        - `"tool_search_tool_regex"`

        - `"tool_search_tool_bm25"`

      - `type: "server_tool_use"`

        - `"server_tool_use"`

    - `WebSearchToolResultBlock object { caller, content, tool_use_id, type }`

      - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

        Tool invocation directly from the model.

        - `DirectCaller object { type }`

          Tool invocation directly from the model.

        - `ServerToolCaller object { tool_id, type }`

          Tool invocation generated by a server-side tool.

        - `ServerToolCaller20260120 object { tool_id, type }`

      - `content: WebSearchToolResultBlockContent`

        - `WebSearchToolResultError object { error_code, type }`

          - `error_code: WebSearchToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"max_uses_exceeded"`

            - `"too_many_requests"`

            - `"query_too_long"`

            - `"request_too_large"`

          - `type: "web_search_tool_result_error"`

            - `"web_search_tool_result_error"`

        - `array of WebSearchResultBlock`

          - `encrypted_content: string`

          - `page_age: string`

          - `title: string`

          - `type: "web_search_result"`

            - `"web_search_result"`

          - `url: string`

      - `tool_use_id: string`

      - `type: "web_search_tool_result"`

        - `"web_search_tool_result"`

    - `WebFetchToolResultBlock object { caller, content, tool_use_id, type }`

      - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

        Tool invocation directly from the model.

        - `DirectCaller object { type }`

          Tool invocation directly from the model.

        - `ServerToolCaller object { tool_id, type }`

          Tool invocation generated by a server-side tool.

        - `ServerToolCaller20260120 object { tool_id, type }`

      - `content: WebFetchToolResultErrorBlock or WebFetchBlock`

        - `WebFetchToolResultErrorBlock object { error_code, type }`

          - `error_code: WebFetchToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"url_too_long"`

            - `"url_not_allowed"`

            - `"url_not_in_prior_context"`

            - `"url_not_accessible"`

            - `"unsupported_content_type"`

            - `"too_many_requests"`

            - `"max_uses_exceeded"`

            - `"unavailable"`

          - `type: "web_fetch_tool_result_error"`

            - `"web_fetch_tool_result_error"`

        - `WebFetchBlock object { content, retrieved_at, type, url }`

          - `content: DocumentBlock`

            - `citations: CitationsConfig`

              Citation configuration for the document

              - `enabled: boolean`

            - `source: Base64PDFSource or PlainTextSource`

              - `Base64PDFSource object { data, media_type, type }`

                - `data: string`

                - `media_type: "application/pdf"`

                  - `"application/pdf"`

                - `type: "base64"`

                  - `"base64"`

              - `PlainTextSource object { data, media_type, type }`

                - `data: string`

                - `media_type: "text/plain"`

                  - `"text/plain"`

                - `type: "text"`

                  - `"text"`

            - `title: string`

              The title of the document

            - `type: "document"`

              - `"document"`

          - `retrieved_at: string`

            ISO 8601 timestamp when the content was retrieved

          - `type: "web_fetch_result"`

            - `"web_fetch_result"`

          - `url: string`

            Fetched content URL

      - `tool_use_id: string`

      - `type: "web_fetch_tool_result"`

        - `"web_fetch_tool_result"`

    - `CodeExecutionToolResultBlock object { content, tool_use_id, type }`

      - `content: CodeExecutionToolResultBlockContent`

        Code execution result with encrypted stdout for PFC + web_search results.

        - `CodeExecutionToolResultError object { error_code, type }`

          - `error_code: CodeExecutionToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"too_many_requests"`

            - `"execution_time_exceeded"`

          - `type: "code_execution_tool_result_error"`

            - `"code_execution_tool_result_error"`

        - `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

          - `content: array of CodeExecutionOutputBlock`

            - `file_id: string`

            - `type: "code_execution_output"`

              - `"code_execution_output"`

          - `return_code: number`

          - `stderr: string`

          - `stdout: string`

          - `type: "code_execution_result"`

            - `"code_execution_result"`

        - `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

          Code execution result with encrypted stdout for PFC + web_search results.

          - `content: array of CodeExecutionOutputBlock`

            - `file_id: string`

            - `type: "code_execution_output"`

          - `encrypted_stdout: string`

          - `return_code: number`

          - `stderr: string`

          - `type: "encrypted_code_execution_result"`

            - `"encrypted_code_execution_result"`

      - `tool_use_id: string`

      - `type: "code_execution_tool_result"`

        - `"code_execution_tool_result"`

    - `BashCodeExecutionToolResultBlock object { content, tool_use_id, type }`

      - `content: BashCodeExecutionToolResultError or BashCodeExecutionResultBlock`

        - `BashCodeExecutionToolResultError object { error_code, type }`

          - `error_code: BashCodeExecutionToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"too_many_requests"`

            - `"execution_time_exceeded"`

            - `"output_file_too_large"`

          - `type: "bash_code_execution_tool_result_error"`

            - `"bash_code_execution_tool_result_error"`

        - `BashCodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

          - `content: array of BashCodeExecutionOutputBlock`

            - `file_id: string`

            - `type: "bash_code_execution_output"`

              - `"bash_code_execution_output"`

          - `return_code: number`

          - `stderr: string`

          - `stdout: string`

          - `type: "bash_code_execution_result"`

            - `"bash_code_execution_result"`

      - `tool_use_id: string`

      - `type: "bash_code_execution_tool_result"`

        - `"bash_code_execution_tool_result"`

    - `TextEditorCodeExecutionToolResultBlock object { content, tool_use_id, type }`

      - `content: TextEditorCodeExecutionToolResultError or TextEditorCodeExecutionViewResultBlock or TextEditorCodeExecutionCreateResultBlock or TextEditorCodeExecutionStrReplaceResultBlock`

        - `TextEditorCodeExecutionToolResultError object { error_code, error_message, type }`

          - `error_code: TextEditorCodeExecutionToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"too_many_requests"`

            - `"execution_time_exceeded"`

            - `"file_not_found"`

          - `error_message: string`

          - `type: "text_editor_code_execution_tool_result_error"`

            - `"text_editor_code_execution_tool_result_error"`

        - `TextEditorCodeExecutionViewResultBlock object { content, file_type, num_lines, 3 more }`

          - `content: string`

          - `file_type: "text" or "image" or "pdf"`

            - `"text"`

            - `"image"`

            - `"pdf"`

          - `num_lines: number`

          - `start_line: number`

          - `total_lines: number`

          - `type: "text_editor_code_execution_view_result"`

            - `"text_editor_code_execution_view_result"`

        - `TextEditorCodeExecutionCreateResultBlock object { is_file_update, type }`

          - `is_file_update: boolean`

          - `type: "text_editor_code_execution_create_result"`

            - `"text_editor_code_execution_create_result"`

        - `TextEditorCodeExecutionStrReplaceResultBlock object { lines, new_lines, new_start, 3 more }`

          - `lines: array of string`

          - `new_lines: number`

          - `new_start: number`

          - `old_lines: number`

          - `old_start: number`

          - `type: "text_editor_code_execution_str_replace_result"`

            - `"text_editor_code_execution_str_replace_result"`

      - `tool_use_id: string`

      - `type: "text_editor_code_execution_tool_result"`

        - `"text_editor_code_execution_tool_result"`

    - `ToolSearchToolResultBlock object { content, tool_use_id, type }`

      - `content: ToolSearchToolResultError or ToolSearchToolSearchResultBlock`

        - `ToolSearchToolResultError object { error_code, error_message, type }`

          - `error_code: ToolSearchToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"too_many_requests"`

            - `"execution_time_exceeded"`

          - `error_message: string`

          - `type: "tool_search_tool_result_error"`

            - `"tool_search_tool_result_error"`

        - `ToolSearchToolSearchResultBlock object { tool_references, type }`

          - `tool_references: array of ToolReferenceBlock`

            - `tool_name: string`

            - `type: "tool_reference"`

              - `"tool_reference"`

          - `type: "tool_search_tool_search_result"`

            - `"tool_search_tool_search_result"`

      - `tool_use_id: string`

      - `type: "tool_search_tool_result"`

        - `"tool_search_tool_result"`

    - `ContainerUploadBlock object { file_id, type }`

      Response model for a file uploaded to the container.

      - `file_id: string`

      - `type: "container_upload"`

        - `"container_upload"`

  - `model: Model`

    The model that will complete your prompt.

    See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

    - `"claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more`

      The model that will complete your prompt.

      See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

      - `"claude-sonnet-5"`

        High-performance model for coding and agents

      - `"claude-fable-5"`

        Next generation of intelligence for the hardest knowledge work and coding problems

      - `"claude-mythos-5"`

        Most capable model for cybersecurity and biology research

      - `"claude-opus-4-8"`

        Frontier intelligence for long-running agents and coding

      - `"claude-opus-4-7"`

        Frontier intelligence for long-running agents and coding

      - `"claude-mythos-preview"`

        New class of intelligence, strongest in coding and cybersecurity

      - `"claude-opus-4-6"`

        Frontier intelligence for long-running agents and coding

      - `"claude-sonnet-4-6"`

        Best combination of speed and intelligence

      - `"claude-haiku-4-5"`

        Fastest model with near-frontier intelligence

      - `"claude-haiku-4-5-20251001"`

        Fastest model with near-frontier intelligence

      - `"claude-opus-4-5"`

        Premium model combining maximum intelligence with practical performance

      - `"claude-opus-4-5-20251101"`

        Premium model combining maximum intelligence with practical performance

      - `"claude-sonnet-4-5"`

        High-performance model for agents and coding

      - `"claude-sonnet-4-5-20250929"`

        High-performance model for agents and coding

      - `"claude-opus-4-1"`

        Exceptional model for specialized complex tasks

      - `"claude-opus-4-1-20250805"`

        Exceptional model for specialized complex tasks

    - `string`

  - `role: "assistant"`

    Conversational role of the generated message.

    This will always be `"assistant"`.

    - `"assistant"`

  - `stop_details: RefusalStopDetails`

    Structured information about a refusal.

    - `category: "cyber" or "bio" or "frontier_llm" or "reasoning_extraction"`

      The policy category that triggered a refusal.

      - `"cyber"`

      - `"bio"`

      - `"frontier_llm"`

      - `"reasoning_extraction"`

    - `explanation: string`

      Human-readable explanation of the refusal.

      This text is not guaranteed to be stable. `null` when no explanation is available for the category.

    - `type: "refusal"`

      - `"refusal"`

  - `stop_reason: StopReason`

    The reason that we stopped.

    This may be one the following values:

    * `"end_turn"`: the model reached a natural stopping point
    * `"max_tokens"`: we exceeded the requested `max_tokens` or the model's maximum
    * `"stop_sequence"`: one of your provided custom `stop_sequences` was generated
    * `"tool_use"`: the model invoked one or more tools
    * `"pause_turn"`: we paused a long-running turn. You may provide the response back as-is in a subsequent request to let the model continue.
    * `"refusal"`: when streaming classifiers intervene to handle potential policy violations

    In non-streaming mode this value is always non-null. In streaming mode, it is null in the `message_start` event and non-null otherwise.

    - `"end_turn"`

    - `"max_tokens"`

    - `"stop_sequence"`

    - `"tool_use"`

    - `"pause_turn"`

    - `"refusal"`

  - `stop_sequence: string`

    Which custom stop sequence was generated, if any.

    This value will be a non-null string if one of your custom stop sequences was generated.

  - `type: "message"`

    Object type.

    For Messages, this is always `"message"`.

    - `"message"`

  - `usage: Usage`

    Billing and rate-limit usage.

    Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems.

    Under the hood, the API transforms requests into a format suitable for the model. The model's output then goes through a parsing stage before becoming an API response. As a result, the token counts in `usage` will not match one-to-one with the exact visible content of an API request or response.

    For example, `output_tokens` will be non-zero, even for an empty string response from Claude.

    Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.

    - `cache_creation: CacheCreation`

      Breakdown of cached tokens by TTL

      - `ephemeral_1h_input_tokens: number`

        The number of input tokens used to create the 1 hour cache entry.

      - `ephemeral_5m_input_tokens: number`

        The number of input tokens used to create the 5 minute cache entry.

    - `cache_creation_input_tokens: number`

      The number of input tokens used to create the cache entry.

    - `cache_read_input_tokens: number`

      The number of input tokens read from the cache.

    - `inference_geo: string`

      The geographic region where inference was performed for this request.

    - `input_tokens: number`

      The number of input tokens which were used.

    - `output_tokens: number`

      The number of output tokens which were used.

    - `output_tokens_details: OutputTokensDetails`

      Breakdown of output tokens by category.

      `output_tokens` remains the inclusive, authoritative total used for billing.
      This object provides a read-only decomposition for observability — for example,
      how many of the billed output tokens were spent on internal reasoning that may
      have been summarized before being returned to you.

      - `thinking_tokens: number`

        Number of output tokens the model generated as internal reasoning, including
        the thinking-block delimiter tokens.

        Reflects the raw reasoning the model produced, not the (possibly shorter)
        summarized thinking text returned in the response body. Computed by
        re-tokenizing the raw reasoning text, so it may differ from the model's exact
        generation count by a small number of tokens. Always ≤ `output_tokens`;
        `output_tokens - thinking_tokens` approximates the non-reasoning output.

    - `server_tool_use: ServerToolUsage`

      The number of server tool requests.

      - `web_fetch_requests: number`

        The number of web fetch tool requests.

      - `web_search_requests: number`

        The number of web search tool requests.

    - `service_tier: "standard" or "priority" or "batch"`

      If the request used the priority, standard, or batch tier.

      - `"standard"`

      - `"priority"`

      - `"batch"`

### Message Count Tokens Tool

- `MessageCountTokensTool = Tool or ToolBash20250124 or CodeExecutionTool20250522 or 16 more`

  Code execution tool with REPL state persistence (daemon mode + gVisor checkpoint).

  - `Tool object { input_schema, name, allowed_callers, 7 more }`

    - `input_schema: object { type, properties, required }`

      [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.

      This defines the shape of the `input` that your tool accepts and that the model will produce.

      - `type: "object"`

        - `"object"`

      - `properties: optional map[unknown]`

      - `required: optional array of string`

    - `name: string`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

      - `type: "ephemeral"`

        - `"ephemeral"`

      - `ttl: optional "5m" or "1h"`

        The time-to-live for the cache control breakpoint.

        This may be one the following values:

        - `5m`: 5 minutes
        - `1h`: 1 hour

        Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

        - `"5m"`

        - `"1h"`

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `description: optional string`

      Description of what this tool does.

      Tool descriptions should be as detailed as possible. The more information that the model has about what the tool is and how to use it, the better it will perform. You can use natural language descriptions to reinforce important aspects of the tool input JSON schema.

    - `eager_input_streaming: optional boolean`

      Enable eager input streaming for this tool. When true, tool input parameters will be streamed incrementally as they are generated, and types will be inferred on-the-fly rather than buffering the full JSON output. When false, streaming is disabled for this tool even if the fine-grained-tool-streaming beta is active. When null (default), uses the default behavior based on beta headers.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `type: optional "custom"`

      - `"custom"`

  - `ToolBash20250124 object { name, type, allowed_callers, 4 more }`

    - `name: "bash"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"bash"`

    - `type: "bash_20250124"`

      - `"bash_20250124"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20250522 object { name, type, allowed_callers, 3 more }`

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20250522"`

      - `"code_execution_20250522"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20250825 object { name, type, allowed_callers, 3 more }`

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20250825"`

      - `"code_execution_20250825"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20260120 object { name, type, allowed_callers, 3 more }`

    Code execution tool with REPL state persistence (daemon mode + gVisor checkpoint).

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20260120"`

      - `"code_execution_20260120"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20260521 object { name, type, allowed_callers, 3 more }`

    Code execution tool with REPL state persistence.

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20260521"`

      - `"code_execution_20260521"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `MemoryTool20250818 object { name, type, allowed_callers, 4 more }`

    - `name: "memory"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"memory"`

    - `type: "memory_20250818"`

      - `"memory_20250818"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolTextEditor20250124 object { name, type, allowed_callers, 4 more }`

    - `name: "str_replace_editor"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"str_replace_editor"`

    - `type: "text_editor_20250124"`

      - `"text_editor_20250124"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolTextEditor20250429 object { name, type, allowed_callers, 4 more }`

    - `name: "str_replace_based_edit_tool"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"str_replace_based_edit_tool"`

    - `type: "text_editor_20250429"`

      - `"text_editor_20250429"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolTextEditor20250728 object { name, type, allowed_callers, 5 more }`

    - `name: "str_replace_based_edit_tool"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"str_replace_based_edit_tool"`

    - `type: "text_editor_20250728"`

      - `"text_editor_20250728"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `max_characters: optional number`

      Maximum number of characters to display when viewing a file. If not specified, defaults to displaying the full file.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `WebSearchTool20250305 object { name, type, allowed_callers, 7 more }`

    - `name: "web_search"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_search"`

    - `type: "web_search_20250305"`

      - `"web_search_20250305"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

    - `blocked_domains: optional array of string`

      If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `user_location: optional UserLocation`

      Parameters for the user's location. Used to provide more relevant search results.

      - `type: "approximate"`

        - `"approximate"`

      - `city: optional string`

        The city of the user.

      - `country: optional string`

        The two letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the user.

      - `region: optional string`

        The region of the user.

      - `timezone: optional string`

        The [IANA timezone](https://nodatime.org/TimeZones) of the user.

  - `WebFetchTool20250910 object { name, type, allowed_callers, 8 more }`

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20250910"`

      - `"web_fetch_20250910"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

      - `enabled: optional boolean`

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `WebSearchTool20260209 object { name, type, allowed_callers, 7 more }`

    - `name: "web_search"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_search"`

    - `type: "web_search_20260209"`

      - `"web_search_20260209"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

    - `blocked_domains: optional array of string`

      If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `user_location: optional UserLocation`

      Parameters for the user's location. Used to provide more relevant search results.

  - `WebFetchTool20260209 object { name, type, allowed_callers, 8 more }`

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20260209"`

      - `"web_fetch_20260209"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `WebFetchTool20260309 object { name, type, allowed_callers, 9 more }`

    Web fetch tool with use_cache parameter for bypassing cached content.

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20260309"`

      - `"web_fetch_20260309"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `use_cache: optional boolean`

      Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.

  - `WebSearchTool20260318 object { name, type, allowed_callers, 8 more }`

    - `name: "web_search"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_search"`

    - `type: "web_search_20260318"`

      - `"web_search_20260318"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

    - `blocked_domains: optional array of string`

      If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `response_inclusion: optional "full" or "excluded"`

      How this tool's result blocks appear in the API response when the result was consumed by a completed code_execution call in the same turn. 'full' returns the complete content (default). 'excluded' drops the nested server_tool_use and result block pair entirely. Results from direct calls, or from code_execution calls that paused before completing, are always returned in full so they can be sent back on the next turn.

      - `"full"`

      - `"excluded"`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `user_location: optional UserLocation`

      Parameters for the user's location. Used to provide more relevant search results.

  - `WebFetchTool20260318 object { name, type, allowed_callers, 10 more }`

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20260318"`

      - `"web_fetch_20260318"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `response_inclusion: optional "full" or "excluded"`

      How this tool's result blocks appear in the API response when the result was consumed by a completed code_execution call in the same turn. 'full' returns the complete content (default). 'excluded' drops the nested server_tool_use and result block pair entirely. Results from direct calls, or from code_execution calls that paused before completing, are always returned in full so they can be sent back on the next turn.

      - `"full"`

      - `"excluded"`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `use_cache: optional boolean`

      Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.

  - `ToolSearchToolBm25_20251119 object { name, type, allowed_callers, 3 more }`

    - `name: "tool_search_tool_bm25"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"tool_search_tool_bm25"`

    - `type: "tool_search_tool_bm25_20251119" or "tool_search_tool_bm25"`

      - `"tool_search_tool_bm25_20251119"`

      - `"tool_search_tool_bm25"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolSearchToolRegex20251119 object { name, type, allowed_callers, 3 more }`

    - `name: "tool_search_tool_regex"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"tool_search_tool_regex"`

    - `type: "tool_search_tool_regex_20251119" or "tool_search_tool_regex"`

      - `"tool_search_tool_regex_20251119"`

      - `"tool_search_tool_regex"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

### Message Delta Usage

- `MessageDeltaUsage object { cache_creation_input_tokens, cache_read_input_tokens, input_tokens, 3 more }`

  - `cache_creation_input_tokens: number`

    The cumulative number of input tokens used to create the cache entry.

  - `cache_read_input_tokens: number`

    The cumulative number of input tokens read from the cache.

  - `input_tokens: number`

    The cumulative number of input tokens which were used.

  - `output_tokens: number`

    The cumulative number of output tokens which were used.

  - `output_tokens_details: OutputTokensDetails`

    Breakdown of output tokens by category.

    `output_tokens` remains the inclusive, authoritative total used for billing.
    This object provides a read-only decomposition for observability — for example,
    how many of the billed output tokens were spent on internal reasoning that may
    have been summarized before being returned to you.

    - `thinking_tokens: number`

      Number of output tokens the model generated as internal reasoning, including
      the thinking-block delimiter tokens.

      Reflects the raw reasoning the model produced, not the (possibly shorter)
      summarized thinking text returned in the response body. Computed by
      re-tokenizing the raw reasoning text, so it may differ from the model's exact
      generation count by a small number of tokens. Always ≤ `output_tokens`;
      `output_tokens - thinking_tokens` approximates the non-reasoning output.

  - `server_tool_use: ServerToolUsage`

    The number of server tool requests.

    - `web_fetch_requests: number`

      The number of web fetch tool requests.

    - `web_search_requests: number`

      The number of web search tool requests.

### Message Param

- `MessageParam object { content, role }`

  - `content: string or array of ContentBlockParam`

    - `string`

    - `array of ContentBlockParam`

      - `TextBlockParam object { text, type, cache_control, citations }`

        - `text: string`

        - `type: "text"`

          - `"text"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

          - `type: "ephemeral"`

            - `"ephemeral"`

          - `ttl: optional "5m" or "1h"`

            The time-to-live for the cache control breakpoint.

            This may be one the following values:

            - `5m`: 5 minutes
            - `1h`: 1 hour

            Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

            - `"5m"`

            - `"1h"`

        - `citations: optional array of TextCitationParam`

          - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_char_index: number`

            - `start_char_index: number`

            - `type: "char_location"`

              - `"char_location"`

          - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_page_number: number`

            - `start_page_number: number`

            - `type: "page_location"`

              - `"page_location"`

          - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `document_index: number`

            - `document_title: string`

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `type: "content_block_location"`

              - `"content_block_location"`

          - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

            - `cited_text: string`

            - `encrypted_index: string`

            - `title: string`

            - `type: "web_search_result_location"`

              - `"web_search_result_location"`

            - `url: string`

          - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `search_result_index: number`

              0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

              Counted separately from `document_index`; server-side web search results are not included in this count.

            - `source: string`

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `title: string`

            - `type: "search_result_location"`

              - `"search_result_location"`

      - `ImageBlockParam object { source, type, cache_control }`

        - `source: Base64ImageSource or URLImageSource`

          - `Base64ImageSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

              - `"image/jpeg"`

              - `"image/png"`

              - `"image/gif"`

              - `"image/webp"`

            - `type: "base64"`

              - `"base64"`

          - `URLImageSource object { type, url }`

            - `type: "url"`

              - `"url"`

            - `url: string`

        - `type: "image"`

          - `"image"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `DocumentBlockParam object { source, type, cache_control, 3 more }`

        - `source: Base64PDFSource or PlainTextSource or ContentBlockSource or URLPDFSource`

          - `Base64PDFSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "application/pdf"`

              - `"application/pdf"`

            - `type: "base64"`

              - `"base64"`

          - `PlainTextSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "text/plain"`

              - `"text/plain"`

            - `type: "text"`

              - `"text"`

          - `ContentBlockSource object { content, type }`

            - `content: string or array of ContentBlockSourceContent`

              - `string`

              - `ContentBlockSourceContent = array of ContentBlockSourceContent`

                - `TextBlockParam object { text, type, cache_control, citations }`

                - `ImageBlockParam object { source, type, cache_control }`

            - `type: "content"`

              - `"content"`

          - `URLPDFSource object { type, url }`

            - `type: "url"`

              - `"url"`

            - `url: string`

        - `type: "document"`

          - `"document"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

          - `enabled: optional boolean`

        - `context: optional string`

        - `title: optional string`

      - `SearchResultBlockParam object { content, source, title, 3 more }`

        - `content: array of TextBlockParam`

          - `text: string`

          - `type: "text"`

          - `cache_control: optional CacheControlEphemeral`

            Create a cache control breakpoint at this content block.

          - `citations: optional array of TextCitationParam`

        - `source: string`

        - `title: string`

        - `type: "search_result"`

          - `"search_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

      - `ThinkingBlockParam object { signature, thinking, type }`

        - `signature: string`

        - `thinking: string`

        - `type: "thinking"`

          - `"thinking"`

      - `RedactedThinkingBlockParam object { data, type }`

        - `data: string`

        - `type: "redacted_thinking"`

          - `"redacted_thinking"`

      - `ToolUseBlockParam object { id, input, name, 3 more }`

        - `id: string`

        - `input: map[unknown]`

        - `name: string`

        - `type: "tool_use"`

          - `"tool_use"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

            - `type: "direct"`

              - `"direct"`

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

            - `tool_id: string`

            - `type: "code_execution_20250825"`

              - `"code_execution_20250825"`

          - `ServerToolCaller20260120 object { tool_id, type }`

            - `tool_id: string`

            - `type: "code_execution_20260120"`

              - `"code_execution_20260120"`

      - `ToolResultBlockParam object { tool_use_id, type, cache_control, 2 more }`

        - `tool_use_id: string`

        - `type: "tool_result"`

          - `"tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `content: optional string or array of TextBlockParam or ImageBlockParam or SearchResultBlockParam or 2 more`

          - `string`

          - `array of TextBlockParam or ImageBlockParam or SearchResultBlockParam or 2 more`

            - `TextBlockParam object { text, type, cache_control, citations }`

            - `ImageBlockParam object { source, type, cache_control }`

            - `SearchResultBlockParam object { content, source, title, 3 more }`

            - `DocumentBlockParam object { source, type, cache_control, 3 more }`

            - `ToolReferenceBlockParam object { tool_name, type, cache_control }`

              Tool reference block that can be included in tool_result content.

              - `tool_name: string`

              - `type: "tool_reference"`

                - `"tool_reference"`

              - `cache_control: optional CacheControlEphemeral`

                Create a cache control breakpoint at this content block.

        - `is_error: optional boolean`

      - `ServerToolUseBlockParam object { id, input, name, 3 more }`

        - `id: string`

        - `input: map[unknown]`

        - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

          - `"web_search"`

          - `"web_fetch"`

          - `"code_execution"`

          - `"bash_code_execution"`

          - `"text_editor_code_execution"`

          - `"tool_search_tool_regex"`

          - `"tool_search_tool_bm25"`

        - `type: "server_tool_use"`

          - `"server_tool_use"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

      - `WebSearchToolResultBlockParam object { content, tool_use_id, type, 2 more }`

        - `content: WebSearchToolResultBlockParamContent`

          - `WebSearchToolResultBlockItem = array of WebSearchResultBlockParam`

            - `encrypted_content: string`

            - `title: string`

            - `type: "web_search_result"`

              - `"web_search_result"`

            - `url: string`

            - `page_age: optional string`

          - `WebSearchToolRequestError object { error_code, type }`

            - `error_code: WebSearchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"max_uses_exceeded"`

              - `"too_many_requests"`

              - `"query_too_long"`

              - `"request_too_large"`

            - `type: "web_search_tool_result_error"`

              - `"web_search_tool_result_error"`

        - `tool_use_id: string`

        - `type: "web_search_tool_result"`

          - `"web_search_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

      - `WebFetchToolResultBlockParam object { content, tool_use_id, type, 2 more }`

        - `content: WebFetchToolResultErrorBlockParam or WebFetchBlockParam`

          - `WebFetchToolResultErrorBlockParam object { error_code, type }`

            - `error_code: WebFetchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"url_too_long"`

              - `"url_not_allowed"`

              - `"url_not_in_prior_context"`

              - `"url_not_accessible"`

              - `"unsupported_content_type"`

              - `"too_many_requests"`

              - `"max_uses_exceeded"`

              - `"unavailable"`

            - `type: "web_fetch_tool_result_error"`

              - `"web_fetch_tool_result_error"`

          - `WebFetchBlockParam object { content, type, url, retrieved_at }`

            - `content: DocumentBlockParam`

            - `type: "web_fetch_result"`

              - `"web_fetch_result"`

            - `url: string`

              Fetched content URL

            - `retrieved_at: optional string`

              ISO 8601 timestamp when the content was retrieved

        - `tool_use_id: string`

        - `type: "web_fetch_tool_result"`

          - `"web_fetch_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

      - `CodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

        - `content: CodeExecutionToolResultBlockParamContent`

          Code execution result with encrypted stdout for PFC + web_search results.

          - `CodeExecutionToolResultErrorParam object { error_code, type }`

            - `error_code: CodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

            - `type: "code_execution_tool_result_error"`

              - `"code_execution_tool_result_error"`

          - `CodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

            - `content: array of CodeExecutionOutputBlockParam`

              - `file_id: string`

              - `type: "code_execution_output"`

                - `"code_execution_output"`

            - `return_code: number`

            - `stderr: string`

            - `stdout: string`

            - `type: "code_execution_result"`

              - `"code_execution_result"`

          - `EncryptedCodeExecutionResultBlockParam object { content, encrypted_stdout, return_code, 2 more }`

            Code execution result with encrypted stdout for PFC + web_search results.

            - `content: array of CodeExecutionOutputBlockParam`

              - `file_id: string`

              - `type: "code_execution_output"`

            - `encrypted_stdout: string`

            - `return_code: number`

            - `stderr: string`

            - `type: "encrypted_code_execution_result"`

              - `"encrypted_code_execution_result"`

        - `tool_use_id: string`

        - `type: "code_execution_tool_result"`

          - `"code_execution_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `BashCodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

        - `content: BashCodeExecutionToolResultErrorParam or BashCodeExecutionResultBlockParam`

          - `BashCodeExecutionToolResultErrorParam object { error_code, type }`

            - `error_code: BashCodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

              - `"output_file_too_large"`

            - `type: "bash_code_execution_tool_result_error"`

              - `"bash_code_execution_tool_result_error"`

          - `BashCodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

            - `content: array of BashCodeExecutionOutputBlockParam`

              - `file_id: string`

              - `type: "bash_code_execution_output"`

                - `"bash_code_execution_output"`

            - `return_code: number`

            - `stderr: string`

            - `stdout: string`

            - `type: "bash_code_execution_result"`

              - `"bash_code_execution_result"`

        - `tool_use_id: string`

        - `type: "bash_code_execution_tool_result"`

          - `"bash_code_execution_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `TextEditorCodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

        - `content: TextEditorCodeExecutionToolResultErrorParam or TextEditorCodeExecutionViewResultBlockParam or TextEditorCodeExecutionCreateResultBlockParam or TextEditorCodeExecutionStrReplaceResultBlockParam`

          - `TextEditorCodeExecutionToolResultErrorParam object { error_code, type, error_message }`

            - `error_code: TextEditorCodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

              - `"file_not_found"`

            - `type: "text_editor_code_execution_tool_result_error"`

              - `"text_editor_code_execution_tool_result_error"`

            - `error_message: optional string`

          - `TextEditorCodeExecutionViewResultBlockParam object { content, file_type, type, 3 more }`

            - `content: string`

            - `file_type: "text" or "image" or "pdf"`

              - `"text"`

              - `"image"`

              - `"pdf"`

            - `type: "text_editor_code_execution_view_result"`

              - `"text_editor_code_execution_view_result"`

            - `num_lines: optional number`

            - `start_line: optional number`

            - `total_lines: optional number`

          - `TextEditorCodeExecutionCreateResultBlockParam object { is_file_update, type }`

            - `is_file_update: boolean`

            - `type: "text_editor_code_execution_create_result"`

              - `"text_editor_code_execution_create_result"`

          - `TextEditorCodeExecutionStrReplaceResultBlockParam object { type, lines, new_lines, 3 more }`

            - `type: "text_editor_code_execution_str_replace_result"`

              - `"text_editor_code_execution_str_replace_result"`

            - `lines: optional array of string`

            - `new_lines: optional number`

            - `new_start: optional number`

            - `old_lines: optional number`

            - `old_start: optional number`

        - `tool_use_id: string`

        - `type: "text_editor_code_execution_tool_result"`

          - `"text_editor_code_execution_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `ToolSearchToolResultBlockParam object { content, tool_use_id, type, cache_control }`

        - `content: ToolSearchToolResultErrorParam or ToolSearchToolSearchResultBlockParam`

          - `ToolSearchToolResultErrorParam object { error_code, type, error_message }`

            - `error_code: ToolSearchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

            - `type: "tool_search_tool_result_error"`

              - `"tool_search_tool_result_error"`

            - `error_message: optional string`

          - `ToolSearchToolSearchResultBlockParam object { tool_references, type }`

            - `tool_references: array of ToolReferenceBlockParam`

              - `tool_name: string`

              - `type: "tool_reference"`

              - `cache_control: optional CacheControlEphemeral`

                Create a cache control breakpoint at this content block.

            - `type: "tool_search_tool_search_result"`

              - `"tool_search_tool_search_result"`

        - `tool_use_id: string`

        - `type: "tool_search_tool_result"`

          - `"tool_search_tool_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `ContainerUploadBlockParam object { file_id, type, cache_control }`

        A content block that represents a file to be uploaded to the container
        Files uploaded via this block will be available in the container's input directory.

        - `file_id: string`

        - `type: "container_upload"`

          - `"container_upload"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `MidConversationSystemBlockParam object { content, type, cache_control }`

        System instructions that appear mid-conversation.

        Use this block to provide or update system-level instructions at a specific
        point in the conversation, rather than only via the top-level `system` parameter.

        - `content: array of TextBlockParam`

          System instruction text blocks.

          - `text: string`

          - `type: "text"`

          - `cache_control: optional CacheControlEphemeral`

            Create a cache control breakpoint at this content block.

          - `citations: optional array of TextCitationParam`

        - `type: "mid_conv_system"`

          - `"mid_conv_system"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

  - `role: "user" or "assistant" or "system"`

    - `"user"`

    - `"assistant"`

    - `"system"`

### Message Tokens Count

- `MessageTokensCount object { input_tokens }`

  - `input_tokens: number`

    The total number of tokens across the provided list of messages, system prompt, and tools.

### Metadata

- `Metadata object { user_id }`

  - `user_id: optional string`

    An external identifier for the user who is associated with the request.

    This should be a uuid, hash value, or other opaque identifier. Anthropic may use this id to help detect abuse. Do not include any identifying information such as name, email address, or phone number.

### Mid Conversation System Block Param

- `MidConversationSystemBlockParam object { content, type, cache_control }`

  System instructions that appear mid-conversation.

  Use this block to provide or update system-level instructions at a specific
  point in the conversation, rather than only via the top-level `system` parameter.

  - `content: array of TextBlockParam`

    System instruction text blocks.

    - `text: string`

    - `type: "text"`

      - `"text"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

      - `type: "ephemeral"`

        - `"ephemeral"`

      - `ttl: optional "5m" or "1h"`

        The time-to-live for the cache control breakpoint.

        This may be one the following values:

        - `5m`: 5 minutes
        - `1h`: 1 hour

        Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

        - `"5m"`

        - `"1h"`

    - `citations: optional array of TextCitationParam`

      - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

        - `cited_text: string`

        - `document_index: number`

        - `document_title: string`

        - `end_char_index: number`

        - `start_char_index: number`

        - `type: "char_location"`

          - `"char_location"`

      - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

        - `cited_text: string`

        - `document_index: number`

        - `document_title: string`

        - `end_page_number: number`

        - `start_page_number: number`

        - `type: "page_location"`

          - `"page_location"`

      - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

        - `cited_text: string`

          The full text of the cited block range, concatenated.

          Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

        - `document_index: number`

        - `document_title: string`

        - `end_block_index: number`

          Exclusive 0-based end index of the cited block range in the source's `content` array.

          Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

        - `start_block_index: number`

          0-based index of the first cited block in the source's `content` array.

        - `type: "content_block_location"`

          - `"content_block_location"`

      - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

        - `cited_text: string`

        - `encrypted_index: string`

        - `title: string`

        - `type: "web_search_result_location"`

          - `"web_search_result_location"`

        - `url: string`

      - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

        - `cited_text: string`

          The full text of the cited block range, concatenated.

          Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

        - `end_block_index: number`

          Exclusive 0-based end index of the cited block range in the source's `content` array.

          Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

        - `search_result_index: number`

          0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

          Counted separately from `document_index`; server-side web search results are not included in this count.

        - `source: string`

        - `start_block_index: number`

          0-based index of the first cited block in the source's `content` array.

        - `title: string`

        - `type: "search_result_location"`

          - `"search_result_location"`

  - `type: "mid_conv_system"`

    - `"mid_conv_system"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

### Model

- `Model = "claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more or string`

  The model that will complete your prompt.

  See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

  - `"claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more`

    The model that will complete your prompt.

    See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

    - `"claude-sonnet-5"`

      High-performance model for coding and agents

    - `"claude-fable-5"`

      Next generation of intelligence for the hardest knowledge work and coding problems

    - `"claude-mythos-5"`

      Most capable model for cybersecurity and biology research

    - `"claude-opus-4-8"`

      Frontier intelligence for long-running agents and coding

    - `"claude-opus-4-7"`

      Frontier intelligence for long-running agents and coding

    - `"claude-mythos-preview"`

      New class of intelligence, strongest in coding and cybersecurity

    - `"claude-opus-4-6"`

      Frontier intelligence for long-running agents and coding

    - `"claude-sonnet-4-6"`

      Best combination of speed and intelligence

    - `"claude-haiku-4-5"`

      Fastest model with near-frontier intelligence

    - `"claude-haiku-4-5-20251001"`

      Fastest model with near-frontier intelligence

    - `"claude-opus-4-5"`

      Premium model combining maximum intelligence with practical performance

    - `"claude-opus-4-5-20251101"`

      Premium model combining maximum intelligence with practical performance

    - `"claude-sonnet-4-5"`

      High-performance model for agents and coding

    - `"claude-sonnet-4-5-20250929"`

      High-performance model for agents and coding

    - `"claude-opus-4-1"`

      Exceptional model for specialized complex tasks

    - `"claude-opus-4-1-20250805"`

      Exceptional model for specialized complex tasks

  - `string`

### Output Config

- `OutputConfig object { effort, format }`

  - `effort: optional "low" or "medium" or "high" or 2 more`

    All possible effort levels.

    - `"low"`

    - `"medium"`

    - `"high"`

    - `"xhigh"`

    - `"max"`

  - `format: optional JSONOutputFormat`

    A schema to specify Claude's output format in responses. See [structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)

    - `schema: map[unknown]`

      The JSON schema of the format

    - `type: "json_schema"`

      - `"json_schema"`

### Output Tokens Details

- `OutputTokensDetails object { thinking_tokens }`

  - `thinking_tokens: number`

    Number of output tokens the model generated as internal reasoning, including
    the thinking-block delimiter tokens.

    Reflects the raw reasoning the model produced, not the (possibly shorter)
    summarized thinking text returned in the response body. Computed by
    re-tokenizing the raw reasoning text, so it may differ from the model's exact
    generation count by a small number of tokens. Always ≤ `output_tokens`;
    `output_tokens - thinking_tokens` approximates the non-reasoning output.

### Plain Text Source

- `PlainTextSource object { data, media_type, type }`

  - `data: string`

  - `media_type: "text/plain"`

    - `"text/plain"`

  - `type: "text"`

    - `"text"`

### Raw Content Block Delta

- `RawContentBlockDelta = TextDelta or InputJSONDelta or CitationsDelta or 2 more`

  - `TextDelta object { text, type }`

    - `text: string`

    - `type: "text_delta"`

      - `"text_delta"`

  - `InputJSONDelta object { partial_json, type }`

    - `partial_json: string`

    - `type: "input_json_delta"`

      - `"input_json_delta"`

  - `CitationsDelta object { citation, type }`

    - `citation: CitationCharLocation or CitationPageLocation or CitationContentBlockLocation or 2 more`

      - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

        - `cited_text: string`

        - `document_index: number`

        - `document_title: string`

        - `end_char_index: number`

        - `file_id: string`

        - `start_char_index: number`

        - `type: "char_location"`

          - `"char_location"`

      - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

        - `cited_text: string`

        - `document_index: number`

        - `document_title: string`

        - `end_page_number: number`

        - `file_id: string`

        - `start_page_number: number`

        - `type: "page_location"`

          - `"page_location"`

      - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

        - `cited_text: string`

          The full text of the cited block range, concatenated.

          Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

        - `document_index: number`

        - `document_title: string`

        - `end_block_index: number`

          Exclusive 0-based end index of the cited block range in the source's `content` array.

          Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

        - `file_id: string`

        - `start_block_index: number`

          0-based index of the first cited block in the source's `content` array.

        - `type: "content_block_location"`

          - `"content_block_location"`

      - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

        - `cited_text: string`

        - `encrypted_index: string`

        - `title: string`

        - `type: "web_search_result_location"`

          - `"web_search_result_location"`

        - `url: string`

      - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

        - `cited_text: string`

          The full text of the cited block range, concatenated.

          Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

        - `end_block_index: number`

          Exclusive 0-based end index of the cited block range in the source's `content` array.

          Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

        - `search_result_index: number`

          0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

          Counted separately from `document_index`; server-side web search results are not included in this count.

        - `source: string`

        - `start_block_index: number`

          0-based index of the first cited block in the source's `content` array.

        - `title: string`

        - `type: "search_result_location"`

          - `"search_result_location"`

    - `type: "citations_delta"`

      - `"citations_delta"`

  - `ThinkingDelta object { thinking, type }`

    - `thinking: string`

    - `type: "thinking_delta"`

      - `"thinking_delta"`

  - `SignatureDelta object { signature, type }`

    - `signature: string`

    - `type: "signature_delta"`

      - `"signature_delta"`

### Raw Content Block Delta Event

- `RawContentBlockDeltaEvent object { delta, index, type }`

  - `delta: RawContentBlockDelta`

    - `TextDelta object { text, type }`

      - `text: string`

      - `type: "text_delta"`

        - `"text_delta"`

    - `InputJSONDelta object { partial_json, type }`

      - `partial_json: string`

      - `type: "input_json_delta"`

        - `"input_json_delta"`

    - `CitationsDelta object { citation, type }`

      - `citation: CitationCharLocation or CitationPageLocation or CitationContentBlockLocation or 2 more`

        - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

          - `cited_text: string`

          - `document_index: number`

          - `document_title: string`

          - `end_char_index: number`

          - `file_id: string`

          - `start_char_index: number`

          - `type: "char_location"`

            - `"char_location"`

        - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

          - `cited_text: string`

          - `document_index: number`

          - `document_title: string`

          - `end_page_number: number`

          - `file_id: string`

          - `start_page_number: number`

          - `type: "page_location"`

            - `"page_location"`

        - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

          - `cited_text: string`

            The full text of the cited block range, concatenated.

            Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

          - `document_index: number`

          - `document_title: string`

          - `end_block_index: number`

            Exclusive 0-based end index of the cited block range in the source's `content` array.

            Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

          - `file_id: string`

          - `start_block_index: number`

            0-based index of the first cited block in the source's `content` array.

          - `type: "content_block_location"`

            - `"content_block_location"`

        - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

          - `cited_text: string`

          - `encrypted_index: string`

          - `title: string`

          - `type: "web_search_result_location"`

            - `"web_search_result_location"`

          - `url: string`

        - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

          - `cited_text: string`

            The full text of the cited block range, concatenated.

            Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

          - `end_block_index: number`

            Exclusive 0-based end index of the cited block range in the source's `content` array.

            Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

          - `search_result_index: number`

            0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

            Counted separately from `document_index`; server-side web search results are not included in this count.

          - `source: string`

          - `start_block_index: number`

            0-based index of the first cited block in the source's `content` array.

          - `title: string`

          - `type: "search_result_location"`

            - `"search_result_location"`

      - `type: "citations_delta"`

        - `"citations_delta"`

    - `ThinkingDelta object { thinking, type }`

      - `thinking: string`

      - `type: "thinking_delta"`

        - `"thinking_delta"`

    - `SignatureDelta object { signature, type }`

      - `signature: string`

      - `type: "signature_delta"`

        - `"signature_delta"`

  - `index: number`

  - `type: "content_block_delta"`

    - `"content_block_delta"`

### Raw Content Block Start Event

- `RawContentBlockStartEvent object { content_block, index, type }`

  - `content_block: TextBlock or ThinkingBlock or RedactedThinkingBlock or 9 more`

    Response model for a file uploaded to the container.

    - `TextBlock object { citations, text, type }`

      - `citations: array of TextCitation`

        Citations supporting the text block.

        The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.

        - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

          - `cited_text: string`

          - `document_index: number`

          - `document_title: string`

          - `end_char_index: number`

          - `file_id: string`

          - `start_char_index: number`

          - `type: "char_location"`

            - `"char_location"`

        - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

          - `cited_text: string`

          - `document_index: number`

          - `document_title: string`

          - `end_page_number: number`

          - `file_id: string`

          - `start_page_number: number`

          - `type: "page_location"`

            - `"page_location"`

        - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

          - `cited_text: string`

            The full text of the cited block range, concatenated.

            Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

          - `document_index: number`

          - `document_title: string`

          - `end_block_index: number`

            Exclusive 0-based end index of the cited block range in the source's `content` array.

            Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

          - `file_id: string`

          - `start_block_index: number`

            0-based index of the first cited block in the source's `content` array.

          - `type: "content_block_location"`

            - `"content_block_location"`

        - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

          - `cited_text: string`

          - `encrypted_index: string`

          - `title: string`

          - `type: "web_search_result_location"`

            - `"web_search_result_location"`

          - `url: string`

        - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

          - `cited_text: string`

            The full text of the cited block range, concatenated.

            Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

          - `end_block_index: number`

            Exclusive 0-based end index of the cited block range in the source's `content` array.

            Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

          - `search_result_index: number`

            0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

            Counted separately from `document_index`; server-side web search results are not included in this count.

          - `source: string`

          - `start_block_index: number`

            0-based index of the first cited block in the source's `content` array.

          - `title: string`

          - `type: "search_result_location"`

            - `"search_result_location"`

      - `text: string`

      - `type: "text"`

        - `"text"`

    - `ThinkingBlock object { signature, thinking, type }`

      - `signature: string`

      - `thinking: string`

      - `type: "thinking"`

        - `"thinking"`

    - `RedactedThinkingBlock object { data, type }`

      - `data: string`

      - `type: "redacted_thinking"`

        - `"redacted_thinking"`

    - `ToolUseBlock object { id, caller, input, 2 more }`

      - `id: string`

      - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

        Tool invocation directly from the model.

        - `DirectCaller object { type }`

          Tool invocation directly from the model.

          - `type: "direct"`

            - `"direct"`

        - `ServerToolCaller object { tool_id, type }`

          Tool invocation generated by a server-side tool.

          - `tool_id: string`

          - `type: "code_execution_20250825"`

            - `"code_execution_20250825"`

        - `ServerToolCaller20260120 object { tool_id, type }`

          - `tool_id: string`

          - `type: "code_execution_20260120"`

            - `"code_execution_20260120"`

      - `input: map[unknown]`

      - `name: string`

      - `type: "tool_use"`

        - `"tool_use"`

    - `ServerToolUseBlock object { id, caller, input, 2 more }`

      - `id: string`

      - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

        Tool invocation directly from the model.

        - `DirectCaller object { type }`

          Tool invocation directly from the model.

        - `ServerToolCaller object { tool_id, type }`

          Tool invocation generated by a server-side tool.

        - `ServerToolCaller20260120 object { tool_id, type }`

      - `input: map[unknown]`

      - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

        - `"web_search"`

        - `"web_fetch"`

        - `"code_execution"`

        - `"bash_code_execution"`

        - `"text_editor_code_execution"`

        - `"tool_search_tool_regex"`

        - `"tool_search_tool_bm25"`

      - `type: "server_tool_use"`

        - `"server_tool_use"`

    - `WebSearchToolResultBlock object { caller, content, tool_use_id, type }`

      - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

        Tool invocation directly from the model.

        - `DirectCaller object { type }`

          Tool invocation directly from the model.

        - `ServerToolCaller object { tool_id, type }`

          Tool invocation generated by a server-side tool.

        - `ServerToolCaller20260120 object { tool_id, type }`

      - `content: WebSearchToolResultBlockContent`

        - `WebSearchToolResultError object { error_code, type }`

          - `error_code: WebSearchToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"max_uses_exceeded"`

            - `"too_many_requests"`

            - `"query_too_long"`

            - `"request_too_large"`

          - `type: "web_search_tool_result_error"`

            - `"web_search_tool_result_error"`

        - `array of WebSearchResultBlock`

          - `encrypted_content: string`

          - `page_age: string`

          - `title: string`

          - `type: "web_search_result"`

            - `"web_search_result"`

          - `url: string`

      - `tool_use_id: string`

      - `type: "web_search_tool_result"`

        - `"web_search_tool_result"`

    - `WebFetchToolResultBlock object { caller, content, tool_use_id, type }`

      - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

        Tool invocation directly from the model.

        - `DirectCaller object { type }`

          Tool invocation directly from the model.

        - `ServerToolCaller object { tool_id, type }`

          Tool invocation generated by a server-side tool.

        - `ServerToolCaller20260120 object { tool_id, type }`

      - `content: WebFetchToolResultErrorBlock or WebFetchBlock`

        - `WebFetchToolResultErrorBlock object { error_code, type }`

          - `error_code: WebFetchToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"url_too_long"`

            - `"url_not_allowed"`

            - `"url_not_in_prior_context"`

            - `"url_not_accessible"`

            - `"unsupported_content_type"`

            - `"too_many_requests"`

            - `"max_uses_exceeded"`

            - `"unavailable"`

          - `type: "web_fetch_tool_result_error"`

            - `"web_fetch_tool_result_error"`

        - `WebFetchBlock object { content, retrieved_at, type, url }`

          - `content: DocumentBlock`

            - `citations: CitationsConfig`

              Citation configuration for the document

              - `enabled: boolean`

            - `source: Base64PDFSource or PlainTextSource`

              - `Base64PDFSource object { data, media_type, type }`

                - `data: string`

                - `media_type: "application/pdf"`

                  - `"application/pdf"`

                - `type: "base64"`

                  - `"base64"`

              - `PlainTextSource object { data, media_type, type }`

                - `data: string`

                - `media_type: "text/plain"`

                  - `"text/plain"`

                - `type: "text"`

                  - `"text"`

            - `title: string`

              The title of the document

            - `type: "document"`

              - `"document"`

          - `retrieved_at: string`

            ISO 8601 timestamp when the content was retrieved

          - `type: "web_fetch_result"`

            - `"web_fetch_result"`

          - `url: string`

            Fetched content URL

      - `tool_use_id: string`

      - `type: "web_fetch_tool_result"`

        - `"web_fetch_tool_result"`

    - `CodeExecutionToolResultBlock object { content, tool_use_id, type }`

      - `content: CodeExecutionToolResultBlockContent`

        Code execution result with encrypted stdout for PFC + web_search results.

        - `CodeExecutionToolResultError object { error_code, type }`

          - `error_code: CodeExecutionToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"too_many_requests"`

            - `"execution_time_exceeded"`

          - `type: "code_execution_tool_result_error"`

            - `"code_execution_tool_result_error"`

        - `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

          - `content: array of CodeExecutionOutputBlock`

            - `file_id: string`

            - `type: "code_execution_output"`

              - `"code_execution_output"`

          - `return_code: number`

          - `stderr: string`

          - `stdout: string`

          - `type: "code_execution_result"`

            - `"code_execution_result"`

        - `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

          Code execution result with encrypted stdout for PFC + web_search results.

          - `content: array of CodeExecutionOutputBlock`

            - `file_id: string`

            - `type: "code_execution_output"`

          - `encrypted_stdout: string`

          - `return_code: number`

          - `stderr: string`

          - `type: "encrypted_code_execution_result"`

            - `"encrypted_code_execution_result"`

      - `tool_use_id: string`

      - `type: "code_execution_tool_result"`

        - `"code_execution_tool_result"`

    - `BashCodeExecutionToolResultBlock object { content, tool_use_id, type }`

      - `content: BashCodeExecutionToolResultError or BashCodeExecutionResultBlock`

        - `BashCodeExecutionToolResultError object { error_code, type }`

          - `error_code: BashCodeExecutionToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"too_many_requests"`

            - `"execution_time_exceeded"`

            - `"output_file_too_large"`

          - `type: "bash_code_execution_tool_result_error"`

            - `"bash_code_execution_tool_result_error"`

        - `BashCodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

          - `content: array of BashCodeExecutionOutputBlock`

            - `file_id: string`

            - `type: "bash_code_execution_output"`

              - `"bash_code_execution_output"`

          - `return_code: number`

          - `stderr: string`

          - `stdout: string`

          - `type: "bash_code_execution_result"`

            - `"bash_code_execution_result"`

      - `tool_use_id: string`

      - `type: "bash_code_execution_tool_result"`

        - `"bash_code_execution_tool_result"`

    - `TextEditorCodeExecutionToolResultBlock object { content, tool_use_id, type }`

      - `content: TextEditorCodeExecutionToolResultError or TextEditorCodeExecutionViewResultBlock or TextEditorCodeExecutionCreateResultBlock or TextEditorCodeExecutionStrReplaceResultBlock`

        - `TextEditorCodeExecutionToolResultError object { error_code, error_message, type }`

          - `error_code: TextEditorCodeExecutionToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"too_many_requests"`

            - `"execution_time_exceeded"`

            - `"file_not_found"`

          - `error_message: string`

          - `type: "text_editor_code_execution_tool_result_error"`

            - `"text_editor_code_execution_tool_result_error"`

        - `TextEditorCodeExecutionViewResultBlock object { content, file_type, num_lines, 3 more }`

          - `content: string`

          - `file_type: "text" or "image" or "pdf"`

            - `"text"`

            - `"image"`

            - `"pdf"`

          - `num_lines: number`

          - `start_line: number`

          - `total_lines: number`

          - `type: "text_editor_code_execution_view_result"`

            - `"text_editor_code_execution_view_result"`

        - `TextEditorCodeExecutionCreateResultBlock object { is_file_update, type }`

          - `is_file_update: boolean`

          - `type: "text_editor_code_execution_create_result"`

            - `"text_editor_code_execution_create_result"`

        - `TextEditorCodeExecutionStrReplaceResultBlock object { lines, new_lines, new_start, 3 more }`

          - `lines: array of string`

          - `new_lines: number`

          - `new_start: number`

          - `old_lines: number`

          - `old_start: number`

          - `type: "text_editor_code_execution_str_replace_result"`

            - `"text_editor_code_execution_str_replace_result"`

      - `tool_use_id: string`

      - `type: "text_editor_code_execution_tool_result"`

        - `"text_editor_code_execution_tool_result"`

    - `ToolSearchToolResultBlock object { content, tool_use_id, type }`

      - `content: ToolSearchToolResultError or ToolSearchToolSearchResultBlock`

        - `ToolSearchToolResultError object { error_code, error_message, type }`

          - `error_code: ToolSearchToolResultErrorCode`

            - `"invalid_tool_input"`

            - `"unavailable"`

            - `"too_many_requests"`

            - `"execution_time_exceeded"`

          - `error_message: string`

          - `type: "tool_search_tool_result_error"`

            - `"tool_search_tool_result_error"`

        - `ToolSearchToolSearchResultBlock object { tool_references, type }`

          - `tool_references: array of ToolReferenceBlock`

            - `tool_name: string`

            - `type: "tool_reference"`

              - `"tool_reference"`

          - `type: "tool_search_tool_search_result"`

            - `"tool_search_tool_search_result"`

      - `tool_use_id: string`

      - `type: "tool_search_tool_result"`

        - `"tool_search_tool_result"`

    - `ContainerUploadBlock object { file_id, type }`

      Response model for a file uploaded to the container.

      - `file_id: string`

      - `type: "container_upload"`

        - `"container_upload"`

  - `index: number`

  - `type: "content_block_start"`

    - `"content_block_start"`

### Raw Content Block Stop Event

- `RawContentBlockStopEvent object { index, type }`

  - `index: number`

  - `type: "content_block_stop"`

    - `"content_block_stop"`

### Raw Message Delta Event

- `RawMessageDeltaEvent object { delta, type, usage }`

  - `delta: object { container, stop_details, stop_reason, stop_sequence }`

    - `container: Container`

      Information about the container used in the request (for the code execution tool)

      - `id: string`

        Identifier for the container used in this request

      - `expires_at: string`

        The time at which the container will expire.

    - `stop_details: RefusalStopDetails`

      Structured information about a refusal.

      - `category: "cyber" or "bio" or "frontier_llm" or "reasoning_extraction"`

        The policy category that triggered a refusal.

        - `"cyber"`

        - `"bio"`

        - `"frontier_llm"`

        - `"reasoning_extraction"`

      - `explanation: string`

        Human-readable explanation of the refusal.

        This text is not guaranteed to be stable. `null` when no explanation is available for the category.

      - `type: "refusal"`

        - `"refusal"`

    - `stop_reason: StopReason`

      - `"end_turn"`

      - `"max_tokens"`

      - `"stop_sequence"`

      - `"tool_use"`

      - `"pause_turn"`

      - `"refusal"`

    - `stop_sequence: string`

  - `type: "message_delta"`

    - `"message_delta"`

  - `usage: MessageDeltaUsage`

    Billing and rate-limit usage.

    Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems.

    Under the hood, the API transforms requests into a format suitable for the model. The model's output then goes through a parsing stage before becoming an API response. As a result, the token counts in `usage` will not match one-to-one with the exact visible content of an API request or response.

    For example, `output_tokens` will be non-zero, even for an empty string response from Claude.

    Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.

    - `cache_creation_input_tokens: number`

      The cumulative number of input tokens used to create the cache entry.

    - `cache_read_input_tokens: number`

      The cumulative number of input tokens read from the cache.

    - `input_tokens: number`

      The cumulative number of input tokens which were used.

    - `output_tokens: number`

      The cumulative number of output tokens which were used.

    - `output_tokens_details: OutputTokensDetails`

      Breakdown of output tokens by category.

      `output_tokens` remains the inclusive, authoritative total used for billing.
      This object provides a read-only decomposition for observability — for example,
      how many of the billed output tokens were spent on internal reasoning that may
      have been summarized before being returned to you.

      - `thinking_tokens: number`

        Number of output tokens the model generated as internal reasoning, including
        the thinking-block delimiter tokens.

        Reflects the raw reasoning the model produced, not the (possibly shorter)
        summarized thinking text returned in the response body. Computed by
        re-tokenizing the raw reasoning text, so it may differ from the model's exact
        generation count by a small number of tokens. Always ≤ `output_tokens`;
        `output_tokens - thinking_tokens` approximates the non-reasoning output.

    - `server_tool_use: ServerToolUsage`

      The number of server tool requests.

      - `web_fetch_requests: number`

        The number of web fetch tool requests.

      - `web_search_requests: number`

        The number of web search tool requests.

### Raw Message Start Event

- `RawMessageStartEvent object { message, type }`

  - `message: Message`

    - `id: string`

      Unique object identifier.

      The format and length of IDs may change over time.

    - `container: Container`

      Information about the container used in the request (for the code execution tool)

      - `id: string`

        Identifier for the container used in this request

      - `expires_at: string`

        The time at which the container will expire.

    - `content: array of ContentBlock`

      Content generated by the model.

      This is an array of content blocks, each of which has a `type` that determines its shape.

      Example:

      ```json
      [{"type": "text", "text": "Hi, I'm Claude."}]
      ```

      If the request input `messages` ended with an `assistant` turn, then the response `content` will continue directly from that last turn. You can use this to constrain the model's output.

      For example, if the input `messages` were:

      ```json
      [
        {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
        {"role": "assistant", "content": "The best answer is ("}
      ]
      ```

      Then the response `content` might be:

      ```json
      [{"type": "text", "text": "B)"}]
      ```

      - `TextBlock object { citations, text, type }`

        - `citations: array of TextCitation`

          Citations supporting the text block.

          The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.

          - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_char_index: number`

            - `file_id: string`

            - `start_char_index: number`

            - `type: "char_location"`

              - `"char_location"`

          - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_page_number: number`

            - `file_id: string`

            - `start_page_number: number`

            - `type: "page_location"`

              - `"page_location"`

          - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `document_index: number`

            - `document_title: string`

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `file_id: string`

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `type: "content_block_location"`

              - `"content_block_location"`

          - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

            - `cited_text: string`

            - `encrypted_index: string`

            - `title: string`

            - `type: "web_search_result_location"`

              - `"web_search_result_location"`

            - `url: string`

          - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `search_result_index: number`

              0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

              Counted separately from `document_index`; server-side web search results are not included in this count.

            - `source: string`

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `title: string`

            - `type: "search_result_location"`

              - `"search_result_location"`

        - `text: string`

        - `type: "text"`

          - `"text"`

      - `ThinkingBlock object { signature, thinking, type }`

        - `signature: string`

        - `thinking: string`

        - `type: "thinking"`

          - `"thinking"`

      - `RedactedThinkingBlock object { data, type }`

        - `data: string`

        - `type: "redacted_thinking"`

          - `"redacted_thinking"`

      - `ToolUseBlock object { id, caller, input, 2 more }`

        - `id: string`

        - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

            - `type: "direct"`

              - `"direct"`

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

            - `tool_id: string`

            - `type: "code_execution_20250825"`

              - `"code_execution_20250825"`

          - `ServerToolCaller20260120 object { tool_id, type }`

            - `tool_id: string`

            - `type: "code_execution_20260120"`

              - `"code_execution_20260120"`

        - `input: map[unknown]`

        - `name: string`

        - `type: "tool_use"`

          - `"tool_use"`

      - `ServerToolUseBlock object { id, caller, input, 2 more }`

        - `id: string`

        - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

        - `input: map[unknown]`

        - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

          - `"web_search"`

          - `"web_fetch"`

          - `"code_execution"`

          - `"bash_code_execution"`

          - `"text_editor_code_execution"`

          - `"tool_search_tool_regex"`

          - `"tool_search_tool_bm25"`

        - `type: "server_tool_use"`

          - `"server_tool_use"`

      - `WebSearchToolResultBlock object { caller, content, tool_use_id, type }`

        - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

        - `content: WebSearchToolResultBlockContent`

          - `WebSearchToolResultError object { error_code, type }`

            - `error_code: WebSearchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"max_uses_exceeded"`

              - `"too_many_requests"`

              - `"query_too_long"`

              - `"request_too_large"`

            - `type: "web_search_tool_result_error"`

              - `"web_search_tool_result_error"`

          - `array of WebSearchResultBlock`

            - `encrypted_content: string`

            - `page_age: string`

            - `title: string`

            - `type: "web_search_result"`

              - `"web_search_result"`

            - `url: string`

        - `tool_use_id: string`

        - `type: "web_search_tool_result"`

          - `"web_search_tool_result"`

      - `WebFetchToolResultBlock object { caller, content, tool_use_id, type }`

        - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

        - `content: WebFetchToolResultErrorBlock or WebFetchBlock`

          - `WebFetchToolResultErrorBlock object { error_code, type }`

            - `error_code: WebFetchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"url_too_long"`

              - `"url_not_allowed"`

              - `"url_not_in_prior_context"`

              - `"url_not_accessible"`

              - `"unsupported_content_type"`

              - `"too_many_requests"`

              - `"max_uses_exceeded"`

              - `"unavailable"`

            - `type: "web_fetch_tool_result_error"`

              - `"web_fetch_tool_result_error"`

          - `WebFetchBlock object { content, retrieved_at, type, url }`

            - `content: DocumentBlock`

              - `citations: CitationsConfig`

                Citation configuration for the document

                - `enabled: boolean`

              - `source: Base64PDFSource or PlainTextSource`

                - `Base64PDFSource object { data, media_type, type }`

                  - `data: string`

                  - `media_type: "application/pdf"`

                    - `"application/pdf"`

                  - `type: "base64"`

                    - `"base64"`

                - `PlainTextSource object { data, media_type, type }`

                  - `data: string`

                  - `media_type: "text/plain"`

                    - `"text/plain"`

                  - `type: "text"`

                    - `"text"`

              - `title: string`

                The title of the document

              - `type: "document"`

                - `"document"`

            - `retrieved_at: string`

              ISO 8601 timestamp when the content was retrieved

            - `type: "web_fetch_result"`

              - `"web_fetch_result"`

            - `url: string`

              Fetched content URL

        - `tool_use_id: string`

        - `type: "web_fetch_tool_result"`

          - `"web_fetch_tool_result"`

      - `CodeExecutionToolResultBlock object { content, tool_use_id, type }`

        - `content: CodeExecutionToolResultBlockContent`

          Code execution result with encrypted stdout for PFC + web_search results.

          - `CodeExecutionToolResultError object { error_code, type }`

            - `error_code: CodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

            - `type: "code_execution_tool_result_error"`

              - `"code_execution_tool_result_error"`

          - `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

            - `content: array of CodeExecutionOutputBlock`

              - `file_id: string`

              - `type: "code_execution_output"`

                - `"code_execution_output"`

            - `return_code: number`

            - `stderr: string`

            - `stdout: string`

            - `type: "code_execution_result"`

              - `"code_execution_result"`

          - `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

            Code execution result with encrypted stdout for PFC + web_search results.

            - `content: array of CodeExecutionOutputBlock`

              - `file_id: string`

              - `type: "code_execution_output"`

            - `encrypted_stdout: string`

            - `return_code: number`

            - `stderr: string`

            - `type: "encrypted_code_execution_result"`

              - `"encrypted_code_execution_result"`

        - `tool_use_id: string`

        - `type: "code_execution_tool_result"`

          - `"code_execution_tool_result"`

      - `BashCodeExecutionToolResultBlock object { content, tool_use_id, type }`

        - `content: BashCodeExecutionToolResultError or BashCodeExecutionResultBlock`

          - `BashCodeExecutionToolResultError object { error_code, type }`

            - `error_code: BashCodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

              - `"output_file_too_large"`

            - `type: "bash_code_execution_tool_result_error"`

              - `"bash_code_execution_tool_result_error"`

          - `BashCodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

            - `content: array of BashCodeExecutionOutputBlock`

              - `file_id: string`

              - `type: "bash_code_execution_output"`

                - `"bash_code_execution_output"`

            - `return_code: number`

            - `stderr: string`

            - `stdout: string`

            - `type: "bash_code_execution_result"`

              - `"bash_code_execution_result"`

        - `tool_use_id: string`

        - `type: "bash_code_execution_tool_result"`

          - `"bash_code_execution_tool_result"`

      - `TextEditorCodeExecutionToolResultBlock object { content, tool_use_id, type }`

        - `content: TextEditorCodeExecutionToolResultError or TextEditorCodeExecutionViewResultBlock or TextEditorCodeExecutionCreateResultBlock or TextEditorCodeExecutionStrReplaceResultBlock`

          - `TextEditorCodeExecutionToolResultError object { error_code, error_message, type }`

            - `error_code: TextEditorCodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

              - `"file_not_found"`

            - `error_message: string`

            - `type: "text_editor_code_execution_tool_result_error"`

              - `"text_editor_code_execution_tool_result_error"`

          - `TextEditorCodeExecutionViewResultBlock object { content, file_type, num_lines, 3 more }`

            - `content: string`

            - `file_type: "text" or "image" or "pdf"`

              - `"text"`

              - `"image"`

              - `"pdf"`

            - `num_lines: number`

            - `start_line: number`

            - `total_lines: number`

            - `type: "text_editor_code_execution_view_result"`

              - `"text_editor_code_execution_view_result"`

          - `TextEditorCodeExecutionCreateResultBlock object { is_file_update, type }`

            - `is_file_update: boolean`

            - `type: "text_editor_code_execution_create_result"`

              - `"text_editor_code_execution_create_result"`

          - `TextEditorCodeExecutionStrReplaceResultBlock object { lines, new_lines, new_start, 3 more }`

            - `lines: array of string`

            - `new_lines: number`

            - `new_start: number`

            - `old_lines: number`

            - `old_start: number`

            - `type: "text_editor_code_execution_str_replace_result"`

              - `"text_editor_code_execution_str_replace_result"`

        - `tool_use_id: string`

        - `type: "text_editor_code_execution_tool_result"`

          - `"text_editor_code_execution_tool_result"`

      - `ToolSearchToolResultBlock object { content, tool_use_id, type }`

        - `content: ToolSearchToolResultError or ToolSearchToolSearchResultBlock`

          - `ToolSearchToolResultError object { error_code, error_message, type }`

            - `error_code: ToolSearchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

            - `error_message: string`

            - `type: "tool_search_tool_result_error"`

              - `"tool_search_tool_result_error"`

          - `ToolSearchToolSearchResultBlock object { tool_references, type }`

            - `tool_references: array of ToolReferenceBlock`

              - `tool_name: string`

              - `type: "tool_reference"`

                - `"tool_reference"`

            - `type: "tool_search_tool_search_result"`

              - `"tool_search_tool_search_result"`

        - `tool_use_id: string`

        - `type: "tool_search_tool_result"`

          - `"tool_search_tool_result"`

      - `ContainerUploadBlock object { file_id, type }`

        Response model for a file uploaded to the container.

        - `file_id: string`

        - `type: "container_upload"`

          - `"container_upload"`

    - `model: Model`

      The model that will complete your prompt.

      See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

      - `"claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more`

        The model that will complete your prompt.

        See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

        - `"claude-sonnet-5"`

          High-performance model for coding and agents

        - `"claude-fable-5"`

          Next generation of intelligence for the hardest knowledge work and coding problems

        - `"claude-mythos-5"`

          Most capable model for cybersecurity and biology research

        - `"claude-opus-4-8"`

          Frontier intelligence for long-running agents and coding

        - `"claude-opus-4-7"`

          Frontier intelligence for long-running agents and coding

        - `"claude-mythos-preview"`

          New class of intelligence, strongest in coding and cybersecurity

        - `"claude-opus-4-6"`

          Frontier intelligence for long-running agents and coding

        - `"claude-sonnet-4-6"`

          Best combination of speed and intelligence

        - `"claude-haiku-4-5"`

          Fastest model with near-frontier intelligence

        - `"claude-haiku-4-5-20251001"`

          Fastest model with near-frontier intelligence

        - `"claude-opus-4-5"`

          Premium model combining maximum intelligence with practical performance

        - `"claude-opus-4-5-20251101"`

          Premium model combining maximum intelligence with practical performance

        - `"claude-sonnet-4-5"`

          High-performance model for agents and coding

        - `"claude-sonnet-4-5-20250929"`

          High-performance model for agents and coding

        - `"claude-opus-4-1"`

          Exceptional model for specialized complex tasks

        - `"claude-opus-4-1-20250805"`

          Exceptional model for specialized complex tasks

      - `string`

    - `role: "assistant"`

      Conversational role of the generated message.

      This will always be `"assistant"`.

      - `"assistant"`

    - `stop_details: RefusalStopDetails`

      Structured information about a refusal.

      - `category: "cyber" or "bio" or "frontier_llm" or "reasoning_extraction"`

        The policy category that triggered a refusal.

        - `"cyber"`

        - `"bio"`

        - `"frontier_llm"`

        - `"reasoning_extraction"`

      - `explanation: string`

        Human-readable explanation of the refusal.

        This text is not guaranteed to be stable. `null` when no explanation is available for the category.

      - `type: "refusal"`

        - `"refusal"`

    - `stop_reason: StopReason`

      The reason that we stopped.

      This may be one the following values:

      * `"end_turn"`: the model reached a natural stopping point
      * `"max_tokens"`: we exceeded the requested `max_tokens` or the model's maximum
      * `"stop_sequence"`: one of your provided custom `stop_sequences` was generated
      * `"tool_use"`: the model invoked one or more tools
      * `"pause_turn"`: we paused a long-running turn. You may provide the response back as-is in a subsequent request to let the model continue.
      * `"refusal"`: when streaming classifiers intervene to handle potential policy violations

      In non-streaming mode this value is always non-null. In streaming mode, it is null in the `message_start` event and non-null otherwise.

      - `"end_turn"`

      - `"max_tokens"`

      - `"stop_sequence"`

      - `"tool_use"`

      - `"pause_turn"`

      - `"refusal"`

    - `stop_sequence: string`

      Which custom stop sequence was generated, if any.

      This value will be a non-null string if one of your custom stop sequences was generated.

    - `type: "message"`

      Object type.

      For Messages, this is always `"message"`.

      - `"message"`

    - `usage: Usage`

      Billing and rate-limit usage.

      Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems.

      Under the hood, the API transforms requests into a format suitable for the model. The model's output then goes through a parsing stage before becoming an API response. As a result, the token counts in `usage` will not match one-to-one with the exact visible content of an API request or response.

      For example, `output_tokens` will be non-zero, even for an empty string response from Claude.

      Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.

      - `cache_creation: CacheCreation`

        Breakdown of cached tokens by TTL

        - `ephemeral_1h_input_tokens: number`

          The number of input tokens used to create the 1 hour cache entry.

        - `ephemeral_5m_input_tokens: number`

          The number of input tokens used to create the 5 minute cache entry.

      - `cache_creation_input_tokens: number`

        The number of input tokens used to create the cache entry.

      - `cache_read_input_tokens: number`

        The number of input tokens read from the cache.

      - `inference_geo: string`

        The geographic region where inference was performed for this request.

      - `input_tokens: number`

        The number of input tokens which were used.

      - `output_tokens: number`

        The number of output tokens which were used.

      - `output_tokens_details: OutputTokensDetails`

        Breakdown of output tokens by category.

        `output_tokens` remains the inclusive, authoritative total used for billing.
        This object provides a read-only decomposition for observability — for example,
        how many of the billed output tokens were spent on internal reasoning that may
        have been summarized before being returned to you.

        - `thinking_tokens: number`

          Number of output tokens the model generated as internal reasoning, including
          the thinking-block delimiter tokens.

          Reflects the raw reasoning the model produced, not the (possibly shorter)
          summarized thinking text returned in the response body. Computed by
          re-tokenizing the raw reasoning text, so it may differ from the model's exact
          generation count by a small number of tokens. Always ≤ `output_tokens`;
          `output_tokens - thinking_tokens` approximates the non-reasoning output.

      - `server_tool_use: ServerToolUsage`

        The number of server tool requests.

        - `web_fetch_requests: number`

          The number of web fetch tool requests.

        - `web_search_requests: number`

          The number of web search tool requests.

      - `service_tier: "standard" or "priority" or "batch"`

        If the request used the priority, standard, or batch tier.

        - `"standard"`

        - `"priority"`

        - `"batch"`

  - `type: "message_start"`

    - `"message_start"`

### Raw Message Stop Event

- `RawMessageStopEvent object { type }`

  - `type: "message_stop"`

    - `"message_stop"`

### Raw Message Stream Event

- `RawMessageStreamEvent = RawMessageStartEvent or RawMessageDeltaEvent or RawMessageStopEvent or 3 more`

  - `RawMessageStartEvent object { message, type }`

    - `message: Message`

      - `id: string`

        Unique object identifier.

        The format and length of IDs may change over time.

      - `container: Container`

        Information about the container used in the request (for the code execution tool)

        - `id: string`

          Identifier for the container used in this request

        - `expires_at: string`

          The time at which the container will expire.

      - `content: array of ContentBlock`

        Content generated by the model.

        This is an array of content blocks, each of which has a `type` that determines its shape.

        Example:

        ```json
        [{"type": "text", "text": "Hi, I'm Claude."}]
        ```

        If the request input `messages` ended with an `assistant` turn, then the response `content` will continue directly from that last turn. You can use this to constrain the model's output.

        For example, if the input `messages` were:

        ```json
        [
          {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
          {"role": "assistant", "content": "The best answer is ("}
        ]
        ```

        Then the response `content` might be:

        ```json
        [{"type": "text", "text": "B)"}]
        ```

        - `TextBlock object { citations, text, type }`

          - `citations: array of TextCitation`

            Citations supporting the text block.

            The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.

            - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

              - `cited_text: string`

              - `document_index: number`

              - `document_title: string`

              - `end_char_index: number`

              - `file_id: string`

              - `start_char_index: number`

              - `type: "char_location"`

                - `"char_location"`

            - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

              - `cited_text: string`

              - `document_index: number`

              - `document_title: string`

              - `end_page_number: number`

              - `file_id: string`

              - `start_page_number: number`

              - `type: "page_location"`

                - `"page_location"`

            - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

              - `cited_text: string`

                The full text of the cited block range, concatenated.

                Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

              - `document_index: number`

              - `document_title: string`

              - `end_block_index: number`

                Exclusive 0-based end index of the cited block range in the source's `content` array.

                Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

              - `file_id: string`

              - `start_block_index: number`

                0-based index of the first cited block in the source's `content` array.

              - `type: "content_block_location"`

                - `"content_block_location"`

            - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

              - `cited_text: string`

              - `encrypted_index: string`

              - `title: string`

              - `type: "web_search_result_location"`

                - `"web_search_result_location"`

              - `url: string`

            - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

              - `cited_text: string`

                The full text of the cited block range, concatenated.

                Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

              - `end_block_index: number`

                Exclusive 0-based end index of the cited block range in the source's `content` array.

                Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

              - `search_result_index: number`

                0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

                Counted separately from `document_index`; server-side web search results are not included in this count.

              - `source: string`

              - `start_block_index: number`

                0-based index of the first cited block in the source's `content` array.

              - `title: string`

              - `type: "search_result_location"`

                - `"search_result_location"`

          - `text: string`

          - `type: "text"`

            - `"text"`

        - `ThinkingBlock object { signature, thinking, type }`

          - `signature: string`

          - `thinking: string`

          - `type: "thinking"`

            - `"thinking"`

        - `RedactedThinkingBlock object { data, type }`

          - `data: string`

          - `type: "redacted_thinking"`

            - `"redacted_thinking"`

        - `ToolUseBlock object { id, caller, input, 2 more }`

          - `id: string`

          - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

            Tool invocation directly from the model.

            - `DirectCaller object { type }`

              Tool invocation directly from the model.

              - `type: "direct"`

                - `"direct"`

            - `ServerToolCaller object { tool_id, type }`

              Tool invocation generated by a server-side tool.

              - `tool_id: string`

              - `type: "code_execution_20250825"`

                - `"code_execution_20250825"`

            - `ServerToolCaller20260120 object { tool_id, type }`

              - `tool_id: string`

              - `type: "code_execution_20260120"`

                - `"code_execution_20260120"`

          - `input: map[unknown]`

          - `name: string`

          - `type: "tool_use"`

            - `"tool_use"`

        - `ServerToolUseBlock object { id, caller, input, 2 more }`

          - `id: string`

          - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

            Tool invocation directly from the model.

            - `DirectCaller object { type }`

              Tool invocation directly from the model.

            - `ServerToolCaller object { tool_id, type }`

              Tool invocation generated by a server-side tool.

            - `ServerToolCaller20260120 object { tool_id, type }`

          - `input: map[unknown]`

          - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

            - `"web_search"`

            - `"web_fetch"`

            - `"code_execution"`

            - `"bash_code_execution"`

            - `"text_editor_code_execution"`

            - `"tool_search_tool_regex"`

            - `"tool_search_tool_bm25"`

          - `type: "server_tool_use"`

            - `"server_tool_use"`

        - `WebSearchToolResultBlock object { caller, content, tool_use_id, type }`

          - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

            Tool invocation directly from the model.

            - `DirectCaller object { type }`

              Tool invocation directly from the model.

            - `ServerToolCaller object { tool_id, type }`

              Tool invocation generated by a server-side tool.

            - `ServerToolCaller20260120 object { tool_id, type }`

          - `content: WebSearchToolResultBlockContent`

            - `WebSearchToolResultError object { error_code, type }`

              - `error_code: WebSearchToolResultErrorCode`

                - `"invalid_tool_input"`

                - `"unavailable"`

                - `"max_uses_exceeded"`

                - `"too_many_requests"`

                - `"query_too_long"`

                - `"request_too_large"`

              - `type: "web_search_tool_result_error"`

                - `"web_search_tool_result_error"`

            - `array of WebSearchResultBlock`

              - `encrypted_content: string`

              - `page_age: string`

              - `title: string`

              - `type: "web_search_result"`

                - `"web_search_result"`

              - `url: string`

          - `tool_use_id: string`

          - `type: "web_search_tool_result"`

            - `"web_search_tool_result"`

        - `WebFetchToolResultBlock object { caller, content, tool_use_id, type }`

          - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

            Tool invocation directly from the model.

            - `DirectCaller object { type }`

              Tool invocation directly from the model.

            - `ServerToolCaller object { tool_id, type }`

              Tool invocation generated by a server-side tool.

            - `ServerToolCaller20260120 object { tool_id, type }`

          - `content: WebFetchToolResultErrorBlock or WebFetchBlock`

            - `WebFetchToolResultErrorBlock object { error_code, type }`

              - `error_code: WebFetchToolResultErrorCode`

                - `"invalid_tool_input"`

                - `"url_too_long"`

                - `"url_not_allowed"`

                - `"url_not_in_prior_context"`

                - `"url_not_accessible"`

                - `"unsupported_content_type"`

                - `"too_many_requests"`

                - `"max_uses_exceeded"`

                - `"unavailable"`

              - `type: "web_fetch_tool_result_error"`

                - `"web_fetch_tool_result_error"`

            - `WebFetchBlock object { content, retrieved_at, type, url }`

              - `content: DocumentBlock`

                - `citations: CitationsConfig`

                  Citation configuration for the document

                  - `enabled: boolean`

                - `source: Base64PDFSource or PlainTextSource`

                  - `Base64PDFSource object { data, media_type, type }`

                    - `data: string`

                    - `media_type: "application/pdf"`

                      - `"application/pdf"`

                    - `type: "base64"`

                      - `"base64"`

                  - `PlainTextSource object { data, media_type, type }`

                    - `data: string`

                    - `media_type: "text/plain"`

                      - `"text/plain"`

                    - `type: "text"`

                      - `"text"`

                - `title: string`

                  The title of the document

                - `type: "document"`

                  - `"document"`

              - `retrieved_at: string`

                ISO 8601 timestamp when the content was retrieved

              - `type: "web_fetch_result"`

                - `"web_fetch_result"`

              - `url: string`

                Fetched content URL

          - `tool_use_id: string`

          - `type: "web_fetch_tool_result"`

            - `"web_fetch_tool_result"`

        - `CodeExecutionToolResultBlock object { content, tool_use_id, type }`

          - `content: CodeExecutionToolResultBlockContent`

            Code execution result with encrypted stdout for PFC + web_search results.

            - `CodeExecutionToolResultError object { error_code, type }`

              - `error_code: CodeExecutionToolResultErrorCode`

                - `"invalid_tool_input"`

                - `"unavailable"`

                - `"too_many_requests"`

                - `"execution_time_exceeded"`

              - `type: "code_execution_tool_result_error"`

                - `"code_execution_tool_result_error"`

            - `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

              - `content: array of CodeExecutionOutputBlock`

                - `file_id: string`

                - `type: "code_execution_output"`

                  - `"code_execution_output"`

              - `return_code: number`

              - `stderr: string`

              - `stdout: string`

              - `type: "code_execution_result"`

                - `"code_execution_result"`

            - `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

              Code execution result with encrypted stdout for PFC + web_search results.

              - `content: array of CodeExecutionOutputBlock`

                - `file_id: string`

                - `type: "code_execution_output"`

              - `encrypted_stdout: string`

              - `return_code: number`

              - `stderr: string`

              - `type: "encrypted_code_execution_result"`

                - `"encrypted_code_execution_result"`

          - `tool_use_id: string`

          - `type: "code_execution_tool_result"`

            - `"code_execution_tool_result"`

        - `BashCodeExecutionToolResultBlock object { content, tool_use_id, type }`

          - `content: BashCodeExecutionToolResultError or BashCodeExecutionResultBlock`

            - `BashCodeExecutionToolResultError object { error_code, type }`

              - `error_code: BashCodeExecutionToolResultErrorCode`

                - `"invalid_tool_input"`

                - `"unavailable"`

                - `"too_many_requests"`

                - `"execution_time_exceeded"`

                - `"output_file_too_large"`

              - `type: "bash_code_execution_tool_result_error"`

                - `"bash_code_execution_tool_result_error"`

            - `BashCodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

              - `content: array of BashCodeExecutionOutputBlock`

                - `file_id: string`

                - `type: "bash_code_execution_output"`

                  - `"bash_code_execution_output"`

              - `return_code: number`

              - `stderr: string`

              - `stdout: string`

              - `type: "bash_code_execution_result"`

                - `"bash_code_execution_result"`

          - `tool_use_id: string`

          - `type: "bash_code_execution_tool_result"`

            - `"bash_code_execution_tool_result"`

        - `TextEditorCodeExecutionToolResultBlock object { content, tool_use_id, type }`

          - `content: TextEditorCodeExecutionToolResultError or TextEditorCodeExecutionViewResultBlock or TextEditorCodeExecutionCreateResultBlock or TextEditorCodeExecutionStrReplaceResultBlock`

            - `TextEditorCodeExecutionToolResultError object { error_code, error_message, type }`

              - `error_code: TextEditorCodeExecutionToolResultErrorCode`

                - `"invalid_tool_input"`

                - `"unavailable"`

                - `"too_many_requests"`

                - `"execution_time_exceeded"`

                - `"file_not_found"`

              - `error_message: string`

              - `type: "text_editor_code_execution_tool_result_error"`

                - `"text_editor_code_execution_tool_result_error"`

            - `TextEditorCodeExecutionViewResultBlock object { content, file_type, num_lines, 3 more }`

              - `content: string`

              - `file_type: "text" or "image" or "pdf"`

                - `"text"`

                - `"image"`

                - `"pdf"`

              - `num_lines: number`

              - `start_line: number`

              - `total_lines: number`

              - `type: "text_editor_code_execution_view_result"`

                - `"text_editor_code_execution_view_result"`

            - `TextEditorCodeExecutionCreateResultBlock object { is_file_update, type }`

              - `is_file_update: boolean`

              - `type: "text_editor_code_execution_create_result"`

                - `"text_editor_code_execution_create_result"`

            - `TextEditorCodeExecutionStrReplaceResultBlock object { lines, new_lines, new_start, 3 more }`

              - `lines: array of string`

              - `new_lines: number`

              - `new_start: number`

              - `old_lines: number`

              - `old_start: number`

              - `type: "text_editor_code_execution_str_replace_result"`

                - `"text_editor_code_execution_str_replace_result"`

          - `tool_use_id: string`

          - `type: "text_editor_code_execution_tool_result"`

            - `"text_editor_code_execution_tool_result"`

        - `ToolSearchToolResultBlock object { content, tool_use_id, type }`

          - `content: ToolSearchToolResultError or ToolSearchToolSearchResultBlock`

            - `ToolSearchToolResultError object { error_code, error_message, type }`

              - `error_code: ToolSearchToolResultErrorCode`

                - `"invalid_tool_input"`

                - `"unavailable"`

                - `"too_many_requests"`

                - `"execution_time_exceeded"`

              - `error_message: string`

              - `type: "tool_search_tool_result_error"`

                - `"tool_search_tool_result_error"`

            - `ToolSearchToolSearchResultBlock object { tool_references, type }`

              - `tool_references: array of ToolReferenceBlock`

                - `tool_name: string`

                - `type: "tool_reference"`

                  - `"tool_reference"`

              - `type: "tool_search_tool_search_result"`

                - `"tool_search_tool_search_result"`

          - `tool_use_id: string`

          - `type: "tool_search_tool_result"`

            - `"tool_search_tool_result"`

        - `ContainerUploadBlock object { file_id, type }`

          Response model for a file uploaded to the container.

          - `file_id: string`

          - `type: "container_upload"`

            - `"container_upload"`

      - `model: Model`

        The model that will complete your prompt.

        See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

        - `"claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more`

          The model that will complete your prompt.

          See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

          - `"claude-sonnet-5"`

            High-performance model for coding and agents

          - `"claude-fable-5"`

            Next generation of intelligence for the hardest knowledge work and coding problems

          - `"claude-mythos-5"`

            Most capable model for cybersecurity and biology research

          - `"claude-opus-4-8"`

            Frontier intelligence for long-running agents and coding

          - `"claude-opus-4-7"`

            Frontier intelligence for long-running agents and coding

          - `"claude-mythos-preview"`

            New class of intelligence, strongest in coding and cybersecurity

          - `"claude-opus-4-6"`

            Frontier intelligence for long-running agents and coding

          - `"claude-sonnet-4-6"`

            Best combination of speed and intelligence

          - `"claude-haiku-4-5"`

            Fastest model with near-frontier intelligence

          - `"claude-haiku-4-5-20251001"`

            Fastest model with near-frontier intelligence

          - `"claude-opus-4-5"`

            Premium model combining maximum intelligence with practical performance

          - `"claude-opus-4-5-20251101"`

            Premium model combining maximum intelligence with practical performance

          - `"claude-sonnet-4-5"`

            High-performance model for agents and coding

          - `"claude-sonnet-4-5-20250929"`

            High-performance model for agents and coding

          - `"claude-opus-4-1"`

            Exceptional model for specialized complex tasks

          - `"claude-opus-4-1-20250805"`

            Exceptional model for specialized complex tasks

        - `string`

      - `role: "assistant"`

        Conversational role of the generated message.

        This will always be `"assistant"`.

        - `"assistant"`

      - `stop_details: RefusalStopDetails`

        Structured information about a refusal.

        - `category: "cyber" or "bio" or "frontier_llm" or "reasoning_extraction"`

          The policy category that triggered a refusal.

          - `"cyber"`

          - `"bio"`

          - `"frontier_llm"`

          - `"reasoning_extraction"`

        - `explanation: string`

          Human-readable explanation of the refusal.

          This text is not guaranteed to be stable. `null` when no explanation is available for the category.

        - `type: "refusal"`

          - `"refusal"`

      - `stop_reason: StopReason`

        The reason that we stopped.

        This may be one the following values:

        * `"end_turn"`: the model reached a natural stopping point
        * `"max_tokens"`: we exceeded the requested `max_tokens` or the model's maximum
        * `"stop_sequence"`: one of your provided custom `stop_sequences` was generated
        * `"tool_use"`: the model invoked one or more tools
        * `"pause_turn"`: we paused a long-running turn. You may provide the response back as-is in a subsequent request to let the model continue.
        * `"refusal"`: when streaming classifiers intervene to handle potential policy violations

        In non-streaming mode this value is always non-null. In streaming mode, it is null in the `message_start` event and non-null otherwise.

        - `"end_turn"`

        - `"max_tokens"`

        - `"stop_sequence"`

        - `"tool_use"`

        - `"pause_turn"`

        - `"refusal"`

      - `stop_sequence: string`

        Which custom stop sequence was generated, if any.

        This value will be a non-null string if one of your custom stop sequences was generated.

      - `type: "message"`

        Object type.

        For Messages, this is always `"message"`.

        - `"message"`

      - `usage: Usage`

        Billing and rate-limit usage.

        Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems.

        Under the hood, the API transforms requests into a format suitable for the model. The model's output then goes through a parsing stage before becoming an API response. As a result, the token counts in `usage` will not match one-to-one with the exact visible content of an API request or response.

        For example, `output_tokens` will be non-zero, even for an empty string response from Claude.

        Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.

        - `cache_creation: CacheCreation`

          Breakdown of cached tokens by TTL

          - `ephemeral_1h_input_tokens: number`

            The number of input tokens used to create the 1 hour cache entry.

          - `ephemeral_5m_input_tokens: number`

            The number of input tokens used to create the 5 minute cache entry.

        - `cache_creation_input_tokens: number`

          The number of input tokens used to create the cache entry.

        - `cache_read_input_tokens: number`

          The number of input tokens read from the cache.

        - `inference_geo: string`

          The geographic region where inference was performed for this request.

        - `input_tokens: number`

          The number of input tokens which were used.

        - `output_tokens: number`

          The number of output tokens which were used.

        - `output_tokens_details: OutputTokensDetails`

          Breakdown of output tokens by category.

          `output_tokens` remains the inclusive, authoritative total used for billing.
          This object provides a read-only decomposition for observability — for example,
          how many of the billed output tokens were spent on internal reasoning that may
          have been summarized before being returned to you.

          - `thinking_tokens: number`

            Number of output tokens the model generated as internal reasoning, including
            the thinking-block delimiter tokens.

            Reflects the raw reasoning the model produced, not the (possibly shorter)
            summarized thinking text returned in the response body. Computed by
            re-tokenizing the raw reasoning text, so it may differ from the model's exact
            generation count by a small number of tokens. Always ≤ `output_tokens`;
            `output_tokens - thinking_tokens` approximates the non-reasoning output.

        - `server_tool_use: ServerToolUsage`

          The number of server tool requests.

          - `web_fetch_requests: number`

            The number of web fetch tool requests.

          - `web_search_requests: number`

            The number of web search tool requests.

        - `service_tier: "standard" or "priority" or "batch"`

          If the request used the priority, standard, or batch tier.

          - `"standard"`

          - `"priority"`

          - `"batch"`

    - `type: "message_start"`

      - `"message_start"`

  - `RawMessageDeltaEvent object { delta, type, usage }`

    - `delta: object { container, stop_details, stop_reason, stop_sequence }`

      - `container: Container`

        Information about the container used in the request (for the code execution tool)

      - `stop_details: RefusalStopDetails`

        Structured information about a refusal.

      - `stop_reason: StopReason`

      - `stop_sequence: string`

    - `type: "message_delta"`

      - `"message_delta"`

    - `usage: MessageDeltaUsage`

      Billing and rate-limit usage.

      Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems.

      Under the hood, the API transforms requests into a format suitable for the model. The model's output then goes through a parsing stage before becoming an API response. As a result, the token counts in `usage` will not match one-to-one with the exact visible content of an API request or response.

      For example, `output_tokens` will be non-zero, even for an empty string response from Claude.

      Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.

      - `cache_creation_input_tokens: number`

        The cumulative number of input tokens used to create the cache entry.

      - `cache_read_input_tokens: number`

        The cumulative number of input tokens read from the cache.

      - `input_tokens: number`

        The cumulative number of input tokens which were used.

      - `output_tokens: number`

        The cumulative number of output tokens which were used.

      - `output_tokens_details: OutputTokensDetails`

        Breakdown of output tokens by category.

        `output_tokens` remains the inclusive, authoritative total used for billing.
        This object provides a read-only decomposition for observability — for example,
        how many of the billed output tokens were spent on internal reasoning that may
        have been summarized before being returned to you.

      - `server_tool_use: ServerToolUsage`

        The number of server tool requests.

  - `RawMessageStopEvent object { type }`

    - `type: "message_stop"`

      - `"message_stop"`

  - `RawContentBlockStartEvent object { content_block, index, type }`

    - `content_block: TextBlock or ThinkingBlock or RedactedThinkingBlock or 9 more`

      Response model for a file uploaded to the container.

      - `TextBlock object { citations, text, type }`

      - `ThinkingBlock object { signature, thinking, type }`

      - `RedactedThinkingBlock object { data, type }`

      - `ToolUseBlock object { id, caller, input, 2 more }`

      - `ServerToolUseBlock object { id, caller, input, 2 more }`

      - `WebSearchToolResultBlock object { caller, content, tool_use_id, type }`

      - `WebFetchToolResultBlock object { caller, content, tool_use_id, type }`

      - `CodeExecutionToolResultBlock object { content, tool_use_id, type }`

      - `BashCodeExecutionToolResultBlock object { content, tool_use_id, type }`

      - `TextEditorCodeExecutionToolResultBlock object { content, tool_use_id, type }`

      - `ToolSearchToolResultBlock object { content, tool_use_id, type }`

      - `ContainerUploadBlock object { file_id, type }`

        Response model for a file uploaded to the container.

    - `index: number`

    - `type: "content_block_start"`

      - `"content_block_start"`

  - `RawContentBlockDeltaEvent object { delta, index, type }`

    - `delta: RawContentBlockDelta`

      - `TextDelta object { text, type }`

        - `text: string`

        - `type: "text_delta"`

          - `"text_delta"`

      - `InputJSONDelta object { partial_json, type }`

        - `partial_json: string`

        - `type: "input_json_delta"`

          - `"input_json_delta"`

      - `CitationsDelta object { citation, type }`

        - `citation: CitationCharLocation or CitationPageLocation or CitationContentBlockLocation or 2 more`

          - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

          - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

          - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

          - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

          - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

        - `type: "citations_delta"`

          - `"citations_delta"`

      - `ThinkingDelta object { thinking, type }`

        - `thinking: string`

        - `type: "thinking_delta"`

          - `"thinking_delta"`

      - `SignatureDelta object { signature, type }`

        - `signature: string`

        - `type: "signature_delta"`

          - `"signature_delta"`

    - `index: number`

    - `type: "content_block_delta"`

      - `"content_block_delta"`

  - `RawContentBlockStopEvent object { index, type }`

    - `index: number`

    - `type: "content_block_stop"`

      - `"content_block_stop"`

### Redacted Thinking Block

- `RedactedThinkingBlock object { data, type }`

  - `data: string`

  - `type: "redacted_thinking"`

    - `"redacted_thinking"`

### Redacted Thinking Block Param

- `RedactedThinkingBlockParam object { data, type }`

  - `data: string`

  - `type: "redacted_thinking"`

    - `"redacted_thinking"`

### Refusal Stop Details

- `RefusalStopDetails object { category, explanation, type }`

  Structured information about a refusal.

  - `category: "cyber" or "bio" or "frontier_llm" or "reasoning_extraction"`

    The policy category that triggered a refusal.

    - `"cyber"`

    - `"bio"`

    - `"frontier_llm"`

    - `"reasoning_extraction"`

  - `explanation: string`

    Human-readable explanation of the refusal.

    This text is not guaranteed to be stable. `null` when no explanation is available for the category.

  - `type: "refusal"`

    - `"refusal"`

### Search Result Block Param

- `SearchResultBlockParam object { content, source, title, 3 more }`

  - `content: array of TextBlockParam`

    - `text: string`

    - `type: "text"`

      - `"text"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

      - `type: "ephemeral"`

        - `"ephemeral"`

      - `ttl: optional "5m" or "1h"`

        The time-to-live for the cache control breakpoint.

        This may be one the following values:

        - `5m`: 5 minutes
        - `1h`: 1 hour

        Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

        - `"5m"`

        - `"1h"`

    - `citations: optional array of TextCitationParam`

      - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

        - `cited_text: string`

        - `document_index: number`

        - `document_title: string`

        - `end_char_index: number`

        - `start_char_index: number`

        - `type: "char_location"`

          - `"char_location"`

      - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

        - `cited_text: string`

        - `document_index: number`

        - `document_title: string`

        - `end_page_number: number`

        - `start_page_number: number`

        - `type: "page_location"`

          - `"page_location"`

      - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

        - `cited_text: string`

          The full text of the cited block range, concatenated.

          Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

        - `document_index: number`

        - `document_title: string`

        - `end_block_index: number`

          Exclusive 0-based end index of the cited block range in the source's `content` array.

          Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

        - `start_block_index: number`

          0-based index of the first cited block in the source's `content` array.

        - `type: "content_block_location"`

          - `"content_block_location"`

      - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

        - `cited_text: string`

        - `encrypted_index: string`

        - `title: string`

        - `type: "web_search_result_location"`

          - `"web_search_result_location"`

        - `url: string`

      - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

        - `cited_text: string`

          The full text of the cited block range, concatenated.

          Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

        - `end_block_index: number`

          Exclusive 0-based end index of the cited block range in the source's `content` array.

          Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

        - `search_result_index: number`

          0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

          Counted separately from `document_index`; server-side web search results are not included in this count.

        - `source: string`

        - `start_block_index: number`

          0-based index of the first cited block in the source's `content` array.

        - `title: string`

        - `type: "search_result_location"`

          - `"search_result_location"`

  - `source: string`

  - `title: string`

  - `type: "search_result"`

    - `"search_result"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

  - `citations: optional CitationsConfigParam`

    - `enabled: optional boolean`

### Server Tool Caller

- `ServerToolCaller object { tool_id, type }`

  Tool invocation generated by a server-side tool.

  - `tool_id: string`

  - `type: "code_execution_20250825"`

    - `"code_execution_20250825"`

### Server Tool Caller 20260120

- `ServerToolCaller20260120 object { tool_id, type }`

  - `tool_id: string`

  - `type: "code_execution_20260120"`

    - `"code_execution_20260120"`

### Server Tool Usage

- `ServerToolUsage object { web_fetch_requests, web_search_requests }`

  - `web_fetch_requests: number`

    The number of web fetch tool requests.

  - `web_search_requests: number`

    The number of web search tool requests.

### Server Tool Use Block

- `ServerToolUseBlock object { id, caller, input, 2 more }`

  - `id: string`

  - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

    Tool invocation directly from the model.

    - `DirectCaller object { type }`

      Tool invocation directly from the model.

      - `type: "direct"`

        - `"direct"`

    - `ServerToolCaller object { tool_id, type }`

      Tool invocation generated by a server-side tool.

      - `tool_id: string`

      - `type: "code_execution_20250825"`

        - `"code_execution_20250825"`

    - `ServerToolCaller20260120 object { tool_id, type }`

      - `tool_id: string`

      - `type: "code_execution_20260120"`

        - `"code_execution_20260120"`

  - `input: map[unknown]`

  - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

    - `"web_search"`

    - `"web_fetch"`

    - `"code_execution"`

    - `"bash_code_execution"`

    - `"text_editor_code_execution"`

    - `"tool_search_tool_regex"`

    - `"tool_search_tool_bm25"`

  - `type: "server_tool_use"`

    - `"server_tool_use"`

### Server Tool Use Block Param

- `ServerToolUseBlockParam object { id, input, name, 3 more }`

  - `id: string`

  - `input: map[unknown]`

  - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

    - `"web_search"`

    - `"web_fetch"`

    - `"code_execution"`

    - `"bash_code_execution"`

    - `"text_editor_code_execution"`

    - `"tool_search_tool_regex"`

    - `"tool_search_tool_bm25"`

  - `type: "server_tool_use"`

    - `"server_tool_use"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

    Tool invocation directly from the model.

    - `DirectCaller object { type }`

      Tool invocation directly from the model.

      - `type: "direct"`

        - `"direct"`

    - `ServerToolCaller object { tool_id, type }`

      Tool invocation generated by a server-side tool.

      - `tool_id: string`

      - `type: "code_execution_20250825"`

        - `"code_execution_20250825"`

    - `ServerToolCaller20260120 object { tool_id, type }`

      - `tool_id: string`

      - `type: "code_execution_20260120"`

        - `"code_execution_20260120"`

### Signature Delta

- `SignatureDelta object { signature, type }`

  - `signature: string`

  - `type: "signature_delta"`

    - `"signature_delta"`

### Stop Reason

- `StopReason = "end_turn" or "max_tokens" or "stop_sequence" or 3 more`

  - `"end_turn"`

  - `"max_tokens"`

  - `"stop_sequence"`

  - `"tool_use"`

  - `"pause_turn"`

  - `"refusal"`

### Text Block

- `TextBlock object { citations, text, type }`

  - `citations: array of TextCitation`

    Citations supporting the text block.

    The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.

    - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

      - `cited_text: string`

      - `document_index: number`

      - `document_title: string`

      - `end_char_index: number`

      - `file_id: string`

      - `start_char_index: number`

      - `type: "char_location"`

        - `"char_location"`

    - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

      - `cited_text: string`

      - `document_index: number`

      - `document_title: string`

      - `end_page_number: number`

      - `file_id: string`

      - `start_page_number: number`

      - `type: "page_location"`

        - `"page_location"`

    - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

      - `cited_text: string`

        The full text of the cited block range, concatenated.

        Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

      - `document_index: number`

      - `document_title: string`

      - `end_block_index: number`

        Exclusive 0-based end index of the cited block range in the source's `content` array.

        Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

      - `file_id: string`

      - `start_block_index: number`

        0-based index of the first cited block in the source's `content` array.

      - `type: "content_block_location"`

        - `"content_block_location"`

    - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

      - `cited_text: string`

      - `encrypted_index: string`

      - `title: string`

      - `type: "web_search_result_location"`

        - `"web_search_result_location"`

      - `url: string`

    - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

      - `cited_text: string`

        The full text of the cited block range, concatenated.

        Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

      - `end_block_index: number`

        Exclusive 0-based end index of the cited block range in the source's `content` array.

        Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

      - `search_result_index: number`

        0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

        Counted separately from `document_index`; server-side web search results are not included in this count.

      - `source: string`

      - `start_block_index: number`

        0-based index of the first cited block in the source's `content` array.

      - `title: string`

      - `type: "search_result_location"`

        - `"search_result_location"`

  - `text: string`

  - `type: "text"`

    - `"text"`

### Text Block Param

- `TextBlockParam object { text, type, cache_control, citations }`

  - `text: string`

  - `type: "text"`

    - `"text"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `citations: optional array of TextCitationParam`

    - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

      - `cited_text: string`

      - `document_index: number`

      - `document_title: string`

      - `end_char_index: number`

      - `start_char_index: number`

      - `type: "char_location"`

        - `"char_location"`

    - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

      - `cited_text: string`

      - `document_index: number`

      - `document_title: string`

      - `end_page_number: number`

      - `start_page_number: number`

      - `type: "page_location"`

        - `"page_location"`

    - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

      - `cited_text: string`

        The full text of the cited block range, concatenated.

        Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

      - `document_index: number`

      - `document_title: string`

      - `end_block_index: number`

        Exclusive 0-based end index of the cited block range in the source's `content` array.

        Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

      - `start_block_index: number`

        0-based index of the first cited block in the source's `content` array.

      - `type: "content_block_location"`

        - `"content_block_location"`

    - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

      - `cited_text: string`

      - `encrypted_index: string`

      - `title: string`

      - `type: "web_search_result_location"`

        - `"web_search_result_location"`

      - `url: string`

    - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

      - `cited_text: string`

        The full text of the cited block range, concatenated.

        Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

      - `end_block_index: number`

        Exclusive 0-based end index of the cited block range in the source's `content` array.

        Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

      - `search_result_index: number`

        0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

        Counted separately from `document_index`; server-side web search results are not included in this count.

      - `source: string`

      - `start_block_index: number`

        0-based index of the first cited block in the source's `content` array.

      - `title: string`

      - `type: "search_result_location"`

        - `"search_result_location"`

### Text Citation

- `TextCitation = CitationCharLocation or CitationPageLocation or CitationContentBlockLocation or 2 more`

  - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

    - `cited_text: string`

    - `document_index: number`

    - `document_title: string`

    - `end_char_index: number`

    - `file_id: string`

    - `start_char_index: number`

    - `type: "char_location"`

      - `"char_location"`

  - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

    - `cited_text: string`

    - `document_index: number`

    - `document_title: string`

    - `end_page_number: number`

    - `file_id: string`

    - `start_page_number: number`

    - `type: "page_location"`

      - `"page_location"`

  - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

    - `cited_text: string`

      The full text of the cited block range, concatenated.

      Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

    - `document_index: number`

    - `document_title: string`

    - `end_block_index: number`

      Exclusive 0-based end index of the cited block range in the source's `content` array.

      Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

    - `file_id: string`

    - `start_block_index: number`

      0-based index of the first cited block in the source's `content` array.

    - `type: "content_block_location"`

      - `"content_block_location"`

  - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

    - `cited_text: string`

    - `encrypted_index: string`

    - `title: string`

    - `type: "web_search_result_location"`

      - `"web_search_result_location"`

    - `url: string`

  - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

    - `cited_text: string`

      The full text of the cited block range, concatenated.

      Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

    - `end_block_index: number`

      Exclusive 0-based end index of the cited block range in the source's `content` array.

      Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

    - `search_result_index: number`

      0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

      Counted separately from `document_index`; server-side web search results are not included in this count.

    - `source: string`

    - `start_block_index: number`

      0-based index of the first cited block in the source's `content` array.

    - `title: string`

    - `type: "search_result_location"`

      - `"search_result_location"`

### Text Citation Param

- `TextCitationParam = CitationCharLocationParam or CitationPageLocationParam or CitationContentBlockLocationParam or 2 more`

  - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

    - `cited_text: string`

    - `document_index: number`

    - `document_title: string`

    - `end_char_index: number`

    - `start_char_index: number`

    - `type: "char_location"`

      - `"char_location"`

  - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

    - `cited_text: string`

    - `document_index: number`

    - `document_title: string`

    - `end_page_number: number`

    - `start_page_number: number`

    - `type: "page_location"`

      - `"page_location"`

  - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

    - `cited_text: string`

      The full text of the cited block range, concatenated.

      Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

    - `document_index: number`

    - `document_title: string`

    - `end_block_index: number`

      Exclusive 0-based end index of the cited block range in the source's `content` array.

      Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

    - `start_block_index: number`

      0-based index of the first cited block in the source's `content` array.

    - `type: "content_block_location"`

      - `"content_block_location"`

  - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

    - `cited_text: string`

    - `encrypted_index: string`

    - `title: string`

    - `type: "web_search_result_location"`

      - `"web_search_result_location"`

    - `url: string`

  - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

    - `cited_text: string`

      The full text of the cited block range, concatenated.

      Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

    - `end_block_index: number`

      Exclusive 0-based end index of the cited block range in the source's `content` array.

      Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

    - `search_result_index: number`

      0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

      Counted separately from `document_index`; server-side web search results are not included in this count.

    - `source: string`

    - `start_block_index: number`

      0-based index of the first cited block in the source's `content` array.

    - `title: string`

    - `type: "search_result_location"`

      - `"search_result_location"`

### Text Delta

- `TextDelta object { text, type }`

  - `text: string`

  - `type: "text_delta"`

    - `"text_delta"`

### Text Editor Code Execution Create Result Block

- `TextEditorCodeExecutionCreateResultBlock object { is_file_update, type }`

  - `is_file_update: boolean`

  - `type: "text_editor_code_execution_create_result"`

    - `"text_editor_code_execution_create_result"`

### Text Editor Code Execution Create Result Block Param

- `TextEditorCodeExecutionCreateResultBlockParam object { is_file_update, type }`

  - `is_file_update: boolean`

  - `type: "text_editor_code_execution_create_result"`

    - `"text_editor_code_execution_create_result"`

### Text Editor Code Execution Str Replace Result Block

- `TextEditorCodeExecutionStrReplaceResultBlock object { lines, new_lines, new_start, 3 more }`

  - `lines: array of string`

  - `new_lines: number`

  - `new_start: number`

  - `old_lines: number`

  - `old_start: number`

  - `type: "text_editor_code_execution_str_replace_result"`

    - `"text_editor_code_execution_str_replace_result"`

### Text Editor Code Execution Str Replace Result Block Param

- `TextEditorCodeExecutionStrReplaceResultBlockParam object { type, lines, new_lines, 3 more }`

  - `type: "text_editor_code_execution_str_replace_result"`

    - `"text_editor_code_execution_str_replace_result"`

  - `lines: optional array of string`

  - `new_lines: optional number`

  - `new_start: optional number`

  - `old_lines: optional number`

  - `old_start: optional number`

### Text Editor Code Execution Tool Result Block

- `TextEditorCodeExecutionToolResultBlock object { content, tool_use_id, type }`

  - `content: TextEditorCodeExecutionToolResultError or TextEditorCodeExecutionViewResultBlock or TextEditorCodeExecutionCreateResultBlock or TextEditorCodeExecutionStrReplaceResultBlock`

    - `TextEditorCodeExecutionToolResultError object { error_code, error_message, type }`

      - `error_code: TextEditorCodeExecutionToolResultErrorCode`

        - `"invalid_tool_input"`

        - `"unavailable"`

        - `"too_many_requests"`

        - `"execution_time_exceeded"`

        - `"file_not_found"`

      - `error_message: string`

      - `type: "text_editor_code_execution_tool_result_error"`

        - `"text_editor_code_execution_tool_result_error"`

    - `TextEditorCodeExecutionViewResultBlock object { content, file_type, num_lines, 3 more }`

      - `content: string`

      - `file_type: "text" or "image" or "pdf"`

        - `"text"`

        - `"image"`

        - `"pdf"`

      - `num_lines: number`

      - `start_line: number`

      - `total_lines: number`

      - `type: "text_editor_code_execution_view_result"`

        - `"text_editor_code_execution_view_result"`

    - `TextEditorCodeExecutionCreateResultBlock object { is_file_update, type }`

      - `is_file_update: boolean`

      - `type: "text_editor_code_execution_create_result"`

        - `"text_editor_code_execution_create_result"`

    - `TextEditorCodeExecutionStrReplaceResultBlock object { lines, new_lines, new_start, 3 more }`

      - `lines: array of string`

      - `new_lines: number`

      - `new_start: number`

      - `old_lines: number`

      - `old_start: number`

      - `type: "text_editor_code_execution_str_replace_result"`

        - `"text_editor_code_execution_str_replace_result"`

  - `tool_use_id: string`

  - `type: "text_editor_code_execution_tool_result"`

    - `"text_editor_code_execution_tool_result"`

### Text Editor Code Execution Tool Result Block Param

- `TextEditorCodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

  - `content: TextEditorCodeExecutionToolResultErrorParam or TextEditorCodeExecutionViewResultBlockParam or TextEditorCodeExecutionCreateResultBlockParam or TextEditorCodeExecutionStrReplaceResultBlockParam`

    - `TextEditorCodeExecutionToolResultErrorParam object { error_code, type, error_message }`

      - `error_code: TextEditorCodeExecutionToolResultErrorCode`

        - `"invalid_tool_input"`

        - `"unavailable"`

        - `"too_many_requests"`

        - `"execution_time_exceeded"`

        - `"file_not_found"`

      - `type: "text_editor_code_execution_tool_result_error"`

        - `"text_editor_code_execution_tool_result_error"`

      - `error_message: optional string`

    - `TextEditorCodeExecutionViewResultBlockParam object { content, file_type, type, 3 more }`

      - `content: string`

      - `file_type: "text" or "image" or "pdf"`

        - `"text"`

        - `"image"`

        - `"pdf"`

      - `type: "text_editor_code_execution_view_result"`

        - `"text_editor_code_execution_view_result"`

      - `num_lines: optional number`

      - `start_line: optional number`

      - `total_lines: optional number`

    - `TextEditorCodeExecutionCreateResultBlockParam object { is_file_update, type }`

      - `is_file_update: boolean`

      - `type: "text_editor_code_execution_create_result"`

        - `"text_editor_code_execution_create_result"`

    - `TextEditorCodeExecutionStrReplaceResultBlockParam object { type, lines, new_lines, 3 more }`

      - `type: "text_editor_code_execution_str_replace_result"`

        - `"text_editor_code_execution_str_replace_result"`

      - `lines: optional array of string`

      - `new_lines: optional number`

      - `new_start: optional number`

      - `old_lines: optional number`

      - `old_start: optional number`

  - `tool_use_id: string`

  - `type: "text_editor_code_execution_tool_result"`

    - `"text_editor_code_execution_tool_result"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

### Text Editor Code Execution Tool Result Error

- `TextEditorCodeExecutionToolResultError object { error_code, error_message, type }`

  - `error_code: TextEditorCodeExecutionToolResultErrorCode`

    - `"invalid_tool_input"`

    - `"unavailable"`

    - `"too_many_requests"`

    - `"execution_time_exceeded"`

    - `"file_not_found"`

  - `error_message: string`

  - `type: "text_editor_code_execution_tool_result_error"`

    - `"text_editor_code_execution_tool_result_error"`

### Text Editor Code Execution Tool Result Error Code

- `TextEditorCodeExecutionToolResultErrorCode = "invalid_tool_input" or "unavailable" or "too_many_requests" or 2 more`

  - `"invalid_tool_input"`

  - `"unavailable"`

  - `"too_many_requests"`

  - `"execution_time_exceeded"`

  - `"file_not_found"`

### Text Editor Code Execution Tool Result Error Param

- `TextEditorCodeExecutionToolResultErrorParam object { error_code, type, error_message }`

  - `error_code: TextEditorCodeExecutionToolResultErrorCode`

    - `"invalid_tool_input"`

    - `"unavailable"`

    - `"too_many_requests"`

    - `"execution_time_exceeded"`

    - `"file_not_found"`

  - `type: "text_editor_code_execution_tool_result_error"`

    - `"text_editor_code_execution_tool_result_error"`

  - `error_message: optional string`

### Text Editor Code Execution View Result Block

- `TextEditorCodeExecutionViewResultBlock object { content, file_type, num_lines, 3 more }`

  - `content: string`

  - `file_type: "text" or "image" or "pdf"`

    - `"text"`

    - `"image"`

    - `"pdf"`

  - `num_lines: number`

  - `start_line: number`

  - `total_lines: number`

  - `type: "text_editor_code_execution_view_result"`

    - `"text_editor_code_execution_view_result"`

### Text Editor Code Execution View Result Block Param

- `TextEditorCodeExecutionViewResultBlockParam object { content, file_type, type, 3 more }`

  - `content: string`

  - `file_type: "text" or "image" or "pdf"`

    - `"text"`

    - `"image"`

    - `"pdf"`

  - `type: "text_editor_code_execution_view_result"`

    - `"text_editor_code_execution_view_result"`

  - `num_lines: optional number`

  - `start_line: optional number`

  - `total_lines: optional number`

### Thinking Block

- `ThinkingBlock object { signature, thinking, type }`

  - `signature: string`

  - `thinking: string`

  - `type: "thinking"`

    - `"thinking"`

### Thinking Block Param

- `ThinkingBlockParam object { signature, thinking, type }`

  - `signature: string`

  - `thinking: string`

  - `type: "thinking"`

    - `"thinking"`

### Thinking Config Adaptive

- `ThinkingConfigAdaptive object { type, display }`

  - `type: "adaptive"`

    - `"adaptive"`

  - `display: optional "summarized" or "omitted"`

    Controls how thinking content appears in the response. When set to `summarized`, thinking is returned normally. When set to `omitted`, thinking content is redacted but a signature is returned for multi-turn continuity. Defaults to `summarized`.

    - `"summarized"`

    - `"omitted"`

### Thinking Config Disabled

- `ThinkingConfigDisabled object { type }`

  - `type: "disabled"`

    - `"disabled"`

### Thinking Config Enabled

- `ThinkingConfigEnabled object { budget_tokens, type, display }`

  - `budget_tokens: number`

    Determines how many tokens Claude can use for its internal reasoning process. Larger budgets can enable more thorough analysis for complex problems, improving response quality.

    Must be ≥1024 and less than `max_tokens`.

    See [extended thinking](https://platform.claude.com/docs/en/build-with-claude/extended-thinking) for details.

  - `type: "enabled"`

    - `"enabled"`

  - `display: optional "summarized" or "omitted"`

    Controls how thinking content appears in the response. When set to `summarized`, thinking is returned normally. When set to `omitted`, thinking content is redacted but a signature is returned for multi-turn continuity. Defaults to `summarized`.

    - `"summarized"`

    - `"omitted"`

### Thinking Config Param

- `ThinkingConfigParam = ThinkingConfigEnabled or ThinkingConfigDisabled or ThinkingConfigAdaptive`

  Configuration for enabling Claude's extended thinking.

  When enabled, responses include `thinking` content blocks showing Claude's thinking process before the final answer. Requires a minimum budget of 1,024 tokens and counts towards your `max_tokens` limit.

  See [extended thinking](https://platform.claude.com/docs/en/build-with-claude/extended-thinking) for details.

  - `ThinkingConfigEnabled object { budget_tokens, type, display }`

    - `budget_tokens: number`

      Determines how many tokens Claude can use for its internal reasoning process. Larger budgets can enable more thorough analysis for complex problems, improving response quality.

      Must be ≥1024 and less than `max_tokens`.

      See [extended thinking](https://platform.claude.com/docs/en/build-with-claude/extended-thinking) for details.

    - `type: "enabled"`

      - `"enabled"`

    - `display: optional "summarized" or "omitted"`

      Controls how thinking content appears in the response. When set to `summarized`, thinking is returned normally. When set to `omitted`, thinking content is redacted but a signature is returned for multi-turn continuity. Defaults to `summarized`.

      - `"summarized"`

      - `"omitted"`

  - `ThinkingConfigDisabled object { type }`

    - `type: "disabled"`

      - `"disabled"`

  - `ThinkingConfigAdaptive object { type, display }`

    - `type: "adaptive"`

      - `"adaptive"`

    - `display: optional "summarized" or "omitted"`

      Controls how thinking content appears in the response. When set to `summarized`, thinking is returned normally. When set to `omitted`, thinking content is redacted but a signature is returned for multi-turn continuity. Defaults to `summarized`.

      - `"summarized"`

      - `"omitted"`

### Thinking Delta

- `ThinkingDelta object { thinking, type }`

  - `thinking: string`

  - `type: "thinking_delta"`

    - `"thinking_delta"`

### Tool

- `Tool object { input_schema, name, allowed_callers, 7 more }`

  - `input_schema: object { type, properties, required }`

    [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.

    This defines the shape of the `input` that your tool accepts and that the model will produce.

    - `type: "object"`

      - `"object"`

    - `properties: optional map[unknown]`

    - `required: optional array of string`

  - `name: string`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `description: optional string`

    Description of what this tool does.

    Tool descriptions should be as detailed as possible. The more information that the model has about what the tool is and how to use it, the better it will perform. You can use natural language descriptions to reinforce important aspects of the tool input JSON schema.

  - `eager_input_streaming: optional boolean`

    Enable eager input streaming for this tool. When true, tool input parameters will be streamed incrementally as they are generated, and types will be inferred on-the-fly rather than buffering the full JSON output. When false, streaming is disabled for this tool even if the fine-grained-tool-streaming beta is active. When null (default), uses the default behavior based on beta headers.

  - `input_examples: optional array of map[unknown]`

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

  - `type: optional "custom"`

    - `"custom"`

### Tool Bash 20250124

- `ToolBash20250124 object { name, type, allowed_callers, 4 more }`

  - `name: "bash"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"bash"`

  - `type: "bash_20250124"`

    - `"bash_20250124"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `input_examples: optional array of map[unknown]`

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Tool Choice

- `ToolChoice = ToolChoiceAuto or ToolChoiceAny or ToolChoiceTool or ToolChoiceNone`

  How the model should use the provided tools. The model can use a specific tool, any available tool, decide by itself, or not use tools at all.

  - `ToolChoiceAuto object { type, disable_parallel_tool_use }`

    The model will automatically decide whether to use tools.

    - `type: "auto"`

      - `"auto"`

    - `disable_parallel_tool_use: optional boolean`

      Whether to disable parallel tool use.

      Defaults to `false`. If set to `true`, the model will output at most one tool use.

  - `ToolChoiceAny object { type, disable_parallel_tool_use }`

    The model will use any available tools.

    - `type: "any"`

      - `"any"`

    - `disable_parallel_tool_use: optional boolean`

      Whether to disable parallel tool use.

      Defaults to `false`. If set to `true`, the model will output exactly one tool use.

  - `ToolChoiceTool object { name, type, disable_parallel_tool_use }`

    The model will use the specified tool with `tool_choice.name`.

    - `name: string`

      The name of the tool to use.

    - `type: "tool"`

      - `"tool"`

    - `disable_parallel_tool_use: optional boolean`

      Whether to disable parallel tool use.

      Defaults to `false`. If set to `true`, the model will output exactly one tool use.

  - `ToolChoiceNone object { type }`

    The model will not be allowed to use tools.

    - `type: "none"`

      - `"none"`

### Tool Choice Any

- `ToolChoiceAny object { type, disable_parallel_tool_use }`

  The model will use any available tools.

  - `type: "any"`

    - `"any"`

  - `disable_parallel_tool_use: optional boolean`

    Whether to disable parallel tool use.

    Defaults to `false`. If set to `true`, the model will output exactly one tool use.

### Tool Choice Auto

- `ToolChoiceAuto object { type, disable_parallel_tool_use }`

  The model will automatically decide whether to use tools.

  - `type: "auto"`

    - `"auto"`

  - `disable_parallel_tool_use: optional boolean`

    Whether to disable parallel tool use.

    Defaults to `false`. If set to `true`, the model will output at most one tool use.

### Tool Choice None

- `ToolChoiceNone object { type }`

  The model will not be allowed to use tools.

  - `type: "none"`

    - `"none"`

### Tool Choice Tool

- `ToolChoiceTool object { name, type, disable_parallel_tool_use }`

  The model will use the specified tool with `tool_choice.name`.

  - `name: string`

    The name of the tool to use.

  - `type: "tool"`

    - `"tool"`

  - `disable_parallel_tool_use: optional boolean`

    Whether to disable parallel tool use.

    Defaults to `false`. If set to `true`, the model will output exactly one tool use.

### Tool Reference Block

- `ToolReferenceBlock object { tool_name, type }`

  - `tool_name: string`

  - `type: "tool_reference"`

    - `"tool_reference"`

### Tool Reference Block Param

- `ToolReferenceBlockParam object { tool_name, type, cache_control }`

  Tool reference block that can be included in tool_result content.

  - `tool_name: string`

  - `type: "tool_reference"`

    - `"tool_reference"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

### Tool Result Block Param

- `ToolResultBlockParam object { tool_use_id, type, cache_control, 2 more }`

  - `tool_use_id: string`

  - `type: "tool_result"`

    - `"tool_result"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `content: optional string or array of TextBlockParam or ImageBlockParam or SearchResultBlockParam or 2 more`

    - `string`

    - `array of TextBlockParam or ImageBlockParam or SearchResultBlockParam or 2 more`

      - `TextBlockParam object { text, type, cache_control, citations }`

        - `text: string`

        - `type: "text"`

          - `"text"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional array of TextCitationParam`

          - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_char_index: number`

            - `start_char_index: number`

            - `type: "char_location"`

              - `"char_location"`

          - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_page_number: number`

            - `start_page_number: number`

            - `type: "page_location"`

              - `"page_location"`

          - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `document_index: number`

            - `document_title: string`

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `type: "content_block_location"`

              - `"content_block_location"`

          - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

            - `cited_text: string`

            - `encrypted_index: string`

            - `title: string`

            - `type: "web_search_result_location"`

              - `"web_search_result_location"`

            - `url: string`

          - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `search_result_index: number`

              0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

              Counted separately from `document_index`; server-side web search results are not included in this count.

            - `source: string`

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `title: string`

            - `type: "search_result_location"`

              - `"search_result_location"`

      - `ImageBlockParam object { source, type, cache_control }`

        - `source: Base64ImageSource or URLImageSource`

          - `Base64ImageSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

              - `"image/jpeg"`

              - `"image/png"`

              - `"image/gif"`

              - `"image/webp"`

            - `type: "base64"`

              - `"base64"`

          - `URLImageSource object { type, url }`

            - `type: "url"`

              - `"url"`

            - `url: string`

        - `type: "image"`

          - `"image"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

      - `SearchResultBlockParam object { content, source, title, 3 more }`

        - `content: array of TextBlockParam`

          - `text: string`

          - `type: "text"`

          - `cache_control: optional CacheControlEphemeral`

            Create a cache control breakpoint at this content block.

          - `citations: optional array of TextCitationParam`

        - `source: string`

        - `title: string`

        - `type: "search_result"`

          - `"search_result"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

          - `enabled: optional boolean`

      - `DocumentBlockParam object { source, type, cache_control, 3 more }`

        - `source: Base64PDFSource or PlainTextSource or ContentBlockSource or URLPDFSource`

          - `Base64PDFSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "application/pdf"`

              - `"application/pdf"`

            - `type: "base64"`

              - `"base64"`

          - `PlainTextSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "text/plain"`

              - `"text/plain"`

            - `type: "text"`

              - `"text"`

          - `ContentBlockSource object { content, type }`

            - `content: string or array of ContentBlockSourceContent`

              - `string`

              - `ContentBlockSourceContent = array of ContentBlockSourceContent`

                - `TextBlockParam object { text, type, cache_control, citations }`

                - `ImageBlockParam object { source, type, cache_control }`

            - `type: "content"`

              - `"content"`

          - `URLPDFSource object { type, url }`

            - `type: "url"`

              - `"url"`

            - `url: string`

        - `type: "document"`

          - `"document"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

        - `context: optional string`

        - `title: optional string`

      - `ToolReferenceBlockParam object { tool_name, type, cache_control }`

        Tool reference block that can be included in tool_result content.

        - `tool_name: string`

        - `type: "tool_reference"`

          - `"tool_reference"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

  - `is_error: optional boolean`

### Tool Search Tool Bm25 20251119

- `ToolSearchToolBm25_20251119 object { name, type, allowed_callers, 3 more }`

  - `name: "tool_search_tool_bm25"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"tool_search_tool_bm25"`

  - `type: "tool_search_tool_bm25_20251119" or "tool_search_tool_bm25"`

    - `"tool_search_tool_bm25_20251119"`

    - `"tool_search_tool_bm25"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Tool Search Tool Regex 20251119

- `ToolSearchToolRegex20251119 object { name, type, allowed_callers, 3 more }`

  - `name: "tool_search_tool_regex"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"tool_search_tool_regex"`

  - `type: "tool_search_tool_regex_20251119" or "tool_search_tool_regex"`

    - `"tool_search_tool_regex_20251119"`

    - `"tool_search_tool_regex"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Tool Search Tool Result Block

- `ToolSearchToolResultBlock object { content, tool_use_id, type }`

  - `content: ToolSearchToolResultError or ToolSearchToolSearchResultBlock`

    - `ToolSearchToolResultError object { error_code, error_message, type }`

      - `error_code: ToolSearchToolResultErrorCode`

        - `"invalid_tool_input"`

        - `"unavailable"`

        - `"too_many_requests"`

        - `"execution_time_exceeded"`

      - `error_message: string`

      - `type: "tool_search_tool_result_error"`

        - `"tool_search_tool_result_error"`

    - `ToolSearchToolSearchResultBlock object { tool_references, type }`

      - `tool_references: array of ToolReferenceBlock`

        - `tool_name: string`

        - `type: "tool_reference"`

          - `"tool_reference"`

      - `type: "tool_search_tool_search_result"`

        - `"tool_search_tool_search_result"`

  - `tool_use_id: string`

  - `type: "tool_search_tool_result"`

    - `"tool_search_tool_result"`

### Tool Search Tool Result Block Param

- `ToolSearchToolResultBlockParam object { content, tool_use_id, type, cache_control }`

  - `content: ToolSearchToolResultErrorParam or ToolSearchToolSearchResultBlockParam`

    - `ToolSearchToolResultErrorParam object { error_code, type, error_message }`

      - `error_code: ToolSearchToolResultErrorCode`

        - `"invalid_tool_input"`

        - `"unavailable"`

        - `"too_many_requests"`

        - `"execution_time_exceeded"`

      - `type: "tool_search_tool_result_error"`

        - `"tool_search_tool_result_error"`

      - `error_message: optional string`

    - `ToolSearchToolSearchResultBlockParam object { tool_references, type }`

      - `tool_references: array of ToolReferenceBlockParam`

        - `tool_name: string`

        - `type: "tool_reference"`

          - `"tool_reference"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

          - `type: "ephemeral"`

            - `"ephemeral"`

          - `ttl: optional "5m" or "1h"`

            The time-to-live for the cache control breakpoint.

            This may be one the following values:

            - `5m`: 5 minutes
            - `1h`: 1 hour

            Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

            - `"5m"`

            - `"1h"`

      - `type: "tool_search_tool_search_result"`

        - `"tool_search_tool_search_result"`

  - `tool_use_id: string`

  - `type: "tool_search_tool_result"`

    - `"tool_search_tool_result"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

### Tool Search Tool Result Error

- `ToolSearchToolResultError object { error_code, error_message, type }`

  - `error_code: ToolSearchToolResultErrorCode`

    - `"invalid_tool_input"`

    - `"unavailable"`

    - `"too_many_requests"`

    - `"execution_time_exceeded"`

  - `error_message: string`

  - `type: "tool_search_tool_result_error"`

    - `"tool_search_tool_result_error"`

### Tool Search Tool Result Error Code

- `ToolSearchToolResultErrorCode = "invalid_tool_input" or "unavailable" or "too_many_requests" or "execution_time_exceeded"`

  - `"invalid_tool_input"`

  - `"unavailable"`

  - `"too_many_requests"`

  - `"execution_time_exceeded"`

### Tool Search Tool Result Error Param

- `ToolSearchToolResultErrorParam object { error_code, type, error_message }`

  - `error_code: ToolSearchToolResultErrorCode`

    - `"invalid_tool_input"`

    - `"unavailable"`

    - `"too_many_requests"`

    - `"execution_time_exceeded"`

  - `type: "tool_search_tool_result_error"`

    - `"tool_search_tool_result_error"`

  - `error_message: optional string`

### Tool Search Tool Search Result Block

- `ToolSearchToolSearchResultBlock object { tool_references, type }`

  - `tool_references: array of ToolReferenceBlock`

    - `tool_name: string`

    - `type: "tool_reference"`

      - `"tool_reference"`

  - `type: "tool_search_tool_search_result"`

    - `"tool_search_tool_search_result"`

### Tool Search Tool Search Result Block Param

- `ToolSearchToolSearchResultBlockParam object { tool_references, type }`

  - `tool_references: array of ToolReferenceBlockParam`

    - `tool_name: string`

    - `type: "tool_reference"`

      - `"tool_reference"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

      - `type: "ephemeral"`

        - `"ephemeral"`

      - `ttl: optional "5m" or "1h"`

        The time-to-live for the cache control breakpoint.

        This may be one the following values:

        - `5m`: 5 minutes
        - `1h`: 1 hour

        Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

        - `"5m"`

        - `"1h"`

  - `type: "tool_search_tool_search_result"`

    - `"tool_search_tool_search_result"`

### Tool Text Editor 20250124

- `ToolTextEditor20250124 object { name, type, allowed_callers, 4 more }`

  - `name: "str_replace_editor"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"str_replace_editor"`

  - `type: "text_editor_20250124"`

    - `"text_editor_20250124"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `input_examples: optional array of map[unknown]`

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Tool Text Editor 20250429

- `ToolTextEditor20250429 object { name, type, allowed_callers, 4 more }`

  - `name: "str_replace_based_edit_tool"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"str_replace_based_edit_tool"`

  - `type: "text_editor_20250429"`

    - `"text_editor_20250429"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `input_examples: optional array of map[unknown]`

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Tool Text Editor 20250728

- `ToolTextEditor20250728 object { name, type, allowed_callers, 5 more }`

  - `name: "str_replace_based_edit_tool"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"str_replace_based_edit_tool"`

  - `type: "text_editor_20250728"`

    - `"text_editor_20250728"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `input_examples: optional array of map[unknown]`

  - `max_characters: optional number`

    Maximum number of characters to display when viewing a file. If not specified, defaults to displaying the full file.

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Tool Union

- `ToolUnion = Tool or ToolBash20250124 or CodeExecutionTool20250522 or 16 more`

  Code execution tool with REPL state persistence (daemon mode + gVisor checkpoint).

  - `Tool object { input_schema, name, allowed_callers, 7 more }`

    - `input_schema: object { type, properties, required }`

      [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.

      This defines the shape of the `input` that your tool accepts and that the model will produce.

      - `type: "object"`

        - `"object"`

      - `properties: optional map[unknown]`

      - `required: optional array of string`

    - `name: string`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

      - `type: "ephemeral"`

        - `"ephemeral"`

      - `ttl: optional "5m" or "1h"`

        The time-to-live for the cache control breakpoint.

        This may be one the following values:

        - `5m`: 5 minutes
        - `1h`: 1 hour

        Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

        - `"5m"`

        - `"1h"`

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `description: optional string`

      Description of what this tool does.

      Tool descriptions should be as detailed as possible. The more information that the model has about what the tool is and how to use it, the better it will perform. You can use natural language descriptions to reinforce important aspects of the tool input JSON schema.

    - `eager_input_streaming: optional boolean`

      Enable eager input streaming for this tool. When true, tool input parameters will be streamed incrementally as they are generated, and types will be inferred on-the-fly rather than buffering the full JSON output. When false, streaming is disabled for this tool even if the fine-grained-tool-streaming beta is active. When null (default), uses the default behavior based on beta headers.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `type: optional "custom"`

      - `"custom"`

  - `ToolBash20250124 object { name, type, allowed_callers, 4 more }`

    - `name: "bash"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"bash"`

    - `type: "bash_20250124"`

      - `"bash_20250124"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20250522 object { name, type, allowed_callers, 3 more }`

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20250522"`

      - `"code_execution_20250522"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20250825 object { name, type, allowed_callers, 3 more }`

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20250825"`

      - `"code_execution_20250825"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20260120 object { name, type, allowed_callers, 3 more }`

    Code execution tool with REPL state persistence (daemon mode + gVisor checkpoint).

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20260120"`

      - `"code_execution_20260120"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `CodeExecutionTool20260521 object { name, type, allowed_callers, 3 more }`

    Code execution tool with REPL state persistence.

    - `name: "code_execution"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"code_execution"`

    - `type: "code_execution_20260521"`

      - `"code_execution_20260521"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `MemoryTool20250818 object { name, type, allowed_callers, 4 more }`

    - `name: "memory"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"memory"`

    - `type: "memory_20250818"`

      - `"memory_20250818"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolTextEditor20250124 object { name, type, allowed_callers, 4 more }`

    - `name: "str_replace_editor"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"str_replace_editor"`

    - `type: "text_editor_20250124"`

      - `"text_editor_20250124"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolTextEditor20250429 object { name, type, allowed_callers, 4 more }`

    - `name: "str_replace_based_edit_tool"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"str_replace_based_edit_tool"`

    - `type: "text_editor_20250429"`

      - `"text_editor_20250429"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolTextEditor20250728 object { name, type, allowed_callers, 5 more }`

    - `name: "str_replace_based_edit_tool"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"str_replace_based_edit_tool"`

    - `type: "text_editor_20250728"`

      - `"text_editor_20250728"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `input_examples: optional array of map[unknown]`

    - `max_characters: optional number`

      Maximum number of characters to display when viewing a file. If not specified, defaults to displaying the full file.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `WebSearchTool20250305 object { name, type, allowed_callers, 7 more }`

    - `name: "web_search"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_search"`

    - `type: "web_search_20250305"`

      - `"web_search_20250305"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

    - `blocked_domains: optional array of string`

      If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `user_location: optional UserLocation`

      Parameters for the user's location. Used to provide more relevant search results.

      - `type: "approximate"`

        - `"approximate"`

      - `city: optional string`

        The city of the user.

      - `country: optional string`

        The two letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the user.

      - `region: optional string`

        The region of the user.

      - `timezone: optional string`

        The [IANA timezone](https://nodatime.org/TimeZones) of the user.

  - `WebFetchTool20250910 object { name, type, allowed_callers, 8 more }`

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20250910"`

      - `"web_fetch_20250910"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

      - `enabled: optional boolean`

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `WebSearchTool20260209 object { name, type, allowed_callers, 7 more }`

    - `name: "web_search"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_search"`

    - `type: "web_search_20260209"`

      - `"web_search_20260209"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

    - `blocked_domains: optional array of string`

      If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `user_location: optional UserLocation`

      Parameters for the user's location. Used to provide more relevant search results.

  - `WebFetchTool20260209 object { name, type, allowed_callers, 8 more }`

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20260209"`

      - `"web_fetch_20260209"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `WebFetchTool20260309 object { name, type, allowed_callers, 9 more }`

    Web fetch tool with use_cache parameter for bypassing cached content.

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20260309"`

      - `"web_fetch_20260309"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `use_cache: optional boolean`

      Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.

  - `WebSearchTool20260318 object { name, type, allowed_callers, 8 more }`

    - `name: "web_search"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_search"`

    - `type: "web_search_20260318"`

      - `"web_search_20260318"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

    - `blocked_domains: optional array of string`

      If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `response_inclusion: optional "full" or "excluded"`

      How this tool's result blocks appear in the API response when the result was consumed by a completed code_execution call in the same turn. 'full' returns the complete content (default). 'excluded' drops the nested server_tool_use and result block pair entirely. Results from direct calls, or from code_execution calls that paused before completing, are always returned in full so they can be sent back on the next turn.

      - `"full"`

      - `"excluded"`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `user_location: optional UserLocation`

      Parameters for the user's location. Used to provide more relevant search results.

  - `WebFetchTool20260318 object { name, type, allowed_callers, 10 more }`

    - `name: "web_fetch"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"web_fetch"`

    - `type: "web_fetch_20260318"`

      - `"web_fetch_20260318"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `allowed_domains: optional array of string`

      List of domains to allow fetching from

    - `blocked_domains: optional array of string`

      List of domains to block fetching from

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      Citations configuration for fetched documents. Citations are disabled by default.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `max_content_tokens: optional number`

      Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

    - `max_uses: optional number`

      Maximum number of times the tool can be used in the API request.

    - `response_inclusion: optional "full" or "excluded"`

      How this tool's result blocks appear in the API response when the result was consumed by a completed code_execution call in the same turn. 'full' returns the complete content (default). 'excluded' drops the nested server_tool_use and result block pair entirely. Results from direct calls, or from code_execution calls that paused before completing, are always returned in full so they can be sent back on the next turn.

      - `"full"`

      - `"excluded"`

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

    - `use_cache: optional boolean`

      Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.

  - `ToolSearchToolBm25_20251119 object { name, type, allowed_callers, 3 more }`

    - `name: "tool_search_tool_bm25"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"tool_search_tool_bm25"`

    - `type: "tool_search_tool_bm25_20251119" or "tool_search_tool_bm25"`

      - `"tool_search_tool_bm25_20251119"`

      - `"tool_search_tool_bm25"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

  - `ToolSearchToolRegex20251119 object { name, type, allowed_callers, 3 more }`

    - `name: "tool_search_tool_regex"`

      Name of the tool.

      This is how the tool will be called by the model and in `tool_use` blocks.

      - `"tool_search_tool_regex"`

    - `type: "tool_search_tool_regex_20251119" or "tool_search_tool_regex"`

      - `"tool_search_tool_regex_20251119"`

      - `"tool_search_tool_regex"`

    - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

      - `"direct"`

      - `"code_execution_20250825"`

      - `"code_execution_20260120"`

      - `"code_execution_20260521"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `defer_loading: optional boolean`

      If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

    - `strict: optional boolean`

      When true, guarantees schema validation on tool names and inputs

### Tool Use Block

- `ToolUseBlock object { id, caller, input, 2 more }`

  - `id: string`

  - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

    Tool invocation directly from the model.

    - `DirectCaller object { type }`

      Tool invocation directly from the model.

      - `type: "direct"`

        - `"direct"`

    - `ServerToolCaller object { tool_id, type }`

      Tool invocation generated by a server-side tool.

      - `tool_id: string`

      - `type: "code_execution_20250825"`

        - `"code_execution_20250825"`

    - `ServerToolCaller20260120 object { tool_id, type }`

      - `tool_id: string`

      - `type: "code_execution_20260120"`

        - `"code_execution_20260120"`

  - `input: map[unknown]`

  - `name: string`

  - `type: "tool_use"`

    - `"tool_use"`

### Tool Use Block Param

- `ToolUseBlockParam object { id, input, name, 3 more }`

  - `id: string`

  - `input: map[unknown]`

  - `name: string`

  - `type: "tool_use"`

    - `"tool_use"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

    Tool invocation directly from the model.

    - `DirectCaller object { type }`

      Tool invocation directly from the model.

      - `type: "direct"`

        - `"direct"`

    - `ServerToolCaller object { tool_id, type }`

      Tool invocation generated by a server-side tool.

      - `tool_id: string`

      - `type: "code_execution_20250825"`

        - `"code_execution_20250825"`

    - `ServerToolCaller20260120 object { tool_id, type }`

      - `tool_id: string`

      - `type: "code_execution_20260120"`

        - `"code_execution_20260120"`

### URL Image Source

- `URLImageSource object { type, url }`

  - `type: "url"`

    - `"url"`

  - `url: string`

### URL PDF Source

- `URLPDFSource object { type, url }`

  - `type: "url"`

    - `"url"`

  - `url: string`

### Usage

- `Usage object { cache_creation, cache_creation_input_tokens, cache_read_input_tokens, 6 more }`

  - `cache_creation: CacheCreation`

    Breakdown of cached tokens by TTL

    - `ephemeral_1h_input_tokens: number`

      The number of input tokens used to create the 1 hour cache entry.

    - `ephemeral_5m_input_tokens: number`

      The number of input tokens used to create the 5 minute cache entry.

  - `cache_creation_input_tokens: number`

    The number of input tokens used to create the cache entry.

  - `cache_read_input_tokens: number`

    The number of input tokens read from the cache.

  - `inference_geo: string`

    The geographic region where inference was performed for this request.

  - `input_tokens: number`

    The number of input tokens which were used.

  - `output_tokens: number`

    The number of output tokens which were used.

  - `output_tokens_details: OutputTokensDetails`

    Breakdown of output tokens by category.

    `output_tokens` remains the inclusive, authoritative total used for billing.
    This object provides a read-only decomposition for observability — for example,
    how many of the billed output tokens were spent on internal reasoning that may
    have been summarized before being returned to you.

    - `thinking_tokens: number`

      Number of output tokens the model generated as internal reasoning, including
      the thinking-block delimiter tokens.

      Reflects the raw reasoning the model produced, not the (possibly shorter)
      summarized thinking text returned in the response body. Computed by
      re-tokenizing the raw reasoning text, so it may differ from the model's exact
      generation count by a small number of tokens. Always ≤ `output_tokens`;
      `output_tokens - thinking_tokens` approximates the non-reasoning output.

  - `server_tool_use: ServerToolUsage`

    The number of server tool requests.

    - `web_fetch_requests: number`

      The number of web fetch tool requests.

    - `web_search_requests: number`

      The number of web search tool requests.

  - `service_tier: "standard" or "priority" or "batch"`

    If the request used the priority, standard, or batch tier.

    - `"standard"`

    - `"priority"`

    - `"batch"`

### User Location

- `UserLocation object { type, city, country, 2 more }`

  - `type: "approximate"`

    - `"approximate"`

  - `city: optional string`

    The city of the user.

  - `country: optional string`

    The two letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the user.

  - `region: optional string`

    The region of the user.

  - `timezone: optional string`

    The [IANA timezone](https://nodatime.org/TimeZones) of the user.

### Web Fetch Block

- `WebFetchBlock object { content, retrieved_at, type, url }`

  - `content: DocumentBlock`

    - `citations: CitationsConfig`

      Citation configuration for the document

      - `enabled: boolean`

    - `source: Base64PDFSource or PlainTextSource`

      - `Base64PDFSource object { data, media_type, type }`

        - `data: string`

        - `media_type: "application/pdf"`

          - `"application/pdf"`

        - `type: "base64"`

          - `"base64"`

      - `PlainTextSource object { data, media_type, type }`

        - `data: string`

        - `media_type: "text/plain"`

          - `"text/plain"`

        - `type: "text"`

          - `"text"`

    - `title: string`

      The title of the document

    - `type: "document"`

      - `"document"`

  - `retrieved_at: string`

    ISO 8601 timestamp when the content was retrieved

  - `type: "web_fetch_result"`

    - `"web_fetch_result"`

  - `url: string`

    Fetched content URL

### Web Fetch Block Param

- `WebFetchBlockParam object { content, type, url, retrieved_at }`

  - `content: DocumentBlockParam`

    - `source: Base64PDFSource or PlainTextSource or ContentBlockSource or URLPDFSource`

      - `Base64PDFSource object { data, media_type, type }`

        - `data: string`

        - `media_type: "application/pdf"`

          - `"application/pdf"`

        - `type: "base64"`

          - `"base64"`

      - `PlainTextSource object { data, media_type, type }`

        - `data: string`

        - `media_type: "text/plain"`

          - `"text/plain"`

        - `type: "text"`

          - `"text"`

      - `ContentBlockSource object { content, type }`

        - `content: string or array of ContentBlockSourceContent`

          - `string`

          - `ContentBlockSourceContent = array of ContentBlockSourceContent`

            - `TextBlockParam object { text, type, cache_control, citations }`

              - `text: string`

              - `type: "text"`

                - `"text"`

              - `cache_control: optional CacheControlEphemeral`

                Create a cache control breakpoint at this content block.

                - `type: "ephemeral"`

                  - `"ephemeral"`

                - `ttl: optional "5m" or "1h"`

                  The time-to-live for the cache control breakpoint.

                  This may be one the following values:

                  - `5m`: 5 minutes
                  - `1h`: 1 hour

                  Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

                  - `"5m"`

                  - `"1h"`

              - `citations: optional array of TextCitationParam`

                - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

                  - `cited_text: string`

                  - `document_index: number`

                  - `document_title: string`

                  - `end_char_index: number`

                  - `start_char_index: number`

                  - `type: "char_location"`

                    - `"char_location"`

                - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

                  - `cited_text: string`

                  - `document_index: number`

                  - `document_title: string`

                  - `end_page_number: number`

                  - `start_page_number: number`

                  - `type: "page_location"`

                    - `"page_location"`

                - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

                  - `cited_text: string`

                    The full text of the cited block range, concatenated.

                    Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

                  - `document_index: number`

                  - `document_title: string`

                  - `end_block_index: number`

                    Exclusive 0-based end index of the cited block range in the source's `content` array.

                    Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

                  - `start_block_index: number`

                    0-based index of the first cited block in the source's `content` array.

                  - `type: "content_block_location"`

                    - `"content_block_location"`

                - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

                  - `cited_text: string`

                  - `encrypted_index: string`

                  - `title: string`

                  - `type: "web_search_result_location"`

                    - `"web_search_result_location"`

                  - `url: string`

                - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

                  - `cited_text: string`

                    The full text of the cited block range, concatenated.

                    Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

                  - `end_block_index: number`

                    Exclusive 0-based end index of the cited block range in the source's `content` array.

                    Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

                  - `search_result_index: number`

                    0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

                    Counted separately from `document_index`; server-side web search results are not included in this count.

                  - `source: string`

                  - `start_block_index: number`

                    0-based index of the first cited block in the source's `content` array.

                  - `title: string`

                  - `type: "search_result_location"`

                    - `"search_result_location"`

            - `ImageBlockParam object { source, type, cache_control }`

              - `source: Base64ImageSource or URLImageSource`

                - `Base64ImageSource object { data, media_type, type }`

                  - `data: string`

                  - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

                    - `"image/jpeg"`

                    - `"image/png"`

                    - `"image/gif"`

                    - `"image/webp"`

                  - `type: "base64"`

                    - `"base64"`

                - `URLImageSource object { type, url }`

                  - `type: "url"`

                    - `"url"`

                  - `url: string`

              - `type: "image"`

                - `"image"`

              - `cache_control: optional CacheControlEphemeral`

                Create a cache control breakpoint at this content block.

        - `type: "content"`

          - `"content"`

      - `URLPDFSource object { type, url }`

        - `type: "url"`

          - `"url"`

        - `url: string`

    - `type: "document"`

      - `"document"`

    - `cache_control: optional CacheControlEphemeral`

      Create a cache control breakpoint at this content block.

    - `citations: optional CitationsConfigParam`

      - `enabled: optional boolean`

    - `context: optional string`

    - `title: optional string`

  - `type: "web_fetch_result"`

    - `"web_fetch_result"`

  - `url: string`

    Fetched content URL

  - `retrieved_at: optional string`

    ISO 8601 timestamp when the content was retrieved

### Web Fetch Tool 20250910

- `WebFetchTool20250910 object { name, type, allowed_callers, 8 more }`

  - `name: "web_fetch"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"web_fetch"`

  - `type: "web_fetch_20250910"`

    - `"web_fetch_20250910"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `allowed_domains: optional array of string`

    List of domains to allow fetching from

  - `blocked_domains: optional array of string`

    List of domains to block fetching from

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `citations: optional CitationsConfigParam`

    Citations configuration for fetched documents. Citations are disabled by default.

    - `enabled: optional boolean`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `max_content_tokens: optional number`

    Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

  - `max_uses: optional number`

    Maximum number of times the tool can be used in the API request.

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Web Fetch Tool 20260209

- `WebFetchTool20260209 object { name, type, allowed_callers, 8 more }`

  - `name: "web_fetch"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"web_fetch"`

  - `type: "web_fetch_20260209"`

    - `"web_fetch_20260209"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `allowed_domains: optional array of string`

    List of domains to allow fetching from

  - `blocked_domains: optional array of string`

    List of domains to block fetching from

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `citations: optional CitationsConfigParam`

    Citations configuration for fetched documents. Citations are disabled by default.

    - `enabled: optional boolean`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `max_content_tokens: optional number`

    Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

  - `max_uses: optional number`

    Maximum number of times the tool can be used in the API request.

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

### Web Fetch Tool 20260309

- `WebFetchTool20260309 object { name, type, allowed_callers, 9 more }`

  Web fetch tool with use_cache parameter for bypassing cached content.

  - `name: "web_fetch"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"web_fetch"`

  - `type: "web_fetch_20260309"`

    - `"web_fetch_20260309"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `allowed_domains: optional array of string`

    List of domains to allow fetching from

  - `blocked_domains: optional array of string`

    List of domains to block fetching from

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `citations: optional CitationsConfigParam`

    Citations configuration for fetched documents. Citations are disabled by default.

    - `enabled: optional boolean`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `max_content_tokens: optional number`

    Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

  - `max_uses: optional number`

    Maximum number of times the tool can be used in the API request.

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

  - `use_cache: optional boolean`

    Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.

### Web Fetch Tool 20260318

- `WebFetchTool20260318 object { name, type, allowed_callers, 10 more }`

  - `name: "web_fetch"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"web_fetch"`

  - `type: "web_fetch_20260318"`

    - `"web_fetch_20260318"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `allowed_domains: optional array of string`

    List of domains to allow fetching from

  - `blocked_domains: optional array of string`

    List of domains to block fetching from

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `citations: optional CitationsConfigParam`

    Citations configuration for fetched documents. Citations are disabled by default.

    - `enabled: optional boolean`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `max_content_tokens: optional number`

    Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

  - `max_uses: optional number`

    Maximum number of times the tool can be used in the API request.

  - `response_inclusion: optional "full" or "excluded"`

    How this tool's result blocks appear in the API response when the result was consumed by a completed code_execution call in the same turn. 'full' returns the complete content (default). 'excluded' drops the nested server_tool_use and result block pair entirely. Results from direct calls, or from code_execution calls that paused before completing, are always returned in full so they can be sent back on the next turn.

    - `"full"`

    - `"excluded"`

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

  - `use_cache: optional boolean`

    Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.

### Web Fetch Tool Result Block

- `WebFetchToolResultBlock object { caller, content, tool_use_id, type }`

  - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

    Tool invocation directly from the model.

    - `DirectCaller object { type }`

      Tool invocation directly from the model.

      - `type: "direct"`

        - `"direct"`

    - `ServerToolCaller object { tool_id, type }`

      Tool invocation generated by a server-side tool.

      - `tool_id: string`

      - `type: "code_execution_20250825"`

        - `"code_execution_20250825"`

    - `ServerToolCaller20260120 object { tool_id, type }`

      - `tool_id: string`

      - `type: "code_execution_20260120"`

        - `"code_execution_20260120"`

  - `content: WebFetchToolResultErrorBlock or WebFetchBlock`

    - `WebFetchToolResultErrorBlock object { error_code, type }`

      - `error_code: WebFetchToolResultErrorCode`

        - `"invalid_tool_input"`

        - `"url_too_long"`

        - `"url_not_allowed"`

        - `"url_not_in_prior_context"`

        - `"url_not_accessible"`

        - `"unsupported_content_type"`

        - `"too_many_requests"`

        - `"max_uses_exceeded"`

        - `"unavailable"`

      - `type: "web_fetch_tool_result_error"`

        - `"web_fetch_tool_result_error"`

    - `WebFetchBlock object { content, retrieved_at, type, url }`

      - `content: DocumentBlock`

        - `citations: CitationsConfig`

          Citation configuration for the document

          - `enabled: boolean`

        - `source: Base64PDFSource or PlainTextSource`

          - `Base64PDFSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "application/pdf"`

              - `"application/pdf"`

            - `type: "base64"`

              - `"base64"`

          - `PlainTextSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "text/plain"`

              - `"text/plain"`

            - `type: "text"`

              - `"text"`

        - `title: string`

          The title of the document

        - `type: "document"`

          - `"document"`

      - `retrieved_at: string`

        ISO 8601 timestamp when the content was retrieved

      - `type: "web_fetch_result"`

        - `"web_fetch_result"`

      - `url: string`

        Fetched content URL

  - `tool_use_id: string`

  - `type: "web_fetch_tool_result"`

    - `"web_fetch_tool_result"`

### Web Fetch Tool Result Block Param

- `WebFetchToolResultBlockParam object { content, tool_use_id, type, 2 more }`

  - `content: WebFetchToolResultErrorBlockParam or WebFetchBlockParam`

    - `WebFetchToolResultErrorBlockParam object { error_code, type }`

      - `error_code: WebFetchToolResultErrorCode`

        - `"invalid_tool_input"`

        - `"url_too_long"`

        - `"url_not_allowed"`

        - `"url_not_in_prior_context"`

        - `"url_not_accessible"`

        - `"unsupported_content_type"`

        - `"too_many_requests"`

        - `"max_uses_exceeded"`

        - `"unavailable"`

      - `type: "web_fetch_tool_result_error"`

        - `"web_fetch_tool_result_error"`

    - `WebFetchBlockParam object { content, type, url, retrieved_at }`

      - `content: DocumentBlockParam`

        - `source: Base64PDFSource or PlainTextSource or ContentBlockSource or URLPDFSource`

          - `Base64PDFSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "application/pdf"`

              - `"application/pdf"`

            - `type: "base64"`

              - `"base64"`

          - `PlainTextSource object { data, media_type, type }`

            - `data: string`

            - `media_type: "text/plain"`

              - `"text/plain"`

            - `type: "text"`

              - `"text"`

          - `ContentBlockSource object { content, type }`

            - `content: string or array of ContentBlockSourceContent`

              - `string`

              - `ContentBlockSourceContent = array of ContentBlockSourceContent`

                - `TextBlockParam object { text, type, cache_control, citations }`

                  - `text: string`

                  - `type: "text"`

                    - `"text"`

                  - `cache_control: optional CacheControlEphemeral`

                    Create a cache control breakpoint at this content block.

                    - `type: "ephemeral"`

                      - `"ephemeral"`

                    - `ttl: optional "5m" or "1h"`

                      The time-to-live for the cache control breakpoint.

                      This may be one the following values:

                      - `5m`: 5 minutes
                      - `1h`: 1 hour

                      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

                      - `"5m"`

                      - `"1h"`

                  - `citations: optional array of TextCitationParam`

                    - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

                      - `cited_text: string`

                      - `document_index: number`

                      - `document_title: string`

                      - `end_char_index: number`

                      - `start_char_index: number`

                      - `type: "char_location"`

                        - `"char_location"`

                    - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

                      - `cited_text: string`

                      - `document_index: number`

                      - `document_title: string`

                      - `end_page_number: number`

                      - `start_page_number: number`

                      - `type: "page_location"`

                        - `"page_location"`

                    - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

                      - `cited_text: string`

                        The full text of the cited block range, concatenated.

                        Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

                      - `document_index: number`

                      - `document_title: string`

                      - `end_block_index: number`

                        Exclusive 0-based end index of the cited block range in the source's `content` array.

                        Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

                      - `start_block_index: number`

                        0-based index of the first cited block in the source's `content` array.

                      - `type: "content_block_location"`

                        - `"content_block_location"`

                    - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

                      - `cited_text: string`

                      - `encrypted_index: string`

                      - `title: string`

                      - `type: "web_search_result_location"`

                        - `"web_search_result_location"`

                      - `url: string`

                    - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

                      - `cited_text: string`

                        The full text of the cited block range, concatenated.

                        Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

                      - `end_block_index: number`

                        Exclusive 0-based end index of the cited block range in the source's `content` array.

                        Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

                      - `search_result_index: number`

                        0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

                        Counted separately from `document_index`; server-side web search results are not included in this count.

                      - `source: string`

                      - `start_block_index: number`

                        0-based index of the first cited block in the source's `content` array.

                      - `title: string`

                      - `type: "search_result_location"`

                        - `"search_result_location"`

                - `ImageBlockParam object { source, type, cache_control }`

                  - `source: Base64ImageSource or URLImageSource`

                    - `Base64ImageSource object { data, media_type, type }`

                      - `data: string`

                      - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

                        - `"image/jpeg"`

                        - `"image/png"`

                        - `"image/gif"`

                        - `"image/webp"`

                      - `type: "base64"`

                        - `"base64"`

                    - `URLImageSource object { type, url }`

                      - `type: "url"`

                        - `"url"`

                      - `url: string`

                  - `type: "image"`

                    - `"image"`

                  - `cache_control: optional CacheControlEphemeral`

                    Create a cache control breakpoint at this content block.

            - `type: "content"`

              - `"content"`

          - `URLPDFSource object { type, url }`

            - `type: "url"`

              - `"url"`

            - `url: string`

        - `type: "document"`

          - `"document"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

          - `enabled: optional boolean`

        - `context: optional string`

        - `title: optional string`

      - `type: "web_fetch_result"`

        - `"web_fetch_result"`

      - `url: string`

        Fetched content URL

      - `retrieved_at: optional string`

        ISO 8601 timestamp when the content was retrieved

  - `tool_use_id: string`

  - `type: "web_fetch_tool_result"`

    - `"web_fetch_tool_result"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

  - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

    Tool invocation directly from the model.

    - `DirectCaller object { type }`

      Tool invocation directly from the model.

      - `type: "direct"`

        - `"direct"`

    - `ServerToolCaller object { tool_id, type }`

      Tool invocation generated by a server-side tool.

      - `tool_id: string`

      - `type: "code_execution_20250825"`

        - `"code_execution_20250825"`

    - `ServerToolCaller20260120 object { tool_id, type }`

      - `tool_id: string`

      - `type: "code_execution_20260120"`

        - `"code_execution_20260120"`

### Web Fetch Tool Result Error Block

- `WebFetchToolResultErrorBlock object { error_code, type }`

  - `error_code: WebFetchToolResultErrorCode`

    - `"invalid_tool_input"`

    - `"url_too_long"`

    - `"url_not_allowed"`

    - `"url_not_in_prior_context"`

    - `"url_not_accessible"`

    - `"unsupported_content_type"`

    - `"too_many_requests"`

    - `"max_uses_exceeded"`

    - `"unavailable"`

  - `type: "web_fetch_tool_result_error"`

    - `"web_fetch_tool_result_error"`

### Web Fetch Tool Result Error Block Param

- `WebFetchToolResultErrorBlockParam object { error_code, type }`

  - `error_code: WebFetchToolResultErrorCode`

    - `"invalid_tool_input"`

    - `"url_too_long"`

    - `"url_not_allowed"`

    - `"url_not_in_prior_context"`

    - `"url_not_accessible"`

    - `"unsupported_content_type"`

    - `"too_many_requests"`

    - `"max_uses_exceeded"`

    - `"unavailable"`

  - `type: "web_fetch_tool_result_error"`

    - `"web_fetch_tool_result_error"`

### Web Fetch Tool Result Error Code

- `WebFetchToolResultErrorCode = "invalid_tool_input" or "url_too_long" or "url_not_allowed" or 6 more`

  - `"invalid_tool_input"`

  - `"url_too_long"`

  - `"url_not_allowed"`

  - `"url_not_in_prior_context"`

  - `"url_not_accessible"`

  - `"unsupported_content_type"`

  - `"too_many_requests"`

  - `"max_uses_exceeded"`

  - `"unavailable"`

### Web Search Result Block

- `WebSearchResultBlock object { encrypted_content, page_age, title, 2 more }`

  - `encrypted_content: string`

  - `page_age: string`

  - `title: string`

  - `type: "web_search_result"`

    - `"web_search_result"`

  - `url: string`

### Web Search Result Block Param

- `WebSearchResultBlockParam object { encrypted_content, title, type, 2 more }`

  - `encrypted_content: string`

  - `title: string`

  - `type: "web_search_result"`

    - `"web_search_result"`

  - `url: string`

  - `page_age: optional string`

### Web Search Tool 20250305

- `WebSearchTool20250305 object { name, type, allowed_callers, 7 more }`

  - `name: "web_search"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"web_search"`

  - `type: "web_search_20250305"`

    - `"web_search_20250305"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `allowed_domains: optional array of string`

    If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

  - `blocked_domains: optional array of string`

    If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `max_uses: optional number`

    Maximum number of times the tool can be used in the API request.

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

  - `user_location: optional UserLocation`

    Parameters for the user's location. Used to provide more relevant search results.

    - `type: "approximate"`

      - `"approximate"`

    - `city: optional string`

      The city of the user.

    - `country: optional string`

      The two letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the user.

    - `region: optional string`

      The region of the user.

    - `timezone: optional string`

      The [IANA timezone](https://nodatime.org/TimeZones) of the user.

### Web Search Tool 20260209

- `WebSearchTool20260209 object { name, type, allowed_callers, 7 more }`

  - `name: "web_search"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"web_search"`

  - `type: "web_search_20260209"`

    - `"web_search_20260209"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `allowed_domains: optional array of string`

    If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

  - `blocked_domains: optional array of string`

    If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `max_uses: optional number`

    Maximum number of times the tool can be used in the API request.

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

  - `user_location: optional UserLocation`

    Parameters for the user's location. Used to provide more relevant search results.

    - `type: "approximate"`

      - `"approximate"`

    - `city: optional string`

      The city of the user.

    - `country: optional string`

      The two letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the user.

    - `region: optional string`

      The region of the user.

    - `timezone: optional string`

      The [IANA timezone](https://nodatime.org/TimeZones) of the user.

### Web Search Tool 20260318

- `WebSearchTool20260318 object { name, type, allowed_callers, 8 more }`

  - `name: "web_search"`

    Name of the tool.

    This is how the tool will be called by the model and in `tool_use` blocks.

    - `"web_search"`

  - `type: "web_search_20260318"`

    - `"web_search_20260318"`

  - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

    - `"direct"`

    - `"code_execution_20250825"`

    - `"code_execution_20260120"`

    - `"code_execution_20260521"`

  - `allowed_domains: optional array of string`

    If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

  - `blocked_domains: optional array of string`

    If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `defer_loading: optional boolean`

    If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

  - `max_uses: optional number`

    Maximum number of times the tool can be used in the API request.

  - `response_inclusion: optional "full" or "excluded"`

    How this tool's result blocks appear in the API response when the result was consumed by a completed code_execution call in the same turn. 'full' returns the complete content (default). 'excluded' drops the nested server_tool_use and result block pair entirely. Results from direct calls, or from code_execution calls that paused before completing, are always returned in full so they can be sent back on the next turn.

    - `"full"`

    - `"excluded"`

  - `strict: optional boolean`

    When true, guarantees schema validation on tool names and inputs

  - `user_location: optional UserLocation`

    Parameters for the user's location. Used to provide more relevant search results.

    - `type: "approximate"`

      - `"approximate"`

    - `city: optional string`

      The city of the user.

    - `country: optional string`

      The two letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the user.

    - `region: optional string`

      The region of the user.

    - `timezone: optional string`

      The [IANA timezone](https://nodatime.org/TimeZones) of the user.

### Web Search Tool Request Error

- `WebSearchToolRequestError object { error_code, type }`

  - `error_code: WebSearchToolResultErrorCode`

    - `"invalid_tool_input"`

    - `"unavailable"`

    - `"max_uses_exceeded"`

    - `"too_many_requests"`

    - `"query_too_long"`

    - `"request_too_large"`

  - `type: "web_search_tool_result_error"`

    - `"web_search_tool_result_error"`

### Web Search Tool Result Block

- `WebSearchToolResultBlock object { caller, content, tool_use_id, type }`

  - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

    Tool invocation directly from the model.

    - `DirectCaller object { type }`

      Tool invocation directly from the model.

      - `type: "direct"`

        - `"direct"`

    - `ServerToolCaller object { tool_id, type }`

      Tool invocation generated by a server-side tool.

      - `tool_id: string`

      - `type: "code_execution_20250825"`

        - `"code_execution_20250825"`

    - `ServerToolCaller20260120 object { tool_id, type }`

      - `tool_id: string`

      - `type: "code_execution_20260120"`

        - `"code_execution_20260120"`

  - `content: WebSearchToolResultBlockContent`

    - `WebSearchToolResultError object { error_code, type }`

      - `error_code: WebSearchToolResultErrorCode`

        - `"invalid_tool_input"`

        - `"unavailable"`

        - `"max_uses_exceeded"`

        - `"too_many_requests"`

        - `"query_too_long"`

        - `"request_too_large"`

      - `type: "web_search_tool_result_error"`

        - `"web_search_tool_result_error"`

    - `array of WebSearchResultBlock`

      - `encrypted_content: string`

      - `page_age: string`

      - `title: string`

      - `type: "web_search_result"`

        - `"web_search_result"`

      - `url: string`

  - `tool_use_id: string`

  - `type: "web_search_tool_result"`

    - `"web_search_tool_result"`

### Web Search Tool Result Block Content

- `WebSearchToolResultBlockContent = WebSearchToolResultError or array of WebSearchResultBlock`

  - `WebSearchToolResultError object { error_code, type }`

    - `error_code: WebSearchToolResultErrorCode`

      - `"invalid_tool_input"`

      - `"unavailable"`

      - `"max_uses_exceeded"`

      - `"too_many_requests"`

      - `"query_too_long"`

      - `"request_too_large"`

    - `type: "web_search_tool_result_error"`

      - `"web_search_tool_result_error"`

  - `array of WebSearchResultBlock`

    - `encrypted_content: string`

    - `page_age: string`

    - `title: string`

    - `type: "web_search_result"`

      - `"web_search_result"`

    - `url: string`

### Web Search Tool Result Block Param

- `WebSearchToolResultBlockParam object { content, tool_use_id, type, 2 more }`

  - `content: WebSearchToolResultBlockParamContent`

    - `WebSearchToolResultBlockItem = array of WebSearchResultBlockParam`

      - `encrypted_content: string`

      - `title: string`

      - `type: "web_search_result"`

        - `"web_search_result"`

      - `url: string`

      - `page_age: optional string`

    - `WebSearchToolRequestError object { error_code, type }`

      - `error_code: WebSearchToolResultErrorCode`

        - `"invalid_tool_input"`

        - `"unavailable"`

        - `"max_uses_exceeded"`

        - `"too_many_requests"`

        - `"query_too_long"`

        - `"request_too_large"`

      - `type: "web_search_tool_result_error"`

        - `"web_search_tool_result_error"`

  - `tool_use_id: string`

  - `type: "web_search_tool_result"`

    - `"web_search_tool_result"`

  - `cache_control: optional CacheControlEphemeral`

    Create a cache control breakpoint at this content block.

    - `type: "ephemeral"`

      - `"ephemeral"`

    - `ttl: optional "5m" or "1h"`

      The time-to-live for the cache control breakpoint.

      This may be one the following values:

      - `5m`: 5 minutes
      - `1h`: 1 hour

      Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

      - `"5m"`

      - `"1h"`

  - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

    Tool invocation directly from the model.

    - `DirectCaller object { type }`

      Tool invocation directly from the model.

      - `type: "direct"`

        - `"direct"`

    - `ServerToolCaller object { tool_id, type }`

      Tool invocation generated by a server-side tool.

      - `tool_id: string`

      - `type: "code_execution_20250825"`

        - `"code_execution_20250825"`

    - `ServerToolCaller20260120 object { tool_id, type }`

      - `tool_id: string`

      - `type: "code_execution_20260120"`

        - `"code_execution_20260120"`

### Web Search Tool Result Block Param Content

- `WebSearchToolResultBlockParamContent = array of WebSearchResultBlockParam or WebSearchToolRequestError`

  - `WebSearchToolResultBlockItem = array of WebSearchResultBlockParam`

    - `encrypted_content: string`

    - `title: string`

    - `type: "web_search_result"`

      - `"web_search_result"`

    - `url: string`

    - `page_age: optional string`

  - `WebSearchToolRequestError object { error_code, type }`

    - `error_code: WebSearchToolResultErrorCode`

      - `"invalid_tool_input"`

      - `"unavailable"`

      - `"max_uses_exceeded"`

      - `"too_many_requests"`

      - `"query_too_long"`

      - `"request_too_large"`

    - `type: "web_search_tool_result_error"`

      - `"web_search_tool_result_error"`

### Web Search Tool Result Error

- `WebSearchToolResultError object { error_code, type }`

  - `error_code: WebSearchToolResultErrorCode`

    - `"invalid_tool_input"`

    - `"unavailable"`

    - `"max_uses_exceeded"`

    - `"too_many_requests"`

    - `"query_too_long"`

    - `"request_too_large"`

  - `type: "web_search_tool_result_error"`

    - `"web_search_tool_result_error"`

### Web Search Tool Result Error Code

- `WebSearchToolResultErrorCode = "invalid_tool_input" or "unavailable" or "max_uses_exceeded" or 3 more`

  - `"invalid_tool_input"`

  - `"unavailable"`

  - `"max_uses_exceeded"`

  - `"too_many_requests"`

  - `"query_too_long"`

  - `"request_too_large"`

# Batches

## Create a Message Batch

**post** `/v1/messages/batches`

Send a batch of Message creation requests.

The Message Batches API can be used to process multiple Messages API requests at once. Once a Message Batch is created, it begins processing immediately. Batches can take up to 24 hours to complete.

Learn more about the Message Batches API in our [user guide](https://platform.claude.com/docs/en/build-with-claude/batch-processing)

### Header Parameters

- `"anthropic-user-profile-id": optional string`

  The user profile ID to attribute the requests in this batch to. Use when acting on behalf of a party other than your organization. Requires the `user-profiles` beta header. Applies to every request in the batch; an individual request whose `user_profile_id` body field conflicts with this header is errored.

### Body Parameters

- `requests: array of object { custom_id, params }`

  List of requests for prompt completion. Each is an individual request to create a Message.

  - `custom_id: string`

    Developer-provided ID created for each request in a Message Batch. Useful for matching results to requests, as results may be given out of request order.

    Must be unique for each request within the Message Batch.

  - `params: object { max_tokens, messages, model, 15 more }`

    Messages API creation parameters for the individual request.

    See the [Messages API reference](https://platform.claude.com/docs/en/api/messages) for full documentation on available parameters.

    - `max_tokens: number`

      The maximum number of tokens to generate before stopping.

      Note that our models may stop _before_ reaching this maximum. This parameter only specifies the absolute maximum number of tokens to generate.

      Set to `0` to populate the [prompt cache](https://platform.claude.com/docs/en/build-with-claude/prompt-caching#pre-warming-the-cache) without generating a response.

      Different models have different maximum values for this parameter.  See [models](https://platform.claude.com/docs/en/about-claude/models/overview) for details.

    - `messages: array of MessageParam`

      Input messages.

      Our models are trained to operate on alternating `user` and `assistant` conversational turns. When creating a new `Message`, you specify the prior conversational turns with the `messages` parameter, and the model then generates the next `Message` in the conversation. Consecutive `user` or `assistant` turns in your request will be combined into a single turn.

      Each input message must be an object with a `role` and `content`. You can specify a single `user`-role message, or you can include multiple `user` and `assistant` messages.

      If the final message uses the `assistant` role, the response content will continue immediately from the content in that message. This can be used to constrain part of the model's response.

      Example with a single `user` message:

      ```json
      [{"role": "user", "content": "Hello, Claude"}]
      ```

      Example with multiple conversational turns:

      ```json
      [
        {"role": "user", "content": "Hello there."},
        {"role": "assistant", "content": "Hi, I'm Claude. How can I help you?"},
        {"role": "user", "content": "Can you explain LLMs in plain English?"},
      ]
      ```

      Example with a partially-filled response from Claude:

      ```json
      [
        {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
        {"role": "assistant", "content": "The best answer is ("},
      ]
      ```

      Each input message `content` may be either a single `string` or an array of content blocks, where each block has a specific `type`. Using a `string` for `content` is shorthand for an array of one content block of type `"text"`. The following input messages are equivalent:

      ```json
      {"role": "user", "content": "Hello, Claude"}
      ```

      ```json
      {"role": "user", "content": [{"type": "text", "text": "Hello, Claude"}]}
      ```

      See [input examples](https://platform.claude.com/docs/en/build-with-claude/working-with-messages).

      Note that if you want to include a [system prompt](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#give-claude-a-role), you can use the top-level `system` parameter — there is no `"system"` role for input messages in the Messages API.

      There is a limit of 100,000 messages in a single request.

      - `content: string or array of ContentBlockParam`

        - `string`

        - `array of ContentBlockParam`

          - `TextBlockParam object { text, type, cache_control, citations }`

            - `text: string`

            - `type: "text"`

              - `"text"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

              - `type: "ephemeral"`

                - `"ephemeral"`

              - `ttl: optional "5m" or "1h"`

                The time-to-live for the cache control breakpoint.

                This may be one the following values:

                - `5m`: 5 minutes
                - `1h`: 1 hour

                Defaults to `5m`. See [prompt caching pricing](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for details.

                - `"5m"`

                - `"1h"`

            - `citations: optional array of TextCitationParam`

              - `CitationCharLocationParam object { cited_text, document_index, document_title, 3 more }`

                - `cited_text: string`

                - `document_index: number`

                - `document_title: string`

                - `end_char_index: number`

                - `start_char_index: number`

                - `type: "char_location"`

                  - `"char_location"`

              - `CitationPageLocationParam object { cited_text, document_index, document_title, 3 more }`

                - `cited_text: string`

                - `document_index: number`

                - `document_title: string`

                - `end_page_number: number`

                - `start_page_number: number`

                - `type: "page_location"`

                  - `"page_location"`

              - `CitationContentBlockLocationParam object { cited_text, document_index, document_title, 3 more }`

                - `cited_text: string`

                  The full text of the cited block range, concatenated.

                  Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

                - `document_index: number`

                - `document_title: string`

                - `end_block_index: number`

                  Exclusive 0-based end index of the cited block range in the source's `content` array.

                  Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

                - `start_block_index: number`

                  0-based index of the first cited block in the source's `content` array.

                - `type: "content_block_location"`

                  - `"content_block_location"`

              - `CitationWebSearchResultLocationParam object { cited_text, encrypted_index, title, 2 more }`

                - `cited_text: string`

                - `encrypted_index: string`

                - `title: string`

                - `type: "web_search_result_location"`

                  - `"web_search_result_location"`

                - `url: string`

              - `CitationSearchResultLocationParam object { cited_text, end_block_index, search_result_index, 4 more }`

                - `cited_text: string`

                  The full text of the cited block range, concatenated.

                  Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

                - `end_block_index: number`

                  Exclusive 0-based end index of the cited block range in the source's `content` array.

                  Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

                - `search_result_index: number`

                  0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

                  Counted separately from `document_index`; server-side web search results are not included in this count.

                - `source: string`

                - `start_block_index: number`

                  0-based index of the first cited block in the source's `content` array.

                - `title: string`

                - `type: "search_result_location"`

                  - `"search_result_location"`

          - `ImageBlockParam object { source, type, cache_control }`

            - `source: Base64ImageSource or URLImageSource`

              - `Base64ImageSource object { data, media_type, type }`

                - `data: string`

                - `media_type: "image/jpeg" or "image/png" or "image/gif" or "image/webp"`

                  - `"image/jpeg"`

                  - `"image/png"`

                  - `"image/gif"`

                  - `"image/webp"`

                - `type: "base64"`

                  - `"base64"`

              - `URLImageSource object { type, url }`

                - `type: "url"`

                  - `"url"`

                - `url: string`

            - `type: "image"`

              - `"image"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

          - `DocumentBlockParam object { source, type, cache_control, 3 more }`

            - `source: Base64PDFSource or PlainTextSource or ContentBlockSource or URLPDFSource`

              - `Base64PDFSource object { data, media_type, type }`

                - `data: string`

                - `media_type: "application/pdf"`

                  - `"application/pdf"`

                - `type: "base64"`

                  - `"base64"`

              - `PlainTextSource object { data, media_type, type }`

                - `data: string`

                - `media_type: "text/plain"`

                  - `"text/plain"`

                - `type: "text"`

                  - `"text"`

              - `ContentBlockSource object { content, type }`

                - `content: string or array of ContentBlockSourceContent`

                  - `string`

                  - `ContentBlockSourceContent = array of ContentBlockSourceContent`

                    - `TextBlockParam object { text, type, cache_control, citations }`

                    - `ImageBlockParam object { source, type, cache_control }`

                - `type: "content"`

                  - `"content"`

              - `URLPDFSource object { type, url }`

                - `type: "url"`

                  - `"url"`

                - `url: string`

            - `type: "document"`

              - `"document"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

            - `citations: optional CitationsConfigParam`

              - `enabled: optional boolean`

            - `context: optional string`

            - `title: optional string`

          - `SearchResultBlockParam object { content, source, title, 3 more }`

            - `content: array of TextBlockParam`

              - `text: string`

              - `type: "text"`

              - `cache_control: optional CacheControlEphemeral`

                Create a cache control breakpoint at this content block.

              - `citations: optional array of TextCitationParam`

            - `source: string`

            - `title: string`

            - `type: "search_result"`

              - `"search_result"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

            - `citations: optional CitationsConfigParam`

          - `ThinkingBlockParam object { signature, thinking, type }`

            - `signature: string`

            - `thinking: string`

            - `type: "thinking"`

              - `"thinking"`

          - `RedactedThinkingBlockParam object { data, type }`

            - `data: string`

            - `type: "redacted_thinking"`

              - `"redacted_thinking"`

          - `ToolUseBlockParam object { id, input, name, 3 more }`

            - `id: string`

            - `input: map[unknown]`

            - `name: string`

            - `type: "tool_use"`

              - `"tool_use"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

            - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

              Tool invocation directly from the model.

              - `DirectCaller object { type }`

                Tool invocation directly from the model.

                - `type: "direct"`

                  - `"direct"`

              - `ServerToolCaller object { tool_id, type }`

                Tool invocation generated by a server-side tool.

                - `tool_id: string`

                - `type: "code_execution_20250825"`

                  - `"code_execution_20250825"`

              - `ServerToolCaller20260120 object { tool_id, type }`

                - `tool_id: string`

                - `type: "code_execution_20260120"`

                  - `"code_execution_20260120"`

          - `ToolResultBlockParam object { tool_use_id, type, cache_control, 2 more }`

            - `tool_use_id: string`

            - `type: "tool_result"`

              - `"tool_result"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

            - `content: optional string or array of TextBlockParam or ImageBlockParam or SearchResultBlockParam or 2 more`

              - `string`

              - `array of TextBlockParam or ImageBlockParam or SearchResultBlockParam or 2 more`

                - `TextBlockParam object { text, type, cache_control, citations }`

                - `ImageBlockParam object { source, type, cache_control }`

                - `SearchResultBlockParam object { content, source, title, 3 more }`

                - `DocumentBlockParam object { source, type, cache_control, 3 more }`

                - `ToolReferenceBlockParam object { tool_name, type, cache_control }`

                  Tool reference block that can be included in tool_result content.

                  - `tool_name: string`

                  - `type: "tool_reference"`

                    - `"tool_reference"`

                  - `cache_control: optional CacheControlEphemeral`

                    Create a cache control breakpoint at this content block.

            - `is_error: optional boolean`

          - `ServerToolUseBlockParam object { id, input, name, 3 more }`

            - `id: string`

            - `input: map[unknown]`

            - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

              - `"web_search"`

              - `"web_fetch"`

              - `"code_execution"`

              - `"bash_code_execution"`

              - `"text_editor_code_execution"`

              - `"tool_search_tool_regex"`

              - `"tool_search_tool_bm25"`

            - `type: "server_tool_use"`

              - `"server_tool_use"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

            - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

              Tool invocation directly from the model.

              - `DirectCaller object { type }`

                Tool invocation directly from the model.

              - `ServerToolCaller object { tool_id, type }`

                Tool invocation generated by a server-side tool.

              - `ServerToolCaller20260120 object { tool_id, type }`

          - `WebSearchToolResultBlockParam object { content, tool_use_id, type, 2 more }`

            - `content: WebSearchToolResultBlockParamContent`

              - `WebSearchToolResultBlockItem = array of WebSearchResultBlockParam`

                - `encrypted_content: string`

                - `title: string`

                - `type: "web_search_result"`

                  - `"web_search_result"`

                - `url: string`

                - `page_age: optional string`

              - `WebSearchToolRequestError object { error_code, type }`

                - `error_code: WebSearchToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"max_uses_exceeded"`

                  - `"too_many_requests"`

                  - `"query_too_long"`

                  - `"request_too_large"`

                - `type: "web_search_tool_result_error"`

                  - `"web_search_tool_result_error"`

            - `tool_use_id: string`

            - `type: "web_search_tool_result"`

              - `"web_search_tool_result"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

            - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

              Tool invocation directly from the model.

              - `DirectCaller object { type }`

                Tool invocation directly from the model.

              - `ServerToolCaller object { tool_id, type }`

                Tool invocation generated by a server-side tool.

              - `ServerToolCaller20260120 object { tool_id, type }`

          - `WebFetchToolResultBlockParam object { content, tool_use_id, type, 2 more }`

            - `content: WebFetchToolResultErrorBlockParam or WebFetchBlockParam`

              - `WebFetchToolResultErrorBlockParam object { error_code, type }`

                - `error_code: WebFetchToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"url_too_long"`

                  - `"url_not_allowed"`

                  - `"url_not_in_prior_context"`

                  - `"url_not_accessible"`

                  - `"unsupported_content_type"`

                  - `"too_many_requests"`

                  - `"max_uses_exceeded"`

                  - `"unavailable"`

                - `type: "web_fetch_tool_result_error"`

                  - `"web_fetch_tool_result_error"`

              - `WebFetchBlockParam object { content, type, url, retrieved_at }`

                - `content: DocumentBlockParam`

                - `type: "web_fetch_result"`

                  - `"web_fetch_result"`

                - `url: string`

                  Fetched content URL

                - `retrieved_at: optional string`

                  ISO 8601 timestamp when the content was retrieved

            - `tool_use_id: string`

            - `type: "web_fetch_tool_result"`

              - `"web_fetch_tool_result"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

            - `caller: optional DirectCaller or ServerToolCaller or ServerToolCaller20260120`

              Tool invocation directly from the model.

              - `DirectCaller object { type }`

                Tool invocation directly from the model.

              - `ServerToolCaller object { tool_id, type }`

                Tool invocation generated by a server-side tool.

              - `ServerToolCaller20260120 object { tool_id, type }`

          - `CodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

            - `content: CodeExecutionToolResultBlockParamContent`

              Code execution result with encrypted stdout for PFC + web_search results.

              - `CodeExecutionToolResultErrorParam object { error_code, type }`

                - `error_code: CodeExecutionToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"too_many_requests"`

                  - `"execution_time_exceeded"`

                - `type: "code_execution_tool_result_error"`

                  - `"code_execution_tool_result_error"`

              - `CodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

                - `content: array of CodeExecutionOutputBlockParam`

                  - `file_id: string`

                  - `type: "code_execution_output"`

                    - `"code_execution_output"`

                - `return_code: number`

                - `stderr: string`

                - `stdout: string`

                - `type: "code_execution_result"`

                  - `"code_execution_result"`

              - `EncryptedCodeExecutionResultBlockParam object { content, encrypted_stdout, return_code, 2 more }`

                Code execution result with encrypted stdout for PFC + web_search results.

                - `content: array of CodeExecutionOutputBlockParam`

                  - `file_id: string`

                  - `type: "code_execution_output"`

                - `encrypted_stdout: string`

                - `return_code: number`

                - `stderr: string`

                - `type: "encrypted_code_execution_result"`

                  - `"encrypted_code_execution_result"`

            - `tool_use_id: string`

            - `type: "code_execution_tool_result"`

              - `"code_execution_tool_result"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

          - `BashCodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

            - `content: BashCodeExecutionToolResultErrorParam or BashCodeExecutionResultBlockParam`

              - `BashCodeExecutionToolResultErrorParam object { error_code, type }`

                - `error_code: BashCodeExecutionToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"too_many_requests"`

                  - `"execution_time_exceeded"`

                  - `"output_file_too_large"`

                - `type: "bash_code_execution_tool_result_error"`

                  - `"bash_code_execution_tool_result_error"`

              - `BashCodeExecutionResultBlockParam object { content, return_code, stderr, 2 more }`

                - `content: array of BashCodeExecutionOutputBlockParam`

                  - `file_id: string`

                  - `type: "bash_code_execution_output"`

                    - `"bash_code_execution_output"`

                - `return_code: number`

                - `stderr: string`

                - `stdout: string`

                - `type: "bash_code_execution_result"`

                  - `"bash_code_execution_result"`

            - `tool_use_id: string`

            - `type: "bash_code_execution_tool_result"`

              - `"bash_code_execution_tool_result"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

          - `TextEditorCodeExecutionToolResultBlockParam object { content, tool_use_id, type, cache_control }`

            - `content: TextEditorCodeExecutionToolResultErrorParam or TextEditorCodeExecutionViewResultBlockParam or TextEditorCodeExecutionCreateResultBlockParam or TextEditorCodeExecutionStrReplaceResultBlockParam`

              - `TextEditorCodeExecutionToolResultErrorParam object { error_code, type, error_message }`

                - `error_code: TextEditorCodeExecutionToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"too_many_requests"`

                  - `"execution_time_exceeded"`

                  - `"file_not_found"`

                - `type: "text_editor_code_execution_tool_result_error"`

                  - `"text_editor_code_execution_tool_result_error"`

                - `error_message: optional string`

              - `TextEditorCodeExecutionViewResultBlockParam object { content, file_type, type, 3 more }`

                - `content: string`

                - `file_type: "text" or "image" or "pdf"`

                  - `"text"`

                  - `"image"`

                  - `"pdf"`

                - `type: "text_editor_code_execution_view_result"`

                  - `"text_editor_code_execution_view_result"`

                - `num_lines: optional number`

                - `start_line: optional number`

                - `total_lines: optional number`

              - `TextEditorCodeExecutionCreateResultBlockParam object { is_file_update, type }`

                - `is_file_update: boolean`

                - `type: "text_editor_code_execution_create_result"`

                  - `"text_editor_code_execution_create_result"`

              - `TextEditorCodeExecutionStrReplaceResultBlockParam object { type, lines, new_lines, 3 more }`

                - `type: "text_editor_code_execution_str_replace_result"`

                  - `"text_editor_code_execution_str_replace_result"`

                - `lines: optional array of string`

                - `new_lines: optional number`

                - `new_start: optional number`

                - `old_lines: optional number`

                - `old_start: optional number`

            - `tool_use_id: string`

            - `type: "text_editor_code_execution_tool_result"`

              - `"text_editor_code_execution_tool_result"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

          - `ToolSearchToolResultBlockParam object { content, tool_use_id, type, cache_control }`

            - `content: ToolSearchToolResultErrorParam or ToolSearchToolSearchResultBlockParam`

              - `ToolSearchToolResultErrorParam object { error_code, type, error_message }`

                - `error_code: ToolSearchToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"too_many_requests"`

                  - `"execution_time_exceeded"`

                - `type: "tool_search_tool_result_error"`

                  - `"tool_search_tool_result_error"`

                - `error_message: optional string`

              - `ToolSearchToolSearchResultBlockParam object { tool_references, type }`

                - `tool_references: array of ToolReferenceBlockParam`

                  - `tool_name: string`

                  - `type: "tool_reference"`

                  - `cache_control: optional CacheControlEphemeral`

                    Create a cache control breakpoint at this content block.

                - `type: "tool_search_tool_search_result"`

                  - `"tool_search_tool_search_result"`

            - `tool_use_id: string`

            - `type: "tool_search_tool_result"`

              - `"tool_search_tool_result"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

          - `ContainerUploadBlockParam object { file_id, type, cache_control }`

            A content block that represents a file to be uploaded to the container
            Files uploaded via this block will be available in the container's input directory.

            - `file_id: string`

            - `type: "container_upload"`

              - `"container_upload"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

          - `MidConversationSystemBlockParam object { content, type, cache_control }`

            System instructions that appear mid-conversation.

            Use this block to provide or update system-level instructions at a specific
            point in the conversation, rather than only via the top-level `system` parameter.

            - `content: array of TextBlockParam`

              System instruction text blocks.

              - `text: string`

              - `type: "text"`

              - `cache_control: optional CacheControlEphemeral`

                Create a cache control breakpoint at this content block.

              - `citations: optional array of TextCitationParam`

            - `type: "mid_conv_system"`

              - `"mid_conv_system"`

            - `cache_control: optional CacheControlEphemeral`

              Create a cache control breakpoint at this content block.

      - `role: "user" or "assistant" or "system"`

        - `"user"`

        - `"assistant"`

        - `"system"`

    - `model: Model`

      The model that will complete your prompt.

      See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

      - `"claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more`

        The model that will complete your prompt.

        See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

        - `"claude-sonnet-5"`

          High-performance model for coding and agents

        - `"claude-fable-5"`

          Next generation of intelligence for the hardest knowledge work and coding problems

        - `"claude-mythos-5"`

          Most capable model for cybersecurity and biology research

        - `"claude-opus-4-8"`

          Frontier intelligence for long-running agents and coding

        - `"claude-opus-4-7"`

          Frontier intelligence for long-running agents and coding

        - `"claude-mythos-preview"`

          New class of intelligence, strongest in coding and cybersecurity

        - `"claude-opus-4-6"`

          Frontier intelligence for long-running agents and coding

        - `"claude-sonnet-4-6"`

          Best combination of speed and intelligence

        - `"claude-haiku-4-5"`

          Fastest model with near-frontier intelligence

        - `"claude-haiku-4-5-20251001"`

          Fastest model with near-frontier intelligence

        - `"claude-opus-4-5"`

          Premium model combining maximum intelligence with practical performance

        - `"claude-opus-4-5-20251101"`

          Premium model combining maximum intelligence with practical performance

        - `"claude-sonnet-4-5"`

          High-performance model for agents and coding

        - `"claude-sonnet-4-5-20250929"`

          High-performance model for agents and coding

        - `"claude-opus-4-1"`

          Exceptional model for specialized complex tasks

        - `"claude-opus-4-1-20250805"`

          Exceptional model for specialized complex tasks

      - `string`

    - `cache_control: optional CacheControlEphemeral`

      Top-level cache control automatically applies a cache_control marker to the last cacheable block in the request.

    - `container: optional string`

      Container identifier for reuse across requests.

    - `inference_geo: optional string`

      Specifies the geographic region for inference processing. If not specified, the workspace's `default_inference_geo` is used.

    - `metadata: optional Metadata`

      An object describing metadata about the request.

      - `user_id: optional string`

        An external identifier for the user who is associated with the request.

        This should be a uuid, hash value, or other opaque identifier. Anthropic may use this id to help detect abuse. Do not include any identifying information such as name, email address, or phone number.

    - `output_config: optional OutputConfig`

      Configuration options for the model's output, such as the output format.

      - `effort: optional "low" or "medium" or "high" or 2 more`

        All possible effort levels.

        - `"low"`

        - `"medium"`

        - `"high"`

        - `"xhigh"`

        - `"max"`

      - `format: optional JSONOutputFormat`

        A schema to specify Claude's output format in responses. See [structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)

        - `schema: map[unknown]`

          The JSON schema of the format

        - `type: "json_schema"`

          - `"json_schema"`

    - `service_tier: optional "auto" or "standard_only"`

      Determines whether to use priority capacity (if available) or standard capacity for this request.

      Anthropic offers different levels of service for your API requests. See [service-tiers](https://platform.claude.com/docs/en/api/service-tiers) for details.

      - `"auto"`

      - `"standard_only"`

    - `stop_sequences: optional array of string`

      Custom text sequences that will cause the model to stop generating.

      Our models will normally stop when they have naturally completed their turn, which will result in a response `stop_reason` of `"end_turn"`.

      If you want the model to stop generating when it encounters custom strings of text, you can use the `stop_sequences` parameter. If the model encounters one of the custom sequences, the response `stop_reason` value will be `"stop_sequence"` and the response `stop_sequence` value will contain the matched stop sequence.

    - `stream: optional boolean`

      Whether to incrementally stream the response using server-sent events.

      See [streaming](https://platform.claude.com/docs/en/build-with-claude/streaming) for details.

    - `system: optional string or array of TextBlockParam`

      System prompt.

      A system prompt is a way of providing context and instructions to Claude, such as specifying a particular goal or role. See our [guide to system prompts](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#give-claude-a-role).

      - `string`

      - `array of TextBlockParam`

        - `text: string`

        - `type: "text"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional array of TextCitationParam`

    - `temperature: optional number`

      Amount of randomness injected into the response.

      Defaults to `1.0`. Ranges from `0.0` to `1.0`. Use `temperature` closer to `0.0` for analytical / multiple choice, and closer to `1.0` for creative and generative tasks.

      Note that even with `temperature` of `0.0`, the results will not be fully deterministic.

    - `thinking: optional ThinkingConfigParam`

      Configuration for enabling Claude's extended thinking.

      When enabled, responses include `thinking` content blocks showing Claude's thinking process before the final answer. Requires a minimum budget of 1,024 tokens and counts towards your `max_tokens` limit.

      See [extended thinking](https://platform.claude.com/docs/en/build-with-claude/extended-thinking) for details.

      - `ThinkingConfigEnabled object { budget_tokens, type, display }`

        - `budget_tokens: number`

          Determines how many tokens Claude can use for its internal reasoning process. Larger budgets can enable more thorough analysis for complex problems, improving response quality.

          Must be ≥1024 and less than `max_tokens`.

          See [extended thinking](https://platform.claude.com/docs/en/build-with-claude/extended-thinking) for details.

        - `type: "enabled"`

          - `"enabled"`

        - `display: optional "summarized" or "omitted"`

          Controls how thinking content appears in the response. When set to `summarized`, thinking is returned normally. When set to `omitted`, thinking content is redacted but a signature is returned for multi-turn continuity. Defaults to `summarized`.

          - `"summarized"`

          - `"omitted"`

      - `ThinkingConfigDisabled object { type }`

        - `type: "disabled"`

          - `"disabled"`

      - `ThinkingConfigAdaptive object { type, display }`

        - `type: "adaptive"`

          - `"adaptive"`

        - `display: optional "summarized" or "omitted"`

          Controls how thinking content appears in the response. When set to `summarized`, thinking is returned normally. When set to `omitted`, thinking content is redacted but a signature is returned for multi-turn continuity. Defaults to `summarized`.

          - `"summarized"`

          - `"omitted"`

    - `tool_choice: optional ToolChoice`

      How the model should use the provided tools. The model can use a specific tool, any available tool, decide by itself, or not use tools at all.

      - `ToolChoiceAuto object { type, disable_parallel_tool_use }`

        The model will automatically decide whether to use tools.

        - `type: "auto"`

          - `"auto"`

        - `disable_parallel_tool_use: optional boolean`

          Whether to disable parallel tool use.

          Defaults to `false`. If set to `true`, the model will output at most one tool use.

      - `ToolChoiceAny object { type, disable_parallel_tool_use }`

        The model will use any available tools.

        - `type: "any"`

          - `"any"`

        - `disable_parallel_tool_use: optional boolean`

          Whether to disable parallel tool use.

          Defaults to `false`. If set to `true`, the model will output exactly one tool use.

      - `ToolChoiceTool object { name, type, disable_parallel_tool_use }`

        The model will use the specified tool with `tool_choice.name`.

        - `name: string`

          The name of the tool to use.

        - `type: "tool"`

          - `"tool"`

        - `disable_parallel_tool_use: optional boolean`

          Whether to disable parallel tool use.

          Defaults to `false`. If set to `true`, the model will output exactly one tool use.

      - `ToolChoiceNone object { type }`

        The model will not be allowed to use tools.

        - `type: "none"`

          - `"none"`

    - `tools: optional array of ToolUnion`

      Definitions of tools that the model may use.

      If you include `tools` in your API request, the model may return `tool_use` content blocks that represent the model's use of those tools. You can then run those tools using the tool input generated by the model and then optionally return results back to the model using `tool_result` content blocks.

      There are two types of tools: **client tools** and **server tools**. The behavior described below applies to client tools. For [server tools](https://platform.claude.com/docs/en/agents-and-tools/tool-use/server-tools), see their individual documentation as each has its own behavior (e.g., the [web search tool](https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-search-tool)).

      Each tool definition includes:

      * `name`: Name of the tool.
      * `description`: Optional, but strongly-recommended description of the tool.
      * `input_schema`: [JSON schema](https://json-schema.org/draft/2020-12) for the tool `input` shape that the model will produce in `tool_use` output content blocks.

      For example, if you defined `tools` as:

      ```json
      [
        {
          "name": "get_stock_price",
          "description": "Get the current stock price for a given ticker symbol.",
          "input_schema": {
            "type": "object",
            "properties": {
              "ticker": {
                "type": "string",
                "description": "The stock ticker symbol, e.g. AAPL for Apple Inc."
              }
            },
            "required": ["ticker"]
          }
        }
      ]
      ```

      And then asked the model "What's the S&P 500 at today?", the model might produce `tool_use` content blocks in the response like this:

      ```json
      [
        {
          "type": "tool_use",
          "id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
          "name": "get_stock_price",
          "input": { "ticker": "^GSPC" }
        }
      ]
      ```

      You might then run your `get_stock_price` tool with `{"ticker": "^GSPC"}` as an input, and return the following back to the model in a subsequent `user` message:

      ```json
      [
        {
          "type": "tool_result",
          "tool_use_id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
          "content": "259.75 USD"
        }
      ]
      ```

      Tools can be used for workflows that include running client-side tools and functions, or more generally whenever you want the model to produce a particular JSON structure of output.

      See our [guide](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview) for more details.

      - `Tool object { input_schema, name, allowed_callers, 7 more }`

        - `input_schema: object { type, properties, required }`

          [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.

          This defines the shape of the `input` that your tool accepts and that the model will produce.

          - `type: "object"`

            - `"object"`

          - `properties: optional map[unknown]`

          - `required: optional array of string`

        - `name: string`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `description: optional string`

          Description of what this tool does.

          Tool descriptions should be as detailed as possible. The more information that the model has about what the tool is and how to use it, the better it will perform. You can use natural language descriptions to reinforce important aspects of the tool input JSON schema.

        - `eager_input_streaming: optional boolean`

          Enable eager input streaming for this tool. When true, tool input parameters will be streamed incrementally as they are generated, and types will be inferred on-the-fly rather than buffering the full JSON output. When false, streaming is disabled for this tool even if the fine-grained-tool-streaming beta is active. When null (default), uses the default behavior based on beta headers.

        - `input_examples: optional array of map[unknown]`

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

        - `type: optional "custom"`

          - `"custom"`

      - `ToolBash20250124 object { name, type, allowed_callers, 4 more }`

        - `name: "bash"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"bash"`

        - `type: "bash_20250124"`

          - `"bash_20250124"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `input_examples: optional array of map[unknown]`

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

      - `CodeExecutionTool20250522 object { name, type, allowed_callers, 3 more }`

        - `name: "code_execution"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"code_execution"`

        - `type: "code_execution_20250522"`

          - `"code_execution_20250522"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

      - `CodeExecutionTool20250825 object { name, type, allowed_callers, 3 more }`

        - `name: "code_execution"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"code_execution"`

        - `type: "code_execution_20250825"`

          - `"code_execution_20250825"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

      - `CodeExecutionTool20260120 object { name, type, allowed_callers, 3 more }`

        Code execution tool with REPL state persistence (daemon mode + gVisor checkpoint).

        - `name: "code_execution"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"code_execution"`

        - `type: "code_execution_20260120"`

          - `"code_execution_20260120"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

      - `CodeExecutionTool20260521 object { name, type, allowed_callers, 3 more }`

        Code execution tool with REPL state persistence.

        - `name: "code_execution"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"code_execution"`

        - `type: "code_execution_20260521"`

          - `"code_execution_20260521"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

      - `MemoryTool20250818 object { name, type, allowed_callers, 4 more }`

        - `name: "memory"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"memory"`

        - `type: "memory_20250818"`

          - `"memory_20250818"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `input_examples: optional array of map[unknown]`

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

      - `ToolTextEditor20250124 object { name, type, allowed_callers, 4 more }`

        - `name: "str_replace_editor"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"str_replace_editor"`

        - `type: "text_editor_20250124"`

          - `"text_editor_20250124"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `input_examples: optional array of map[unknown]`

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

      - `ToolTextEditor20250429 object { name, type, allowed_callers, 4 more }`

        - `name: "str_replace_based_edit_tool"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"str_replace_based_edit_tool"`

        - `type: "text_editor_20250429"`

          - `"text_editor_20250429"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `input_examples: optional array of map[unknown]`

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

      - `ToolTextEditor20250728 object { name, type, allowed_callers, 5 more }`

        - `name: "str_replace_based_edit_tool"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"str_replace_based_edit_tool"`

        - `type: "text_editor_20250728"`

          - `"text_editor_20250728"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `input_examples: optional array of map[unknown]`

        - `max_characters: optional number`

          Maximum number of characters to display when viewing a file. If not specified, defaults to displaying the full file.

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

      - `WebSearchTool20250305 object { name, type, allowed_callers, 7 more }`

        - `name: "web_search"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"web_search"`

        - `type: "web_search_20250305"`

          - `"web_search_20250305"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `allowed_domains: optional array of string`

          If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

        - `blocked_domains: optional array of string`

          If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `max_uses: optional number`

          Maximum number of times the tool can be used in the API request.

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

        - `user_location: optional UserLocation`

          Parameters for the user's location. Used to provide more relevant search results.

          - `type: "approximate"`

            - `"approximate"`

          - `city: optional string`

            The city of the user.

          - `country: optional string`

            The two letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the user.

          - `region: optional string`

            The region of the user.

          - `timezone: optional string`

            The [IANA timezone](https://nodatime.org/TimeZones) of the user.

      - `WebFetchTool20250910 object { name, type, allowed_callers, 8 more }`

        - `name: "web_fetch"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"web_fetch"`

        - `type: "web_fetch_20250910"`

          - `"web_fetch_20250910"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `allowed_domains: optional array of string`

          List of domains to allow fetching from

        - `blocked_domains: optional array of string`

          List of domains to block fetching from

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

          Citations configuration for fetched documents. Citations are disabled by default.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `max_content_tokens: optional number`

          Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

        - `max_uses: optional number`

          Maximum number of times the tool can be used in the API request.

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

      - `WebSearchTool20260209 object { name, type, allowed_callers, 7 more }`

        - `name: "web_search"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"web_search"`

        - `type: "web_search_20260209"`

          - `"web_search_20260209"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `allowed_domains: optional array of string`

          If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

        - `blocked_domains: optional array of string`

          If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `max_uses: optional number`

          Maximum number of times the tool can be used in the API request.

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

        - `user_location: optional UserLocation`

          Parameters for the user's location. Used to provide more relevant search results.

      - `WebFetchTool20260209 object { name, type, allowed_callers, 8 more }`

        - `name: "web_fetch"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"web_fetch"`

        - `type: "web_fetch_20260209"`

          - `"web_fetch_20260209"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `allowed_domains: optional array of string`

          List of domains to allow fetching from

        - `blocked_domains: optional array of string`

          List of domains to block fetching from

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

          Citations configuration for fetched documents. Citations are disabled by default.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `max_content_tokens: optional number`

          Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

        - `max_uses: optional number`

          Maximum number of times the tool can be used in the API request.

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

      - `WebFetchTool20260309 object { name, type, allowed_callers, 9 more }`

        Web fetch tool with use_cache parameter for bypassing cached content.

        - `name: "web_fetch"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"web_fetch"`

        - `type: "web_fetch_20260309"`

          - `"web_fetch_20260309"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `allowed_domains: optional array of string`

          List of domains to allow fetching from

        - `blocked_domains: optional array of string`

          List of domains to block fetching from

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

          Citations configuration for fetched documents. Citations are disabled by default.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `max_content_tokens: optional number`

          Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

        - `max_uses: optional number`

          Maximum number of times the tool can be used in the API request.

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

        - `use_cache: optional boolean`

          Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.

      - `WebSearchTool20260318 object { name, type, allowed_callers, 8 more }`

        - `name: "web_search"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"web_search"`

        - `type: "web_search_20260318"`

          - `"web_search_20260318"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `allowed_domains: optional array of string`

          If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.

        - `blocked_domains: optional array of string`

          If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `max_uses: optional number`

          Maximum number of times the tool can be used in the API request.

        - `response_inclusion: optional "full" or "excluded"`

          How this tool's result blocks appear in the API response when the result was consumed by a completed code_execution call in the same turn. 'full' returns the complete content (default). 'excluded' drops the nested server_tool_use and result block pair entirely. Results from direct calls, or from code_execution calls that paused before completing, are always returned in full so they can be sent back on the next turn.

          - `"full"`

          - `"excluded"`

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

        - `user_location: optional UserLocation`

          Parameters for the user's location. Used to provide more relevant search results.

      - `WebFetchTool20260318 object { name, type, allowed_callers, 10 more }`

        - `name: "web_fetch"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"web_fetch"`

        - `type: "web_fetch_20260318"`

          - `"web_fetch_20260318"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `allowed_domains: optional array of string`

          List of domains to allow fetching from

        - `blocked_domains: optional array of string`

          List of domains to block fetching from

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `citations: optional CitationsConfigParam`

          Citations configuration for fetched documents. Citations are disabled by default.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `max_content_tokens: optional number`

          Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.

        - `max_uses: optional number`

          Maximum number of times the tool can be used in the API request.

        - `response_inclusion: optional "full" or "excluded"`

          How this tool's result blocks appear in the API response when the result was consumed by a completed code_execution call in the same turn. 'full' returns the complete content (default). 'excluded' drops the nested server_tool_use and result block pair entirely. Results from direct calls, or from code_execution calls that paused before completing, are always returned in full so they can be sent back on the next turn.

          - `"full"`

          - `"excluded"`

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

        - `use_cache: optional boolean`

          Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.

      - `ToolSearchToolBm25_20251119 object { name, type, allowed_callers, 3 more }`

        - `name: "tool_search_tool_bm25"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"tool_search_tool_bm25"`

        - `type: "tool_search_tool_bm25_20251119" or "tool_search_tool_bm25"`

          - `"tool_search_tool_bm25_20251119"`

          - `"tool_search_tool_bm25"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

      - `ToolSearchToolRegex20251119 object { name, type, allowed_callers, 3 more }`

        - `name: "tool_search_tool_regex"`

          Name of the tool.

          This is how the tool will be called by the model and in `tool_use` blocks.

          - `"tool_search_tool_regex"`

        - `type: "tool_search_tool_regex_20251119" or "tool_search_tool_regex"`

          - `"tool_search_tool_regex_20251119"`

          - `"tool_search_tool_regex"`

        - `allowed_callers: optional array of "direct" or "code_execution_20250825" or "code_execution_20260120" or "code_execution_20260521"`

          - `"direct"`

          - `"code_execution_20250825"`

          - `"code_execution_20260120"`

          - `"code_execution_20260521"`

        - `cache_control: optional CacheControlEphemeral`

          Create a cache control breakpoint at this content block.

        - `defer_loading: optional boolean`

          If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.

        - `strict: optional boolean`

          When true, guarantees schema validation on tool names and inputs

    - `top_k: optional number`

      Only sample from the top K options for each subsequent token.

      Used to remove "long tail" low probability responses. [Learn more technical details here](https://towardsdatascience.com/how-to-sample-from-language-models-682bceb97277).

      Recommended for advanced use cases only.

    - `top_p: optional number`

      Use nucleus sampling.

      In nucleus sampling, we compute the cumulative distribution over all the options for each subsequent token in decreasing probability order and cut it off once it reaches a particular probability specified by `top_p`.

      Recommended for advanced use cases only.

### Returns

- `MessageBatch object { id, archived_at, cancel_initiated_at, 7 more }`

  - `id: string`

    Unique object identifier.

    The format and length of IDs may change over time.

  - `archived_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch was archived and its results became unavailable.

  - `cancel_initiated_at: string`

    RFC 3339 datetime string representing the time at which cancellation was initiated for the Message Batch. Specified only if cancellation was initiated.

  - `created_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch was created.

  - `ended_at: string`

    RFC 3339 datetime string representing the time at which processing for the Message Batch ended. Specified only once processing ends.

    Processing ends when every request in a Message Batch has either succeeded, errored, canceled, or expired.

  - `expires_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch will expire and end processing, which is 24 hours after creation.

  - `processing_status: "in_progress" or "canceling" or "ended"`

    Processing status of the Message Batch.

    - `"in_progress"`

    - `"canceling"`

    - `"ended"`

  - `request_counts: MessageBatchRequestCounts`

    Tallies requests within the Message Batch, categorized by their status.

    Requests start as `processing` and move to one of the other statuses only once processing of the entire batch ends. The sum of all values always matches the total number of requests in the batch.

    - `canceled: number`

      Number of requests in the Message Batch that have been canceled.

      This is zero until processing of the entire Message Batch has ended.

    - `errored: number`

      Number of requests in the Message Batch that encountered an error.

      This is zero until processing of the entire Message Batch has ended.

    - `expired: number`

      Number of requests in the Message Batch that have expired.

      This is zero until processing of the entire Message Batch has ended.

    - `processing: number`

      Number of requests in the Message Batch that are processing.

    - `succeeded: number`

      Number of requests in the Message Batch that have completed successfully.

      This is zero until processing of the entire Message Batch has ended.

  - `results_url: string`

    URL to a `.jsonl` file containing the results of the Message Batch requests. Specified only once processing ends.

    Results in the file are not guaranteed to be in the same order as requests. Use the `custom_id` field to match results to requests.

  - `type: "message_batch"`

    Object type.

    For Message Batches, this is always `"message_batch"`.

    - `"message_batch"`

### Example

```http
curl https://api.anthropic.com/v1/messages/batches \
    -H 'Content-Type: application/json' \
    -H 'anthropic-version: 2023-06-01' \
    -H "X-Api-Key: $ANTHROPIC_API_KEY" \
    -d '{
          "requests": [
            {
              "custom_id": "my-custom-id-1",
              "params": {
                "max_tokens": 1024,
                "messages": [
                  {
                    "content": "Hello, world",
                    "role": "user"
                  }
                ],
                "model": "claude-opus-4-6"
              }
            }
          ]
        }'
```

#### Response

```json
{
  "id": "msgbatch_013Zva2CMHLNnXjNJJKqJ2EF",
  "archived_at": "2024-08-20T18:37:24.100435Z",
  "cancel_initiated_at": "2024-08-20T18:37:24.100435Z",
  "created_at": "2024-08-20T18:37:24.100435Z",
  "ended_at": "2024-08-20T18:37:24.100435Z",
  "expires_at": "2024-08-20T18:37:24.100435Z",
  "processing_status": "in_progress",
  "request_counts": {
    "canceled": 10,
    "errored": 30,
    "expired": 10,
    "processing": 100,
    "succeeded": 50
  },
  "results_url": "https://api.anthropic.com/v1/messages/batches/msgbatch_013Zva2CMHLNnXjNJJKqJ2EF/results",
  "type": "message_batch"
}
```

## Retrieve a Message Batch

**get** `/v1/messages/batches/{message_batch_id}`

This endpoint is idempotent and can be used to poll for Message Batch completion. To access the results of a Message Batch, make a request to the `results_url` field in the response.

Learn more about the Message Batches API in our [user guide](https://platform.claude.com/docs/en/build-with-claude/batch-processing)

### Path Parameters

- `message_batch_id: string`

  ID of the Message Batch.

### Returns

- `MessageBatch object { id, archived_at, cancel_initiated_at, 7 more }`

  - `id: string`

    Unique object identifier.

    The format and length of IDs may change over time.

  - `archived_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch was archived and its results became unavailable.

  - `cancel_initiated_at: string`

    RFC 3339 datetime string representing the time at which cancellation was initiated for the Message Batch. Specified only if cancellation was initiated.

  - `created_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch was created.

  - `ended_at: string`

    RFC 3339 datetime string representing the time at which processing for the Message Batch ended. Specified only once processing ends.

    Processing ends when every request in a Message Batch has either succeeded, errored, canceled, or expired.

  - `expires_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch will expire and end processing, which is 24 hours after creation.

  - `processing_status: "in_progress" or "canceling" or "ended"`

    Processing status of the Message Batch.

    - `"in_progress"`

    - `"canceling"`

    - `"ended"`

  - `request_counts: MessageBatchRequestCounts`

    Tallies requests within the Message Batch, categorized by their status.

    Requests start as `processing` and move to one of the other statuses only once processing of the entire batch ends. The sum of all values always matches the total number of requests in the batch.

    - `canceled: number`

      Number of requests in the Message Batch that have been canceled.

      This is zero until processing of the entire Message Batch has ended.

    - `errored: number`

      Number of requests in the Message Batch that encountered an error.

      This is zero until processing of the entire Message Batch has ended.

    - `expired: number`

      Number of requests in the Message Batch that have expired.

      This is zero until processing of the entire Message Batch has ended.

    - `processing: number`

      Number of requests in the Message Batch that are processing.

    - `succeeded: number`

      Number of requests in the Message Batch that have completed successfully.

      This is zero until processing of the entire Message Batch has ended.

  - `results_url: string`

    URL to a `.jsonl` file containing the results of the Message Batch requests. Specified only once processing ends.

    Results in the file are not guaranteed to be in the same order as requests. Use the `custom_id` field to match results to requests.

  - `type: "message_batch"`

    Object type.

    For Message Batches, this is always `"message_batch"`.

    - `"message_batch"`

### Example

```http
curl https://api.anthropic.com/v1/messages/batches/$MESSAGE_BATCH_ID \
    -H 'anthropic-version: 2023-06-01' \
    -H "X-Api-Key: $ANTHROPIC_API_KEY"
```

#### Response

```json
{
  "id": "msgbatch_013Zva2CMHLNnXjNJJKqJ2EF",
  "archived_at": "2024-08-20T18:37:24.100435Z",
  "cancel_initiated_at": "2024-08-20T18:37:24.100435Z",
  "created_at": "2024-08-20T18:37:24.100435Z",
  "ended_at": "2024-08-20T18:37:24.100435Z",
  "expires_at": "2024-08-20T18:37:24.100435Z",
  "processing_status": "in_progress",
  "request_counts": {
    "canceled": 10,
    "errored": 30,
    "expired": 10,
    "processing": 100,
    "succeeded": 50
  },
  "results_url": "https://api.anthropic.com/v1/messages/batches/msgbatch_013Zva2CMHLNnXjNJJKqJ2EF/results",
  "type": "message_batch"
}
```

## List Message Batches

**get** `/v1/messages/batches`

List all Message Batches within a Workspace. Most recently created batches are returned first.

Learn more about the Message Batches API in our [user guide](https://platform.claude.com/docs/en/build-with-claude/batch-processing)

### Query Parameters

- `after_id: optional string`

  ID of the object to use as a cursor for pagination. When provided, returns the page of results immediately after this object.

- `before_id: optional string`

  ID of the object to use as a cursor for pagination. When provided, returns the page of results immediately before this object.

- `limit: optional number`

  Number of items to return per page.

  Defaults to `20`. Ranges from `1` to `1000`.

### Returns

- `data: array of MessageBatch`

  - `id: string`

    Unique object identifier.

    The format and length of IDs may change over time.

  - `archived_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch was archived and its results became unavailable.

  - `cancel_initiated_at: string`

    RFC 3339 datetime string representing the time at which cancellation was initiated for the Message Batch. Specified only if cancellation was initiated.

  - `created_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch was created.

  - `ended_at: string`

    RFC 3339 datetime string representing the time at which processing for the Message Batch ended. Specified only once processing ends.

    Processing ends when every request in a Message Batch has either succeeded, errored, canceled, or expired.

  - `expires_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch will expire and end processing, which is 24 hours after creation.

  - `processing_status: "in_progress" or "canceling" or "ended"`

    Processing status of the Message Batch.

    - `"in_progress"`

    - `"canceling"`

    - `"ended"`

  - `request_counts: MessageBatchRequestCounts`

    Tallies requests within the Message Batch, categorized by their status.

    Requests start as `processing` and move to one of the other statuses only once processing of the entire batch ends. The sum of all values always matches the total number of requests in the batch.

    - `canceled: number`

      Number of requests in the Message Batch that have been canceled.

      This is zero until processing of the entire Message Batch has ended.

    - `errored: number`

      Number of requests in the Message Batch that encountered an error.

      This is zero until processing of the entire Message Batch has ended.

    - `expired: number`

      Number of requests in the Message Batch that have expired.

      This is zero until processing of the entire Message Batch has ended.

    - `processing: number`

      Number of requests in the Message Batch that are processing.

    - `succeeded: number`

      Number of requests in the Message Batch that have completed successfully.

      This is zero until processing of the entire Message Batch has ended.

  - `results_url: string`

    URL to a `.jsonl` file containing the results of the Message Batch requests. Specified only once processing ends.

    Results in the file are not guaranteed to be in the same order as requests. Use the `custom_id` field to match results to requests.

  - `type: "message_batch"`

    Object type.

    For Message Batches, this is always `"message_batch"`.

    - `"message_batch"`

- `first_id: string`

  First ID in the `data` list. Can be used as the `before_id` for the previous page.

- `has_more: boolean`

  Indicates if there are more results in the requested page direction.

- `last_id: string`

  Last ID in the `data` list. Can be used as the `after_id` for the next page.

### Example

```http
curl https://api.anthropic.com/v1/messages/batches \
    -H 'anthropic-version: 2023-06-01' \
    -H "X-Api-Key: $ANTHROPIC_API_KEY"
```

#### Response

```json
{
  "data": [
    {
      "id": "msgbatch_013Zva2CMHLNnXjNJJKqJ2EF",
      "archived_at": "2024-08-20T18:37:24.100435Z",
      "cancel_initiated_at": "2024-08-20T18:37:24.100435Z",
      "created_at": "2024-08-20T18:37:24.100435Z",
      "ended_at": "2024-08-20T18:37:24.100435Z",
      "expires_at": "2024-08-20T18:37:24.100435Z",
      "processing_status": "in_progress",
      "request_counts": {
        "canceled": 10,
        "errored": 30,
        "expired": 10,
        "processing": 100,
        "succeeded": 50
      },
      "results_url": "https://api.anthropic.com/v1/messages/batches/msgbatch_013Zva2CMHLNnXjNJJKqJ2EF/results",
      "type": "message_batch"
    }
  ],
  "first_id": "first_id",
  "has_more": true,
  "last_id": "last_id"
}
```

## Cancel a Message Batch

**post** `/v1/messages/batches/{message_batch_id}/cancel`

Batches may be canceled any time before processing ends. Once cancellation is initiated, the batch enters a `canceling` state, at which time the system may complete any in-progress, non-interruptible requests before finalizing cancellation.

The number of canceled requests is specified in `request_counts`. To determine which requests were canceled, check the individual results within the batch. Note that cancellation may not result in any canceled requests if they were non-interruptible.

Learn more about the Message Batches API in our [user guide](https://platform.claude.com/docs/en/build-with-claude/batch-processing)

### Path Parameters

- `message_batch_id: string`

  ID of the Message Batch.

### Returns

- `MessageBatch object { id, archived_at, cancel_initiated_at, 7 more }`

  - `id: string`

    Unique object identifier.

    The format and length of IDs may change over time.

  - `archived_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch was archived and its results became unavailable.

  - `cancel_initiated_at: string`

    RFC 3339 datetime string representing the time at which cancellation was initiated for the Message Batch. Specified only if cancellation was initiated.

  - `created_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch was created.

  - `ended_at: string`

    RFC 3339 datetime string representing the time at which processing for the Message Batch ended. Specified only once processing ends.

    Processing ends when every request in a Message Batch has either succeeded, errored, canceled, or expired.

  - `expires_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch will expire and end processing, which is 24 hours after creation.

  - `processing_status: "in_progress" or "canceling" or "ended"`

    Processing status of the Message Batch.

    - `"in_progress"`

    - `"canceling"`

    - `"ended"`

  - `request_counts: MessageBatchRequestCounts`

    Tallies requests within the Message Batch, categorized by their status.

    Requests start as `processing` and move to one of the other statuses only once processing of the entire batch ends. The sum of all values always matches the total number of requests in the batch.

    - `canceled: number`

      Number of requests in the Message Batch that have been canceled.

      This is zero until processing of the entire Message Batch has ended.

    - `errored: number`

      Number of requests in the Message Batch that encountered an error.

      This is zero until processing of the entire Message Batch has ended.

    - `expired: number`

      Number of requests in the Message Batch that have expired.

      This is zero until processing of the entire Message Batch has ended.

    - `processing: number`

      Number of requests in the Message Batch that are processing.

    - `succeeded: number`

      Number of requests in the Message Batch that have completed successfully.

      This is zero until processing of the entire Message Batch has ended.

  - `results_url: string`

    URL to a `.jsonl` file containing the results of the Message Batch requests. Specified only once processing ends.

    Results in the file are not guaranteed to be in the same order as requests. Use the `custom_id` field to match results to requests.

  - `type: "message_batch"`

    Object type.

    For Message Batches, this is always `"message_batch"`.

    - `"message_batch"`

### Example

```http
curl https://api.anthropic.com/v1/messages/batches/$MESSAGE_BATCH_ID/cancel \
    -X POST \
    -H 'anthropic-version: 2023-06-01' \
    -H "X-Api-Key: $ANTHROPIC_API_KEY"
```

#### Response

```json
{
  "id": "msgbatch_013Zva2CMHLNnXjNJJKqJ2EF",
  "archived_at": "2024-08-20T18:37:24.100435Z",
  "cancel_initiated_at": "2024-08-20T18:37:24.100435Z",
  "created_at": "2024-08-20T18:37:24.100435Z",
  "ended_at": "2024-08-20T18:37:24.100435Z",
  "expires_at": "2024-08-20T18:37:24.100435Z",
  "processing_status": "in_progress",
  "request_counts": {
    "canceled": 10,
    "errored": 30,
    "expired": 10,
    "processing": 100,
    "succeeded": 50
  },
  "results_url": "https://api.anthropic.com/v1/messages/batches/msgbatch_013Zva2CMHLNnXjNJJKqJ2EF/results",
  "type": "message_batch"
}
```

## Delete a Message Batch

**delete** `/v1/messages/batches/{message_batch_id}`

Delete a Message Batch.

Message Batches can only be deleted once they've finished processing. If you'd like to delete an in-progress batch, you must first cancel it.

Learn more about the Message Batches API in our [user guide](https://platform.claude.com/docs/en/build-with-claude/batch-processing)

### Path Parameters

- `message_batch_id: string`

  ID of the Message Batch.

### Returns

- `DeletedMessageBatch object { id, type }`

  - `id: string`

    ID of the Message Batch.

  - `type: "message_batch_deleted"`

    Deleted object type.

    For Message Batches, this is always `"message_batch_deleted"`.

    - `"message_batch_deleted"`

### Example

```http
curl https://api.anthropic.com/v1/messages/batches/$MESSAGE_BATCH_ID \
    -X DELETE \
    -H 'anthropic-version: 2023-06-01' \
    -H "X-Api-Key: $ANTHROPIC_API_KEY"
```

#### Response

```json
{
  "id": "msgbatch_013Zva2CMHLNnXjNJJKqJ2EF",
  "type": "message_batch_deleted"
}
```

## Retrieve Message Batch results

**get** `/v1/messages/batches/{message_batch_id}/results`

Streams the results of a Message Batch as a `.jsonl` file.

Each line in the file is a JSON object containing the result of a single request in the Message Batch. Results are not guaranteed to be in the same order as requests. Use the `custom_id` field to match results to requests.

Learn more about the Message Batches API in our [user guide](https://platform.claude.com/docs/en/build-with-claude/batch-processing)

### Path Parameters

- `message_batch_id: string`

  ID of the Message Batch.

### Returns

- `MessageBatchIndividualResponse object { custom_id, result }`

  This is a single line in the response `.jsonl` file and does not represent the response as a whole.

  - `custom_id: string`

    Developer-provided ID created for each request in a Message Batch. Useful for matching results to requests, as results may be given out of request order.

    Must be unique for each request within the Message Batch.

  - `result: MessageBatchResult`

    Processing result for this request.

    Contains a Message output if processing was successful, an error response if processing failed, or the reason why processing was not attempted, such as cancellation or expiration.

    - `MessageBatchSucceededResult object { message, type }`

      - `message: Message`

        - `id: string`

          Unique object identifier.

          The format and length of IDs may change over time.

        - `container: Container`

          Information about the container used in the request (for the code execution tool)

          - `id: string`

            Identifier for the container used in this request

          - `expires_at: string`

            The time at which the container will expire.

        - `content: array of ContentBlock`

          Content generated by the model.

          This is an array of content blocks, each of which has a `type` that determines its shape.

          Example:

          ```json
          [{"type": "text", "text": "Hi, I'm Claude."}]
          ```

          If the request input `messages` ended with an `assistant` turn, then the response `content` will continue directly from that last turn. You can use this to constrain the model's output.

          For example, if the input `messages` were:

          ```json
          [
            {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
            {"role": "assistant", "content": "The best answer is ("}
          ]
          ```

          Then the response `content` might be:

          ```json
          [{"type": "text", "text": "B)"}]
          ```

          - `TextBlock object { citations, text, type }`

            - `citations: array of TextCitation`

              Citations supporting the text block.

              The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.

              - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

                - `cited_text: string`

                - `document_index: number`

                - `document_title: string`

                - `end_char_index: number`

                - `file_id: string`

                - `start_char_index: number`

                - `type: "char_location"`

                  - `"char_location"`

              - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

                - `cited_text: string`

                - `document_index: number`

                - `document_title: string`

                - `end_page_number: number`

                - `file_id: string`

                - `start_page_number: number`

                - `type: "page_location"`

                  - `"page_location"`

              - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

                - `cited_text: string`

                  The full text of the cited block range, concatenated.

                  Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

                - `document_index: number`

                - `document_title: string`

                - `end_block_index: number`

                  Exclusive 0-based end index of the cited block range in the source's `content` array.

                  Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

                - `file_id: string`

                - `start_block_index: number`

                  0-based index of the first cited block in the source's `content` array.

                - `type: "content_block_location"`

                  - `"content_block_location"`

              - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

                - `cited_text: string`

                - `encrypted_index: string`

                - `title: string`

                - `type: "web_search_result_location"`

                  - `"web_search_result_location"`

                - `url: string`

              - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

                - `cited_text: string`

                  The full text of the cited block range, concatenated.

                  Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

                - `end_block_index: number`

                  Exclusive 0-based end index of the cited block range in the source's `content` array.

                  Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

                - `search_result_index: number`

                  0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

                  Counted separately from `document_index`; server-side web search results are not included in this count.

                - `source: string`

                - `start_block_index: number`

                  0-based index of the first cited block in the source's `content` array.

                - `title: string`

                - `type: "search_result_location"`

                  - `"search_result_location"`

            - `text: string`

            - `type: "text"`

              - `"text"`

          - `ThinkingBlock object { signature, thinking, type }`

            - `signature: string`

            - `thinking: string`

            - `type: "thinking"`

              - `"thinking"`

          - `RedactedThinkingBlock object { data, type }`

            - `data: string`

            - `type: "redacted_thinking"`

              - `"redacted_thinking"`

          - `ToolUseBlock object { id, caller, input, 2 more }`

            - `id: string`

            - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

              Tool invocation directly from the model.

              - `DirectCaller object { type }`

                Tool invocation directly from the model.

                - `type: "direct"`

                  - `"direct"`

              - `ServerToolCaller object { tool_id, type }`

                Tool invocation generated by a server-side tool.

                - `tool_id: string`

                - `type: "code_execution_20250825"`

                  - `"code_execution_20250825"`

              - `ServerToolCaller20260120 object { tool_id, type }`

                - `tool_id: string`

                - `type: "code_execution_20260120"`

                  - `"code_execution_20260120"`

            - `input: map[unknown]`

            - `name: string`

            - `type: "tool_use"`

              - `"tool_use"`

          - `ServerToolUseBlock object { id, caller, input, 2 more }`

            - `id: string`

            - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

              Tool invocation directly from the model.

              - `DirectCaller object { type }`

                Tool invocation directly from the model.

              - `ServerToolCaller object { tool_id, type }`

                Tool invocation generated by a server-side tool.

              - `ServerToolCaller20260120 object { tool_id, type }`

            - `input: map[unknown]`

            - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

              - `"web_search"`

              - `"web_fetch"`

              - `"code_execution"`

              - `"bash_code_execution"`

              - `"text_editor_code_execution"`

              - `"tool_search_tool_regex"`

              - `"tool_search_tool_bm25"`

            - `type: "server_tool_use"`

              - `"server_tool_use"`

          - `WebSearchToolResultBlock object { caller, content, tool_use_id, type }`

            - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

              Tool invocation directly from the model.

              - `DirectCaller object { type }`

                Tool invocation directly from the model.

              - `ServerToolCaller object { tool_id, type }`

                Tool invocation generated by a server-side tool.

              - `ServerToolCaller20260120 object { tool_id, type }`

            - `content: WebSearchToolResultBlockContent`

              - `WebSearchToolResultError object { error_code, type }`

                - `error_code: WebSearchToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"max_uses_exceeded"`

                  - `"too_many_requests"`

                  - `"query_too_long"`

                  - `"request_too_large"`

                - `type: "web_search_tool_result_error"`

                  - `"web_search_tool_result_error"`

              - `array of WebSearchResultBlock`

                - `encrypted_content: string`

                - `page_age: string`

                - `title: string`

                - `type: "web_search_result"`

                  - `"web_search_result"`

                - `url: string`

            - `tool_use_id: string`

            - `type: "web_search_tool_result"`

              - `"web_search_tool_result"`

          - `WebFetchToolResultBlock object { caller, content, tool_use_id, type }`

            - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

              Tool invocation directly from the model.

              - `DirectCaller object { type }`

                Tool invocation directly from the model.

              - `ServerToolCaller object { tool_id, type }`

                Tool invocation generated by a server-side tool.

              - `ServerToolCaller20260120 object { tool_id, type }`

            - `content: WebFetchToolResultErrorBlock or WebFetchBlock`

              - `WebFetchToolResultErrorBlock object { error_code, type }`

                - `error_code: WebFetchToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"url_too_long"`

                  - `"url_not_allowed"`

                  - `"url_not_in_prior_context"`

                  - `"url_not_accessible"`

                  - `"unsupported_content_type"`

                  - `"too_many_requests"`

                  - `"max_uses_exceeded"`

                  - `"unavailable"`

                - `type: "web_fetch_tool_result_error"`

                  - `"web_fetch_tool_result_error"`

              - `WebFetchBlock object { content, retrieved_at, type, url }`

                - `content: DocumentBlock`

                  - `citations: CitationsConfig`

                    Citation configuration for the document

                    - `enabled: boolean`

                  - `source: Base64PDFSource or PlainTextSource`

                    - `Base64PDFSource object { data, media_type, type }`

                      - `data: string`

                      - `media_type: "application/pdf"`

                        - `"application/pdf"`

                      - `type: "base64"`

                        - `"base64"`

                    - `PlainTextSource object { data, media_type, type }`

                      - `data: string`

                      - `media_type: "text/plain"`

                        - `"text/plain"`

                      - `type: "text"`

                        - `"text"`

                  - `title: string`

                    The title of the document

                  - `type: "document"`

                    - `"document"`

                - `retrieved_at: string`

                  ISO 8601 timestamp when the content was retrieved

                - `type: "web_fetch_result"`

                  - `"web_fetch_result"`

                - `url: string`

                  Fetched content URL

            - `tool_use_id: string`

            - `type: "web_fetch_tool_result"`

              - `"web_fetch_tool_result"`

          - `CodeExecutionToolResultBlock object { content, tool_use_id, type }`

            - `content: CodeExecutionToolResultBlockContent`

              Code execution result with encrypted stdout for PFC + web_search results.

              - `CodeExecutionToolResultError object { error_code, type }`

                - `error_code: CodeExecutionToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"too_many_requests"`

                  - `"execution_time_exceeded"`

                - `type: "code_execution_tool_result_error"`

                  - `"code_execution_tool_result_error"`

              - `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

                - `content: array of CodeExecutionOutputBlock`

                  - `file_id: string`

                  - `type: "code_execution_output"`

                    - `"code_execution_output"`

                - `return_code: number`

                - `stderr: string`

                - `stdout: string`

                - `type: "code_execution_result"`

                  - `"code_execution_result"`

              - `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

                Code execution result with encrypted stdout for PFC + web_search results.

                - `content: array of CodeExecutionOutputBlock`

                  - `file_id: string`

                  - `type: "code_execution_output"`

                - `encrypted_stdout: string`

                - `return_code: number`

                - `stderr: string`

                - `type: "encrypted_code_execution_result"`

                  - `"encrypted_code_execution_result"`

            - `tool_use_id: string`

            - `type: "code_execution_tool_result"`

              - `"code_execution_tool_result"`

          - `BashCodeExecutionToolResultBlock object { content, tool_use_id, type }`

            - `content: BashCodeExecutionToolResultError or BashCodeExecutionResultBlock`

              - `BashCodeExecutionToolResultError object { error_code, type }`

                - `error_code: BashCodeExecutionToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"too_many_requests"`

                  - `"execution_time_exceeded"`

                  - `"output_file_too_large"`

                - `type: "bash_code_execution_tool_result_error"`

                  - `"bash_code_execution_tool_result_error"`

              - `BashCodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

                - `content: array of BashCodeExecutionOutputBlock`

                  - `file_id: string`

                  - `type: "bash_code_execution_output"`

                    - `"bash_code_execution_output"`

                - `return_code: number`

                - `stderr: string`

                - `stdout: string`

                - `type: "bash_code_execution_result"`

                  - `"bash_code_execution_result"`

            - `tool_use_id: string`

            - `type: "bash_code_execution_tool_result"`

              - `"bash_code_execution_tool_result"`

          - `TextEditorCodeExecutionToolResultBlock object { content, tool_use_id, type }`

            - `content: TextEditorCodeExecutionToolResultError or TextEditorCodeExecutionViewResultBlock or TextEditorCodeExecutionCreateResultBlock or TextEditorCodeExecutionStrReplaceResultBlock`

              - `TextEditorCodeExecutionToolResultError object { error_code, error_message, type }`

                - `error_code: TextEditorCodeExecutionToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"too_many_requests"`

                  - `"execution_time_exceeded"`

                  - `"file_not_found"`

                - `error_message: string`

                - `type: "text_editor_code_execution_tool_result_error"`

                  - `"text_editor_code_execution_tool_result_error"`

              - `TextEditorCodeExecutionViewResultBlock object { content, file_type, num_lines, 3 more }`

                - `content: string`

                - `file_type: "text" or "image" or "pdf"`

                  - `"text"`

                  - `"image"`

                  - `"pdf"`

                - `num_lines: number`

                - `start_line: number`

                - `total_lines: number`

                - `type: "text_editor_code_execution_view_result"`

                  - `"text_editor_code_execution_view_result"`

              - `TextEditorCodeExecutionCreateResultBlock object { is_file_update, type }`

                - `is_file_update: boolean`

                - `type: "text_editor_code_execution_create_result"`

                  - `"text_editor_code_execution_create_result"`

              - `TextEditorCodeExecutionStrReplaceResultBlock object { lines, new_lines, new_start, 3 more }`

                - `lines: array of string`

                - `new_lines: number`

                - `new_start: number`

                - `old_lines: number`

                - `old_start: number`

                - `type: "text_editor_code_execution_str_replace_result"`

                  - `"text_editor_code_execution_str_replace_result"`

            - `tool_use_id: string`

            - `type: "text_editor_code_execution_tool_result"`

              - `"text_editor_code_execution_tool_result"`

          - `ToolSearchToolResultBlock object { content, tool_use_id, type }`

            - `content: ToolSearchToolResultError or ToolSearchToolSearchResultBlock`

              - `ToolSearchToolResultError object { error_code, error_message, type }`

                - `error_code: ToolSearchToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"too_many_requests"`

                  - `"execution_time_exceeded"`

                - `error_message: string`

                - `type: "tool_search_tool_result_error"`

                  - `"tool_search_tool_result_error"`

              - `ToolSearchToolSearchResultBlock object { tool_references, type }`

                - `tool_references: array of ToolReferenceBlock`

                  - `tool_name: string`

                  - `type: "tool_reference"`

                    - `"tool_reference"`

                - `type: "tool_search_tool_search_result"`

                  - `"tool_search_tool_search_result"`

            - `tool_use_id: string`

            - `type: "tool_search_tool_result"`

              - `"tool_search_tool_result"`

          - `ContainerUploadBlock object { file_id, type }`

            Response model for a file uploaded to the container.

            - `file_id: string`

            - `type: "container_upload"`

              - `"container_upload"`

        - `model: Model`

          The model that will complete your prompt.

          See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

          - `"claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more`

            The model that will complete your prompt.

            See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

            - `"claude-sonnet-5"`

              High-performance model for coding and agents

            - `"claude-fable-5"`

              Next generation of intelligence for the hardest knowledge work and coding problems

            - `"claude-mythos-5"`

              Most capable model for cybersecurity and biology research

            - `"claude-opus-4-8"`

              Frontier intelligence for long-running agents and coding

            - `"claude-opus-4-7"`

              Frontier intelligence for long-running agents and coding

            - `"claude-mythos-preview"`

              New class of intelligence, strongest in coding and cybersecurity

            - `"claude-opus-4-6"`

              Frontier intelligence for long-running agents and coding

            - `"claude-sonnet-4-6"`

              Best combination of speed and intelligence

            - `"claude-haiku-4-5"`

              Fastest model with near-frontier intelligence

            - `"claude-haiku-4-5-20251001"`

              Fastest model with near-frontier intelligence

            - `"claude-opus-4-5"`

              Premium model combining maximum intelligence with practical performance

            - `"claude-opus-4-5-20251101"`

              Premium model combining maximum intelligence with practical performance

            - `"claude-sonnet-4-5"`

              High-performance model for agents and coding

            - `"claude-sonnet-4-5-20250929"`

              High-performance model for agents and coding

            - `"claude-opus-4-1"`

              Exceptional model for specialized complex tasks

            - `"claude-opus-4-1-20250805"`

              Exceptional model for specialized complex tasks

          - `string`

        - `role: "assistant"`

          Conversational role of the generated message.

          This will always be `"assistant"`.

          - `"assistant"`

        - `stop_details: RefusalStopDetails`

          Structured information about a refusal.

          - `category: "cyber" or "bio" or "frontier_llm" or "reasoning_extraction"`

            The policy category that triggered a refusal.

            - `"cyber"`

            - `"bio"`

            - `"frontier_llm"`

            - `"reasoning_extraction"`

          - `explanation: string`

            Human-readable explanation of the refusal.

            This text is not guaranteed to be stable. `null` when no explanation is available for the category.

          - `type: "refusal"`

            - `"refusal"`

        - `stop_reason: StopReason`

          The reason that we stopped.

          This may be one the following values:

          * `"end_turn"`: the model reached a natural stopping point
          * `"max_tokens"`: we exceeded the requested `max_tokens` or the model's maximum
          * `"stop_sequence"`: one of your provided custom `stop_sequences` was generated
          * `"tool_use"`: the model invoked one or more tools
          * `"pause_turn"`: we paused a long-running turn. You may provide the response back as-is in a subsequent request to let the model continue.
          * `"refusal"`: when streaming classifiers intervene to handle potential policy violations

          In non-streaming mode this value is always non-null. In streaming mode, it is null in the `message_start` event and non-null otherwise.

          - `"end_turn"`

          - `"max_tokens"`

          - `"stop_sequence"`

          - `"tool_use"`

          - `"pause_turn"`

          - `"refusal"`

        - `stop_sequence: string`

          Which custom stop sequence was generated, if any.

          This value will be a non-null string if one of your custom stop sequences was generated.

        - `type: "message"`

          Object type.

          For Messages, this is always `"message"`.

          - `"message"`

        - `usage: Usage`

          Billing and rate-limit usage.

          Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems.

          Under the hood, the API transforms requests into a format suitable for the model. The model's output then goes through a parsing stage before becoming an API response. As a result, the token counts in `usage` will not match one-to-one with the exact visible content of an API request or response.

          For example, `output_tokens` will be non-zero, even for an empty string response from Claude.

          Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.

          - `cache_creation: CacheCreation`

            Breakdown of cached tokens by TTL

            - `ephemeral_1h_input_tokens: number`

              The number of input tokens used to create the 1 hour cache entry.

            - `ephemeral_5m_input_tokens: number`

              The number of input tokens used to create the 5 minute cache entry.

          - `cache_creation_input_tokens: number`

            The number of input tokens used to create the cache entry.

          - `cache_read_input_tokens: number`

            The number of input tokens read from the cache.

          - `inference_geo: string`

            The geographic region where inference was performed for this request.

          - `input_tokens: number`

            The number of input tokens which were used.

          - `output_tokens: number`

            The number of output tokens which were used.

          - `output_tokens_details: OutputTokensDetails`

            Breakdown of output tokens by category.

            `output_tokens` remains the inclusive, authoritative total used for billing.
            This object provides a read-only decomposition for observability — for example,
            how many of the billed output tokens were spent on internal reasoning that may
            have been summarized before being returned to you.

            - `thinking_tokens: number`

              Number of output tokens the model generated as internal reasoning, including
              the thinking-block delimiter tokens.

              Reflects the raw reasoning the model produced, not the (possibly shorter)
              summarized thinking text returned in the response body. Computed by
              re-tokenizing the raw reasoning text, so it may differ from the model's exact
              generation count by a small number of tokens. Always ≤ `output_tokens`;
              `output_tokens - thinking_tokens` approximates the non-reasoning output.

          - `server_tool_use: ServerToolUsage`

            The number of server tool requests.

            - `web_fetch_requests: number`

              The number of web fetch tool requests.

            - `web_search_requests: number`

              The number of web search tool requests.

          - `service_tier: "standard" or "priority" or "batch"`

            If the request used the priority, standard, or batch tier.

            - `"standard"`

            - `"priority"`

            - `"batch"`

      - `type: "succeeded"`

        - `"succeeded"`

    - `MessageBatchErroredResult object { error, type }`

      - `error: ErrorResponse`

        - `error: ErrorObject`

          - `InvalidRequestError object { message, type }`

            - `message: string`

            - `type: "invalid_request_error"`

              - `"invalid_request_error"`

          - `AuthenticationError object { message, type }`

            - `message: string`

            - `type: "authentication_error"`

              - `"authentication_error"`

          - `BillingError object { message, type }`

            - `message: string`

            - `type: "billing_error"`

              - `"billing_error"`

          - `PermissionError object { message, type }`

            - `message: string`

            - `type: "permission_error"`

              - `"permission_error"`

          - `NotFoundError object { message, type }`

            - `message: string`

            - `type: "not_found_error"`

              - `"not_found_error"`

          - `RateLimitError object { message, type }`

            - `message: string`

            - `type: "rate_limit_error"`

              - `"rate_limit_error"`

          - `GatewayTimeoutError object { message, type }`

            - `message: string`

            - `type: "timeout_error"`

              - `"timeout_error"`

          - `APIErrorObject object { message, type }`

            - `message: string`

            - `type: "api_error"`

              - `"api_error"`

          - `OverloadedError object { message, type }`

            - `message: string`

            - `type: "overloaded_error"`

              - `"overloaded_error"`

        - `request_id: string`

        - `type: "error"`

          - `"error"`

      - `type: "errored"`

        - `"errored"`

    - `MessageBatchCanceledResult object { type }`

      - `type: "canceled"`

        - `"canceled"`

    - `MessageBatchExpiredResult object { type }`

      - `type: "expired"`

        - `"expired"`

### Example

```http
curl https://api.anthropic.com/v1/messages/batches/$MESSAGE_BATCH_ID/results \
    -H 'anthropic-version: 2023-06-01' \
    -H "X-Api-Key: $ANTHROPIC_API_KEY"
```

## Domain Types

### Deleted Message Batch

- `DeletedMessageBatch object { id, type }`

  - `id: string`

    ID of the Message Batch.

  - `type: "message_batch_deleted"`

    Deleted object type.

    For Message Batches, this is always `"message_batch_deleted"`.

    - `"message_batch_deleted"`

### Message Batch

- `MessageBatch object { id, archived_at, cancel_initiated_at, 7 more }`

  - `id: string`

    Unique object identifier.

    The format and length of IDs may change over time.

  - `archived_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch was archived and its results became unavailable.

  - `cancel_initiated_at: string`

    RFC 3339 datetime string representing the time at which cancellation was initiated for the Message Batch. Specified only if cancellation was initiated.

  - `created_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch was created.

  - `ended_at: string`

    RFC 3339 datetime string representing the time at which processing for the Message Batch ended. Specified only once processing ends.

    Processing ends when every request in a Message Batch has either succeeded, errored, canceled, or expired.

  - `expires_at: string`

    RFC 3339 datetime string representing the time at which the Message Batch will expire and end processing, which is 24 hours after creation.

  - `processing_status: "in_progress" or "canceling" or "ended"`

    Processing status of the Message Batch.

    - `"in_progress"`

    - `"canceling"`

    - `"ended"`

  - `request_counts: MessageBatchRequestCounts`

    Tallies requests within the Message Batch, categorized by their status.

    Requests start as `processing` and move to one of the other statuses only once processing of the entire batch ends. The sum of all values always matches the total number of requests in the batch.

    - `canceled: number`

      Number of requests in the Message Batch that have been canceled.

      This is zero until processing of the entire Message Batch has ended.

    - `errored: number`

      Number of requests in the Message Batch that encountered an error.

      This is zero until processing of the entire Message Batch has ended.

    - `expired: number`

      Number of requests in the Message Batch that have expired.

      This is zero until processing of the entire Message Batch has ended.

    - `processing: number`

      Number of requests in the Message Batch that are processing.

    - `succeeded: number`

      Number of requests in the Message Batch that have completed successfully.

      This is zero until processing of the entire Message Batch has ended.

  - `results_url: string`

    URL to a `.jsonl` file containing the results of the Message Batch requests. Specified only once processing ends.

    Results in the file are not guaranteed to be in the same order as requests. Use the `custom_id` field to match results to requests.

  - `type: "message_batch"`

    Object type.

    For Message Batches, this is always `"message_batch"`.

    - `"message_batch"`

### Message Batch Canceled Result

- `MessageBatchCanceledResult object { type }`

  - `type: "canceled"`

    - `"canceled"`

### Message Batch Errored Result

- `MessageBatchErroredResult object { error, type }`

  - `error: ErrorResponse`

    - `error: ErrorObject`

      - `InvalidRequestError object { message, type }`

        - `message: string`

        - `type: "invalid_request_error"`

          - `"invalid_request_error"`

      - `AuthenticationError object { message, type }`

        - `message: string`

        - `type: "authentication_error"`

          - `"authentication_error"`

      - `BillingError object { message, type }`

        - `message: string`

        - `type: "billing_error"`

          - `"billing_error"`

      - `PermissionError object { message, type }`

        - `message: string`

        - `type: "permission_error"`

          - `"permission_error"`

      - `NotFoundError object { message, type }`

        - `message: string`

        - `type: "not_found_error"`

          - `"not_found_error"`

      - `RateLimitError object { message, type }`

        - `message: string`

        - `type: "rate_limit_error"`

          - `"rate_limit_error"`

      - `GatewayTimeoutError object { message, type }`

        - `message: string`

        - `type: "timeout_error"`

          - `"timeout_error"`

      - `APIErrorObject object { message, type }`

        - `message: string`

        - `type: "api_error"`

          - `"api_error"`

      - `OverloadedError object { message, type }`

        - `message: string`

        - `type: "overloaded_error"`

          - `"overloaded_error"`

    - `request_id: string`

    - `type: "error"`

      - `"error"`

  - `type: "errored"`

    - `"errored"`

### Message Batch Expired Result

- `MessageBatchExpiredResult object { type }`

  - `type: "expired"`

    - `"expired"`

### Message Batch Individual Response

- `MessageBatchIndividualResponse object { custom_id, result }`

  This is a single line in the response `.jsonl` file and does not represent the response as a whole.

  - `custom_id: string`

    Developer-provided ID created for each request in a Message Batch. Useful for matching results to requests, as results may be given out of request order.

    Must be unique for each request within the Message Batch.

  - `result: MessageBatchResult`

    Processing result for this request.

    Contains a Message output if processing was successful, an error response if processing failed, or the reason why processing was not attempted, such as cancellation or expiration.

    - `MessageBatchSucceededResult object { message, type }`

      - `message: Message`

        - `id: string`

          Unique object identifier.

          The format and length of IDs may change over time.

        - `container: Container`

          Information about the container used in the request (for the code execution tool)

          - `id: string`

            Identifier for the container used in this request

          - `expires_at: string`

            The time at which the container will expire.

        - `content: array of ContentBlock`

          Content generated by the model.

          This is an array of content blocks, each of which has a `type` that determines its shape.

          Example:

          ```json
          [{"type": "text", "text": "Hi, I'm Claude."}]
          ```

          If the request input `messages` ended with an `assistant` turn, then the response `content` will continue directly from that last turn. You can use this to constrain the model's output.

          For example, if the input `messages` were:

          ```json
          [
            {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
            {"role": "assistant", "content": "The best answer is ("}
          ]
          ```

          Then the response `content` might be:

          ```json
          [{"type": "text", "text": "B)"}]
          ```

          - `TextBlock object { citations, text, type }`

            - `citations: array of TextCitation`

              Citations supporting the text block.

              The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.

              - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

                - `cited_text: string`

                - `document_index: number`

                - `document_title: string`

                - `end_char_index: number`

                - `file_id: string`

                - `start_char_index: number`

                - `type: "char_location"`

                  - `"char_location"`

              - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

                - `cited_text: string`

                - `document_index: number`

                - `document_title: string`

                - `end_page_number: number`

                - `file_id: string`

                - `start_page_number: number`

                - `type: "page_location"`

                  - `"page_location"`

              - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

                - `cited_text: string`

                  The full text of the cited block range, concatenated.

                  Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

                - `document_index: number`

                - `document_title: string`

                - `end_block_index: number`

                  Exclusive 0-based end index of the cited block range in the source's `content` array.

                  Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

                - `file_id: string`

                - `start_block_index: number`

                  0-based index of the first cited block in the source's `content` array.

                - `type: "content_block_location"`

                  - `"content_block_location"`

              - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

                - `cited_text: string`

                - `encrypted_index: string`

                - `title: string`

                - `type: "web_search_result_location"`

                  - `"web_search_result_location"`

                - `url: string`

              - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

                - `cited_text: string`

                  The full text of the cited block range, concatenated.

                  Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

                - `end_block_index: number`

                  Exclusive 0-based end index of the cited block range in the source's `content` array.

                  Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

                - `search_result_index: number`

                  0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

                  Counted separately from `document_index`; server-side web search results are not included in this count.

                - `source: string`

                - `start_block_index: number`

                  0-based index of the first cited block in the source's `content` array.

                - `title: string`

                - `type: "search_result_location"`

                  - `"search_result_location"`

            - `text: string`

            - `type: "text"`

              - `"text"`

          - `ThinkingBlock object { signature, thinking, type }`

            - `signature: string`

            - `thinking: string`

            - `type: "thinking"`

              - `"thinking"`

          - `RedactedThinkingBlock object { data, type }`

            - `data: string`

            - `type: "redacted_thinking"`

              - `"redacted_thinking"`

          - `ToolUseBlock object { id, caller, input, 2 more }`

            - `id: string`

            - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

              Tool invocation directly from the model.

              - `DirectCaller object { type }`

                Tool invocation directly from the model.

                - `type: "direct"`

                  - `"direct"`

              - `ServerToolCaller object { tool_id, type }`

                Tool invocation generated by a server-side tool.

                - `tool_id: string`

                - `type: "code_execution_20250825"`

                  - `"code_execution_20250825"`

              - `ServerToolCaller20260120 object { tool_id, type }`

                - `tool_id: string`

                - `type: "code_execution_20260120"`

                  - `"code_execution_20260120"`

            - `input: map[unknown]`

            - `name: string`

            - `type: "tool_use"`

              - `"tool_use"`

          - `ServerToolUseBlock object { id, caller, input, 2 more }`

            - `id: string`

            - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

              Tool invocation directly from the model.

              - `DirectCaller object { type }`

                Tool invocation directly from the model.

              - `ServerToolCaller object { tool_id, type }`

                Tool invocation generated by a server-side tool.

              - `ServerToolCaller20260120 object { tool_id, type }`

            - `input: map[unknown]`

            - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

              - `"web_search"`

              - `"web_fetch"`

              - `"code_execution"`

              - `"bash_code_execution"`

              - `"text_editor_code_execution"`

              - `"tool_search_tool_regex"`

              - `"tool_search_tool_bm25"`

            - `type: "server_tool_use"`

              - `"server_tool_use"`

          - `WebSearchToolResultBlock object { caller, content, tool_use_id, type }`

            - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

              Tool invocation directly from the model.

              - `DirectCaller object { type }`

                Tool invocation directly from the model.

              - `ServerToolCaller object { tool_id, type }`

                Tool invocation generated by a server-side tool.

              - `ServerToolCaller20260120 object { tool_id, type }`

            - `content: WebSearchToolResultBlockContent`

              - `WebSearchToolResultError object { error_code, type }`

                - `error_code: WebSearchToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"max_uses_exceeded"`

                  - `"too_many_requests"`

                  - `"query_too_long"`

                  - `"request_too_large"`

                - `type: "web_search_tool_result_error"`

                  - `"web_search_tool_result_error"`

              - `array of WebSearchResultBlock`

                - `encrypted_content: string`

                - `page_age: string`

                - `title: string`

                - `type: "web_search_result"`

                  - `"web_search_result"`

                - `url: string`

            - `tool_use_id: string`

            - `type: "web_search_tool_result"`

              - `"web_search_tool_result"`

          - `WebFetchToolResultBlock object { caller, content, tool_use_id, type }`

            - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

              Tool invocation directly from the model.

              - `DirectCaller object { type }`

                Tool invocation directly from the model.

              - `ServerToolCaller object { tool_id, type }`

                Tool invocation generated by a server-side tool.

              - `ServerToolCaller20260120 object { tool_id, type }`

            - `content: WebFetchToolResultErrorBlock or WebFetchBlock`

              - `WebFetchToolResultErrorBlock object { error_code, type }`

                - `error_code: WebFetchToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"url_too_long"`

                  - `"url_not_allowed"`

                  - `"url_not_in_prior_context"`

                  - `"url_not_accessible"`

                  - `"unsupported_content_type"`

                  - `"too_many_requests"`

                  - `"max_uses_exceeded"`

                  - `"unavailable"`

                - `type: "web_fetch_tool_result_error"`

                  - `"web_fetch_tool_result_error"`

              - `WebFetchBlock object { content, retrieved_at, type, url }`

                - `content: DocumentBlock`

                  - `citations: CitationsConfig`

                    Citation configuration for the document

                    - `enabled: boolean`

                  - `source: Base64PDFSource or PlainTextSource`

                    - `Base64PDFSource object { data, media_type, type }`

                      - `data: string`

                      - `media_type: "application/pdf"`

                        - `"application/pdf"`

                      - `type: "base64"`

                        - `"base64"`

                    - `PlainTextSource object { data, media_type, type }`

                      - `data: string`

                      - `media_type: "text/plain"`

                        - `"text/plain"`

                      - `type: "text"`

                        - `"text"`

                  - `title: string`

                    The title of the document

                  - `type: "document"`

                    - `"document"`

                - `retrieved_at: string`

                  ISO 8601 timestamp when the content was retrieved

                - `type: "web_fetch_result"`

                  - `"web_fetch_result"`

                - `url: string`

                  Fetched content URL

            - `tool_use_id: string`

            - `type: "web_fetch_tool_result"`

              - `"web_fetch_tool_result"`

          - `CodeExecutionToolResultBlock object { content, tool_use_id, type }`

            - `content: CodeExecutionToolResultBlockContent`

              Code execution result with encrypted stdout for PFC + web_search results.

              - `CodeExecutionToolResultError object { error_code, type }`

                - `error_code: CodeExecutionToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"too_many_requests"`

                  - `"execution_time_exceeded"`

                - `type: "code_execution_tool_result_error"`

                  - `"code_execution_tool_result_error"`

              - `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

                - `content: array of CodeExecutionOutputBlock`

                  - `file_id: string`

                  - `type: "code_execution_output"`

                    - `"code_execution_output"`

                - `return_code: number`

                - `stderr: string`

                - `stdout: string`

                - `type: "code_execution_result"`

                  - `"code_execution_result"`

              - `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

                Code execution result with encrypted stdout for PFC + web_search results.

                - `content: array of CodeExecutionOutputBlock`

                  - `file_id: string`

                  - `type: "code_execution_output"`

                - `encrypted_stdout: string`

                - `return_code: number`

                - `stderr: string`

                - `type: "encrypted_code_execution_result"`

                  - `"encrypted_code_execution_result"`

            - `tool_use_id: string`

            - `type: "code_execution_tool_result"`

              - `"code_execution_tool_result"`

          - `BashCodeExecutionToolResultBlock object { content, tool_use_id, type }`

            - `content: BashCodeExecutionToolResultError or BashCodeExecutionResultBlock`

              - `BashCodeExecutionToolResultError object { error_code, type }`

                - `error_code: BashCodeExecutionToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"too_many_requests"`

                  - `"execution_time_exceeded"`

                  - `"output_file_too_large"`

                - `type: "bash_code_execution_tool_result_error"`

                  - `"bash_code_execution_tool_result_error"`

              - `BashCodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

                - `content: array of BashCodeExecutionOutputBlock`

                  - `file_id: string`

                  - `type: "bash_code_execution_output"`

                    - `"bash_code_execution_output"`

                - `return_code: number`

                - `stderr: string`

                - `stdout: string`

                - `type: "bash_code_execution_result"`

                  - `"bash_code_execution_result"`

            - `tool_use_id: string`

            - `type: "bash_code_execution_tool_result"`

              - `"bash_code_execution_tool_result"`

          - `TextEditorCodeExecutionToolResultBlock object { content, tool_use_id, type }`

            - `content: TextEditorCodeExecutionToolResultError or TextEditorCodeExecutionViewResultBlock or TextEditorCodeExecutionCreateResultBlock or TextEditorCodeExecutionStrReplaceResultBlock`

              - `TextEditorCodeExecutionToolResultError object { error_code, error_message, type }`

                - `error_code: TextEditorCodeExecutionToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"too_many_requests"`

                  - `"execution_time_exceeded"`

                  - `"file_not_found"`

                - `error_message: string`

                - `type: "text_editor_code_execution_tool_result_error"`

                  - `"text_editor_code_execution_tool_result_error"`

              - `TextEditorCodeExecutionViewResultBlock object { content, file_type, num_lines, 3 more }`

                - `content: string`

                - `file_type: "text" or "image" or "pdf"`

                  - `"text"`

                  - `"image"`

                  - `"pdf"`

                - `num_lines: number`

                - `start_line: number`

                - `total_lines: number`

                - `type: "text_editor_code_execution_view_result"`

                  - `"text_editor_code_execution_view_result"`

              - `TextEditorCodeExecutionCreateResultBlock object { is_file_update, type }`

                - `is_file_update: boolean`

                - `type: "text_editor_code_execution_create_result"`

                  - `"text_editor_code_execution_create_result"`

              - `TextEditorCodeExecutionStrReplaceResultBlock object { lines, new_lines, new_start, 3 more }`

                - `lines: array of string`

                - `new_lines: number`

                - `new_start: number`

                - `old_lines: number`

                - `old_start: number`

                - `type: "text_editor_code_execution_str_replace_result"`

                  - `"text_editor_code_execution_str_replace_result"`

            - `tool_use_id: string`

            - `type: "text_editor_code_execution_tool_result"`

              - `"text_editor_code_execution_tool_result"`

          - `ToolSearchToolResultBlock object { content, tool_use_id, type }`

            - `content: ToolSearchToolResultError or ToolSearchToolSearchResultBlock`

              - `ToolSearchToolResultError object { error_code, error_message, type }`

                - `error_code: ToolSearchToolResultErrorCode`

                  - `"invalid_tool_input"`

                  - `"unavailable"`

                  - `"too_many_requests"`

                  - `"execution_time_exceeded"`

                - `error_message: string`

                - `type: "tool_search_tool_result_error"`

                  - `"tool_search_tool_result_error"`

              - `ToolSearchToolSearchResultBlock object { tool_references, type }`

                - `tool_references: array of ToolReferenceBlock`

                  - `tool_name: string`

                  - `type: "tool_reference"`

                    - `"tool_reference"`

                - `type: "tool_search_tool_search_result"`

                  - `"tool_search_tool_search_result"`

            - `tool_use_id: string`

            - `type: "tool_search_tool_result"`

              - `"tool_search_tool_result"`

          - `ContainerUploadBlock object { file_id, type }`

            Response model for a file uploaded to the container.

            - `file_id: string`

            - `type: "container_upload"`

              - `"container_upload"`

        - `model: Model`

          The model that will complete your prompt.

          See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

          - `"claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more`

            The model that will complete your prompt.

            See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

            - `"claude-sonnet-5"`

              High-performance model for coding and agents

            - `"claude-fable-5"`

              Next generation of intelligence for the hardest knowledge work and coding problems

            - `"claude-mythos-5"`

              Most capable model for cybersecurity and biology research

            - `"claude-opus-4-8"`

              Frontier intelligence for long-running agents and coding

            - `"claude-opus-4-7"`

              Frontier intelligence for long-running agents and coding

            - `"claude-mythos-preview"`

              New class of intelligence, strongest in coding and cybersecurity

            - `"claude-opus-4-6"`

              Frontier intelligence for long-running agents and coding

            - `"claude-sonnet-4-6"`

              Best combination of speed and intelligence

            - `"claude-haiku-4-5"`

              Fastest model with near-frontier intelligence

            - `"claude-haiku-4-5-20251001"`

              Fastest model with near-frontier intelligence

            - `"claude-opus-4-5"`

              Premium model combining maximum intelligence with practical performance

            - `"claude-opus-4-5-20251101"`

              Premium model combining maximum intelligence with practical performance

            - `"claude-sonnet-4-5"`

              High-performance model for agents and coding

            - `"claude-sonnet-4-5-20250929"`

              High-performance model for agents and coding

            - `"claude-opus-4-1"`

              Exceptional model for specialized complex tasks

            - `"claude-opus-4-1-20250805"`

              Exceptional model for specialized complex tasks

          - `string`

        - `role: "assistant"`

          Conversational role of the generated message.

          This will always be `"assistant"`.

          - `"assistant"`

        - `stop_details: RefusalStopDetails`

          Structured information about a refusal.

          - `category: "cyber" or "bio" or "frontier_llm" or "reasoning_extraction"`

            The policy category that triggered a refusal.

            - `"cyber"`

            - `"bio"`

            - `"frontier_llm"`

            - `"reasoning_extraction"`

          - `explanation: string`

            Human-readable explanation of the refusal.

            This text is not guaranteed to be stable. `null` when no explanation is available for the category.

          - `type: "refusal"`

            - `"refusal"`

        - `stop_reason: StopReason`

          The reason that we stopped.

          This may be one the following values:

          * `"end_turn"`: the model reached a natural stopping point
          * `"max_tokens"`: we exceeded the requested `max_tokens` or the model's maximum
          * `"stop_sequence"`: one of your provided custom `stop_sequences` was generated
          * `"tool_use"`: the model invoked one or more tools
          * `"pause_turn"`: we paused a long-running turn. You may provide the response back as-is in a subsequent request to let the model continue.
          * `"refusal"`: when streaming classifiers intervene to handle potential policy violations

          In non-streaming mode this value is always non-null. In streaming mode, it is null in the `message_start` event and non-null otherwise.

          - `"end_turn"`

          - `"max_tokens"`

          - `"stop_sequence"`

          - `"tool_use"`

          - `"pause_turn"`

          - `"refusal"`

        - `stop_sequence: string`

          Which custom stop sequence was generated, if any.

          This value will be a non-null string if one of your custom stop sequences was generated.

        - `type: "message"`

          Object type.

          For Messages, this is always `"message"`.

          - `"message"`

        - `usage: Usage`

          Billing and rate-limit usage.

          Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems.

          Under the hood, the API transforms requests into a format suitable for the model. The model's output then goes through a parsing stage before becoming an API response. As a result, the token counts in `usage` will not match one-to-one with the exact visible content of an API request or response.

          For example, `output_tokens` will be non-zero, even for an empty string response from Claude.

          Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.

          - `cache_creation: CacheCreation`

            Breakdown of cached tokens by TTL

            - `ephemeral_1h_input_tokens: number`

              The number of input tokens used to create the 1 hour cache entry.

            - `ephemeral_5m_input_tokens: number`

              The number of input tokens used to create the 5 minute cache entry.

          - `cache_creation_input_tokens: number`

            The number of input tokens used to create the cache entry.

          - `cache_read_input_tokens: number`

            The number of input tokens read from the cache.

          - `inference_geo: string`

            The geographic region where inference was performed for this request.

          - `input_tokens: number`

            The number of input tokens which were used.

          - `output_tokens: number`

            The number of output tokens which were used.

          - `output_tokens_details: OutputTokensDetails`

            Breakdown of output tokens by category.

            `output_tokens` remains the inclusive, authoritative total used for billing.
            This object provides a read-only decomposition for observability — for example,
            how many of the billed output tokens were spent on internal reasoning that may
            have been summarized before being returned to you.

            - `thinking_tokens: number`

              Number of output tokens the model generated as internal reasoning, including
              the thinking-block delimiter tokens.

              Reflects the raw reasoning the model produced, not the (possibly shorter)
              summarized thinking text returned in the response body. Computed by
              re-tokenizing the raw reasoning text, so it may differ from the model's exact
              generation count by a small number of tokens. Always ≤ `output_tokens`;
              `output_tokens - thinking_tokens` approximates the non-reasoning output.

          - `server_tool_use: ServerToolUsage`

            The number of server tool requests.

            - `web_fetch_requests: number`

              The number of web fetch tool requests.

            - `web_search_requests: number`

              The number of web search tool requests.

          - `service_tier: "standard" or "priority" or "batch"`

            If the request used the priority, standard, or batch tier.

            - `"standard"`

            - `"priority"`

            - `"batch"`

      - `type: "succeeded"`

        - `"succeeded"`

    - `MessageBatchErroredResult object { error, type }`

      - `error: ErrorResponse`

        - `error: ErrorObject`

          - `InvalidRequestError object { message, type }`

            - `message: string`

            - `type: "invalid_request_error"`

              - `"invalid_request_error"`

          - `AuthenticationError object { message, type }`

            - `message: string`

            - `type: "authentication_error"`

              - `"authentication_error"`

          - `BillingError object { message, type }`

            - `message: string`

            - `type: "billing_error"`

              - `"billing_error"`

          - `PermissionError object { message, type }`

            - `message: string`

            - `type: "permission_error"`

              - `"permission_error"`

          - `NotFoundError object { message, type }`

            - `message: string`

            - `type: "not_found_error"`

              - `"not_found_error"`

          - `RateLimitError object { message, type }`

            - `message: string`

            - `type: "rate_limit_error"`

              - `"rate_limit_error"`

          - `GatewayTimeoutError object { message, type }`

            - `message: string`

            - `type: "timeout_error"`

              - `"timeout_error"`

          - `APIErrorObject object { message, type }`

            - `message: string`

            - `type: "api_error"`

              - `"api_error"`

          - `OverloadedError object { message, type }`

            - `message: string`

            - `type: "overloaded_error"`

              - `"overloaded_error"`

        - `request_id: string`

        - `type: "error"`

          - `"error"`

      - `type: "errored"`

        - `"errored"`

    - `MessageBatchCanceledResult object { type }`

      - `type: "canceled"`

        - `"canceled"`

    - `MessageBatchExpiredResult object { type }`

      - `type: "expired"`

        - `"expired"`

### Message Batch Request Counts

- `MessageBatchRequestCounts object { canceled, errored, expired, 2 more }`

  - `canceled: number`

    Number of requests in the Message Batch that have been canceled.

    This is zero until processing of the entire Message Batch has ended.

  - `errored: number`

    Number of requests in the Message Batch that encountered an error.

    This is zero until processing of the entire Message Batch has ended.

  - `expired: number`

    Number of requests in the Message Batch that have expired.

    This is zero until processing of the entire Message Batch has ended.

  - `processing: number`

    Number of requests in the Message Batch that are processing.

  - `succeeded: number`

    Number of requests in the Message Batch that have completed successfully.

    This is zero until processing of the entire Message Batch has ended.

### Message Batch Result

- `MessageBatchResult = MessageBatchSucceededResult or MessageBatchErroredResult or MessageBatchCanceledResult or MessageBatchExpiredResult`

  Processing result for this request.

  Contains a Message output if processing was successful, an error response if processing failed, or the reason why processing was not attempted, such as cancellation or expiration.

  - `MessageBatchSucceededResult object { message, type }`

    - `message: Message`

      - `id: string`

        Unique object identifier.

        The format and length of IDs may change over time.

      - `container: Container`

        Information about the container used in the request (for the code execution tool)

        - `id: string`

          Identifier for the container used in this request

        - `expires_at: string`

          The time at which the container will expire.

      - `content: array of ContentBlock`

        Content generated by the model.

        This is an array of content blocks, each of which has a `type` that determines its shape.

        Example:

        ```json
        [{"type": "text", "text": "Hi, I'm Claude."}]
        ```

        If the request input `messages` ended with an `assistant` turn, then the response `content` will continue directly from that last turn. You can use this to constrain the model's output.

        For example, if the input `messages` were:

        ```json
        [
          {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
          {"role": "assistant", "content": "The best answer is ("}
        ]
        ```

        Then the response `content` might be:

        ```json
        [{"type": "text", "text": "B)"}]
        ```

        - `TextBlock object { citations, text, type }`

          - `citations: array of TextCitation`

            Citations supporting the text block.

            The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.

            - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

              - `cited_text: string`

              - `document_index: number`

              - `document_title: string`

              - `end_char_index: number`

              - `file_id: string`

              - `start_char_index: number`

              - `type: "char_location"`

                - `"char_location"`

            - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

              - `cited_text: string`

              - `document_index: number`

              - `document_title: string`

              - `end_page_number: number`

              - `file_id: string`

              - `start_page_number: number`

              - `type: "page_location"`

                - `"page_location"`

            - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

              - `cited_text: string`

                The full text of the cited block range, concatenated.

                Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

              - `document_index: number`

              - `document_title: string`

              - `end_block_index: number`

                Exclusive 0-based end index of the cited block range in the source's `content` array.

                Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

              - `file_id: string`

              - `start_block_index: number`

                0-based index of the first cited block in the source's `content` array.

              - `type: "content_block_location"`

                - `"content_block_location"`

            - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

              - `cited_text: string`

              - `encrypted_index: string`

              - `title: string`

              - `type: "web_search_result_location"`

                - `"web_search_result_location"`

              - `url: string`

            - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

              - `cited_text: string`

                The full text of the cited block range, concatenated.

                Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

              - `end_block_index: number`

                Exclusive 0-based end index of the cited block range in the source's `content` array.

                Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

              - `search_result_index: number`

                0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

                Counted separately from `document_index`; server-side web search results are not included in this count.

              - `source: string`

              - `start_block_index: number`

                0-based index of the first cited block in the source's `content` array.

              - `title: string`

              - `type: "search_result_location"`

                - `"search_result_location"`

          - `text: string`

          - `type: "text"`

            - `"text"`

        - `ThinkingBlock object { signature, thinking, type }`

          - `signature: string`

          - `thinking: string`

          - `type: "thinking"`

            - `"thinking"`

        - `RedactedThinkingBlock object { data, type }`

          - `data: string`

          - `type: "redacted_thinking"`

            - `"redacted_thinking"`

        - `ToolUseBlock object { id, caller, input, 2 more }`

          - `id: string`

          - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

            Tool invocation directly from the model.

            - `DirectCaller object { type }`

              Tool invocation directly from the model.

              - `type: "direct"`

                - `"direct"`

            - `ServerToolCaller object { tool_id, type }`

              Tool invocation generated by a server-side tool.

              - `tool_id: string`

              - `type: "code_execution_20250825"`

                - `"code_execution_20250825"`

            - `ServerToolCaller20260120 object { tool_id, type }`

              - `tool_id: string`

              - `type: "code_execution_20260120"`

                - `"code_execution_20260120"`

          - `input: map[unknown]`

          - `name: string`

          - `type: "tool_use"`

            - `"tool_use"`

        - `ServerToolUseBlock object { id, caller, input, 2 more }`

          - `id: string`

          - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

            Tool invocation directly from the model.

            - `DirectCaller object { type }`

              Tool invocation directly from the model.

            - `ServerToolCaller object { tool_id, type }`

              Tool invocation generated by a server-side tool.

            - `ServerToolCaller20260120 object { tool_id, type }`

          - `input: map[unknown]`

          - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

            - `"web_search"`

            - `"web_fetch"`

            - `"code_execution"`

            - `"bash_code_execution"`

            - `"text_editor_code_execution"`

            - `"tool_search_tool_regex"`

            - `"tool_search_tool_bm25"`

          - `type: "server_tool_use"`

            - `"server_tool_use"`

        - `WebSearchToolResultBlock object { caller, content, tool_use_id, type }`

          - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

            Tool invocation directly from the model.

            - `DirectCaller object { type }`

              Tool invocation directly from the model.

            - `ServerToolCaller object { tool_id, type }`

              Tool invocation generated by a server-side tool.

            - `ServerToolCaller20260120 object { tool_id, type }`

          - `content: WebSearchToolResultBlockContent`

            - `WebSearchToolResultError object { error_code, type }`

              - `error_code: WebSearchToolResultErrorCode`

                - `"invalid_tool_input"`

                - `"unavailable"`

                - `"max_uses_exceeded"`

                - `"too_many_requests"`

                - `"query_too_long"`

                - `"request_too_large"`

              - `type: "web_search_tool_result_error"`

                - `"web_search_tool_result_error"`

            - `array of WebSearchResultBlock`

              - `encrypted_content: string`

              - `page_age: string`

              - `title: string`

              - `type: "web_search_result"`

                - `"web_search_result"`

              - `url: string`

          - `tool_use_id: string`

          - `type: "web_search_tool_result"`

            - `"web_search_tool_result"`

        - `WebFetchToolResultBlock object { caller, content, tool_use_id, type }`

          - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

            Tool invocation directly from the model.

            - `DirectCaller object { type }`

              Tool invocation directly from the model.

            - `ServerToolCaller object { tool_id, type }`

              Tool invocation generated by a server-side tool.

            - `ServerToolCaller20260120 object { tool_id, type }`

          - `content: WebFetchToolResultErrorBlock or WebFetchBlock`

            - `WebFetchToolResultErrorBlock object { error_code, type }`

              - `error_code: WebFetchToolResultErrorCode`

                - `"invalid_tool_input"`

                - `"url_too_long"`

                - `"url_not_allowed"`

                - `"url_not_in_prior_context"`

                - `"url_not_accessible"`

                - `"unsupported_content_type"`

                - `"too_many_requests"`

                - `"max_uses_exceeded"`

                - `"unavailable"`

              - `type: "web_fetch_tool_result_error"`

                - `"web_fetch_tool_result_error"`

            - `WebFetchBlock object { content, retrieved_at, type, url }`

              - `content: DocumentBlock`

                - `citations: CitationsConfig`

                  Citation configuration for the document

                  - `enabled: boolean`

                - `source: Base64PDFSource or PlainTextSource`

                  - `Base64PDFSource object { data, media_type, type }`

                    - `data: string`

                    - `media_type: "application/pdf"`

                      - `"application/pdf"`

                    - `type: "base64"`

                      - `"base64"`

                  - `PlainTextSource object { data, media_type, type }`

                    - `data: string`

                    - `media_type: "text/plain"`

                      - `"text/plain"`

                    - `type: "text"`

                      - `"text"`

                - `title: string`

                  The title of the document

                - `type: "document"`

                  - `"document"`

              - `retrieved_at: string`

                ISO 8601 timestamp when the content was retrieved

              - `type: "web_fetch_result"`

                - `"web_fetch_result"`

              - `url: string`

                Fetched content URL

          - `tool_use_id: string`

          - `type: "web_fetch_tool_result"`

            - `"web_fetch_tool_result"`

        - `CodeExecutionToolResultBlock object { content, tool_use_id, type }`

          - `content: CodeExecutionToolResultBlockContent`

            Code execution result with encrypted stdout for PFC + web_search results.

            - `CodeExecutionToolResultError object { error_code, type }`

              - `error_code: CodeExecutionToolResultErrorCode`

                - `"invalid_tool_input"`

                - `"unavailable"`

                - `"too_many_requests"`

                - `"execution_time_exceeded"`

              - `type: "code_execution_tool_result_error"`

                - `"code_execution_tool_result_error"`

            - `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

              - `content: array of CodeExecutionOutputBlock`

                - `file_id: string`

                - `type: "code_execution_output"`

                  - `"code_execution_output"`

              - `return_code: number`

              - `stderr: string`

              - `stdout: string`

              - `type: "code_execution_result"`

                - `"code_execution_result"`

            - `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

              Code execution result with encrypted stdout for PFC + web_search results.

              - `content: array of CodeExecutionOutputBlock`

                - `file_id: string`

                - `type: "code_execution_output"`

              - `encrypted_stdout: string`

              - `return_code: number`

              - `stderr: string`

              - `type: "encrypted_code_execution_result"`

                - `"encrypted_code_execution_result"`

          - `tool_use_id: string`

          - `type: "code_execution_tool_result"`

            - `"code_execution_tool_result"`

        - `BashCodeExecutionToolResultBlock object { content, tool_use_id, type }`

          - `content: BashCodeExecutionToolResultError or BashCodeExecutionResultBlock`

            - `BashCodeExecutionToolResultError object { error_code, type }`

              - `error_code: BashCodeExecutionToolResultErrorCode`

                - `"invalid_tool_input"`

                - `"unavailable"`

                - `"too_many_requests"`

                - `"execution_time_exceeded"`

                - `"output_file_too_large"`

              - `type: "bash_code_execution_tool_result_error"`

                - `"bash_code_execution_tool_result_error"`

            - `BashCodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

              - `content: array of BashCodeExecutionOutputBlock`

                - `file_id: string`

                - `type: "bash_code_execution_output"`

                  - `"bash_code_execution_output"`

              - `return_code: number`

              - `stderr: string`

              - `stdout: string`

              - `type: "bash_code_execution_result"`

                - `"bash_code_execution_result"`

          - `tool_use_id: string`

          - `type: "bash_code_execution_tool_result"`

            - `"bash_code_execution_tool_result"`

        - `TextEditorCodeExecutionToolResultBlock object { content, tool_use_id, type }`

          - `content: TextEditorCodeExecutionToolResultError or TextEditorCodeExecutionViewResultBlock or TextEditorCodeExecutionCreateResultBlock or TextEditorCodeExecutionStrReplaceResultBlock`

            - `TextEditorCodeExecutionToolResultError object { error_code, error_message, type }`

              - `error_code: TextEditorCodeExecutionToolResultErrorCode`

                - `"invalid_tool_input"`

                - `"unavailable"`

                - `"too_many_requests"`

                - `"execution_time_exceeded"`

                - `"file_not_found"`

              - `error_message: string`

              - `type: "text_editor_code_execution_tool_result_error"`

                - `"text_editor_code_execution_tool_result_error"`

            - `TextEditorCodeExecutionViewResultBlock object { content, file_type, num_lines, 3 more }`

              - `content: string`

              - `file_type: "text" or "image" or "pdf"`

                - `"text"`

                - `"image"`

                - `"pdf"`

              - `num_lines: number`

              - `start_line: number`

              - `total_lines: number`

              - `type: "text_editor_code_execution_view_result"`

                - `"text_editor_code_execution_view_result"`

            - `TextEditorCodeExecutionCreateResultBlock object { is_file_update, type }`

              - `is_file_update: boolean`

              - `type: "text_editor_code_execution_create_result"`

                - `"text_editor_code_execution_create_result"`

            - `TextEditorCodeExecutionStrReplaceResultBlock object { lines, new_lines, new_start, 3 more }`

              - `lines: array of string`

              - `new_lines: number`

              - `new_start: number`

              - `old_lines: number`

              - `old_start: number`

              - `type: "text_editor_code_execution_str_replace_result"`

                - `"text_editor_code_execution_str_replace_result"`

          - `tool_use_id: string`

          - `type: "text_editor_code_execution_tool_result"`

            - `"text_editor_code_execution_tool_result"`

        - `ToolSearchToolResultBlock object { content, tool_use_id, type }`

          - `content: ToolSearchToolResultError or ToolSearchToolSearchResultBlock`

            - `ToolSearchToolResultError object { error_code, error_message, type }`

              - `error_code: ToolSearchToolResultErrorCode`

                - `"invalid_tool_input"`

                - `"unavailable"`

                - `"too_many_requests"`

                - `"execution_time_exceeded"`

              - `error_message: string`

              - `type: "tool_search_tool_result_error"`

                - `"tool_search_tool_result_error"`

            - `ToolSearchToolSearchResultBlock object { tool_references, type }`

              - `tool_references: array of ToolReferenceBlock`

                - `tool_name: string`

                - `type: "tool_reference"`

                  - `"tool_reference"`

              - `type: "tool_search_tool_search_result"`

                - `"tool_search_tool_search_result"`

          - `tool_use_id: string`

          - `type: "tool_search_tool_result"`

            - `"tool_search_tool_result"`

        - `ContainerUploadBlock object { file_id, type }`

          Response model for a file uploaded to the container.

          - `file_id: string`

          - `type: "container_upload"`

            - `"container_upload"`

      - `model: Model`

        The model that will complete your prompt.

        See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

        - `"claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more`

          The model that will complete your prompt.

          See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

          - `"claude-sonnet-5"`

            High-performance model for coding and agents

          - `"claude-fable-5"`

            Next generation of intelligence for the hardest knowledge work and coding problems

          - `"claude-mythos-5"`

            Most capable model for cybersecurity and biology research

          - `"claude-opus-4-8"`

            Frontier intelligence for long-running agents and coding

          - `"claude-opus-4-7"`

            Frontier intelligence for long-running agents and coding

          - `"claude-mythos-preview"`

            New class of intelligence, strongest in coding and cybersecurity

          - `"claude-opus-4-6"`

            Frontier intelligence for long-running agents and coding

          - `"claude-sonnet-4-6"`

            Best combination of speed and intelligence

          - `"claude-haiku-4-5"`

            Fastest model with near-frontier intelligence

          - `"claude-haiku-4-5-20251001"`

            Fastest model with near-frontier intelligence

          - `"claude-opus-4-5"`

            Premium model combining maximum intelligence with practical performance

          - `"claude-opus-4-5-20251101"`

            Premium model combining maximum intelligence with practical performance

          - `"claude-sonnet-4-5"`

            High-performance model for agents and coding

          - `"claude-sonnet-4-5-20250929"`

            High-performance model for agents and coding

          - `"claude-opus-4-1"`

            Exceptional model for specialized complex tasks

          - `"claude-opus-4-1-20250805"`

            Exceptional model for specialized complex tasks

        - `string`

      - `role: "assistant"`

        Conversational role of the generated message.

        This will always be `"assistant"`.

        - `"assistant"`

      - `stop_details: RefusalStopDetails`

        Structured information about a refusal.

        - `category: "cyber" or "bio" or "frontier_llm" or "reasoning_extraction"`

          The policy category that triggered a refusal.

          - `"cyber"`

          - `"bio"`

          - `"frontier_llm"`

          - `"reasoning_extraction"`

        - `explanation: string`

          Human-readable explanation of the refusal.

          This text is not guaranteed to be stable. `null` when no explanation is available for the category.

        - `type: "refusal"`

          - `"refusal"`

      - `stop_reason: StopReason`

        The reason that we stopped.

        This may be one the following values:

        * `"end_turn"`: the model reached a natural stopping point
        * `"max_tokens"`: we exceeded the requested `max_tokens` or the model's maximum
        * `"stop_sequence"`: one of your provided custom `stop_sequences` was generated
        * `"tool_use"`: the model invoked one or more tools
        * `"pause_turn"`: we paused a long-running turn. You may provide the response back as-is in a subsequent request to let the model continue.
        * `"refusal"`: when streaming classifiers intervene to handle potential policy violations

        In non-streaming mode this value is always non-null. In streaming mode, it is null in the `message_start` event and non-null otherwise.

        - `"end_turn"`

        - `"max_tokens"`

        - `"stop_sequence"`

        - `"tool_use"`

        - `"pause_turn"`

        - `"refusal"`

      - `stop_sequence: string`

        Which custom stop sequence was generated, if any.

        This value will be a non-null string if one of your custom stop sequences was generated.

      - `type: "message"`

        Object type.

        For Messages, this is always `"message"`.

        - `"message"`

      - `usage: Usage`

        Billing and rate-limit usage.

        Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems.

        Under the hood, the API transforms requests into a format suitable for the model. The model's output then goes through a parsing stage before becoming an API response. As a result, the token counts in `usage` will not match one-to-one with the exact visible content of an API request or response.

        For example, `output_tokens` will be non-zero, even for an empty string response from Claude.

        Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.

        - `cache_creation: CacheCreation`

          Breakdown of cached tokens by TTL

          - `ephemeral_1h_input_tokens: number`

            The number of input tokens used to create the 1 hour cache entry.

          - `ephemeral_5m_input_tokens: number`

            The number of input tokens used to create the 5 minute cache entry.

        - `cache_creation_input_tokens: number`

          The number of input tokens used to create the cache entry.

        - `cache_read_input_tokens: number`

          The number of input tokens read from the cache.

        - `inference_geo: string`

          The geographic region where inference was performed for this request.

        - `input_tokens: number`

          The number of input tokens which were used.

        - `output_tokens: number`

          The number of output tokens which were used.

        - `output_tokens_details: OutputTokensDetails`

          Breakdown of output tokens by category.

          `output_tokens` remains the inclusive, authoritative total used for billing.
          This object provides a read-only decomposition for observability — for example,
          how many of the billed output tokens were spent on internal reasoning that may
          have been summarized before being returned to you.

          - `thinking_tokens: number`

            Number of output tokens the model generated as internal reasoning, including
            the thinking-block delimiter tokens.

            Reflects the raw reasoning the model produced, not the (possibly shorter)
            summarized thinking text returned in the response body. Computed by
            re-tokenizing the raw reasoning text, so it may differ from the model's exact
            generation count by a small number of tokens. Always ≤ `output_tokens`;
            `output_tokens - thinking_tokens` approximates the non-reasoning output.

        - `server_tool_use: ServerToolUsage`

          The number of server tool requests.

          - `web_fetch_requests: number`

            The number of web fetch tool requests.

          - `web_search_requests: number`

            The number of web search tool requests.

        - `service_tier: "standard" or "priority" or "batch"`

          If the request used the priority, standard, or batch tier.

          - `"standard"`

          - `"priority"`

          - `"batch"`

    - `type: "succeeded"`

      - `"succeeded"`

  - `MessageBatchErroredResult object { error, type }`

    - `error: ErrorResponse`

      - `error: ErrorObject`

        - `InvalidRequestError object { message, type }`

          - `message: string`

          - `type: "invalid_request_error"`

            - `"invalid_request_error"`

        - `AuthenticationError object { message, type }`

          - `message: string`

          - `type: "authentication_error"`

            - `"authentication_error"`

        - `BillingError object { message, type }`

          - `message: string`

          - `type: "billing_error"`

            - `"billing_error"`

        - `PermissionError object { message, type }`

          - `message: string`

          - `type: "permission_error"`

            - `"permission_error"`

        - `NotFoundError object { message, type }`

          - `message: string`

          - `type: "not_found_error"`

            - `"not_found_error"`

        - `RateLimitError object { message, type }`

          - `message: string`

          - `type: "rate_limit_error"`

            - `"rate_limit_error"`

        - `GatewayTimeoutError object { message, type }`

          - `message: string`

          - `type: "timeout_error"`

            - `"timeout_error"`

        - `APIErrorObject object { message, type }`

          - `message: string`

          - `type: "api_error"`

            - `"api_error"`

        - `OverloadedError object { message, type }`

          - `message: string`

          - `type: "overloaded_error"`

            - `"overloaded_error"`

      - `request_id: string`

      - `type: "error"`

        - `"error"`

    - `type: "errored"`

      - `"errored"`

  - `MessageBatchCanceledResult object { type }`

    - `type: "canceled"`

      - `"canceled"`

  - `MessageBatchExpiredResult object { type }`

    - `type: "expired"`

      - `"expired"`

### Message Batch Succeeded Result

- `MessageBatchSucceededResult object { message, type }`

  - `message: Message`

    - `id: string`

      Unique object identifier.

      The format and length of IDs may change over time.

    - `container: Container`

      Information about the container used in the request (for the code execution tool)

      - `id: string`

        Identifier for the container used in this request

      - `expires_at: string`

        The time at which the container will expire.

    - `content: array of ContentBlock`

      Content generated by the model.

      This is an array of content blocks, each of which has a `type` that determines its shape.

      Example:

      ```json
      [{"type": "text", "text": "Hi, I'm Claude."}]
      ```

      If the request input `messages` ended with an `assistant` turn, then the response `content` will continue directly from that last turn. You can use this to constrain the model's output.

      For example, if the input `messages` were:

      ```json
      [
        {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
        {"role": "assistant", "content": "The best answer is ("}
      ]
      ```

      Then the response `content` might be:

      ```json
      [{"type": "text", "text": "B)"}]
      ```

      - `TextBlock object { citations, text, type }`

        - `citations: array of TextCitation`

          Citations supporting the text block.

          The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.

          - `CitationCharLocation object { cited_text, document_index, document_title, 4 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_char_index: number`

            - `file_id: string`

            - `start_char_index: number`

            - `type: "char_location"`

              - `"char_location"`

          - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }`

            - `cited_text: string`

            - `document_index: number`

            - `document_title: string`

            - `end_page_number: number`

            - `file_id: string`

            - `start_page_number: number`

            - `type: "page_location"`

              - `"page_location"`

          - `CitationContentBlockLocation object { cited_text, document_index, document_title, 4 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `document_index: number`

            - `document_title: string`

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `file_id: string`

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `type: "content_block_location"`

              - `"content_block_location"`

          - `CitationsWebSearchResultLocation object { cited_text, encrypted_index, title, 2 more }`

            - `cited_text: string`

            - `encrypted_index: string`

            - `title: string`

            - `type: "web_search_result_location"`

              - `"web_search_result_location"`

            - `url: string`

          - `CitationsSearchResultLocation object { cited_text, end_block_index, search_result_index, 4 more }`

            - `cited_text: string`

              The full text of the cited block range, concatenated.

              Always equals the contents of `content[start_block_index:end_block_index]` joined together. The text block is the minimal citable unit; this field is never a substring of a single block. Not counted toward output tokens, and not counted toward input tokens when sent back in subsequent turns.

            - `end_block_index: number`

              Exclusive 0-based end index of the cited block range in the source's `content` array.

              Always greater than `start_block_index`; a single-block citation has `end_block_index = start_block_index + 1`.

            - `search_result_index: number`

              0-based index of the cited search result among all `search_result` content blocks in the request, in the order they appear across messages and tool results.

              Counted separately from `document_index`; server-side web search results are not included in this count.

            - `source: string`

            - `start_block_index: number`

              0-based index of the first cited block in the source's `content` array.

            - `title: string`

            - `type: "search_result_location"`

              - `"search_result_location"`

        - `text: string`

        - `type: "text"`

          - `"text"`

      - `ThinkingBlock object { signature, thinking, type }`

        - `signature: string`

        - `thinking: string`

        - `type: "thinking"`

          - `"thinking"`

      - `RedactedThinkingBlock object { data, type }`

        - `data: string`

        - `type: "redacted_thinking"`

          - `"redacted_thinking"`

      - `ToolUseBlock object { id, caller, input, 2 more }`

        - `id: string`

        - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

            - `type: "direct"`

              - `"direct"`

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

            - `tool_id: string`

            - `type: "code_execution_20250825"`

              - `"code_execution_20250825"`

          - `ServerToolCaller20260120 object { tool_id, type }`

            - `tool_id: string`

            - `type: "code_execution_20260120"`

              - `"code_execution_20260120"`

        - `input: map[unknown]`

        - `name: string`

        - `type: "tool_use"`

          - `"tool_use"`

      - `ServerToolUseBlock object { id, caller, input, 2 more }`

        - `id: string`

        - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

        - `input: map[unknown]`

        - `name: "web_search" or "web_fetch" or "code_execution" or 4 more`

          - `"web_search"`

          - `"web_fetch"`

          - `"code_execution"`

          - `"bash_code_execution"`

          - `"text_editor_code_execution"`

          - `"tool_search_tool_regex"`

          - `"tool_search_tool_bm25"`

        - `type: "server_tool_use"`

          - `"server_tool_use"`

      - `WebSearchToolResultBlock object { caller, content, tool_use_id, type }`

        - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

        - `content: WebSearchToolResultBlockContent`

          - `WebSearchToolResultError object { error_code, type }`

            - `error_code: WebSearchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"max_uses_exceeded"`

              - `"too_many_requests"`

              - `"query_too_long"`

              - `"request_too_large"`

            - `type: "web_search_tool_result_error"`

              - `"web_search_tool_result_error"`

          - `array of WebSearchResultBlock`

            - `encrypted_content: string`

            - `page_age: string`

            - `title: string`

            - `type: "web_search_result"`

              - `"web_search_result"`

            - `url: string`

        - `tool_use_id: string`

        - `type: "web_search_tool_result"`

          - `"web_search_tool_result"`

      - `WebFetchToolResultBlock object { caller, content, tool_use_id, type }`

        - `caller: DirectCaller or ServerToolCaller or ServerToolCaller20260120`

          Tool invocation directly from the model.

          - `DirectCaller object { type }`

            Tool invocation directly from the model.

          - `ServerToolCaller object { tool_id, type }`

            Tool invocation generated by a server-side tool.

          - `ServerToolCaller20260120 object { tool_id, type }`

        - `content: WebFetchToolResultErrorBlock or WebFetchBlock`

          - `WebFetchToolResultErrorBlock object { error_code, type }`

            - `error_code: WebFetchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"url_too_long"`

              - `"url_not_allowed"`

              - `"url_not_in_prior_context"`

              - `"url_not_accessible"`

              - `"unsupported_content_type"`

              - `"too_many_requests"`

              - `"max_uses_exceeded"`

              - `"unavailable"`

            - `type: "web_fetch_tool_result_error"`

              - `"web_fetch_tool_result_error"`

          - `WebFetchBlock object { content, retrieved_at, type, url }`

            - `content: DocumentBlock`

              - `citations: CitationsConfig`

                Citation configuration for the document

                - `enabled: boolean`

              - `source: Base64PDFSource or PlainTextSource`

                - `Base64PDFSource object { data, media_type, type }`

                  - `data: string`

                  - `media_type: "application/pdf"`

                    - `"application/pdf"`

                  - `type: "base64"`

                    - `"base64"`

                - `PlainTextSource object { data, media_type, type }`

                  - `data: string`

                  - `media_type: "text/plain"`

                    - `"text/plain"`

                  - `type: "text"`

                    - `"text"`

              - `title: string`

                The title of the document

              - `type: "document"`

                - `"document"`

            - `retrieved_at: string`

              ISO 8601 timestamp when the content was retrieved

            - `type: "web_fetch_result"`

              - `"web_fetch_result"`

            - `url: string`

              Fetched content URL

        - `tool_use_id: string`

        - `type: "web_fetch_tool_result"`

          - `"web_fetch_tool_result"`

      - `CodeExecutionToolResultBlock object { content, tool_use_id, type }`

        - `content: CodeExecutionToolResultBlockContent`

          Code execution result with encrypted stdout for PFC + web_search results.

          - `CodeExecutionToolResultError object { error_code, type }`

            - `error_code: CodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

            - `type: "code_execution_tool_result_error"`

              - `"code_execution_tool_result_error"`

          - `CodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

            - `content: array of CodeExecutionOutputBlock`

              - `file_id: string`

              - `type: "code_execution_output"`

                - `"code_execution_output"`

            - `return_code: number`

            - `stderr: string`

            - `stdout: string`

            - `type: "code_execution_result"`

              - `"code_execution_result"`

          - `EncryptedCodeExecutionResultBlock object { content, encrypted_stdout, return_code, 2 more }`

            Code execution result with encrypted stdout for PFC + web_search results.

            - `content: array of CodeExecutionOutputBlock`

              - `file_id: string`

              - `type: "code_execution_output"`

            - `encrypted_stdout: string`

            - `return_code: number`

            - `stderr: string`

            - `type: "encrypted_code_execution_result"`

              - `"encrypted_code_execution_result"`

        - `tool_use_id: string`

        - `type: "code_execution_tool_result"`

          - `"code_execution_tool_result"`

      - `BashCodeExecutionToolResultBlock object { content, tool_use_id, type }`

        - `content: BashCodeExecutionToolResultError or BashCodeExecutionResultBlock`

          - `BashCodeExecutionToolResultError object { error_code, type }`

            - `error_code: BashCodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

              - `"output_file_too_large"`

            - `type: "bash_code_execution_tool_result_error"`

              - `"bash_code_execution_tool_result_error"`

          - `BashCodeExecutionResultBlock object { content, return_code, stderr, 2 more }`

            - `content: array of BashCodeExecutionOutputBlock`

              - `file_id: string`

              - `type: "bash_code_execution_output"`

                - `"bash_code_execution_output"`

            - `return_code: number`

            - `stderr: string`

            - `stdout: string`

            - `type: "bash_code_execution_result"`

              - `"bash_code_execution_result"`

        - `tool_use_id: string`

        - `type: "bash_code_execution_tool_result"`

          - `"bash_code_execution_tool_result"`

      - `TextEditorCodeExecutionToolResultBlock object { content, tool_use_id, type }`

        - `content: TextEditorCodeExecutionToolResultError or TextEditorCodeExecutionViewResultBlock or TextEditorCodeExecutionCreateResultBlock or TextEditorCodeExecutionStrReplaceResultBlock`

          - `TextEditorCodeExecutionToolResultError object { error_code, error_message, type }`

            - `error_code: TextEditorCodeExecutionToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

              - `"file_not_found"`

            - `error_message: string`

            - `type: "text_editor_code_execution_tool_result_error"`

              - `"text_editor_code_execution_tool_result_error"`

          - `TextEditorCodeExecutionViewResultBlock object { content, file_type, num_lines, 3 more }`

            - `content: string`

            - `file_type: "text" or "image" or "pdf"`

              - `"text"`

              - `"image"`

              - `"pdf"`

            - `num_lines: number`

            - `start_line: number`

            - `total_lines: number`

            - `type: "text_editor_code_execution_view_result"`

              - `"text_editor_code_execution_view_result"`

          - `TextEditorCodeExecutionCreateResultBlock object { is_file_update, type }`

            - `is_file_update: boolean`

            - `type: "text_editor_code_execution_create_result"`

              - `"text_editor_code_execution_create_result"`

          - `TextEditorCodeExecutionStrReplaceResultBlock object { lines, new_lines, new_start, 3 more }`

            - `lines: array of string`

            - `new_lines: number`

            - `new_start: number`

            - `old_lines: number`

            - `old_start: number`

            - `type: "text_editor_code_execution_str_replace_result"`

              - `"text_editor_code_execution_str_replace_result"`

        - `tool_use_id: string`

        - `type: "text_editor_code_execution_tool_result"`

          - `"text_editor_code_execution_tool_result"`

      - `ToolSearchToolResultBlock object { content, tool_use_id, type }`

        - `content: ToolSearchToolResultError or ToolSearchToolSearchResultBlock`

          - `ToolSearchToolResultError object { error_code, error_message, type }`

            - `error_code: ToolSearchToolResultErrorCode`

              - `"invalid_tool_input"`

              - `"unavailable"`

              - `"too_many_requests"`

              - `"execution_time_exceeded"`

            - `error_message: string`

            - `type: "tool_search_tool_result_error"`

              - `"tool_search_tool_result_error"`

          - `ToolSearchToolSearchResultBlock object { tool_references, type }`

            - `tool_references: array of ToolReferenceBlock`

              - `tool_name: string`

              - `type: "tool_reference"`

                - `"tool_reference"`

            - `type: "tool_search_tool_search_result"`

              - `"tool_search_tool_search_result"`

        - `tool_use_id: string`

        - `type: "tool_search_tool_result"`

          - `"tool_search_tool_result"`

      - `ContainerUploadBlock object { file_id, type }`

        Response model for a file uploaded to the container.

        - `file_id: string`

        - `type: "container_upload"`

          - `"container_upload"`

    - `model: Model`

      The model that will complete your prompt.

      See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

      - `"claude-sonnet-5" or "claude-fable-5" or "claude-mythos-5" or 13 more`

        The model that will complete your prompt.

        See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.

        - `"claude-sonnet-5"`

          High-performance model for coding and agents

        - `"claude-fable-5"`

          Next generation of intelligence for the hardest knowledge work and coding problems

        - `"claude-mythos-5"`

          Most capable model for cybersecurity and biology research

        - `"claude-opus-4-8"`

          Frontier intelligence for long-running agents and coding

        - `"claude-opus-4-7"`

          Frontier intelligence for long-running agents and coding

        - `"claude-mythos-preview"`

          New class of intelligence, strongest in coding and cybersecurity

        - `"claude-opus-4-6"`

          Frontier intelligence for long-running agents and coding

        - `"claude-sonnet-4-6"`

          Best combination of speed and intelligence

        - `"claude-haiku-4-5"`

          Fastest model with near-frontier intelligence

        - `"claude-haiku-4-5-20251001"`

          Fastest model with near-frontier intelligence

        - `"claude-opus-4-5"`

          Premium model combining maximum intelligence with practical performance

        - `"claude-opus-4-5-20251101"`

          Premium model combining maximum intelligence with practical performance

        - `"claude-sonnet-4-5"`

          High-performance model for agents and coding

        - `"claude-sonnet-4-5-20250929"`

          High-performance model for agents and coding

        - `"claude-opus-4-1"`

          Exceptional model for specialized complex tasks

        - `"claude-opus-4-1-20250805"`

          Exceptional model for specialized complex tasks

      - `string`

    - `role: "assistant"`

      Conversational role of the generated message.

      This will always be `"assistant"`.

      - `"assistant"`

    - `stop_details: RefusalStopDetails`

      Structured information about a refusal.

      - `category: "cyber" or "bio" or "frontier_llm" or "reasoning_extraction"`

        The policy category that triggered a refusal.

        - `"cyber"`

        - `"bio"`

        - `"frontier_llm"`

        - `"reasoning_extraction"`

      - `explanation: string`

        Human-readable explanation of the refusal.

        This text is not guaranteed to be stable. `null` when no explanation is available for the category.

      - `type: "refusal"`

        - `"refusal"`

    - `stop_reason: StopReason`

      The reason that we stopped.

      This may be one the following values:

      * `"end_turn"`: the model reached a natural stopping point
      * `"max_tokens"`: we exceeded the requested `max_tokens` or the model's maximum
      * `"stop_sequence"`: one of your provided custom `stop_sequences` was generated
      * `"tool_use"`: the model invoked one or more tools
      * `"pause_turn"`: we paused a long-running turn. You may provide the response back as-is in a subsequent request to let the model continue.
      * `"refusal"`: when streaming classifiers intervene to handle potential policy violations

      In non-streaming mode this value is always non-null. In streaming mode, it is null in the `message_start` event and non-null otherwise.

      - `"end_turn"`

      - `"max_tokens"`

      - `"stop_sequence"`

      - `"tool_use"`

      - `"pause_turn"`

      - `"refusal"`

    - `stop_sequence: string`

      Which custom stop sequence was generated, if any.

      This value will be a non-null string if one of your custom stop sequences was generated.

    - `type: "message"`

      Object type.

      For Messages, this is always `"message"`.

      - `"message"`

    - `usage: Usage`

      Billing and rate-limit usage.

      Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems.

      Under the hood, the API transforms requests into a format suitable for the model. The model's output then goes through a parsing stage before becoming an API response. As a result, the token counts in `usage` will not match one-to-one with the exact visible content of an API request or response.

      For example, `output_tokens` will be non-zero, even for an empty string response from Claude.

      Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.

      - `cache_creation: CacheCreation`

        Breakdown of cached tokens by TTL

        - `ephemeral_1h_input_tokens: number`

          The number of input tokens used to create the 1 hour cache entry.

        - `ephemeral_5m_input_tokens: number`

          The number of input tokens used to create the 5 minute cache entry.

      - `cache_creation_input_tokens: number`

        The number of input tokens used to create the cache entry.

      - `cache_read_input_tokens: number`

        The number of input tokens read from the cache.

      - `inference_geo: string`

        The geographic region where inference was performed for this request.

      - `input_tokens: number`

        The number of input tokens which were used.

      - `output_tokens: number`

        The number of output tokens which were used.

      - `output_tokens_details: OutputTokensDetails`

        Breakdown of output tokens by category.

        `output_tokens` remains the inclusive, authoritative total used for billing.
        This object provides a read-only decomposition for observability — for example,
        how many of the billed output tokens were spent on internal reasoning that may
        have been summarized before being returned to you.

        - `thinking_tokens: number`

          Number of output tokens the model generated as internal reasoning, including
          the thinking-block delimiter tokens.

          Reflects the raw reasoning the model produced, not the (possibly shorter)
          summarized thinking text returned in the response body. Computed by
          re-tokenizing the raw reasoning text, so it may differ from the model's exact
          generation count by a small number of tokens. Always ≤ `output_tokens`;
          `output_tokens - thinking_tokens` approximates the non-reasoning output.

      - `server_tool_use: ServerToolUsage`

        The number of server tool requests.

        - `web_fetch_requests: number`

          The number of web fetch tool requests.

        - `web_search_requests: number`

          The number of web search tool requests.

      - `service_tier: "standard" or "priority" or "batch"`

        If the request used the priority, standard, or batch tier.

        - `"standard"`

        - `"priority"`

        - `"batch"`

  - `type: "succeeded"`

    - `"succeeded"`
