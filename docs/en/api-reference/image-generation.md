# Image Generation API

## Overview

AxonHub supports non-streaming image generation and editing via the OpenAI-compatible `/v1/images/generations` and `/v1/images/edits` endpoints.

**Note**: Images SSE is not currently supported. Omit `stream` or set it to `false`. Generation JSON, edit JSON, and edit multipart requests with `stream: true` are explicitly rejected before upstream execution; AxonHub does not synthesize image streaming events. `partial_images` does not enable streaming.

Codex uses native Images endpoints by default, reusing the channel's Base URL, authentication, proxy, and explicit Images endpoint overrides. Existing URL markers and path-prefix rules still apply; the default Codex Base URL is `https://chatgpt.com/backend-api/codex#`. Responses endpoints do not imply an Images API at the same Base URL, including on Fenno and other non-Codex channels. Existing native image provider endpoints remain available, and users can explicitly configure Images endpoints on other channels. Images endpoints do not inherit custom Responses Base URLs or paths. Images requests are no longer converted to Responses `image_generation` tool calls. Native `/v1/responses` requests and their image tools retain their own behavior.

## API Usage

To generate images, send a request to the `/v1/images/generations` endpoint.

### Example

```python
import requests
import json

url = "https://your-axonhub-instance/v1/images/generations"
headers = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}

payload = {
    "model": "gpt-image-1",
    "prompt": "Generate a beautiful sunset over mountains",
    "size": "1024x1024",
    "quality": "high",
    "n": 1
}

response = requests.post(url, headers=headers, json=payload)
result = response.json()

# Access generated images
for image in result.get("data", []):
    if "b64_json" in image:
        print(f"Image (base64): {image['b64_json'][:50]}...")
    if "url" in image:
        print(f"Image URL: {image['url']}")
    if "revised_prompt" in image:
        print(f"Revised prompt: {image['revised_prompt']}")
```

```typescript
const response = await fetch("https://your-axonhub-instance/v1/images/generations", {
  method: "POST",
  headers: {
    Authorization: `Bearer ${API_KEY}`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    model: "gpt-image-1",
    prompt: "Generate a beautiful sunset over mountains",
    size: "1024x1024",
    quality: "high",
    n: 1,
  }),
});

const result = await response.json();

// Access generated images
if (result.data) {
  result.data.forEach((image, index) => {
    if (image.b64_json) {
      console.log(`Image ${index + 1} (base64): ${image.b64_json.substring(0, 50)}...`);
    }
    if (image.url) {
      console.log(`Image ${index + 1} URL: ${image.url}`);
    }
    if (image.revised_prompt) {
      console.log(`Revised prompt: ${image.revised_prompt}`);
    }
  });
}
```

## Response Format

```json
{
  "created": 1699000000,
  "data": [
    {
      "b64_json": "iVBORw0KGgoAAAANSUhEUgAA...",
      "url": "https://...",
      "revised_prompt": "A beautiful sunset over mountains with orange and purple sky"
    }
  ]
}
```

The JSON response preserves upstream `created`, `data[].b64_json`, `data[].url`, `data[].revised_prompt`, and `data[].generation_id` when returned. `generation_id` belongs to each data item. Top-level `safety_identifier` (including null), `output_format`, `size`, `quality`, `background`, and image `usage` are preserved when supplied, including input/output token details for image and text tokens. Fields and parameter support depend on the upstream model; AxonHub does not substitute text-model usage for image usage.

## Request Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `prompt` | string | **Required.** A text description of the desired image(s). | - |
| `model` | string | The model to use for image generation. | `dall-e-2` |
| `n` | integer | The number of images to generate. | 1 |
| `quality` | string | The quality of the image: `"standard"`, `"hd"`, `"high"`, `"medium"`, `"low"`, or `"auto"`. | `"auto"` |
| `response_format` | string | The format in which to return the images: `"url"` or `"b64_json"`. | `"b64_json"` |
| `size` | string | The size of the generated images: `"256x256"`, `"512x512"`, or `"1024x1024"`. | `"1024x1024"` |
| `style` | string | The style of the generated images (DALL-E 3 only): `"vivid"` or `"natural"`. | - |
| `user` | string | A unique identifier representing your end-user. | - |
| `background` | string | Background style: `"opaque"` or `"transparent"`. | - |
| `output_format` | string | Image format: `"png"`, `"webp"`, or `"jpeg"`. | `"png"` |
| `output_compression` | number | Compression level (0-100%). | 100 |
| `moderation` | string | Content moderation level: `"low"` or `"auto"`. | - |
| `stream` | boolean | Only omitted or `false` is supported; `true` is rejected. | `false` |

## Image Edit (Inpainting)

To edit an image, use the `/v1/images/edits` endpoint. Multipart/form-data
requests require the `image` field; application/json requests accept either
the `image` field or the newer `images` array. Codex sends edits upstream as JSON: multipart image and mask files become data URLs, while JSON `image_url` and `file_id` references are retained. Other native image providers retain their existing upstream encoding. Upstream support for file IDs and remote URLs varies by provider.

For example, a JSON edit can use the following body (replace the placeholder with your image data URL or supported URL):

```json
{
  "model": "gpt-image-2",
  "prompt": "Change the color to blue",
  "images": [{"image_url": "data:image/png;base64,<image-bytes>"}],
  "stream": false
}
```

Multipart example:

```python
import requests

url = "https://your-axonhub-instance/v1/images/edits"
headers = {
    "Authorization": f"Bearer {API_KEY}"
}

with open("image.png", "rb") as image_file, open("mask.png", "rb") as mask_file:
    files = {
        "image": image_file,
        "mask": mask_file
    }
    data = {
        "model": "gpt-image-1",
        "prompt": "Change the color to white",
        "size": "1024x1024",
        "n": 1
    }
    
    response = requests.post(url, headers=headers, files=files, data=data)
    result = response.json()
```

### Image Edit Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `image` | file | **Required for multipart/form-data.** The image to edit. For application/json requests, either `image` or `images` must be present. | - |
| `images` | array | JSON image references: data URL strings or objects with `image_url` or `file_id`. Each object must specify exactly one reference. Either `image` or `images` must be present. | - |
| `prompt` | string | **Required.** A text description of the desired edit. | - |
| `mask` | file / JSON reference | Optional multipart mask file or JSON reference with `image_url` or `file_id`. Transparent areas indicate where to edit. | - |
| `model` | string | The model to use. | `dall-e-2` |
| `n` | integer | The number of images to generate. | 1 |
| `size` | string | The size of the generated images. | `"1024x1024"` |
| `response_format` | string | The format: `"url"` or `"b64_json"`. | `"b64_json"` |
| `user` | string | A unique identifier for your end-user. | - |
| `background` | string | Background style: `"opaque"` or `"transparent"`. | - |
| `output_format` | string | Image format: `"png"`, `"webp"`, or `"jpeg"`. | `"png"` |
| `output_compression` | number | Compression level (0-100%). | 100 |
| `input_fidelity` | string | Input fidelity level. | - |
| `stream` | boolean | Only omitted or `false` is supported; `true` is rejected, including in multipart forms. | `false` |

## Supported Providers

| Provider             | Status  | Supported Models                                              | Notes                 |
| -------------------- | ------- | ------------------------------------------------------------- | --------------------- |
| **OpenAI**           | ✅ Done | gpt-image-1, dall-e-2, dall-e-3, etc.                         | No streaming support  |
| **Codex**            | ✅ Done | gpt-image-2                                                    | Native non-streaming generation and JSON upstream edits; availability depends on account access |
| **ByteDance Doubao** | ✅ Done | doubao-seed-dream-4-0, etc.                                   | No streaming support  |
| **OpenRouter**       | ✅ Done | gpt-image-1, gemini-2.5-flash-image-preview, etc.             | No streaming support  |
| **Gemini**           | ✅ Done | gemini-2.5-flash-image, gemini-2.0-flash-preview-image-generation, etc. | No streaming support  |
| **ZAI**              | ✅ Done | -                                                             | Generation only, no edit support |

## Related Resources

- [OpenAI API](openai-api.md)
- [Anthropic API](anthropic-api.md)
- [Gemini API](gemini-api.md)
- [Claude Code Integration](../guides/claude-code-integration.md)
