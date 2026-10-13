package responses

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestResponsesToolOutputConversionPreservesItemIdentity(t *testing.T) {
	for _, kind := range []string{"function_call_output", "custom_tool_call_output"} {
		t.Run(kind, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{
				"model": "synthetic-model", "input": []map[string]any{{
					"type": strings.TrimSuffix(kind, "_output"), "id": "call-item", "call_id": "paired-call",
					"name": "payload", "arguments": "{}", "input": "synthetic-input",
				}, {
					"type": kind, "id": "output-item", "call_id": "paired-call",
					"output": "synthetic-output",
				}},
			})
			require.NoError(t, err)
			request, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: body})
			require.NoError(t, err)
			items := convertInputFromMessages(request.Messages, request.TransformOptions)
			encoded, err := json.Marshal(items)
			require.NoError(t, err)
			require.Equal(t, "output-item", gjson.GetBytes(encoded, "1.id").String())
			require.Equal(t, kind, gjson.GetBytes(encoded, "1.type").String())
			require.Equal(t, "paired-call", gjson.GetBytes(encoded, "1.call_id").String())
			require.Equal(t, "synthetic-output", gjson.GetBytes(encoded, "1.output").String())
		})
	}
}
