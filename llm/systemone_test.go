package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemOneSerialization(t *testing.T) {
	reqJSON := `{
		"state": {"ticket_id": "123", "text": "Stripe payment failed"},
		"questions": {
			"is_urgent": {
				"type": "noul",
				"instructions": "Is urgent?"
			},
			"dept": {
				"type": "choice",
				"instructions": "Which dept?",
				"criteria": {"billing": "Billing issues", "tech": "Bugs"}
			}
		}
	}`

	var sysReq SystemOneRequest
	err := json.Unmarshal([]byte(reqJSON), &sysReq)
	require.NoError(t, err)
	require.NotNil(t, sysReq.State)
	require.Len(t, sysReq.Questions, 2)
	require.Equal(t, "noul", sysReq.Questions["is_urgent"].Type)
	require.Equal(t, "choice", sysReq.Questions["dept"].Type)

	conf := 0.95
	resp := SystemOneResponse{
		Answers: map[string]SystemOneAnswer{
			"dept": {
				Type:       "choice",
				Choice:     "billing",
				Confidence: &conf,
			},
		},
	}
	respBytes, err := json.Marshal(resp)
	require.NoError(t, err)
	require.Contains(t, string(respBytes), `"choice":"billing"`)
}

func TestSystemOneAnswer_PreserveExtraFields(t *testing.T) {
	rawJSON := `{"answers":{"fraud":{"type":"choice","choice":"yes","confidence":0.99,"custom_flag":true,"debug_info":{"step":"evaluate"}}}}`

	var resp SystemOneResponse
	err := json.Unmarshal([]byte(rawJSON), &resp)
	require.NoError(t, err)

	ans, ok := resp.Answers["fraud"]
	require.True(t, ok)
	require.Equal(t, "choice", ans.Type)
	require.Equal(t, "yes", ans.Choice)
	require.NotNil(t, ans.Confidence)
	require.InDelta(t, 0.99, *ans.Confidence, 0.0001)

	// Check that custom fields are preserved in Extra
	require.NotNil(t, ans.Extra)
	require.Contains(t, ans.Extra, "custom_flag")
	require.Equal(t, `true`, string(ans.Extra["custom_flag"]))
	require.Contains(t, ans.Extra, "debug_info")
	require.Equal(t, `{"step":"evaluate"}`, string(ans.Extra["debug_info"]))

	// Marshal again and check that custom fields survive
	marshaled, err := json.Marshal(resp)
	require.NoError(t, err)
	require.Contains(t, string(marshaled), `"custom_flag":true`)
	require.Contains(t, string(marshaled), `"debug_info":{"step":"evaluate"}`)
}
