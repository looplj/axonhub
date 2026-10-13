# 图像生成 API

## 概述

AxonHub 通过 OpenAI 兼容的 `/v1/images/generations` 和 `/v1/images/edits` 端点支持非流式图像生成与编辑。

**注意**：目前不支持 Images SSE。请省略 `stream` 或设置为 `false`。生成 JSON、编辑 JSON 和编辑 multipart 请求中的 `stream: true` 会在执行上游请求前被明确拒绝；AxonHub 不会合成图像流式事件。`partial_images` 不会启用流式传输。

Codex 默认使用原生 Images 端点，复用渠道 Base URL、认证、代理和显式 Images 端点覆盖配置。现有 URL 标记与路径前缀规则仍然生效；Codex 默认 Base URL 为 `https://chatgpt.com/backend-api/codex#`。Responses 端点不代表相同 Base URL 下存在 Images API，Fenno 和其他非 Codex 渠道也不例外。已有原生图像提供商端点继续可用，用户也可以为其他渠道显式配置 Images 端点。Images 端点不会继承自定义 Responses 的 Base URL 或路径。Images 请求不再转换为 Responses `image_generation` 工具调用。原生 `/v1/responses` 请求及其图像工具保留各自的行为。

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

JSON 响应会保留上游返回的 `created`、`data[].b64_json`、`data[].url`、`data[].revised_prompt` 和 `data[].generation_id`。`generation_id` 属于各个 data 项。上游提供时，还会保留顶层 `safety_identifier`（包括 null）、`output_format`、`size`、`quality`、`background` 和图像 `usage`，包括输入与输出的图像和文本 token 明细。字段及参数支持取决于上游模型；AxonHub 不会用文本模型用量替代图像用量。

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
| `stream` | boolean | 仅支持省略或 `false`；`true` 会被拒绝。 | `false` |

## 图像编辑（局部重绘）

要编辑图像，请使用 `/v1/images/edits` 端点。multipart/form-data 请求必须提供
`image` 字段；application/json 请求提供 `image` 或 `images` 数组之一即可。Codex 上游编辑使用 JSON：multipart 图像和蒙版文件转换为 data URL，JSON 中的 `image_url` 和 `file_id` 引用保留。其他原生图像提供商沿用已有上游编码。文件 ID 和远程 URL 的支持情况取决于提供商。

例如，JSON 编辑可使用以下请求体（将占位符替换为图像 data URL 或受支持的 URL）：

```json
{
  "model": "gpt-image-2",
  "prompt": "将颜色改为蓝色",
  "images": [{"image_url": "data:image/png;base64,<image-bytes>"}],
  "stream": false
}
```

Multipart 示例：

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

### 图像编辑参数

| 参数 | 类型 | 描述 | 默认值 |
|-----------|------|-------------|---------|
| `image` | file | **multipart/form-data 必填。** 要编辑的图像。application/json 请求提供 `image` 或 `images` 之一即可。 | - |
| `images` | array | JSON 图像引用：data URL 字符串，或包含 `image_url` 或 `file_id` 的对象。每个对象必须且只能指定一种引用。必须提供 `image` 或 `images` 之一。 | - |
| `prompt` | string | **必填。** 所需编辑的文本描述。 | - |
| `mask` | file / JSON reference | 可选 multipart 蒙版文件，或包含 `image_url` 或 `file_id` 的 JSON 引用。透明区域表示要编辑的位置。 | - |
| `model` | string | 要使用的模型。 | `dall-e-2` |
| `n` | integer | 要生成的图像数量。 | 1 |
| `size` | string | 生成图像的尺寸。 | `"1024x1024"` |
| `response_format` | string | 格式：`"url"` 或 `"b64_json"`。 | `"b64_json"` |
| `user` | string | 最终用户的唯一标识符。 | - |
| `background` | string | 背景样式：`"opaque"` 或 `"transparent"`。 | - |
| `output_format` | string | 图像格式：`"png"`、`"webp"` 或 `"jpeg"`。 | `"png"` |
| `output_compression` | number | 压缩级别 (0-100%)。 | 100 |
| `input_fidelity` | string | 输入保真度级别。 | - |
| `stream` | boolean | 仅支持省略或 `false`；`true` 会被拒绝，multipart 表单同样适用。 | `false` |

## 支持的提供商

| 提供商 | 状态 | 支持的模型 | 备注 |
| -------------------- | ------- | ------------------------------------------------------------- | --------------------- |
| **OpenAI** | ✅ 完成 | gpt-image-1、dall-e-2、dall-e-3 等 | 不支持流式传输 |
| **Codex** | ✅ 完成 | gpt-image-2 | 原生非流式生成与 JSON 上游编辑；可用性取决于账号权限 |
| **字节跳动豆包** | ✅ 完成 | doubao-seed-dream-4-0 等 | 不支持流式传输 |
| **OpenRouter** | ✅ 完成 | gpt-image-1、gemini-2.5-flash-image-preview 等 | 不支持流式传输 |
| **Gemini** | ✅ 完成 | gemini-2.5-flash-image、gemini-2.0-flash-preview-image-generation 等 | 不支持流式传输 |
| **ZAI** | ✅ 完成 | - | 仅支持生成，不支持编辑 |

## 相关资源

- [OpenAI API](openai-api.md)
- [Anthropic API](anthropic-api.md)
- [Gemini API](gemini-api.md)
- [Claude Code 集成](../guides/claude-code-integration.md)
