# 图像生成 API

## 概述

AxonHub 通过聊天补全 API 支持图像生成功能，类似于 [OpenRouter 的多模态功能](https://openrouter.ai/docs/features/multimodal/image-generation)。

**注意**：图像生成目前不支持流式传输。

## API 使用

要生成图像，请向 `/api/v1/chat/completions` 端点发送请求，并将 `modalities` 参数设置为包含 `"image"` 和 `"text"`。

### 示例

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

# 生成的图像将在助手的消息中
if result.get("choices"):
    message = result["choices"][0]["message"]

    for content in message.get("content", []):
        if content.type == "image_url":
            image_url = content.image_url.url  # Base64 数据 URL
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

// 生成的图像将在助手的消息中
if (result.choices) {
  const message = result.choices[0].message;
  if (message.content) {
    message.content.forEach((content, index) => {
      if (content.type === "image_url") {
        const imageUrl = content.image_url.url; // Base64 数据 URL
        console.log(
          `Generated image ${index + 1}: ${imageUrl.substring(0, 50)}...`
        );
      }
    });
  }
}
```

## 响应格式

生成图像时，助手消息包含一个 `images` 字段，其中包含生成的图像：

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

## 使用自定义图像工具

或者，您可以通过在请求中使用 `image_generation` 工具来生成图像。这种方法提供了对图像生成参数的更多控制。

### 基于工具的图像生成

要使用自定义图像工具，请在请求的 `tools` 数组中包含 `image_generation` 工具：

### 示例

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

# 生成的图像将在 tool_calls 中
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

// 生成的图像将在 tool_calls 中
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

### 自定义图像生成参数

`image_generation` 工具支持以下参数：

| 参数 | 类型 | 描述 | 默认值 |
| ----------- | -------- | --------------------------- | -------- |
| `background` | string | 背景样式："opaque" 或 "transparent" | - |
| `input_fidelity` | string | 输入保真度级别 | - |
| `input_image_mask` | object | 用于修复的图像掩码 | - |
| `moderation` | string | 内容审核级别："low" 或 "auto" | - |
| `output_compression` | number | 压缩级别 (0-100%) | 100 |
| `output_format` | string | 图像格式："png"、"webp" 或 "jpeg" | "png" |
| `partial_images` | number | 要生成的图像数量 | 1 |
| `quality` | string | 图像质量："auto"、"high"、"medium"、"low"、"hd"、"standard" | "auto" |
| `size` | string | 图像大小："256x256"、"512x512" 或 "1024x1024" | "1024x1024" |
| `watermark` | boolean | 是否添加水印 | 取决于模型 |

## 支持的提供商

| 提供商 | 状态 | 支持的模型 | 备注 |
| -------------------------- | ------- | ------------------------------------------------------------ | ------------------- |
| **OpenAI** | ✅ 完成 | gpt-image-1 等 | 不支持流式传输 |
| **字节跳动豆包** | ✅ 完成 | doubao-seed-dream-4-0 等 | 不支持流式传输 |
| **OpenRouter** | ✅ 完成 | gpt-image-1、gemini-2.5-flash-image-preview(nana banana) 等 | 不支持流式传输 |
| **Gemini** | 📝 待办 | - | 未实现 |

## 相关资源

- [聊天补全 API](unified-api.md#openai-chat-completions-api)
- [Anthropic 消息 API](unified-api.md#anthropic-messages-api)
- [Claude Code 集成](../guides/claude-code-integration.md)
