# Image Generation API

## Overview

AxonHub supports image generation via the chat completions API, similar to [OpenRouter's multimodal capabilities](https://openrouter.ai/docs/features/multimodal/image-generation).

**Note**: Streaming is not currently supported for image generation.

## API Usage

To generate images, send a request to the `/api/v1/chat/completions` endpoint with the `modalities` parameter set to include both `"image"` and `"text"`.

### Example

```python
import requests
import json

url = "https://your-axonhub-instance/v1/chat/completions"
headers = {
    "Authorization": f"Bearer {API_KEY_REF}",
    "Content-Type": "application/json"
}

payload = {
    "model": "{{MODEL}}",
    "messages": [
        {
            "role": "user",
            "content": "Generate a beautiful sunset over mountains"
        }
    ],
    "modalities": ["image", "text"]
}

response = requests.post(url, headers=headers, json=payload)
result = response.json()

# The generated image will be in the assistant message
if result.get("choices"):
    message = result["choices"][0]["message"]

    for content in message.get("content", []):
        if content.type == "image_url":
            image_url = content.image_url.url  # Base64 data URL
            print(f"Generated image: {image_url[:50]}...")
```

```typescript
const response = await fetch("https://your-axonhub-instance/v1/chat/completions", {
  method: "POST",
  headers: {
    Authorization: `Bearer ${API_KEY_REF}`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    model: "{{MODEL}}",
    messages: [
      {
        role: "user",
        content: "Generate a beautiful sunset over mountains",
      },
    ],
    modalities: ["image", "text"],
  }),
});

const result = await response.json();

// The generated image will be in the assistant message
if (result.choices) {
  const message = result.choices[0].message;
  if (message.content) {
    message.content.forEach((content, index) => {
      if (content.type === "image_url") {
        const imageUrl = content.image_url.url; // Base64 data URL
        console.log(
          `Generated image ${index + 1}: ${imageUrl.substring(0, 50)}...`
        );
      }
    });
  }
}
```

## Response Format

When generating images, the assistant message includes an `images` field containing the generated images:

```json
{
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": [
          {
            "type": "image_url",
            "image_url": {
              "url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA..."
            }
          }
        ]
      }
    }
  ]
}
```

## Using Custom Image Tools

Alternatively, you can generate images by using the `image_generation` tool in your request. This approach provides more control over image generation parameters.

### Tool-based Image Generation

To use custom image tools, include an `image_generation` tool in the `tools` array of your request:

### Example

```python
import requests
import json

url = "https://your-axonhub-instance/v1/chat/completions"
headers = {
    "Authorization": f"Bearer {API_KEY_REF}",
    "Content-Type": "application/json"
}

payload = {
    "model": "{{MODEL}}",
    "messages": [
        {
            "role": "user",
            "content": "Generate a beautiful sunset over mountains"
        }
    ],
    "tools": [
        {
            "type": "image_generation",
            "image_generation": {
                "quality": "high",
                "size": "1024x1024",
                "output_format": "png",
                "background": "opaque"
            }
        }
    ]
}

response = requests.post(url, headers=headers, json=payload)
result = response.json()

# The generated image will be in the tool_calls
if result.get("choices"):
    message = result["choices"][0]["message"]
    for tool_call in message.get("tool_calls", []):
        if tool_call.get("type") == "image_generation":
            print(f"Image generated with tool call ID: {tool_call.get('id')}")
```

```typescript
const response = await fetch("https://your-axonhub-instance/v1/chat/completions", {
  method: "POST",
  headers: {
    Authorization: `Bearer ${API_KEY_REF}`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    model: "{{MODEL}}",
    messages: [
      {
        role: "user",
        content: "Generate a beautiful sunset over mountains",
      },
    ],
    tools: [
      {
        type: "image_generation",
        image_generation: {
          quality: "high",
          size: "1024x1024",
          output_format: "png",
          background: "opaque",
        },
      },
    ],
  }),
});

const result = await response.json();

// The generated image will be in the tool_calls
if (result.choices) {
  const message = result.choices[0].message;
  if (message.tool_calls) {
    message.tool_calls.forEach((toolCall) => {
      if (toolCall.type === "image_generation") {
        console.log(`Image generated with tool call ID: ${toolCall.id}`);
      }
    });
  }
}
```

### Custom Image Generation Parameters

The `image_generation` tool supports the following parameters:

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `background` | string | Background style: "opaque" or "transparent" | - |
| `input_fidelity` | string | Input fidelity level | - |
| `input_image_mask` | object | Image mask for inpainting | - |
| `moderation` | string | Content moderation level: "low" or "auto" | - |
| `output_compression` | number | Compression level (0-100%) | 100 |
| `output_format` | string | Image format: "png", "webp", or "jpeg" | "png" |
| `partial_images` | number | Number of images to generate | 1 |
| `quality` | string | Image quality: "auto", "high", "medium", "low", "hd", "standard" | "auto" |
| `size` | string | Image size: "256x256", "512x512", or "1024x1024" | "1024x1024" |
| `watermark` | boolean | Whether to add watermark | depends on the model |

## Supported Providers

| Provider             | Status  | Supported Models                                              | Notes                 |
| -------------------- | ------- | ------------------------------------------------------------- | --------------------- |
| **OpenAI**           | ✅ Done | gpt-image-1, etc.                                             | No streaming support  |
| **ByteDance Doubao** | ✅ Done | doubao-seed-dream-4-0, etc.                                   | No streaming support  |
| **OpenRouter**       | ✅ Done | gpt-image-1,gemini-2.5-flash-image-preview(nana banana), etc. | No streaming support  |
| **Gemini**           | 📝 Todo | -                                                             | Not implemented       |

## Related Resources

- [Chat Completions API](unified-api.md#openai-chat-completions-api)
- [Anthropic Messages API](unified-api.md#anthropic-messages-api)
- [Claude Code Integration](../guides/claude-code-integration.md)
