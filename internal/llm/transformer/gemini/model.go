package gemini

import (
	"encoding/json"
)

// GenerateContentRequest represents the Gemini API generateContent request format.
// Reference: https://ai.google.dev/api/generate-content
type GenerateContentRequest struct {
	// Contents is the content of the current conversation with the model.
	Contents []*Content `json:"contents"`

	// SystemInstruction is the developer set system instruction.
	SystemInstruction *Content `json:"systemInstruction,omitempty"`

	// Tools is a list of Tools the Model may use to generate the next response.
	Tools []*Tool `json:"tools,omitempty"`

	// ToolConfig is the tool configuration for any Tool specified in the request.
	ToolConfig *ToolConfig `json:"toolConfig,omitempty"`

	// GenerationConfig is the configuration options for model generation and outputs.
	GenerationConfig *GenerationConfig `json:"generationConfig,omitempty"`

	// SafetySettings is a list of unique SafetySetting instances for blocking unsafe content.
	SafetySettings []*SafetySetting `json:"safetySettings,omitempty"`

	// CachedContent is the name of the cached content used as context to serve the prediction.
	CachedContent string `json:"cachedContent,omitempty"`
}

// Content represents the multi-part content of a message.
type Content struct {
	// Parts is the ordered Parts that constitute a single message.
	Parts []*Part `json:"parts,omitempty"`

	// Role is the producer of the content. Must be either 'user' or 'model'.
	Role string `json:"role,omitempty"`
}

// Part represents a datatype containing media that is part of a multi-part Content message.
type Part struct {
	// Text is inline text.
	Text string `json:"text,omitempty"`

	// InlineData is inline media bytes.
	InlineData *Blob `json:"inlineData,omitempty"`

	// FileData is a URI based data.
	FileData *FileData `json:"fileData,omitempty"`

	// FunctionCall is a predicted FunctionCall returned from the model.
	FunctionCall *FunctionCall `json:"functionCall,omitempty"`

	// FunctionResponse is the result output of a FunctionCall.
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`

	// Thought indicates if the part is thought from the model.
	Thought bool `json:"thought,omitempty"`

	// ThoughtSignature is an opaque signature for the thought.
	ThoughtSignature string `json:"thoughtSignature,omitempty"`
}

// Blob represents raw media bytes.
type Blob struct {
	// MIMEType is the IANA standard MIME type of the source data.
	MIMEType string `json:"mimeType,omitempty"`

	// Data is the raw bytes.
	Data string `json:"data,omitempty"`
}

// FileData represents URI based data.
type FileData struct {
	// MIMEType is the IANA standard MIME type of the source data.
	MIMEType string `json:"mimeType,omitempty"`

	// FileURI is the URI.
	FileURI string `json:"fileUri,omitempty"`
}

// FunctionCall represents a predicted function call.
type FunctionCall struct {
	// ID is the unique ID of the function call.
	ID string `json:"id,omitempty"`

	// Name is the name of the function to call.
	Name string `json:"name,omitempty"`

	// Args is the function parameters and values in JSON object format.
	Args map[string]any `json:"args,omitempty"`
}

// FunctionResponse represents the result output of a FunctionCall.
type FunctionResponse struct {
	// ID is the ID of the function call this response is for.
	ID string `json:"id,omitempty"`

	// Name is the name of the function.
	Name string `json:"name,omitempty"`

	// Response is the function response in JSON object format.
	// Required. The function response in JSON object format. Use "output" key to specify
	// function output and "error" key to specify error details (if any). If "output" and
	// "error" keys are not specified, then whole "response" is treated as function output.
	Response map[string]any `json:"response,omitempty"`
}

// Tool represents a tool that the model may use.
type Tool struct {
	// FunctionDeclarations is a list of FunctionDeclarations available to the model.
	FunctionDeclarations []*FunctionDeclaration `json:"functionDeclarations,omitempty"`

	// CodeExecution enables the model to execute code as part of generation.
	CodeExecution *CodeExecution `json:"codeExecution,omitempty"`

	// GoogleSearch enables Google Search grounding.
	GoogleSearch *GoogleSearch `json:"googleSearch,omitempty"`

	// UrlContext enables URL context grounding for Gemini 2.0+.
	// This allows the model to fetch and process content from specified URLs.
	UrlContext *UrlContext `json:"urlContext,omitempty"`
}

// FunctionDeclaration represents a function declaration.
type FunctionDeclaration struct {
	// Optional. Defines the function behavior.
	// Behavior `json:"behavior,omitempty"`

	// Optional. Description and purpose of the function. Model uses it to decide how and
	// whether to call the function.
	Description string `json:"description,omitempty"`
	// Required. The name of the function to call. Must start with a letter or an underscore.
	// Must be a-z, A-Z, 0-9, or contain underscores, dots and dashes, with a maximum length
	// of 64.
	Name string `json:"name,omitempty"`
	// Optional. Describes the parameters to this function in JSON Schema Object format.
	// Reflects the Open API 3.03 Parameter Object. string Key: the name of the parameter.
	// Parameter names are case sensitive. Schema Value: the Schema defining the type used
	// for the parameter. For function with no parameters, this can be left unset. Parameter
	// names must start with a letter or an underscore and must only contain chars a-z,
	// A-Z, 0-9, or underscores with a maximum length of 64. Example with 1 required and
	// 1 optional parameter: type: OBJECT properties: param1: type: STRING param2: type:
	// INTEGER required: - param1
	Parameters json.RawMessage `json:"parameters,omitempty"`
	// Optional. Describes the parameters to the function in JSON Schema format. The schema
	// must describe an object where the properties are the parameters to the function.
	// For example: ``` { "type": "object", "properties": { "name": { "type": "string" },
	// "age": { "type": "integer" } }, "additionalProperties": false, "required": ["name",
	// "age"], "propertyOrdering": ["name", "age"] } ``` This field is mutually exclusive
	// with `parameters`.
	ParametersJsonSchema json.RawMessage `json:"parametersJsonSchema,omitempty"`
	// Optional. Describes the output from this function in JSON Schema format. Reflects
	// the Open API 3.03 Response Object. The Schema defines the type used for the response
	// value of the function.
	Response json.RawMessage `json:"response,omitempty"`
	// Optional. Describes the output from this function in JSON Schema format. The value
	// specified by the schema is the response value of the function. This field is mutually
	// exclusive with `response`.
	ResponseJsonSchema json.RawMessage `json:"responseJsonSchema,omitempty"`
}

// CodeExecution enables code execution.
type CodeExecution struct{}

// GoogleSearch enables Google Search.
type GoogleSearch struct{}

// UrlContext enables URL context grounding for Gemini 2.0+.
// This allows the model to fetch and process content from specified URLs.
type UrlContext struct{}

// ToolConfig is the tool configuration.
type ToolConfig struct {
	// FunctionCallingConfig is the function calling config.
	FunctionCallingConfig *FunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

// FunctionCallingConfig is the function calling config.
type FunctionCallingConfig struct {
	// Mode is the function calling mode. One of: AUTO, ANY, NONE.
	Mode string `json:"mode,omitempty"`

	// AllowedFunctionNames is the function names to call.
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

// GenerationConfig is the configuration options for model generation.
type GenerationConfig struct {
	// StopSequences is the set of character sequences that will stop output generation.
	StopSequences []string `json:"stopSequences,omitempty"`

	// ResponseModalities specifies the output types that the model should generate.
	// Supported values: "TEXT", "IMAGE", "AUDIO".
	ResponseModalities []string `json:"responseModalities,omitempty"`

	// ResponseMIMEType is the MIME type of the generated candidate text.
	ResponseMIMEType string `json:"responseMimeType,omitempty"`

	// ResponseSchema is the output schema of the generated candidate text.
	ResponseSchema json.RawMessage `json:"responseSchema,omitempty"`

	// CandidateCount is the number of generated responses to return.
	CandidateCount int64 `json:"candidateCount,omitempty"`

	// MaxOutputTokens is the maximum number of tokens to include in a candidate.
	MaxOutputTokens int64 `json:"maxOutputTokens,omitempty"`

	// Temperature controls the randomness of the output.
	Temperature *float64 `json:"temperature,omitempty"`

	// TopP is the maximum cumulative probability of tokens to consider when sampling.
	TopP *float64 `json:"topP,omitempty"`

	// TopK is the maximum number of tokens to consider when sampling.
	TopK *int64 `json:"topK,omitempty"`

	// PresencePenalty penalizes tokens that already appear in the generated text.
	PresencePenalty *float64 `json:"presencePenalty,omitempty"`

	// FrequencyPenalty penalizes tokens that repeatedly appear in the generated text.
	FrequencyPenalty *float64 `json:"frequencyPenalty,omitempty"`

	// Seed is used for deterministic generation.
	Seed *int64 `json:"seed,omitempty"`

	// ResponseLogprobs indicates whether to return log probabilities.
	ResponseLogprobs bool `json:"responseLogprobs,omitempty"`

	// Logprobs is the number of top candidate tokens to return log probabilities for.
	Logprobs *int64 `json:"logprobs,omitempty"`

	// ThinkingConfig is the thinking features configuration.
	ThinkingConfig *ThinkingConfig `json:"thinkingConfig,omitempty"`

	ImageConfig *ImageConfig `json:"imageConfig,omitempty"`
}

type ImageConfig struct {
	// Optional. Aspect ratio of the generated images. Supported values are
	// "1:1", "2:3", "3:2", "3:4", "4:3", "9:16", "16:9", and "21:9".
	AspectRatio string `json:"aspectRatio,omitempty"`
	// Optional. Specifies the size of generated images. Supported
	// values are `1K`, `2K`, `4K`. If not specified, the model will use default
	// value `1K`.
	ImageSize string `json:"imageSize,omitempty"`
}

// ThinkingConfig is the thinking features configuration.
type ThinkingConfig struct {
	// IncludeThoughts indicates whether to include thoughts in the response.
	IncludeThoughts bool `json:"includeThoughts,omitempty"`

	// ThinkingBudget is the thinking budget in tokens.
	ThinkingBudget *int64 `json:"thinkingBudget,omitempty"`

	// Optional. The level of thoughts tokens that the model should generate.
	ThinkingLevel string `json:"thinkingLevel,omitempty"`
}

// SafetySetting is a safety setting.
type SafetySetting struct {
	// Category is the harm category.
	Category string `json:"category,omitempty"`

	// Threshold is the harm block threshold.
	Threshold string `json:"threshold,omitempty"`
}

// GenerateContentResponse represents the Gemini API generateContent response format.
type GenerateContentResponse struct {
	// Candidates is the list of candidate responses from the model.
	Candidates []*Candidate `json:"candidates,omitempty"`

	// PromptFeedback contains content filter results for the prompt.
	PromptFeedback *PromptFeedback `json:"promptFeedback,omitempty"`

	// UsageMetadata is the usage metadata about the response.
	UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`

	// ModelVersion is the model version used to generate the response.
	ModelVersion string `json:"modelVersion,omitempty"`

	// ResponseID is used to identify each response.
	ResponseID string `json:"responseId,omitempty"`
}

// Candidate represents a response candidate generated from the model.
type Candidate struct {
	// Content is the generated content returned from the model.
	Content *Content `json:"content,omitempty"`

	// FinishReason is the reason why the model stopped generating tokens.
	FinishReason string `json:"finishReason,omitempty"`

	// Index is the index of the candidate in the list of response candidates.
	Index int64 `json:"index"`

	// SafetyRatings is the list of ratings for the safety of a response candidate.
	SafetyRatings []*SafetyRating `json:"safetyRatings,omitempty"`

	// CitationMetadata is the citation information for model-generated candidate.
	CitationMetadata *CitationMetadata `json:"citationMetadata,omitempty"`

	// TokenCount is the number of tokens for this candidate.
	TokenCount int64 `json:"tokenCount,omitempty"`

	// AvgLogprobs is the average log probability score of the candidate.
	AvgLogprobs float64 `json:"avgLogprobs,omitempty"`

	// LogprobsResult is the log-likelihood scores for the response tokens.
	LogprobsResult *LogprobsResult `json:"logprobsResult,omitempty"`

	// GroundingMetadata is the metadata specifies sources used to ground generated content.
	GroundingMetadata *GroundingMetadata `json:"groundingMetadata,omitempty"`
}

// SafetyRating is a safety rating for a piece of content.
type SafetyRating struct {
	// Category is the harm category.
	Category string `json:"category,omitempty"`

	// Probability is the harm probability level.
	Probability string `json:"probability,omitempty"`

	// Blocked indicates whether the content was filtered.
	Blocked bool `json:"blocked,omitempty"`
}

// CitationMetadata contains citation information.
type CitationMetadata struct {
	// Citations is the list of citations.
	Citations []*Citation `json:"citations,omitempty"`
}

// Citation is a citation to a source.
type Citation struct {
	// StartIndex is the start index into the content.
	StartIndex int64 `json:"startIndex,omitempty"`

	// EndIndex is the end index into the content.
	EndIndex int64 `json:"endIndex,omitempty"`

	// URI is the URI reference of the attribution.
	URI string `json:"uri,omitempty"`

	// Title is the title of the attribution.
	Title string `json:"title,omitempty"`

	// License is the license of the attribution.
	License string `json:"license,omitempty"`
}

// LogprobsResult contains log probability results.
type LogprobsResult struct {
	// TopCandidates is the list of top candidate tokens.
	TopCandidates []*TopCandidates `json:"topCandidates,omitempty"`

	// ChosenCandidates is the list of chosen candidate tokens.
	ChosenCandidates []*LogprobsCandidate `json:"chosenCandidates,omitempty"`
}

// TopCandidates contains top candidate tokens at each decoding step.
type TopCandidates struct {
	// Candidates is the list of candidates.
	Candidates []*LogprobsCandidate `json:"candidates,omitempty"`
}

// LogprobsCandidate is a candidate for logprobs.
type LogprobsCandidate struct {
	// Token is the candidate's token string value.
	Token string `json:"token,omitempty"`

	// TokenID is the candidate's token ID value.
	TokenID int64 `json:"tokenId,omitempty"`

	// LogProbability is the candidate's log probability.
	LogProbability float64 `json:"logProbability,omitempty"`
}

// PromptFeedback contains content filter results for a prompt.
type PromptFeedback struct {
	// BlockReason is the reason why the prompt was blocked.
	//
	// Enums
	// BLOCK_REASON_UNSPECIFIED	Default value. This value is unused.
	// SAFETY	Prompt was blocked due to safety reasons. Inspect safetyRatings to understand which safety category blocked it.
	// OTHER	Prompt was blocked due to unknown reasons.
	// BLOCKLIST	Prompt was blocked due to the terms which are included from the terminology blocklist.
	// PROHIBITED_CONTENT	Prompt was blocked due to prohibited content.
	// IMAGE_SAFETY	Candidates blocked due to unsafe image generation content.
	BlockReason string `json:"blockReason,omitempty"`

	// SafetyRatings is the list of safety ratings for the prompt.
	SafetyRatings []*SafetyRating `json:"safetyRatings,omitempty"`
}

// UsageMetadata contains usage metadata about the response.
type UsageMetadata struct {
	// PromptTokenCount is the number of tokens in the prompt.
	PromptTokenCount int64 `json:"promptTokenCount,omitempty"`

	// CandidatesTokenCount is the total number of tokens across all generated candidates.
	CandidatesTokenCount int64 `json:"candidatesTokenCount,omitempty"`

	// TotalTokenCount is the total number of tokens.
	TotalTokenCount int64 `json:"totalTokenCount,omitempty"`

	// CachedContentTokenCount is the number of tokens in the cached content.
	CachedContentTokenCount int64 `json:"cachedContentTokenCount,omitempty"`

	// ThoughtsTokenCount is the number of tokens in the model's thoughts.
	ThoughtsTokenCount int64 `json:"thoughtsTokenCount,omitempty"`

	// Output only. A detailed breakdown of the token count for each modality in the candidates.
	CandidatesTokensDetails []*ModalityTokenCount `json:"candidatesTokensDetails,omitempty"`

	// Output only. A detailed breakdown of the token count for each modality in the prompt.
	PromptTokensDetails []*ModalityTokenCount `json:"promptTokensDetails,omitempty"`
}

type ModalityTokenCount struct {
	Modality string `json:"modality,omitempty"`
	// Number of tokens.
	TokenCount int64 `json:"tokenCount,omitempty"`
}

// GeminiError represents an error response from the Gemini API.
type GeminiError struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error details.
type ErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// GroundingMetadata is the metadata returned to client when grounding is enabled.
type GroundingMetadata struct {
	// GroundingChunks is the list of supporting references retrieved from specified grounding source.
	GroundingChunks []*GroundingChunk `json:"groundingChunks,omitempty"`

	// GroundingSupports is the list of grounding support.
	GroundingSupports []*GroundingSupport `json:"groundingSupports,omitempty"`

	// RetrievalMetadata is the retrieval metadata.
	RetrievalMetadata *RetrievalMetadata `json:"retrievalMetadata,omitempty"`

	// SearchEntryPoint is the Google search entry for the following-up web searches.
	SearchEntryPoint *SearchEntryPoint `json:"searchEntryPoint,omitempty"`

	// WebSearchQueries is the web search queries for the following-up web search.
	WebSearchQueries []string `json:"webSearchQueries,omitempty"`
}

// GroundingChunk is a grounding chunk.
type GroundingChunk struct {
	// Web is the grounding chunk from the web.
	Web *GroundingChunkWeb `json:"web,omitempty"`

	// RetrievedContext is the grounding chunk from context retrieved by the retrieval tools.
	RetrievedContext *GroundingChunkRetrievedContext `json:"retrievedContext,omitempty"`
}

// GroundingChunkWeb is a chunk from the web.
type GroundingChunkWeb struct {
	// URI is the URI reference of the chunk.
	URI string `json:"uri,omitempty"`

	// Title is the title of the chunk.
	Title string `json:"title,omitempty"`

	// Domain is the domain of the (original) URI.
	Domain string `json:"domain,omitempty"`
}

// GroundingChunkRetrievedContext is a chunk from context retrieved by the retrieval tools.
type GroundingChunkRetrievedContext struct {
	// URI is the URI reference of the attribution.
	URI string `json:"uri,omitempty"`

	// Title is the title of the attribution.
	Title string `json:"title,omitempty"`

	// Text is the text of the attribution.
	Text string `json:"text,omitempty"`
}

// GroundingSupport is the grounding support.
type GroundingSupport struct {
	// Segment is the segment of the content this support belongs to.
	Segment *Segment `json:"segment,omitempty"`

	// GroundingChunkIndices is the list of indices (into 'grounding_chunk') specifying the citations.
	GroundingChunkIndices []int32 `json:"groundingChunkIndices,omitempty"`

	// ConfidenceScores is the confidence score of the support references.
	ConfidenceScores []float32 `json:"confidenceScores,omitempty"`
}

// Segment is the segment of the content.
type Segment struct {
	// StartIndex is the start index in the given Part, measured in bytes.
	StartIndex int32 `json:"startIndex,omitempty"`

	// EndIndex is the end index in the given Part, measured in bytes.
	EndIndex int32 `json:"endIndex,omitempty"`

	// PartIndex is the index of a Part object within its parent Content object.
	PartIndex int32 `json:"partIndex,omitempty"`

	// Text is the text corresponding to the segment from the response.
	Text string `json:"text,omitempty"`
}

// RetrievalMetadata is the metadata related to retrieval in the grounding flow.
type RetrievalMetadata struct {
	// GoogleSearchDynamicRetrievalScore is the score indicating how likely information
	// from Google Search could help answer the prompt.
	GoogleSearchDynamicRetrievalScore float32 `json:"googleSearchDynamicRetrievalScore,omitempty"`
}

// SearchEntryPoint is the Google search entry point.
type SearchEntryPoint struct {
	// RenderedContent is the web content snippet that can be embedded in a web page or an app webview.
	RenderedContent string `json:"renderedContent,omitempty"`

	// SDKBlob is the base64 encoded JSON representing array of tuple.
	SDKBlob []byte `json:"sdkBlob,omitempty"`
}
