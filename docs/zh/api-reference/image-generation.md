# 图像生成 API

## 概述

AxonHub 通过 OpenAI 兼容的 `/v1/images/generations` 端点支持图像生成功能。

**流式传输**：`/v1/images/generations` 和 `/v1/images/edits` 通过原生 OpenAI 兼容 Images 渠道支持 `stream: true` 流式生成与编辑图片。上游模型和渠道也必须支持图片流式传输。

## API 使用

要生成图像，请向 `/v1/images/generations` 端点发送请求。

### 示例

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

# 访问生成的图像
for image in result.get("data", []):
    if "b64_json" in image:
        print(f"图像 (base64): {image['b64_json'][:50]}...")
    if "url" in image:
        print(f"图像 URL: {image['url']}")
    if "revised_prompt" in image:
        print(f"优化后的提示词: {image['revised_prompt']}")
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

// 访问生成的图像
if (result.data) {
  result.data.forEach((image, index) => {
    if (image.b64_json) {
      console.log(`图像 ${index + 1} (base64): ${image.b64_json.substring(0, 50)}...`);
    }
    if (image.url) {
      console.log(`图像 ${index + 1} URL: ${image.url}`);
    }
    if (image.revised_prompt) {
      console.log(`优化后的提示词: ${image.revised_prompt}`);
    }
  });
}
```

## 响应格式

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

## 请求参数

| 参数 | 类型 | 描述 | 默认值 |
|-----------|------|-------------|---------|
| `prompt` | string | **必填。** 所需图像的文本描述。 | - |
| `model` | string | 用于图像生成的模型。 | `dall-e-2` |
| `n` | integer | 要生成的图像数量。 | 1 |
| `quality` | string | 图像质量：`"standard"`、`"hd"`、`"high"`、`"medium"`、`"low"` 或 `"auto"`。 | `"auto"` |
| `response_format` | string | 返回图像的格式：`"url"` 或 `"b64_json"`。 | `"b64_json"` |
| `size` | string | 生成图像的尺寸：`"256x256"`、`"512x512"` 或 `"1024x1024"`。 | `"1024x1024"` |
| `style` | string | 生成图像的风格（仅 DALL-E 3）：`"vivid"` 或 `"natural"`。 | - |
| `user` | string | 代表最终用户的唯一标识符。 | - |
| `background` | string | 背景样式：`"opaque"` 或 `"transparent"`。 | - |
| `output_format` | string | 图像格式：`"png"`、`"webp"` 或 `"jpeg"`。 | `"png"` |
| `output_compression` | number | 压缩级别 (0-100%)。 | 100 |
| `moderation` | string | 内容审核级别：`"low"` 或 `"auto"`。 | - |
| `stream` | boolean | 返回 Images SSE 事件，需要支持流式传输的模型。 | `false` |
| `partial_images` | integer | 流式请求希望收到的预览数量，范围 0–3；上游可能返回更少预览。 | 上游默认值 |

## 图像编辑（局部重绘）

要编辑图像，请使用 `/v1/images/edits` 端点。multipart/form-data 请求必须提供
`image` 字段；application/json 请求提供 `image` 或 `images` 数组之一即可：

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
        "prompt": "将颜色改为白色",
        "size": "1024x1024",
        "n": 1
    }
    
    response = requests.post(url, headers=headers, files=files, data=data)
    result = response.json()
```

JSON 编辑可在 `images[].image_url` 中传入 data URL，例如：

```json
{
  "model": "gpt-image-1",
  "prompt": "将蓝色圆形改成红色，保留白色背景",
  "images": [{"image_url": "data:image/png;base64,..."}],
  "stream": true,
  "partial_images": 2,
  "n": 2
}
```

AxonHub 解码这些图片，并向原生 Images 上游发送 multipart/form-data。
现有入站解析器不支持远程图片 URL、文件 ID 和 JSON 对象形式的 mask；
本次流式改造没有扩展这些输入类型。

### 图像编辑参数

| 参数 | 类型 | 描述 | 默认值 |
|-----------|------|-------------|---------|
| `image` | file | **multipart/form-data 必填。** 要编辑的图像。application/json 请求提供 `image` 或 `images` 之一即可。 | - |
| `images` | array | application/json 请求下可替代 `image`：data URL 字符串数组，或 `{"image_url": "<data URL>"}` 对象数组。当 `image` 已提供图时忽略本字段；application/json 请求必须提供 `image` 或 `images` 之一。 | - |
| `prompt` | string | **必填。** 所需编辑的文本描述。 | - |
| `mask` | file | 可选的蒙版图像。透明区域表示要编辑的位置。 | - |
| `model` | string | 要使用的模型。 | `dall-e-2` |
| `n` | integer | 要生成的图像数量。 | 1 |
| `size` | string | 生成图像的尺寸。 | `"1024x1024"` |
| `response_format` | string | 格式：`"url"` 或 `"b64_json"`。 | `"b64_json"` |
| `user` | string | 最终用户的唯一标识符。 | - |
| `background` | string | 背景样式：`"opaque"` 或 `"transparent"`。 | - |
| `output_format` | string | 图像格式：`"png"`、`"webp"` 或 `"jpeg"`。 | `"png"` |
| `output_compression` | number | 压缩级别 (0-100%)。 | 100 |
| `input_fidelity` | string | 输入保真度级别。 | - |
| `stream` | boolean | 返回 Images SSE 事件；multipart 请求使用字符串 `true` 表单字段。 | `false` |
| `partial_images` | integer | 流式请求希望收到的预览数量，范围 0–3。 | 上游默认值 |

## 流式生成与编辑

设置 `stream: true`，并明确指定支持流式传输的图片模型。原生 OpenAI 兼容
Images SSE 支持 `n=1–10`，实际数量还受上游模型限制；省略 `n` 时请求一张。
DALL-E 模型以及
`/v1/images/variations` 不支持流式传输；非流式请求的响应格式和图片数量
行为保持原有支持。

流式 multipart 编辑请求中的非空 `n` 和 `partial_images` 必须是上述范围内的
有效整数。非法字符串、小数和整数溢出会作为无效请求拒绝；省略或空白值仍为可选。

```sh
curl -N "$AXONHUB_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $AXONHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-image-1","prompt":"a cat","stream":true,"partial_images":2,"n":2}'
```

生成接口可能输出 `image_generation.partial_image` 预览，并为每张成图输出一个
`image_generation.completed`。编辑接口使用
`image_edit.partial_image` 和 `image_edit.completed`。JSON 与 multipart 编辑请求
均可传入流式参数。

```text
event: image_generation.partial_image
data: {"type":"image_generation.partial_image","b64_json":"...","partial_image_index":0,"created_at":123}

event: image_generation.completed
data: {"type":"image_generation.completed","b64_json":"...","created_at":123,"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}}

event: image_generation.completed
data: {"type":"image_generation.completed","b64_json":"...second image...","created_at":124,"usage":{"input_tokens":10,"output_tokens":21,"total_tokens":31}}
```

每个事件的 `b64_json` 包含图片的 base64 数据，预览序号从零开始，它表示预览编号，
不是多图批次中的图片编号。
完成事件包含最终图片、上游返回的实际图片参数，以及上游报告的用量。
Images 事件流不包含 Chat Completions 的 `[DONE]` 标记。
收到 `n` 个完成事件后，整个请求才算完成。在此之前 EOF 或报错均表示请求未完整
完成，即使已经收到部分成图。提前收到 `[DONE]` 也不能表示图片批次成功。预览不会替代最终图片。聚合记录保留所有成图，并
累加各完成事件报告的用量。预览是否返回取决于上游；接受 `partial_images` 不代表
一定会发送预览。

本实现尚未将其他提供商的专有图片流转换为 Images SSE；这些渠道请使用
非流式请求。

## 支持的提供商

| 提供商 | 状态 | 支持的模型 | 备注 |
| -------------------- | ------- | ------------------------------------------------------------- | --------------------- |
| **OpenAI** | ✅ 完成 | gpt-image-1、dall-e-2、dall-e-3 等 | 支持流式传输的 GPT Image 模型可使用 Images SSE；DALL-E 仅支持非流式 |
| **字节跳动豆包** | ✅ 完成 | doubao-seed-dream-4-0 等 | 不支持流式传输 |
| **OpenRouter** | ✅ 完成 | gpt-image-1、gemini-2.5-flash-image-preview 等 | 不支持流式传输 |
| **Gemini** | ✅ 完成 | gemini-2.5-flash-image、gemini-2.0-flash-preview-image-generation 等 | 不支持流式传输 |
| **ZAI** | ✅ 完成 | - | 仅支持生成，不支持编辑 |

## 相关资源

- [OpenAI API](openai-api.md)
- [Anthropic API](anthropic-api.md)
- [Gemini API](gemini-api.md)
- [Claude Code 集成](../guides/claude-code-integration.md)
