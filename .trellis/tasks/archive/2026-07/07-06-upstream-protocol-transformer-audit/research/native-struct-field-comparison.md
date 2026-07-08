# Native struct field comparison

## `llm/model.go` `Request`
- upstream (47): `Messages`, `Model`, `FrequencyPenalty`, `Logprobs`, `MaxCompletionTokens`, `MaxTokens`, `PresencePenalty`, `Seed`, `Store`, `Temperature`, `TopLogprobs`, `TopP`, `PromptCacheKey`, `PreviousResponseID`, `SafetyIdentifier`, `User`, `LogitBias`, `Metadata`, `Modalities`, `ReasoningEffort`, `ReasoningBudget`, `ReasoningSummary`, `ServiceTier`, `Stop`, `Stream`, `StreamOptions`, `ParallelToolCalls`, `Tools`, `ToolChoice`, `ResponseFormat`, `Verbosity`, `ExtraBody`, `Embedding`, `Rerank`, `Image`, `Video`, `Compact`, `Completion`, `Speech`, `Transcription`, `Translation`, `RawRequest`, `RequestType`, `APIFormat`, `TransformOptions`, `TransformerMetadata`, `ProviderExtensions`
- current (47): `Messages`, `Model`, `FrequencyPenalty`, `Logprobs`, `MaxCompletionTokens`, `MaxTokens`, `PresencePenalty`, `Seed`, `Store`, `Temperature`, `TopLogprobs`, `TopP`, `PromptCacheKey`, `PreviousResponseID`, `SafetyIdentifier`, `User`, `LogitBias`, `Metadata`, `Modalities`, `ReasoningEffort`, `ReasoningBudget`, `ReasoningSummary`, `ServiceTier`, `Stop`, `Stream`, `StreamOptions`, `ParallelToolCalls`, `Tools`, `ToolChoice`, `ResponseFormat`, `Verbosity`, `ExtraBody`, `Embedding`, `Rerank`, `Image`, `Video`, `Compact`, `Completion`, `Speech`, `Transcription`, `Translation`, `RawRequest`, `RequestType`, `APIFormat`, `TransformOptions`, `TransformerMetadata`, `ProviderExtensions`
- added_current: `-`
- removed_current: `-`

## `llm/transformer/openai/model.go` `Request`
- upstream (30): `Messages`, `Model`, `FrequencyPenalty`, `Logprobs`, `MaxCompletionTokens`, `MaxTokens`, `PresencePenalty`, `Seed`, `Store`, `Temperature`, `TopLogprobs`, `TopP`, `PromptCacheKey`, `SafetyIdentifier`, `User`, `LogitBias`, `Metadata`, `Modalities`, `ReasoningEffort`, `ReasoningBudget`, `ReasoningSummary`, `ServiceTier`, `Stop`, `Stream`, `StreamOptions`, `ParallelToolCalls`, `Tools`, `ToolChoice`, `ResponseFormat`, `Verbosity`
- current (36): `Messages`, `Model`, `FrequencyPenalty`, `Logprobs`, `MaxCompletionTokens`, `MaxTokens`, `PresencePenalty`, `Seed`, `Store`, `Temperature`, `TopLogprobs`, `TopP`, `TopK`, `RepetitionPenalty`, `MinP`, `TopA`, `PromptCacheKey`, `SafetyIdentifier`, `User`, `LogitBias`, `Metadata`, `CacheControl`, `Modalities`, `ReasoningEffort`, `ReasoningBudget`, `ReasoningSummary`, `Reasoning`, `ServiceTier`, `Stop`, `Stream`, `StreamOptions`, `ParallelToolCalls`, `Tools`, `ToolChoice`, `ResponseFormat`, `Verbosity`
- added_current: `CacheControl`, `MinP`, `Reasoning`, `RepetitionPenalty`, `TopA`, `TopK`
- removed_current: `-`

## `llm/transformer/openai/responses/model.go` `Request`
- upstream (26): `Model`, `Instructions`, `Temperature`, `Input`, `Tools`, `ParallelToolCalls`, `Background`, `Stream`, `Store`, `ServiceTier`, `SafetyIdentifier`, `User`, `Metadata`, `MaxOutputTokens`, `MaxToolCalls`, `Text`, `Include`, `PreviousResponseID`, `PromptCacheKey`, `PromptCacheRetention`, `Reasoning`, `StreamOptions`, `ToolChoice`, `Truncation`, `TopLogprobs`, `TopP`
- current (33): `Model`, `Instructions`, `Temperature`, `FrequencyPenalty`, `PresencePenalty`, `Input`, `Tools`, `ParallelToolCalls`, `Background`, `Stream`, `Store`, `ServiceTier`, `SafetyIdentifier`, `User`, `Metadata`, `ClientMetadata`, `CacheControl`, `MaxOutputTokens`, `MaxToolCalls`, `Text`, `Include`, `PreviousResponseID`, `Prompt`, `PromptCacheKey`, `PromptCacheRetention`, `Reasoning`, `StreamOptions`, `ToolChoice`, `Truncation`, `TopLogprobs`, `TopP`, `TopK`, `Modalities`
- added_current: `CacheControl`, `ClientMetadata`, `FrequencyPenalty`, `Modalities`, `PresencePenalty`, `Prompt`, `TopK`
- removed_current: `-`

## `llm/transformer/anthropic/model.go` `MessageRequest`
- upstream (18): `MaxTokens`, `Messages`, `Model`, `AnthropicVersion`, `AnthropicBeta`, `Temperature`, `TopK`, `TopP`, `Metadata`, `ServiceTier`, `StopSequences`, `System`, `Thinking`, `OutputConfig`, `Tools`, `ToolChoice`, `Stream`, `CacheControl`
- current (19): `MaxTokens`, `Messages`, `Model`, `AnthropicVersion`, `AnthropicBeta`, `Temperature`, `TopK`, `TopP`, `Metadata`, `ServiceTier`, `StopSequences`, `System`, `Thinking`, `OutputConfig`, `Tools`, `ToolChoice`, `Stream`, `CacheControl`, `ContextManagement`
- added_current: `ContextManagement`
- removed_current: `-`

## `llm/provider_extensions.go` `OpenAIResponsesRequestExtensions`
- upstream (4): `RawTools`, `ToolSignatures`, `RawToolChoice`, `RawInputItems`
- current (9): `ClientMetadata`, `RawTopLevelFields`, `NativeTools`, `AdditionalTools`, `RawTools`, `ToolSignatures`, `RawToolChoice`, `RawInputItems`, `PrependCount`
- added_current: `AdditionalTools`, `ClientMetadata`, `NativeTools`, `PrependCount`, `RawTopLevelFields`
- removed_current: `-`

