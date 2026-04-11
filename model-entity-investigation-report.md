# AxonHub Model Entity Investigation Report

## Summary
AxonHub tracks model capabilities through a combination of model type enums, model card metadata, and channel associations. While models are associated with specific endpoint types via channels, the architecture supports multi-homing through channnel associations and cross-endpoint transformation via the transformer framework.

---

## 1. Model Type Tracking

### ✅ YES - AxonHub tracks which endpoint types each model supports

**Location:** `internal/ent/schema/model.go`

The Model entity has a **`type` field** which is an Enum with the following values:

```go
field.Enum("type").Values("chat", "embedding", "rerank", "image_generation", "video_generation").Default("chat").Comment("model type")
```

This enum tracks the **model's primary type** (upstream capabilities), while channels determine which **endpoints** are used to access those capabilities.

### Model Type Map

| Model Type | Primary Use Case | Typical Channels |
|------------|------------------|------------------|
| chat | Conversational AI, text generation | openai, openai_responses, anthropic, anthropic_aws, gemini_openai, etc. |
| embedding | Text vectorization (RAG, semantic search) | openai, openai_responses, anthropic, gemini, gemini_vertex, etc. |
| rerank | Semantic reranking for search results | openai, openai_responses |
| image_generation | Image generation (DALL-E, Stable Diffusion) | openai, openai_responses, anthropic |
| video_generation | Video generation (Runway, Kling) | openai, openai_responses, anthropic_aws |

---

## 2. Model Type Field on Models

### ✅ YES - There is a `type` field on models

**Location:** `internal/ent/schema/model.go:47`

```go
field.Enum("type").Values("chat", "embedding", "rerank", "image_generation", "video_generation").Default("chat").Comment("model type")
```

**Location:** `internal/ent/model/model.go:98-117`

```go
type Type string

const (
    TypeChat            Type = "chat"
    TypeEmbedding       Type = "embedding"
    TypeRerank          Type = "rerank"
    TypeImageGeneration Type = "image_generation"
    TypeVideoGeneration Type = "video_generation"
)

func (svc *ModelService) UpdateModel(ctx context.Context, id int, input *ent.UpdateModelInput) (*ent.Model, error) {
    // Note: SetNillableType(input.Type) is used in CreateModel()
}
```

**Default Value:** `TypeChat` (if not specified)

---

## 3. How Model Capabilities Are Discovered/Configured

### 3.1. Model Card Metadata (Primary Discovery Mechanism)

**Location:** `internal/objects/model.go:26-33`

```go
type ModelCard struct {
    Reasoning   ModelCardReasoning  `json:"reasoning"`
    ToolCall    bool                `json:"toolCall"`
    Temperature bool                `json:"temperature"`
    Modalities  ModelCardModalities `json:"modalities"`
    Vision      bool                `json:"vision"`
    Cost        ModelCardCost       `json:"cost"`
    Limit       ModelCardLimit      `json:"limit"`
    Knowledge   string              `json:"knowledge"`
    ReleaseDate string              `json:"releaseDate"`
    LastUpdated string              `json:"lastUpdated"`
}
```

**Key Capability Fields in Model Card:**

| Field | Type | Purpose |
|-------|------|---------|
| `Modalities.Input` | `[]string` | Supported input modalities: "text", "image", "video" |
| `Modalities.Output` | `[]string` | Supported output modalities: "text", "image", "video" |
| `Vision` | `bool` | Whether model supports vision (image-instruct) |
| `Reasoning.Supported` | `bool` | Whether model supports reasoning capabilities |
| `ToolCall` | `bool` | Whether model supports function/tool calling |
| `Temperature` | `bool` | Whether temperature is configurable |
| `Cost` | struct | Input/output/CacheRead/CacheWrite costs |
| `Limit` | struct | Context window and output limits |

### 3.2. Model Settings and Associations (Channel Association Mechanism)

**Location:** `internal/objects/model.go:39-95`

```go
type ModelSettings struct {
    Associations []*ModelAssociation `json:"associations"`
}

type ModelAssociation struct {
    // Association types (determines how model maps to channels)
    Type             string                       `json:"type"`
    Priority         int                          `json:"priority"`
    Disabled         bool                         `json:"disabled"`

    // Channel-specific associations
    ChannelModel     *ChannelModelAssociation     `json:"channelModel"`
    ChannelRegex     *ChannelRegexAssociation     `json:"channelRegex"`
    Regex            *RegexAssociation            `json:"regex"`
    ModelID          *ModelIDAssociation          `json:"modelId"`
    ChannelTagsModel *ChannelTagsModelAssociation `json:"channelTagsModel"`
    ChannelTagsRegex *ChannelTagsRegexAssociation `json:"channelTagsRegex"`
}
```

**Model Association Types:**
1. `channelModel` - Direct model-to-channel mapping with specific model ID
2. `channelRegex` - Pattern-based mapping to channel models
3. `regex` - Pattern-based mapping across all channels (with exclusions)
4. `modelId` - Direct model-to-model association
5. `channelTagsModel` - Pattern matching using channel tags
6. `channelTagsRegex` - Pattern matching using channel tags

### 3.3. Channel Configuration (Endpoint Availability)

**Location:** `internal/ent/schema/channel.go:107-113`

```go
field.Strings("supported_models"),           // List of model IDs available
field.Strings("manual_models").Optional().Default([]string{}),  // User-configured models
field.Bool("auto_sync_supported_models").Default(false),
field.String("auto_sync_model_pattern").Optional().Default("").Comment("Regex pattern to filter models during auto-sync.")
```

---

## 4. Can a Model Be Associated with Multiple Endpoint Types?

### ✅ YES - A model can be associated with multiple endpoint types

### Mechanism:

1. **Through Channel Associations:**
   - A model can have multiple `ModelAssociation` entries
   - Each association maps to a **different channel**
   - Each channel has its own `type` field (e.g., "openai", "openai_responses", "anthropic_aws")
   - **Thus, one model can be mapped to multiple endpoint types via different channels**

2. **Through Channel Type Diversity:**
   - Channels are configured with their endpoint type (e.g., `TypeOpenai`, `TypeOpenaiResponses`, etc.)
   - Model associations determine which channels the model is available on
   - By having the same model ID in `supported_models` on multiple channel types, the model becomes available on those endpoint types

3. **Example Scenario:**
   ```
   Channel #1: type=openai, supports_models=[gpt-4]
   Channel #2: type=openai_responses, supports_models=[gpt-4]

   Result:
   - Model "gpt-4" is theoretically available on BOTH "chat" (openai) and "responses" (openai_responses) endpoint types
   - However, the actual model type field determines upstream capabilities
   ```

### Currently **No Multi-Type Field:**

The model does **NOT** have a field like `supported_endpoint_types` or `capabilities`. Instead:

1. **Model Type (`type` enum)** indicates primary use case (upstream)
2. **Model Card** provides granular capability metadata (modalities, vision, reasoning, etc.)
3. **Model Associations** with channels indicate endpoint mapping availability

### Cross-Endpoint Transformation Support:

**Location:** `llm/transformer/interfaces.go`

AxonHub supports cross-endpoint transformation via:

1. **Inbound Transformers** (`Inbound` interface):
   - Convert client request to unified request
   - Transform unified response back to client format
   - Example: `openai_responses` endpoint accepts responses format and transforms to OpenAI format

2. **Outbound Transformers** (`Outbound` interface):
   - Convert unified request to provider-specific request
   - Transform provider response to unified response
   - Example: `openai_responses` transformer handles the conversion

3. **Supported Transformers:**
   - `openai.InboundTransformer()` - OpenAI chat format
   - `openai.NewEmbeddingInboundTransformer()` - Embeddings endpoint
   - `openai.NewImageGenerationInboundTransformer()` - Image generation endpoint
   - `openai.NewImageEditInboundTransformer()` - Image editing endpoint
   - `openai.NewImageVariationInboundTransformer()` - Image variation endpoint
   - `openai.VideoInboundTransformer()` - Video endpoint
   - `antigravity`, `gemini`, `doubao`, etc. (various providers)
   - **All transformer implementations support both `TransformRequest` and `TransformResponse`**

---

## Edge Cases & Observations

### 1. Model Type vs. Channel Type

| Model `type` Enum | Typical Channel Types |
|------------------|------------------------|
| chat | openai, openai_responses, anthropic, anthropic_aws, gemini, gemini_vertex, deepseek, etc. |
| embedding | openai, openai_responses, anthropic, gemini, gemini_vertex |
| rerank | openai, openai_responses |
| image_generation | openai, openai_responses, anthropic_aws |
| video_generation | openai, openai_responses, anthropic_aws, anthropic |

**No explicit 1:1 mapping** between model type and channel type is enforced in schema. The association is at the channel level.

### 2. Capability Flags in Model Card

The ModelCard structure provides granular capability information:

- **`Modalities.Input/Output`**: Lists supported input/output modalities ("text", "image", "video")
- **`Vision`**: Boolean flag for vision capabilities
- **`Reasoning.Supported`**: Boolean flag for reasoning
- **`ToolCall`**: Boolean flag for tool/function calling

These fields could be used to programmatically determine which endpoint types (e.g., vision vs text-only) are applicable.

### 3. Frontend Configuration Examples

**Location:** `frontend/src/features/models/data/providers.json`

Example embedding models:
```json
{
  "id": "text-embedding-3-large",
  "name": "text-embedding-3-large",
  "family": "text-embedding",
  "display_name": "text-embedding-3-large",
  "type": "embedding"
}
```

Example image generation models:
```json
{
  "id": "dall-e-3",
  "name": "dall-e-3",
  "family": "image-generation",
  "type": "image_generation"
}
```

---

## Conclusion

| Question | Answer |
|----------|--------|
| 1. Does AxonHub track which endpoint types each model supports? | ✅ **YES** - Via model type enum (`chat`, `embedding`, `rerank`, `image_generation`, `video_generation`) and Model Card metadata |
| 2. Is there a 'type' field on models? | ✅ **YES** - Enum field with 5 values |
| 3. How are model capabilities discovered/configured? | - Model Card (modalities, vision, reasoning, tool_call, etc.)<br>- Model settings and associations with channels<br>- Channel configuration (supported_models, auto_sync, manual_models) |
| 4. Can a model be associated with multiple endpoint types? | ✅ **YES** - via ModelAssociation entries to multiple channels, each with different `type` values (`openai`, `openai_responses`, etc.)<br>✅ **YES** - OpenAI `gpt-4` can appear in both `TypeOpenai` and `TypeOpenaiResponses` channels |

**(Cross-endpoint transformation is supported via the transformer framework at `llm/transformer/`)**

---

## Recommendation for Cross-Endpoint Support

If a model needs to support both "chat" AND "responses" (openai_responses) endpoints:

1. **Model type**: Set to `chat` (upstream capability)
2. **Associations**:
   - Create `ModelAssociation` with `Type: "channelModel"` for a channel of type `TypeOpenai`
   - Create `ModelAssociation` with `Type: "channelModel"` for a channel of type `TypeOpenaiResponses`
3. **Channel configuration**:
   - Ensure the model ID is in both channels' `supported_models` lists
4. **No explicit multi-type field needed** - the channel associations provide this mapping