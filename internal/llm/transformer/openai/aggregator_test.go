package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestAggregateStreamChunks(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     *llm.Response
	}{
		{
			name:     "openai stream chunks",
			filename: "openai-1.stream.jsonl",
			want: &llm.Response{
				ID:                "gen-1754577344-bfGaoVZhBY3iT78Psu02",
				Model:             "gpt-4o-mini",
				Object:            "chat.completion",
				Created:           1754577344,
				SystemFingerprint: "fp_efad92c60b",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr(
									"Sure! Here’s the output from 1 to 20, with 5 numbers on each line:\n\n```\n1 2 3 4 5\n6 7 8 9 10\n11 12 13 14 15\n16 17 18 19 20\n```",
								),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     19,
					CompletionTokens: 65,
					TotalTokens:      84,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load test data
			chunks, err := loadStreamChunks(tt.filename)
			require.NoError(t, err)

			// Test the function
			gotBytes, err := AggregateStreamChunks(context.Background(), chunks)
			require.NoError(t, err)

			// Parse the result
			var got llm.Response

			err = json.Unmarshal(gotBytes, &got)
			require.NoError(t, err)

			// Assert the result
			assert.Equal(t, tt.want.ID, got.ID)
			assert.Equal(t, tt.want.Model, got.Model)
			assert.Equal(t, tt.want.Object, got.Object)
			assert.Equal(t, tt.want.Created, got.Created)
			assert.Equal(t, tt.want.SystemFingerprint, got.SystemFingerprint)
			assert.Len(t, got.Choices, len(tt.want.Choices))

			if len(got.Choices) > 0 {
				assert.Equal(t, tt.want.Choices[0].Index, got.Choices[0].Index)
				assert.Equal(t, tt.want.Choices[0].Message.Role, got.Choices[0].Message.Role)
				assert.Equal(t, *tt.want.Choices[0].Message.Content.Content, *got.Choices[0].Message.Content.Content)
				assert.Equal(t, *tt.want.Choices[0].FinishReason, *got.Choices[0].FinishReason)
			}

			if tt.want.Usage != nil {
				require.NotNil(t, got.Usage)
				assert.Equal(t, tt.want.Usage.PromptTokens, got.Usage.PromptTokens)
				assert.Equal(t, tt.want.Usage.CompletionTokens, got.Usage.CompletionTokens)
				assert.Equal(t, tt.want.Usage.TotalTokens, got.Usage.TotalTokens)
			}
		})
	}
}

func TestAggregateStreamChunks_EmptyChunks(t *testing.T) {
	gotBytes, err := AggregateStreamChunks(context.Background(), nil)
	require.NoError(t, err)

	var got llm.Response

	err = json.Unmarshal(gotBytes, &got)
	require.NoError(t, err)

	assert.Equal(t, llm.Response{}, got)
}

// loadStreamChunks loads stream chunks from a JSONL file in testdata directory.
func loadStreamChunks(filename string) ([]*httpclient.StreamEvent, error) {
	file, err := os.Open("testdata/" + filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var chunks []*httpclient.StreamEvent

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse the line as a temporary struct to handle the Data field correctly
		var temp struct {
			LastEventID string `json:"LastEventID"`
			Type        string `json:"Type"`
			Data        string `json:"Data"` // Data is a JSON string in the test file
		}

		if err := json.Unmarshal([]byte(line), &temp); err != nil {
			return nil, err
		}

		// Create the StreamEvent with Data as []byte
		streamEvent := &httpclient.StreamEvent{
			LastEventID: temp.LastEventID,
			Type:        temp.Type,
			Data:        []byte(temp.Data), // Convert string to []byte
		}

		chunks = append(chunks, streamEvent)
	}

	return chunks, scanner.Err()
}
