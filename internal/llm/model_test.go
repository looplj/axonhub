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
			want:    `""`,
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
