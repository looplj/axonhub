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

func TestItemMarshalJSON_ReasoningSummaryBehavior(t *testing.T) {
	cases := []struct {
		name        string
		item        Item
		expect      string
		notContains string
	}{
		{
			name: "nil summary emits empty array",
			item: Item{
				Type: "reasoning",
			},
			expect: `"summary":[]`,
		},
		{
			name: "empty summary emits empty array",
			item: Item{
				Type:    "reasoning",
				Summary: []ReasoningSummary{},
			},
			expect: `"summary":[]`,
		},
		{
			name: "summary preserves content",
			item: Item{
				Type: "reasoning",
				Summary: []ReasoningSummary{
					{Type: "summary_text", Text: "Thinking about this."},
				},
			},
			expect:      `"summary":`,
			notContains: `"summary":[]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.item)
			require.NoError(t, err)
			require.Contains(t, string(data), tc.expect)
			if tc.notContains != "" {
				require.NotContains(t, string(data), tc.notContains)
			}

			if tc.name == "summary preserves content" {
				var parsed Item
				err := json.Unmarshal(data, &parsed)
				require.NoError(t, err)
				require.Len(t, parsed.Summary, 1)
				require.Equal(t, "summary_text", parsed.Summary[0].Type)
				require.Equal(t, "Thinking about this.", parsed.Summary[0].Text)
			}
		})
	}
}
