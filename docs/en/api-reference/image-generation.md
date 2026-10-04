# Image Generation API

## Overview

AxonHub supports image generation via the OpenAI-compatible `/v1/images/generations` endpoint.

**Streaming**: The `/v1/images/generations` and `/v1/images/edits` endpoints support Images SSE with `stream: true` through native OpenAI-compatible Images channels. The upstream model and channel must support image streaming.

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
| `stream` | boolean | Return Images SSE events. Requires a streaming-capable model. | `false` |
| `partial_images` | integer | Requested preview count when streaming, from 0 to 3. The upstream may return fewer previews. | Upstream default |

## Image Edit (Inpainting)

To edit an image, use the `/v1/images/edits` endpoint. Multipart/form-data
requests require the `image` field; application/json requests accept either
the `image` field or the newer `images` array:

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

JSON editing accepts data URLs in `images[].image_url`, for example:

```json
{
  "model": "gpt-image-1",
  "prompt": "Change the blue circle to red; keep the white background",
  "images": [{"image_url": "data:image/png;base64,..."}],
  "stream": true,
  "partial_images": 2,
  "n": 2
}
```

AxonHub decodes these images and sends multipart/form-data to the native Images
upstream. Remote image URLs, file IDs and JSON mask objects are not supported by
the existing input parser; this streaming change does not add those input types.

### Image Edit Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `image` | file | **Required for multipart/form-data.** The image to edit. For application/json requests, either `image` or `images` must be present. | - |
| `images` | array | For application/json requests, either `image` or `images` must be present. Accepts an array of data URLs or of `{"image_url": "<data URL>"}` objects; ignored when `image` yields at least one image. | - |
| `prompt` | string | **Required.** A text description of the desired edit. | - |
| `mask` | file | An optional mask image. Transparent areas indicate where to edit. | - |
| `model` | string | The model to use. | `dall-e-2` |
| `n` | integer | The number of images to generate. | 1 |
| `size` | string | The size of the generated images. | `"1024x1024"` |
| `response_format` | string | The format: `"url"` or `"b64_json"`. | `"b64_json"` |
| `user` | string | A unique identifier for your end-user. | - |
| `background` | string | Background style: `"opaque"` or `"transparent"`. | - |
| `output_format` | string | Image format: `"png"`, `"webp"`, or `"jpeg"`. | `"png"` |
| `output_compression` | number | Compression level (0-100%). | 100 |
| `input_fidelity` | string | Input fidelity level. | - |
| `stream` | boolean | Return Images SSE events; send `true` as a form field for multipart requests. | `false` |
| `partial_images` | integer | Requested preview count when streaming, from 0 to 3. | Upstream default |

## Streaming generation and editing

Set `stream` to `true` and explicitly select a streaming-capable image model.
Native OpenAI-compatible Images SSE supports `n` from 1 to 10, subject to the
upstream model's limits; omitting `n` requests one image. DALL-E models and
`/v1/images/variations` do not support streaming. Non-streaming requests retain
their existing response format and image-count behavior.

For streaming multipart edits, non-empty `n` and `partial_images` must be
valid integers within the ranges above. Malformed strings, decimals and integer
overflow are rejected as invalid requests; omitted or blank values stay optional.

```sh
curl -N "$AXONHUB_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $AXONHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-image-1","prompt":"a cat","stream":true,"partial_images":2,"n":2}'
```

Generation may emit `image_generation.partial_image` previews and emits one
`image_generation.completed` per final image. Editing uses `image_edit.partial_image` and
`image_edit.completed`. JSON editing and multipart editing both accept the
streaming options.

```text
event: image_generation.partial_image
data: {"type":"image_generation.partial_image","b64_json":"...","partial_image_index":0,"created_at":123}

event: image_generation.completed
data: {"type":"image_generation.completed","b64_json":"...","created_at":123,"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}}

event: image_generation.completed
data: {"type":"image_generation.completed","b64_json":"...second image...","created_at":124,"usage":{"input_tokens":10,"output_tokens":21,"total_tokens":31}}
```

Each event contains a base64 image in `b64_json`. The preview index starts at
zero and identifies a preview, not an image in a multi-image batch. The
completed event carries the final image, resolved image options when
provided upstream, and usage when reported. This is an Images event stream,
with no Chat Completions `[DONE]` marker. Count completed events until `n` final
images arrive. EOF, an early `[DONE]`, or an error before that is an incomplete or failed request,
even if an earlier image completed. Previews are never substituted for final
results. Aggregated records retain all final images and sum the usage reported
by their completed events. Preview delivery depends on the upstream; accepting
`partial_images` does not guarantee previews will be sent.

Other provider-specific image streams are not converted to Images SSE by this
implementation. Use non-streaming requests on those channels.

## Supported Providers

| Provider             | Status  | Supported Models                                              | Notes                 |
| -------------------- | ------- | ------------------------------------------------------------- | --------------------- |
| **OpenAI**           | ✅ Done | gpt-image-1, dall-e-2, dall-e-3, etc.                         | Images SSE for supported GPT Image models; DALL-E is non-streaming |
| **ByteDance Doubao** | ✅ Done | doubao-seed-dream-4-0, etc.                                   | No streaming support  |
| **OpenRouter**       | ✅ Done | gpt-image-1, gemini-2.5-flash-image-preview, etc.             | No streaming support  |
| **Gemini**           | ✅ Done | gemini-2.5-flash-image, gemini-2.0-flash-preview-image-generation, etc. | No streaming support  |
| **ZAI**              | ✅ Done | -                                                             | Generation only, no edit support |

## Related Resources

- [OpenAI API](openai-api.md)
- [Anthropic API](anthropic-api.md)
- [Gemini API](gemini-api.md)
- [Claude Code Integration](../guides/claude-code-integration.md)
