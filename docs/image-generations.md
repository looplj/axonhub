### Image Generation

AxonHub supports image generation via the chat completions API. Like the [OpenRouter](https://openrouter.ai/docs/features/multimodal/image-generation).

For now, the streaming is not supported for image generation.

#### API Usage

To generate images, send a request to the `/api/v1/chat/completions` endpoint with the `modalities` parameter set to include both `"image"` and `"text"`.

##### Example

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

### 🤖 Supported Providers

| Provider             | Status  | Supported Models                                              | Notes                 |
| -------------------- | ------- | ------------------------------------------------------------- | --------------------- |
| **OpenAI**           | ✅ Done | gpt-image-1, etc.                                             | No streaming support  |
| **ByteDance Doubao** | ✅ Done | doubao-seed-dream-4-0, etc.                                   | No streaming support  |
| **OpenRouter**       | ✅ Done | gpt-image-1,gemini-2.5-flash-image-preview(nana banana), etc. | No streaming support  |
| **Gemini**           | 📝 Todo | -                                                             | Not implemented       |

---
