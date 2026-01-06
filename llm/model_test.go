package llm

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestMessageContent_MarshalJSON(t *testing.T) {
	t.Run("Empty content", func(t *testing.T) {
		message := Message{
			Content: MessageContent{
				Content:         nil,
				MultipleContent: nil,
			},
		}
		got, err := json.Marshal(message)
		require.NoError(t, err)
		println(string(got))
	})

	type fields struct {
		Content         *string
		MultipleContent []MessageContentPart
	}

	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "test1",
			fields: fields{
				Content:         nil,
				MultipleContent: nil,
			},
			want:    `null`,
			wantErr: false,
		},
		{
			name: "test2",
			fields: fields{
				Content:         lo.ToPtr("Hello"),
				MultipleContent: nil,
			},
			want:    `"Hello"`,
			wantErr: false,
		},
		{
			name: "test3",
			fields: fields{
				Content:         nil,
				MultipleContent: []MessageContentPart{{Type: "text", Text: lo.ToPtr("Hello")}},
			},
			want:    `"Hello"`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := MessageContent{
				Content:         tt.fields.Content,
				MultipleContent: tt.fields.MultipleContent,
			}

			got, err := c.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("MessageContent.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(string(got), tt.want) {
				t.Errorf("MessageContent.MarshalJSON() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestResponseError_Error(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		e    ResponseError
		want string
	}{
		{
			name: "with request id",
			e: ResponseError{
				StatusCode: 400,
				Detail: ErrorDetail{
					Message:   "test1",
					Code:      "test1",
					Type:      "test1",
					RequestID: "test1",
				},
			},
			want: "Request failed: Bad Request, error: test1, code: test1, type: test1, request_id: test1",
		},
		{
			name: "without request id",
			e: ResponseError{
				StatusCode: 400,
				Detail: ErrorDetail{
					Message: "test1",
					Code:    "test1",
					Type:    "test1",
				},
			},
			want: "Request failed: Bad Request, error: test1, code: test1, type: test1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.e.Error()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMessage_JSONOmitZero(t *testing.T) {
	tests := []struct {
		name     string
		message  Message
		expected string
	}{
		{
			name: "zero content should be omitted",
			message: Message{
				Role: "user",
			},
			expected: `{"role":"user"}`,
		},
		{
			name: "empty MessageContent should be omitted",
			message: Message{
				Role:    "user",
				Content: MessageContent{},
			},
			expected: `{"role":"user"}`,
		},
		{
			name: "nil content should be omitted",
			message: Message{
				Role: "user",
				Content: MessageContent{
					Content:         nil,
					MultipleContent: nil,
				},
			},
			expected: `{"role":"user"}`,
		},
		{
			name: "empty string content should be included as empty string",
			message: Message{
				Role: "user",
				Content: MessageContent{
					Content:         lo.ToPtr(""),
					MultipleContent: nil,
				},
			},
			expected: `{"role":"user","content":""}`,
		},
		{
			name: "empty slice content should be included as null",
			message: Message{
				Role: "user",
				Content: MessageContent{
					Content:         nil,
					MultipleContent: []MessageContentPart{},
				},
			},
			expected: `{"role":"user","content":null}`,
		},
		{
			name: "non-empty content should be included",
			message: Message{
				Role: "user",
				Content: MessageContent{
					Content:         lo.ToPtr("Hello"),
					MultipleContent: nil,
				},
			},
			expected: `{"role":"user","content":"Hello"}`,
		},
		{
			name: "non-empty multiple content should be included",
			message: Message{
				Role: "user",
				Content: MessageContent{
					Content:         nil,
					MultipleContent: []MessageContentPart{{Type: "text", Text: lo.ToPtr("Hello")}},
				},
			},
			expected: `{"role":"user","content":"Hello"}`,
		},
		{
			name: "multiple content parts should be included as array",
			message: Message{
				Role: "user",
				Content: MessageContent{
					Content: nil,
					MultipleContent: []MessageContentPart{
						{Type: "text", Text: lo.ToPtr("Hello")},
						{Type: "text", Text: lo.ToPtr("World")},
					},
				},
			},
			expected: `{"role":"user","content":[{"type":"text","text":"Hello"},{"type":"text","text":"World"}]}`,
		},
		{
			name: "other zero fields should be omitted",
			message: Message{
				Role:    "user",
				Content: MessageContent{Content: lo.ToPtr("Hello")},
				Name:    nil,
			},
			expected: `{"role":"user","content":"Hello"}`,
		},
		{
			name: "non-zero fields should be included",
			message: Message{
				Role:    "user",
				Content: MessageContent{Content: lo.ToPtr("Hello")},
				Name:    lo.ToPtr("John"),
				Refusal: "Cannot answer",
			},
			expected: `{"role":"user","content":"Hello","name":"John","refusal":"Cannot answer"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.message)
			require.NoError(t, err)
			require.JSONEq(t, tt.expected, string(got))
		})
	}
}
