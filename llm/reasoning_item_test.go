package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessage_ReasoningItemsSurviveJSONRoundTrip(t *testing.T) {
	original := Message{
		Role: "assistant",
		ReasoningItems: []ReasoningItem{
			{ID: "rs_first", Content: "first", Signature: "gAAAA_FIRST_BLOB"},
			{ID: "rs_second", Content: "second", Signature: "gAAAA_SECOND_BLOB"},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Message
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, original.ReasoningItems, decoded.ReasoningItems)
}
