# System One API 参考

## 概述

AxonHub 原生支持 **System One** 决策推理 API，并首发支持 **TypeSafe Jev** 决策模型。

System One 源于认知心理学中的“系统一”（快速、直觉、确定性决策）概念。不同于逐字生成自由文本的传统生成式大语言模型（LLM），System One 专注于在给定状态（State）上下文下进行快速、高置信度、带校准概率的离散决策与评估。

> [!IMPORTANT]
> **System One 不等于 Chat Completion**
> - System One 是与 OpenAI Chat Completions、OpenAI Responses、Anthropic Messages、Embedding、Rerank 平级的**原生一级 API Format**。
> - 不支持通过 `/v1/chat/completions` 调用 Jev 模型。
> - 请求与响应严格遵循原生 System One Wire Format，无需也不存在任何伪装转换。

## 支持的端点

- `POST /v1/systemone` - 原生 System One API（主入口）
- `POST /typesafe/v1/systemone` - TypeSafe 服务商命名空间入口

## 认证方式

System One API 使用标准 Bearer 令牌认证：

- **请求头**：`Authorization: Bearer <your-axonhub-api-key>`
- **内容类型**：`Content-Type: application/json`

---

## 请求格式

请求体为 JSON 格式，顶层包含 `model`、`state` 与 `questions`：

```json
{
  "model": "jev-latest",
  "state": {
    "user_id": "usr_94821",
    "recent_transactions": [
      { "amount": 12000, "currency": "USD", "country": "NG" },
      { "amount": 15, "currency": "USD", "country": "US" }
    ],
    "account_age_days": 2
  },
  "questions": {
    "is_fraud": {
      "type": "noul",
      "instructions": "判断此用户行为是否存在洗钱或欺诈风险"
    },
    "risk_level": {
      "type": "choice",
      "instructions": "根据交易风险等级进行分类",
      "criteria": {
        "low": "常规低风险交易",
        "medium": "金额异常或新账号异地交易",
        "high": "高危欺诈或洗钱嫌疑"
      }
    },
    "compliance_score": {
      "type": "score",
      "instructions": "针对该账号的合规度进行打分",
      "criteria": [
        "严重不合规",
        "存疑需复核",
        "基本合规",
        "完全合规"
      ]
    }
  }
}
```

### 顶层参数说明

| 字段 | 类型 | 必填 | 说明 |
| :--- | :--- | :---: | :--- |
| `model` | string | ✅ | 调用的模型名称（如 `jev-latest`、`jev-preview`、`jev-1.13.0` 或配置的自定义映射模型）。 |
| `state` | string / object / array | ✅ | 被评估的实体或上下文信息。AxonHub 完整保留其结构体或数组，不会被字符串化。 |
| `questions` | map<string, Question> | ✅ | 问题定义字典，键为问题标识符，值为问题规格。 |

---

## 三大决策能力原语（Primitives）

每个 Question 规格中必须指定 `type`，目前支持以下 3 种原生原语：

### 1. `noul`（真值判断）
用于判断某个陈述是否属实的二元判定问题。

- **请求定义**：
  - `type`: `"noul"`
  - `instructions`: string / object / array，指导模型如何评估。
  - `criteria`: 可选，针对 true/false 的具体准则。
- **返回结果**：
  - `noul`: `float` (0.0 ~ 1.0)，代表真值的校准概率。`noul` 是自身完整的伯努利分布，因此官方规范不返回单独的 `choice`、`probabilities` 或 `confidence`。

### 2. `choice`（多选分类）
用于从预设的多个候选类别中选择最优选项。

- **请求定义**：
  - `type`: `"choice"`
  - `instructions`: string / object / array。
  - `criteria`: map<string, any>，各候选选项及其说明。
- **返回结果**：
  - `choice`: `string`，胜出选项标签。
  - `probabilities`: `Record<string, float>`，各选项归一化概率分布。
  - `confidence`: `float` (0.0 ~ 1.0)，决策置信度。

### 3. `score`（规则评分）
用于依据有序阶梯准则对输入进行连续打分。

- **请求定义**：
  - `type`: `"score"`
  - `instructions`: string / object / array。
  - `criteria`: ordered array，有序评分标准等级列表。
- **返回结果**：
  - `score`: `float`，连续概率加权得分（含小数）。
  - `legend`: 评分图例。
  - `probabilities`: `Record<string, float>`，各等级维度的概率分布。
  - `confidence`: `float` (0.0 ~ 1.0)，置信度。

---

## 响应格式

```json
{
  "model": "jev-latest",
  "answers": {
    "is_fraud": {
      "type": "noul",
      "noul": 0.892
    },
    "risk_level": {
      "type": "choice",
      "choice": "high",
      "probabilities": {
        "low": 0.012,
        "medium": 0.108,
        "high": 0.880
      },
      "confidence": 0.88
    },
    "compliance_score": {
      "type": "score",
      "score": 1.25,
      "legend": [
        "严重不合规",
        "存疑需复核",
        "基本合规",
        "完全合规"
      ],
      "probabilities": {
        "0": 0.75,
        "1": 0.25,
        "2": 0.00,
        "3": 0.00
      },
      "confidence": 0.75
    }
  },
  "usage": {
    "input_tokens": 142,
    "output_tokens": 3
  }
}
```

AxonHub 会将 `usage.input_tokens` 与 `usage.output_tokens` 自动映射记录至系统的请求日志与费用统计中，同时返回给客户端原生字段。

---

## 调用示例

### cURL 示例

```bash
curl -X POST "http://localhost:8090/v1/systemone" \
  -H "Authorization: Bearer your-axonhub-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "jev-latest",
    "state": "Customer says: I received the wrong size and want a replacement.",
    "questions": {
      "intent": {
        "type": "choice",
        "instructions": "Classify user intent",
        "criteria": {
          "refund": "Customer wants money back",
          "exchange": "Customer wants a replacement",
          "inquiry": "General product inquiry"
        }
      }
    }
  }'
```

### Python 示例

```python
import requests

url = "http://localhost:8090/v1/systemone"
headers = {
    "Authorization": "Bearer your-axonhub-api-key",
    "Content-Type": "application/json"
}

payload = {
    "model": "jev-latest",
    "state": "The user reported receiving a damaged parcel upon delivery.",
    "questions": {
        "is_complaint": {
            "type": "noul",
            "instructions": "Determine whether this input constitutes a formal customer complaint."
        },
        "urgency": {
            "type": "choice",
            "instructions": "Determine priority urgency level",
            "criteria": {
                "low": "Non-urgent standard issue",
                "high": "Damaged goods, immediate action needed"
            }
        }
    }
}

response = requests.post(url, headers=headers, json=payload)
data = response.json()

print(f"Complaint Probability: {data['answers']['is_complaint']['noul']}")
print(f"Urgency Choice: {data['answers']['urgency']['choice']}")
print(f"Confidence: {data['answers']['urgency']['confidence']}")
```

### Go 示例

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	payload := map[string]any{
		"model": "jev-latest",
		"state": "Order payment delayed by 48 hours",
		"questions": map[string]any{
			"action": map[string]any{
				"type":         "choice",
				"instructions": "Select next system operation",
				"criteria": map[string]string{
					"wait":   "Keep waiting for gateway callback",
					"cancel": "Cancel order and release inventory",
				},
			},
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "http://localhost:8090/v1/systemone", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer your-axonhub-api-key")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Println(string(respBody))
}
```

---

## 渠道配置与高可用特性

### 1. TypeSafe 渠道配置
在 AxonHub 管理控制台中添加渠道：
- **渠道服务商**：选择 `TypeSafe`。
- **基础 URL**：默认为 `https://api.typesafe.ai/v1`。
- **协议端点**：默认且固定锁定为 `typesafe/systemone`（自动适配 `/systemone` 路由）。
- **默认模型**：系统预设包含 `jev-latest`、`jev-preview`、`jev-1.13.0`。
- **API 密钥**：支持配置单个或多个 API Key（系统支持轮询与粘性重试）。

### 2. 模型映射（Model Mapping）
客户端请求可通过 AxonHub 的模型重定向能力无缝路由：
- 客户端请求通用别名：`"model": "jev"`
- 在渠道中配置模型映射：`jev -> jev-latest`
- AxonHub 会在向上游转发时自动通过轻量级字节重写替换顶层 `model`，同时 100% 保留 `state` 和 `questions` 的原始 Pass-Through 负载。

### 3. 故障转移与重试（Failover）
- **429（限流）/ 529（上游过载）**：AxonHub 自动识别并触发渠道重试或故障转移到备用 TypeSafe 渠道。
- **401（鉴权失败）/ 422（参数校验错误）**：精准透传上游错误信息，不进行无效的无限重试。

---

## 已知限制与注意事项

1. **不支持流式传输（Streaming）**：System One 是单次确定的决策原语调用，非逐字生成的聊天模型，请求中包含 `stream: true` 会被拦截并返回 400 错误。
2. **不支持 Chat 专属参数**：请勿传入 `temperature`、`top_p`、`max_tokens`、`tools` 或 `messages` 等生成式参数。
3. **键名读取与大小写**：请通过键名读取 `criteria` 和 `probabilities`（区分大小写）。根据 JSON 规范，对象键顺序不属于稳定的 API 契约。
