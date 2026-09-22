# System One API Reference

## Overview

AxonHub natively supports the **System One** decision inference API, launched initially with support for the **TypeSafe Jev** decision model.

System One is rooted in cognitive psychology's "System 1" thinking (fast, intuitive, and deterministic decision-making). Unlike conventional generative Large Language Models (LLMs) that produce free text token-by-token, System One focuses on making fast, highly calibrated, probabilistic judgments and classifications over a structured input state.

> [!IMPORTANT]
> **System One is NOT Chat Completion**
> - System One is a **native first-class API format**, on equal footing with OpenAI Chat Completions, OpenAI Responses, Anthropic Messages, Embeddings, and Rerank.
> - Invoking Jev models via `/v1/chat/completions` is not supported.
> - Both requests and responses strictly adhere to the native System One Wire Format with zero masquerading or conversational overhead.

## Supported Endpoints

- `POST /v1/systemone` - Native System One API (Primary Entrypoint)
- `POST /typesafe/v1/systemone` - TypeSafe provider namespace alias

## Authentication

The System One API uses standard Bearer token authentication:

- **Header**: `Authorization: Bearer <your-axonhub-api-key>`
- **Content-Type**: `application/json`

---

## Request Format

The request body is a JSON object containing `model`, `state`, and `questions`:

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
      "instructions": "Determine if this activity exhibits fraud or money laundering risk"
    },
    "risk_level": {
      "type": "choice",
      "instructions": "Classify transaction risk tier",
      "criteria": {
        "low": "Standard low-risk transaction",
        "medium": "Unusual amount or new account cross-border transaction",
        "high": "High risk of fraud or money laundering"
      }
    },
    "compliance_score": {
      "type": "score",
      "instructions": "Score overall compliance for this account",
      "criteria": [
        "Critically non-compliant",
        "Dubious, needs review",
        "Basically compliant",
        "Fully compliant"
      ]
    }
  }
}
```

### Parameters

| Parameter | Type | Required | Description |
| :--- | :--- | :---: | :--- |
| `model` | string | ✅ | Target model name (e.g., `jev-latest`, `jev-preview`, `jev-1.13.0`, or a configured model mapping alias). |
| `state` | string / object / array | ✅ | Context or entity to evaluate. AxonHub preserves objects and arrays without lossy stringification. |
| `questions` | map<string, Question> | ✅ | Map of question identifiers to question specifications. |

---

## Three Decision Primitives

Each question specification requires a `type` matching one of the 3 supported primitives:

### 1. `noul` (Truth Assessment)
Evaluates whether a statement or assertion is true in a binary judgment context.

- **Request Definition**:
  - `type`: `"noul"`
  - `instructions`: string / object / array guiding evaluation.
  - `criteria`: Optional true/false rubric.
- **Response Fields**:
  - `noul`: `float` (0.0 ~ 1.0), representing calibrated truth probability. Under the official specification, `noul` represents a complete 1-D Bernoulli distribution, so separate `choice`, `probabilities`, or `confidence` fields are not returned.

### 2. `choice` (Categorical Selection)
Selects the winning category from predefined discrete options.

- **Request Definition**:
  - `type`: `"choice"`
  - `instructions`: string / object / array.
  - `criteria`: map<string, any> defining candidate options and descriptions.
- **Response Fields**:
  - `choice`: `string`, selected option label.
  - `probabilities`: `Record<string, float>`, normalized probability distribution across options.
  - `confidence`: `float` (0.0 ~ 1.0), overall decision confidence.

### 3. `score` (Rubric-based Scoring)
Interpolates continuous numeric scores against an ordered ladder of qualitative levels.

- **Request Definition**:
  - `type`: `"score"`
  - `instructions`: string / object / array.
  - `criteria`: ordered array of rubric levels.
- **Response Fields**:
  - `score`: `float`, continuous probability-weighted score.
  - `legend`: Level descriptions.
  - `probabilities`: `Record<string, float>`, probability distribution across level indices.
  - `confidence`: `float` (0.0 ~ 1.0), certainty score.

---

## Response Format

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
        "Critically non-compliant",
        "Dubious, needs review",
        "Basically compliant",
        "Fully compliant"
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

AxonHub maps `usage.input_tokens` and `usage.output_tokens` into prompt and completion metrics for request auditing and billing while preserving native names in the response.

---

## Examples

### cURL

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

### Python

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

### Go

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

## Channel Setup and Features

### 1. TypeSafe Channel Configuration
Add a channel in AxonHub:
- **Provider**: Select `TypeSafe`.
- **Base URL**: Defaults to `https://api.typesafe.ai/v1`.
- **Protocol Endpoint**: Strictly defaulted and locked to `typesafe/systemone`.
- **Default Models**: Preset with `jev-latest`, `jev-preview`, and `jev-1.13.0`.
- **API Keys**: Supports single or multiple API keys with sticky / round-robin selection.

### 2. Model Mapping
Client requests can use abstracted aliases via model mapping:
- Request model: `"model": "jev"`
- Channel mapping rule: `jev -> jev-latest`
- AxonHub modifies only the `"model"` byte segment while keeping `"state"` and `"questions"` in pass-through mode.

### 3. Failover and Retries
- **429 (Rate Limit) / 529 (Overloaded)**: AxonHub triggers retry or failover to alternative TypeSafe channels.
- **401 (Auth Failed) / 422 (Unprocessable Entity)**: Pass through immediately without redundant retries.

---

## Known Limitations

1. **No Streaming Support**: System One is a deterministic single-step evaluation. Requests with `stream: true` are rejected with HTTP 400.
2. **No Chat Generation Parameters**: Do not pass `messages`, `temperature`, `top_p`, `max_tokens`, or `tools`.
