package responses

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestItemMarshalJSON_OmitsSummaryForNonReasoning(t *testing.T) {
	item := Item{
		Role: "user",
		Content: &Input{
			Items: []Item{
				{
					Type: "input_text",
					Text: lo.ToPtr("hello"),
				},
			},
		},
	}

	data, err := json.Marshal(item)
	require.NoError(t, err)
	require.NotContains(t, string(data), `"summary"`)
}
