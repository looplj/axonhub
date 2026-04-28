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

func TestItemMarshalJSON_Compaction(t *testing.T) {
	cases := []struct {
		name     string
		item     Item
		validate func(t *testing.T, data []byte)
	}{
		{
			name: "compaction item with all fields",
			item: Item{
				ID:               "compaction_123",
				Type:             "compaction",
				EncryptedContent: lo.ToPtr("encrypted_data_here"),
				CreatedBy:        lo.ToPtr("user_abc"),
			},
			validate: func(t *testing.T, data []byte) {
				require.Contains(t, string(data), `"type":"compaction"`)
				require.Contains(t, string(data), `"id":"compaction_123"`)
				require.Contains(t, string(data), `"encrypted_content":"encrypted_data_here"`)
				require.Contains(t, string(data), `"created_by":"user_abc"`)

				var parsed Item

				err := json.Unmarshal(data, &parsed)
				require.NoError(t, err)
				require.Equal(t, "compaction", parsed.Type)
				require.Equal(t, "compaction_123", parsed.ID)
				require.NotNil(t, parsed.EncryptedContent)
				require.Equal(t, "encrypted_data_here", *parsed.EncryptedContent)
				require.NotNil(t, parsed.CreatedBy)
				require.Equal(t, "user_abc", *parsed.CreatedBy)
			},
		},
		{
			name: "compaction item with empty encrypted_content",
			item: Item{
				ID:               "compaction_456",
				Type:             "compaction",
				EncryptedContent: lo.ToPtr(""),
			},
			validate: func(t *testing.T, data []byte) {
				require.Contains(t, string(data), `"type":"compaction"`)
				require.Contains(t, string(data), `"encrypted_content":""`)

				var parsed Item

				err := json.Unmarshal(data, &parsed)
				require.NoError(t, err)
				require.Equal(t, "compaction", parsed.Type)
				require.NotNil(t, parsed.EncryptedContent)
				require.Equal(t, "", *parsed.EncryptedContent)
			},
		},
		{
			name: "compaction item without created_by",
			item: Item{
				ID:               "compaction_789",
				Type:             "compaction",
				EncryptedContent: lo.ToPtr("encrypted_only"),
			},
			validate: func(t *testing.T, data []byte) {
				require.Contains(t, string(data), `"type":"compaction"`)
				require.Contains(t, string(data), `"encrypted_content":"encrypted_only"`)
				require.NotContains(t, string(data), `"created_by"`)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.item)
			require.NoError(t, err)
			tc.validate(t, data)
		})
	}
}

func TestItemUnmarshalJSON_Compaction(t *testing.T) {
	cases := []struct {
		name     string
		json     string
		validate func(t *testing.T, item Item)
	}{
		{
			name: "compaction item from json",
			json: `{"id":"compaction_abc","type":"compaction","encrypted_content":"base64encoded","created_by":"assistant"}`,
			validate: func(t *testing.T, item Item) {
				require.Equal(t, "compaction", item.Type)
				require.Equal(t, "compaction_abc", item.ID)
				require.NotNil(t, item.EncryptedContent)
				require.Equal(t, "base64encoded", *item.EncryptedContent)
				require.NotNil(t, item.CreatedBy)
				require.Equal(t, "assistant", *item.CreatedBy)
			},
		},
		{
			name: "compaction item without created_by",
			json: `{"id":"compaction_xyz","type":"compaction","encrypted_content":"data"}`,
			validate: func(t *testing.T, item Item) {
				require.Equal(t, "compaction", item.Type)
				require.Equal(t, "compaction_xyz", item.ID)
				require.NotNil(t, item.EncryptedContent)
				require.Equal(t, "data", *item.EncryptedContent)
				require.Nil(t, item.CreatedBy)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var item Item

			err := json.Unmarshal([]byte(tc.json), &item)
			require.NoError(t, err)
			tc.validate(t, item)
		})
	}
}

func TestInputUnmarshalJSON_ClearsConflictingRepresentation(t *testing.T) {
	input := Input{
		Text: lo.ToPtr("stale"),
		Items: []Item{
			{Type: "input_text", Text: lo.ToPtr("old")},
		},
	}

	err := json.Unmarshal([]byte(`"fresh"`), &input)
	require.NoError(t, err)
	require.NotNil(t, input.Text)
	require.Equal(t, "fresh", *input.Text)
	require.Nil(t, input.Items)

	err = json.Unmarshal([]byte(`[{"type":"input_text","text":"part"}]`), &input)
	require.NoError(t, err)
	require.Nil(t, input.Text)
	require.Len(t, input.Items, 1)
	require.Equal(t, "input_text", input.Items[0].Type)
	require.NotNil(t, input.Items[0].Text)
	require.Equal(t, "part", *input.Items[0].Text)
}

func TestResponseToolChoiceUnmarshalJSON_ClearsConflictingRepresentation(t *testing.T) {
	choice := ResponseToolChoice{
		StringValue: "auto",
		ObjectValue: &ToolChoice{Mode: lo.ToPtr("required")},
	}

	err := json.Unmarshal([]byte(`"none"`), &choice)
	require.NoError(t, err)
	require.Equal(t, "none", choice.StringValue)
	require.Nil(t, choice.ObjectValue)

	err = json.Unmarshal([]byte(`{"mode":"required","type":"function","name":"get_weather"}`), &choice)
	require.NoError(t, err)
	require.Equal(t, "", choice.StringValue)
	require.NotNil(t, choice.ObjectValue)
	require.NotNil(t, choice.ObjectValue.Mode)
	require.Equal(t, "required", *choice.ObjectValue.Mode)
	require.NotNil(t, choice.ObjectValue.Type)
	require.Equal(t, "function", *choice.ObjectValue.Type)
	require.NotNil(t, choice.ObjectValue.Name)
	require.Equal(t, "get_weather", *choice.ObjectValue.Name)
}
