# NanoGPT GLM-4.7 Tool Calling Issue - Investigation Report

## Executive Summary

**Issue**: Tool calling fails when going through AxonHub for NanoGPT's GLM-4.7 model, but works when accessing NanoGPT directly.

**Root Cause**: The ZAI transformer in AxonHub does not properly handle the `reasoning` field that NanoGPT returns in responses. This field is stripped, causing tool calling validation failures in synbad tests.

---

## Test Results Comparison

### Via AxonHub (Failing)
```json
{
  "role": "assistant",
  "content": "",
  "tool_calls": [...]
}
```

### Via NanoGPT Direct (Passing)
```json
{
  "role": "assistant",
  "content": null,
  "reasoning": "2/packages/backend/ts",
  "tool_calls": [...]
}
```

**Key Differences**:
1. `content` is `""` (empty string) via AxonHub vs `null` via NanoGPT directly
2. `reasoning` field is **MISSING** via AxonHub but present via NanoGPT

---

## Root Cause Analysis

### 1. Channel Type Mismatch

The user is using a **ZAI channel** (channel type `zai` or `zhipu`) configured in AxonHub to connect to NanoGPT.

**Channel Types Defined**:
- `TypeZai` - Uses the ZAI transformer (`/home/djdembeck/projects/github/axonhub/internal/ent/channel/channel.go:203`)
- `TypeZhipu` - Also uses the ZAI transformer (`/home/djdembeck/projects/github/axonhub/internal/ent/channel/channel.go:204`)

**ZAI Transformer Location**: `/home/djdembeck/projects/github/axonhub/llm/transformer/zai/outbound.go`

### 2. ZAI Transformer Response Handling (BROKEN)

The ZAI transformer delegates response handling to the standard OpenAI transformer:

```go
// Line 200-202 of /home/djdembeck/projects/github/axonhub/llm/transformer/zai/outbound.go
func (t *OutboundTransformer) TransformResponse(...) (*llm.Response, error) {
    // ...
    return t.Outbound.TransformResponse(ctx, httpResp)
}
```

The standard OpenAI transformer does NOT handle the `reasoning` field that NanoGPT returns.

### 3. OpenRouter Transformer (WORKING)

The OpenRouter transformer has proper handling for the `reasoning` field:

```go
// Lines 283-290 of /home/djdembeck/projects/github/axonhub/llm/transformer/openrouter/outbound.go
func (t *OutboundTransformer) TransformResponse(...) (*llm.Response, error) {
    var chatResp Response
    err := json.Unmarshal(httpResp.Body, &chatResp)
    return chatResp.ToOpenAIResponse().ToLLMResponse(), nil
}
```

**OpenRouter Message Model** (`/home/djdembeck/projects/github/axonhub/llm/transformer/openrouter/model.go:48-75`):
```go
type Message struct {
    openai.Message
    Reasoning        *string           `json:"reasoning,omitempty"`
    ReasoningDetails []ReasoningDetail `json:"reasoning_details,omitempty"`
    Images           []Image           `json:"images,omitempty"`
}

func (m *Message) ToOpenAIMessage() openai.Message {
    // Handle reasoning content - prefer reasoning_details if available, fallback to reasoning
    if len(m.ReasoningDetails) > 0 {
        // ... aggregate reasoning_details
        m.ReasoningContent = &reasoning
    } else if m.Reasoning != nil {
        m.ReasoningContent = m.Reasoning  // <-- THIS IS THE KEY LINE
    }
    // ...
}
```

---

## The Problem

NanoGPT returns a non-standard `reasoning` field in responses (similar to OpenRouter):
```json
{
  "choices": [{
    "message": {
      "content": null,
      "reasoning": "some reasoning text",
      "tool_calls": [...]
    }
  }]
}
```

The ZAI transformer doesn't recognize this field, so it's lost during transformation, resulting in:
```json
{
  "content": "",
  "tool_calls": [...]
  // reasoning field is MISSING
}
```

This causes synbad tests to fail because they expect either `content` or `reasoning` to be non-empty.

---

## Solution

### Option 1: Fix ZAI Transformer (Recommended)

Update the ZAI transformer to handle the `reasoning` field in responses, similar to the OpenRouter transformer.

**Files to Modify**:
1. `/home/djdembeck/projects/github/axonhub/llm/transformer/zai/model.go` - Create a new file with response models
2. `/home/djdembeck/projects/github/axonhub/llm/transformer/zai/outbound.go` - Update `TransformResponse` method

### Option 2: Use OpenRouter Channel Type

Configure the channel as `openrouter` type instead of `zai` type. The OpenRouter transformer already handles the reasoning field properly.

**Note**: This might require adjusting the base URL and API key configuration.

---

## Recommended Fix

Implement Option 1 by updating the ZAI transformer to handle the `reasoning` field. This maintains backward compatibility and fixes the issue for all ZAI channel users.

### Implementation Details

1. **Create `/home/djdembeck/projects/github/axonhub/llm/transformer/zai/model.go`**:
   - Define `Response`, `Choice`, and `Message` structs
   - Include `Reasoning` field in Message struct
   - Implement `ToOpenAIResponse()` method to map reasoning to `ReasoningContent`

2. **Update `/home/djdembeck/projects/github/axonhub/llm/transformer/zai/outbound.go`**:
   - Modify `TransformResponse` to use ZAI-specific response parsing
   - Map `reasoning` field to `ReasoningContent`

---

## Files Involved

| File | Purpose |
|------|---------|
| `/home/djdembeck/projects/github/axonhub/llm/transformer/zai/outbound.go` | ZAI transformer - needs fix |
| `/home/djdembeck/projects/github/axonhub/llm/transformer/openrouter/model.go` | Reference implementation |
| `/home/djdembeck/projects/github/axonhub/llm/transformer/openrouter/outbound.go` | Reference implementation |
| `/home/djdembeck/projects/github/axonhub/internal/server/biz/channel_llm.go` | Channel type mapping |
| `/home/djdembeck/projects/github/axonhub/internal/ent/channel/channel.go` | Channel type definitions |

---

## Verification

After the fix, the test should pass:
```bash
export AXON_KEY="ah-9200718d76437c70f426434211149532d5f547a792e74deb4f8a34fa6db25972"
synbad eval --env-var AXON_KEY \
  --base-url "https://axonhub.theiahd.nl/v1" \
  --model "glm-4.7"
```

All tests should show ✅ instead of ❌.
