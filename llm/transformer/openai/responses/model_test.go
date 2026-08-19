package responses

import (
	"encoding/json"
	"strconv"
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

func TestItemUnmarshalJSON_PolymorphicReasoningContent(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		roundTrip string
		validate  func(t *testing.T, content *PolymorphicReasoningContent)
	}{
		{
			name:    "chat-compatible function call string",
			payload: `{"type":"function_call","name":"get_weather","arguments":"{}","reasoning_content":"checking the weather"}`,
			validate: func(t *testing.T, content *PolymorphicReasoningContent) {
				require.NotNil(t, content)
				require.NotNil(t, content.Text)
				require.Equal(t, "checking the weather", *content.Text)
				require.Nil(t, content.Items)
			},
		},
		{
			name:      "native responses reasoning array",
			payload:   `{"type":"reasoning","reasoning_content":[{"type":"reasoning_text","text":"first"},{"type":"reasoning_text","text":"second"}]}`,
			roundTrip: `{"type":"reasoning","summary":[],"reasoning_content":[{"type":"reasoning_text","text":"first"},{"type":"reasoning_text","text":"second"}]}`,
			validate: func(t *testing.T, content *PolymorphicReasoningContent) {
				require.NotNil(t, content)
				require.Nil(t, content.Text)
				require.Equal(t, []ReasoningContent{
					{Type: "reasoning_text", Text: "first"},
					{Type: "reasoning_text", Text: "second"},
				}, content.Items)
			},
		},
		{
			name:      "null remains absent",
			payload:   `{"type":"function_call","name":"get_weather","arguments":"{}","reasoning_content":null}`,
			roundTrip: `{"type":"function_call","name":"get_weather","arguments":"{}"}`,
			validate: func(t *testing.T, content *PolymorphicReasoningContent) {
				require.Nil(t, content)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item Item
			err := json.Unmarshal([]byte(tt.payload), &item)
			require.NoError(t, err)
			tt.validate(t, item.ReasoningContent)

			encoded, err := json.Marshal(item)
			require.NoError(t, err)
			expected := tt.payload
			if tt.roundTrip != "" {
				expected = tt.roundTrip
			}
			require.JSONEq(t, expected, string(encoded))
		})
	}
}

func TestItemUnmarshalJSON_RejectsInvalidReasoningContent(t *testing.T) {
	var item Item
	err := json.Unmarshal([]byte(`{"type":"function_call","arguments":"{}","reasoning_content":{"text":"invalid"}}`), &item)
	require.ErrorContains(t, err, "invalid reasoning_content")
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

func TestItemUnmarshalJSON_AcceptsObjectArguments(t *testing.T) {
	var item Item
	err := json.Unmarshal([]byte(`{
		"type": "tool_search_call",
		"call_id": "call_123",
		"arguments": {"query":"image generation","limit":10}
	}`), &item)
	require.NoError(t, err)
	require.Equal(t, "tool_search_call", item.Type)
	require.Equal(t, `{"query":"image generation","limit":10}`, item.Arguments)
}

func TestItemMarshalJSON_ToolSearchCallRequiresObjectArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		expected  string
	}{
		{name: "null becomes empty object", arguments: "null", expected: `{}`},
		{name: "array becomes empty object", arguments: `["query"]`, expected: `{}`},
		{name: "object is preserved", arguments: `{"query":"agents"}`, expected: `{"query":"agents"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := Item{Type: "tool_search_call", CallID: "call_search", Arguments: tt.arguments}

			data, err := json.Marshal(item)
			require.NoError(t, err)

			var payload struct {
				Arguments json.RawMessage `json:"arguments"`
			}
			require.NoError(t, json.Unmarshal(data, &payload))
			require.JSONEq(t, tt.expected, string(payload.Arguments))
		})
	}
}

func TestItemUnmarshalJSON_AcceptsStringArguments(t *testing.T) {
	var item Item
	err := json.Unmarshal([]byte(`{
		"type": "function_call",
		"call_id": "call_123",
		"arguments": "{\"location\":\"NYC\"}"
	}`), &item)
	require.NoError(t, err)
	require.Equal(t, `{"location":"NYC"}`, item.Arguments)
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

func TestToolChoiceMarshalJSON_PreservesStringModes(t *testing.T) {
	cases := []struct {
		name     string
		choice   ToolChoice
		expected string
	}{
		{
			name: "auto marshals as string",
			choice: ToolChoice{
				Mode: lo.ToPtr("auto"),
			},
			expected: `"auto"`,
		},
		{
			name: "required marshals as string",
			choice: ToolChoice{
				Mode: lo.ToPtr("required"),
			},
			expected: `"required"`,
		},
		{
			name: "none marshals as string",
			choice: ToolChoice{
				Mode: lo.ToPtr("none"),
			},
			expected: `"none"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(&tc.choice)
			require.NoError(t, err)
			require.JSONEq(t, tc.expected, string(data))
		})
	}
}

func TestResponseUnmarshalJSON_CreatedAtCompatibility(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		want        int64
		wantErr     bool
		errContains string
	}{
		{
			name:    "integer created_at",
			payload: `{"id":"resp_1","created_at":1786360449,"model":"gpt-5","output":[]}`,
			want:    1786360449,
		},
		{
			name:    "float-encoded integral created_at",
			payload: `{"id":"resp_1","created_at":1786360449.0,"model":"gpt-5","output":[]}`,
			want:    1786360449,
		},
		{
			name:    "float-encoded integral created_at with trailing zeros",
			payload: `{"id":"resp_1","created_at":1786360449.000,"model":"gpt-5","output":[]}`,
			want:    1786360449,
		},
		{
			name:    "zero created_at",
			payload: `{"id":"resp_1","created_at":0,"model":"gpt-5","output":[]}`,
			want:    0,
		},
		{
			name:    "zero created_at as float",
			payload: `{"id":"resp_1","created_at":0.0,"model":"gpt-5","output":[]}`,
			want:    0,
		},
		{
			name:    "zero created_at with trailing zero fraction",
			payload: `{"id":"resp_1","created_at":0.000,"model":"gpt-5","output":[]}`,
			want:    0,
		},
		{
			name:    "exponent-form integral created_at",
			payload: `{"id":"resp_1","created_at":1.7e9,"model":"gpt-5","output":[]}`,
			want:    1700000000,
		},
		{
			name:    "missing created_at keeps zero value",
			payload: `{"id":"resp_1","model":"gpt-5","output":[]}`,
			want:    0,
		},
		{
			name:    "non-normalized scientific notation with canceling exponent",
			payload: `{"id":"resp_1","created_at":170000000000000000000000000000e-20,"model":"gpt-5","output":[]}`,
			want:    1700000000,
		},
		{
			name:    "max int64 created_at",
			payload: `{"id":"resp_1","created_at":9223372036854775807,"model":"gpt-5","output":[]}`,
			want:    9223372036854775807,
		},
		{
			name:    "min int64 created_at",
			payload: `{"id":"resp_1","created_at":-9223372036854775808,"model":"gpt-5","output":[]}`,
			want:    -9223372036854775808,
		},
		{
			name:        "fractional created_at is rejected",
			payload:     `{"id":"resp_1","created_at":1786360449.5,"model":"gpt-5","output":[]}`,
			wantErr:     true,
			errContains: "created_at",
		},
		{
			name:        "quoted created_at is rejected",
			payload:     `{"id":"resp_1","created_at":"1786360449","model":"gpt-5","output":[]}`,
			wantErr:     true,
			errContains: "created_at",
		},
		{
			name:        "quoted float created_at is rejected",
			payload:     `{"id":"resp_1","created_at":"1786360449.0","model":"gpt-5","output":[]}`,
			wantErr:     true,
			errContains: "created_at",
		},
		{
			name:        "created_at with excessive exponent is rejected without expansion",
			payload:     `{"id":"resp_1","created_at":1e1000000,"model":"gpt-5","output":[]}`,
			wantErr:     true,
			errContains: "out of int64 range",
		},
		{
			name:        "created_at with min-int exponent is rejected as non-integral",
			payload:     `{"id":"resp_1","created_at":1e-9223372036854775808,"model":"gpt-5","output":[]}`,
			wantErr:     true,
			errContains: "integer number of seconds",
		},
		{
			name:        "created_at with max-int exponent is rejected without overflow",
			payload:     `{"id":"resp_1","created_at":1e9223372036854775807,"model":"gpt-5","output":[]}`,
			wantErr:     true,
			errContains: "out of int64 range",
		},
		{
			name:        "created_at exceeding int64 is rejected",
			payload:     `{"id":"resp_1","created_at":9223372036854775808,"model":"gpt-5","output":[]}`,
			wantErr:     true,
			errContains: "created_at",
		},
		{
			name:        "created_at below int64 is rejected",
			payload:     `{"id":"resp_1","created_at":-9223372036854775809,"model":"gpt-5","output":[]}`,
			wantErr:     true,
			errContains: "created_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp Response
			err := json.Unmarshal([]byte(tt.payload), &resp)
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.errContains)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, resp.CreatedAt)

			// Other fields must keep their default decoding.
			require.Equal(t, "resp_1", resp.ID)
			require.Equal(t, "gpt-5", resp.Model)

			// Marshaling always emits the integer form, never 1786360449.0.
			data, err := json.Marshal(resp)
			require.NoError(t, err)
			require.Contains(t, string(data), `"created_at":`+strconv.FormatInt(tt.want, 10))
			require.NotContains(t, string(data), ".0")
		})
	}
}

func TestResponseUnmarshalJSON_FullResponseWithAnnotations(t *testing.T) {
	payload := `{
		"id": "resp_xxx",
		"object": "response",
		"created_at": 1786360449.0,
		"model": "parallel",
		"status": "completed",
		"output": [
			{
				"type": "message",
				"role": "assistant",
				"content": [
					{
						"type": "output_text",
						"text": "测试内容",
						"annotations": [
							{
								"type": "url_citation",
								"start_index": 0,
								"end_index": 4,
								"title": "Example",
								"url": "https://example.com"
							}
						]
					}
				]
			}
		],
		"usage": {
			"input_tokens": 3,
			"output_tokens": 7,
			"total_tokens": 10
		}
	}`

	var resp Response
	err := json.Unmarshal([]byte(payload), &resp)
	require.NoError(t, err)

	require.Equal(t, int64(1786360449), resp.CreatedAt)
	require.Equal(t, "resp_xxx", resp.ID)
	require.Equal(t, "response", resp.Object)
	require.Equal(t, "parallel", resp.Model)
	require.NotNil(t, resp.Status)
	require.Equal(t, "completed", *resp.Status)

	require.Len(t, resp.Output, 1)
	msg := resp.Output[0]
	require.Equal(t, "message", msg.Type)
	require.Equal(t, "assistant", msg.Role)
	require.NotNil(t, msg.Content)
	require.Len(t, msg.Content.Items, 1)

	textItem := msg.Content.Items[0]
	require.Equal(t, "output_text", textItem.Type)
	require.NotNil(t, textItem.Text)
	require.Equal(t, "测试内容", *textItem.Text)

	require.Len(t, textItem.Annotations, 1)
	annotation := textItem.Annotations[0]
	require.Equal(t, "url_citation", annotation.Type)
	require.NotNil(t, annotation.StartIndex)
	require.Equal(t, int64(0), *annotation.StartIndex)
	require.NotNil(t, annotation.EndIndex)
	require.Equal(t, int64(4), *annotation.EndIndex)
	require.NotNil(t, annotation.URLCitation)
	require.Equal(t, "Example", annotation.URLCitation.Title)
	require.Equal(t, "https://example.com", annotation.URLCitation.URL)

	require.NotNil(t, resp.Usage)
	require.Equal(t, int64(3), resp.Usage.InputTokens)
	require.Equal(t, int64(7), resp.Usage.OutputTokens)
	require.Equal(t, int64(10), resp.Usage.TotalTokens)
}

func TestResponseToolChoiceUnmarshalJSONErrorDoesNotMutate(t *testing.T) {
	invalidInputs := []string{`{`, `1`, `true`, `[]`, `["auto"]`}
	for _, input := range invalidInputs {
		t.Run(input, func(t *testing.T) {
			choice := ResponseToolChoice{
				StringValue: "required",
				ObjectValue: &ToolChoice{Type: lo.ToPtr("function"), Name: lo.ToPtr("lookup")},
			}
			before := choice
			// Deep snapshot the pointee; a shallow copy would share the
			// pointer and hide in-place mutations of the ObjectValue.
			before.ObjectValue = deepCopyToolChoice(choice.ObjectValue)

			require.Error(t, json.Unmarshal([]byte(input), &choice))
			require.Equal(t, before, choice)
		})
	}
}

func TestToolChoiceUnmarshalJSON_ClearsPreviousVariant(t *testing.T) {
	choice := ToolChoice{
		Type: lo.ToPtr("function"),
		Name: lo.ToPtr("lookup"),
	}

	require.NoError(t, json.Unmarshal([]byte(`"auto"`), &choice))
	require.Equal(t, "auto", lo.FromPtr(choice.Mode))
	require.Nil(t, choice.Type)
	require.Nil(t, choice.Name)
	require.Nil(t, choice.Tools)

	require.NoError(t, json.Unmarshal([]byte(`{"type":"function","name":"lookup"}`), &choice))
	require.Nil(t, choice.Mode)
	require.Equal(t, "function", lo.FromPtr(choice.Type))
	require.Equal(t, "lookup", lo.FromPtr(choice.Name))
	require.Nil(t, choice.Tools)

	require.NoError(t, json.Unmarshal([]byte(`{"type":"allowed_tools","mode":"required","tools":[]}`), &choice))
	require.Equal(t, "required", lo.FromPtr(choice.Mode))
	require.Equal(t, "allowed_tools", lo.FromPtr(choice.Type))
	require.Nil(t, choice.Name)
	require.NotNil(t, choice.Tools)
	require.Empty(t, choice.Tools)
}

func TestToolChoiceMarshalJSON_PreservesEmptyAllowedTools(t *testing.T) {
	cases := []struct {
		name     string
		mode     *string
		tools    []ToolOption
		expected string
	}{
		{
			name:     "nil mode and nil tools",
			expected: `{"type":"allowed_tools","tools":[]}`,
		},
		{
			name:     "auto with nil tools",
			mode:     lo.ToPtr("auto"),
			expected: `{"type":"allowed_tools","mode":"auto","tools":[]}`,
		},
		{
			name:     "auto with empty tools",
			mode:     lo.ToPtr("auto"),
			tools:    []ToolOption{},
			expected: `{"type":"allowed_tools","mode":"auto","tools":[]}`,
		},
		{
			name:     "required with nil tools",
			mode:     lo.ToPtr("required"),
			expected: `{"type":"allowed_tools","mode":"required","tools":[]}`,
		},
		{
			name:     "required with empty tools",
			mode:     lo.ToPtr("required"),
			tools:    []ToolOption{},
			expected: `{"type":"allowed_tools","mode":"required","tools":[]}`,
		},
		{
			name:     "type-only tool option omits name",
			mode:     lo.ToPtr("required"),
			tools:    []ToolOption{{Type: "image_generation"}},
			expected: `{"type":"allowed_tools","mode":"required","tools":[{"type":"image_generation"}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			choice := ToolChoice{Type: lo.ToPtr("allowed_tools"), Mode: tc.mode, Tools: tc.tools}
			data, err := json.Marshal(&choice)
			require.NoError(t, err)
			require.JSONEq(t, tc.expected, string(data))
		})
	}
}

func TestToolChoiceJSONPrimitiveMatrix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, choice ToolChoice)
	}{
		{
			name:  "auto mode",
			input: `"auto"`,
			validate: func(t *testing.T, choice ToolChoice) {
				require.Equal(t, "auto", lo.FromPtr(choice.Mode))
				require.Nil(t, choice.Type)
				require.Nil(t, choice.Name)
			},
		},
		{
			name:  "named function",
			input: `{"type":"function","name":"lookup"}`,
			validate: func(t *testing.T, choice ToolChoice) {
				require.Equal(t, "function", lo.FromPtr(choice.Type))
				require.Equal(t, "lookup", lo.FromPtr(choice.Name))
			},
		},
		{
			name:  "named custom",
			input: `{"type":"custom","name":"apply_patch"}`,
			validate: func(t *testing.T, choice ToolChoice) {
				require.Equal(t, "custom", lo.FromPtr(choice.Type))
				require.Equal(t, "apply_patch", lo.FromPtr(choice.Name))
			},
		},
		{
			name:  "named namespace",
			input: `{"type":"namespace","name":"collaboration"}`,
			validate: func(t *testing.T, choice ToolChoice) {
				require.Equal(t, "namespace", lo.FromPtr(choice.Type))
				require.Equal(t, "collaboration", lo.FromPtr(choice.Name))
			},
		},
		{
			name:  "named tool search",
			input: `{"type":"tool_search","name":"discover"}`,
			validate: func(t *testing.T, choice ToolChoice) {
				require.Equal(t, "tool_search", lo.FromPtr(choice.Type))
				require.Equal(t, "discover", lo.FromPtr(choice.Name))
			},
		},
		{
			name:  "named future client primitive",
			input: `{"type":"future_client_tool","name":"later"}`,
			validate: func(t *testing.T, choice ToolChoice) {
				require.Equal(t, "future_client_tool", lo.FromPtr(choice.Type))
				require.Equal(t, "later", lo.FromPtr(choice.Name))
			},
		},
		{
			name:  "type only hosted selector",
			input: `{"type":"web_search"}`,
			validate: func(t *testing.T, choice ToolChoice) {
				require.Equal(t, "web_search", lo.FromPtr(choice.Type))
				require.Nil(t, choice.Name)
			},
		},
		{
			name: "allowed primitive matrix keeps type name and order",
			input: `{"type":"allowed_tools","mode":"required","tools":[
				{"type":"function","name":"same"},
				{"type":"custom","name":"same"},
				{"type":"namespace","name":"workspace"},
				{"type":"tool_search","name":"discover"},
				{"type":"future_client_tool","name":"later"},
				{"type":"future_server_tool","name":"hosted"}
			]}`,
			validate: func(t *testing.T, choice ToolChoice) {
				require.Equal(t, "allowed_tools", lo.FromPtr(choice.Type))
				require.Equal(t, "required", lo.FromPtr(choice.Mode))
				require.Equal(t, []ToolOption{
					{Type: "function", Name: "same"},
					{Type: "custom", Name: "same"},
					{Type: "namespace", Name: "workspace"},
					{Type: "tool_search", Name: "discover"},
					{Type: "future_client_tool", Name: "later"},
					{Type: "future_server_tool", Name: "hosted"},
				}, choice.Tools)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var decoded ToolChoice
			require.NoError(t, json.Unmarshal([]byte(tt.input), &decoded))
			tt.validate(t, decoded)

			encoded, err := json.Marshal(&decoded)
			require.NoError(t, err)
			require.JSONEq(t, tt.input, string(encoded))

			var roundTripped ToolChoice
			require.NoError(t, json.Unmarshal(encoded, &roundTripped))
			require.Equal(t, decoded, roundTripped)
		})
	}
}

func TestToolChoiceUnmarshalJSONErrorDoesNotMutate(t *testing.T) {
	invalidInputs := []string{`{`, `1`, `true`, `[]`, `["auto"]`}
	for _, input := range invalidInputs {
		t.Run(input, func(t *testing.T) {
			choice := ToolChoice{
				Mode:  lo.ToPtr("required"),
				Type:  lo.ToPtr("allowed_tools"),
				Name:  lo.ToPtr("stale"),
				Tools: []ToolOption{{Type: "function", Name: "lookup"}},
			}
			// Snapshot pointers and copy Tools into a fresh slice so the
			// comparison does not share the mutated backing array.
			before := *deepCopyToolChoice(&choice)

			require.Error(t, json.Unmarshal([]byte(input), &choice))
			require.Equal(t, lo.FromPtr(before.Mode), lo.FromPtr(choice.Mode))
			require.Equal(t, lo.FromPtr(before.Type), lo.FromPtr(choice.Type))
			require.Equal(t, lo.FromPtr(before.Name), lo.FromPtr(choice.Name))
			require.Len(t, choice.Tools, len(before.Tools))
			for i := range before.Tools {
				require.Equal(t, before.Tools[i], choice.Tools[i])
			}
		})
	}
}

// deepCopyToolChoice returns a snapshot of tc that shares no pointer fields
// or slice backing array with the original, for mutation assertions.
func deepCopyToolChoice(tc *ToolChoice) *ToolChoice {
	if tc == nil {
		return nil
	}
	cp := *tc
	if tc.Mode != nil {
		cp.Mode = lo.ToPtr(*tc.Mode)
	}
	if tc.Type != nil {
		cp.Type = lo.ToPtr(*tc.Type)
	}
	if tc.Name != nil {
		cp.Name = lo.ToPtr(*tc.Name)
	}
	cp.Tools = append([]ToolOption(nil), tc.Tools...)
	return &cp
}

// FuzzToolChoiceJSONRoundTrip fuzzes ToolChoice (not ResponseToolChoice).
// It asserts that encoding is idempotent: re-encoding a decoded value is
// stable. It does not assert fidelity to the original raw input.
func FuzzToolChoiceJSONRoundTrip(f *testing.F) {
	seeds := []string{
		`"auto"`,
		`{"type":"function","name":"lookup"}`,
		`{"type":"custom","name":"apply_patch"}`,
		`{"type":"future_client_tool","name":"later"}`,
		`{"type":"web_search"}`,
		`{"type":"mcp","server_label":"docs","name":"search"}`,
		`{"type":"allowed_tools","mode":"auto","tools":[]}`,
		`{"type":"allowed_tools","mode":"required","tools":[{"type":"function","name":"lookup"},{"type":"custom","name":"apply_patch"}]}`,
		`null`,
		`{}`,
		`{`,
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		var decoded ToolChoice
		if err := json.Unmarshal([]byte(input), &decoded); err != nil {
			return
		}

		encoded, err := json.Marshal(&decoded)
		require.NoError(t, err)
		var roundTripped ToolChoice
		require.NoError(t, json.Unmarshal(encoded, &roundTripped))
		reencoded, err := json.Marshal(&roundTripped)
		require.NoError(t, err)
		require.JSONEq(t, string(encoded), string(reencoded))
	})
}
