package anthropic

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

func TestInboundTransformer_TransformRequest_WithTestData(t *testing.T) {
	tests := []struct {
		name        string
		requestFile string
		validate    func(t *testing.T, result *llm.Request, httpReq *httpclient.Request)
	}{
		{
			name:        "simple text request transformation",
			requestFile: "anthropic-simple-inbound.request.json",
			validate: func(t *testing.T, result *llm.Request, httpReq *httpclient.Request) {
				t.Helper()

				// Verify basic request properties
				require.Equal(t, "claude-3-sonnet-20240229", result.Model)
				require.NotNil(t, result.MaxTokens)
				require.Equal(t, int64(1024), *result.MaxTokens)
				require.NotNil(t, result.Temperature)
				require.Equal(t, 0.7, *result.Temperature)

				// Verify stop sequences
				require.NotNil(t, result.Stop)
				require.Equal(t, []string{"Human:", "Assistant:"}, result.Stop.MultipleStop)

				// Verify messages
				require.Len(t, result.Messages, 1)
				require.Equal(t, "user", result.Messages[0].Role)
				require.NotNil(t, result.Messages[0].Content.Content)
				require.Equal(t, "Hello, Claude! How are you today?", *result.Messages[0].Content.Content)
			},
		},
		{
			name:        "multimodal request transformation",
			requestFile: "anthropic-multimodal-inbound.request.json",
			validate: func(t *testing.T, result *llm.Request, httpReq *httpclient.Request) {
				t.Helper()

				// Verify basic request properties
				require.Equal(t, "claude-3-sonnet-20240229", result.Model)
				require.NotNil(t, result.MaxTokens)
				require.Equal(t, int64(1024), *result.MaxTokens)

				// Verify messages (should include system message)
				require.Len(t, result.Messages, 2)

				// First message should be system
				require.Equal(t, "system", result.Messages[0].Role)
				require.NotNil(t, result.Messages[0].Content.Content)
				require.Equal(t, "You are a helpful assistant that can analyze images.", *result.Messages[0].Content.Content)

				// Second message should be user with multimodal content
				require.Equal(t, "user", result.Messages[1].Role)
				require.Len(t, result.Messages[1].Content.MultipleContent, 2)

				// First content part should be text
				require.Equal(t, "text", result.Messages[1].Content.MultipleContent[0].Type)
				require.NotNil(t, result.Messages[1].Content.MultipleContent[0].Text)
				require.Equal(t, "What's in this image?", *result.Messages[1].Content.MultipleContent[0].Text)

				// Second content part should be image
				require.Equal(t, "image_url", result.Messages[1].Content.MultipleContent[1].Type)
				require.NotNil(t, result.Messages[1].Content.MultipleContent[1].ImageURL)
				require.Equal(t, "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEAYABgAAD//gA7Q1JFQVR", result.Messages[1].Content.MultipleContent[1].ImageURL.URL)
			},
		},
		{
			name:        "tool use request transformation",
			requestFile: "anthropic-tool-inbound.request.json",
			validate: func(t *testing.T, result *llm.Request, httpReq *httpclient.Request) {
				t.Helper()

				// Verify basic request properties
				require.Equal(t, "claude-sonnet-4-20250514", result.Model)
				require.NotNil(t, result.MaxTokens)
				require.Equal(t, int64(1024), *result.MaxTokens)

				// Verify messages
				require.Len(t, result.Messages, 1)
				require.Equal(t, "user", result.Messages[0].Role)
				require.NotNil(t, result.Messages[0].Content.Content)
				require.Equal(t, "What is the weather in San Francisco, CA?", *result.Messages[0].Content.Content)

				// Verify tools transformation
				require.NotNil(t, result.Tools)
				require.Len(t, result.Tools, 3)

				// Verify first tool (get_coordinates)
				require.Equal(t, "function", result.Tools[0].Type)
				require.Equal(t, "get_coordinates", result.Tools[0].Function.Name)
				require.Equal(t, "Accepts a place as an address, then returns the latitude and longitude coordinates.", result.Tools[0].Function.Description)

				// Verify tool parameters schema
				var schema map[string]interface{}
				err := json.Unmarshal(result.Tools[0].Function.Parameters, &schema)
				require.NoError(t, err)
				require.Equal(t, "object", schema["type"])

				properties, ok := schema["properties"].(map[string]interface{})
				require.True(t, ok)
				location, ok := properties["location"].(map[string]interface{})
				require.True(t, ok)
				require.Equal(t, "string", location["type"])
				require.Equal(t, "The location to look up.", location["description"])

				// Verify second tool (get_temperature_unit)
				require.Equal(t, "function", result.Tools[1].Type)
				require.Equal(t, "get_temperature_unit", result.Tools[1].Function.Name)

				// Verify third tool (get_weather)
				require.Equal(t, "function", result.Tools[2].Type)
				require.Equal(t, "get_weather", result.Tools[2].Function.Name)
				require.Equal(t, "Get the weather at a specific location", result.Tools[2].Function.Description)

				// Verify third tool parameters
				var weatherSchema map[string]interface{}
				err = json.Unmarshal(result.Tools[2].Function.Parameters, &weatherSchema)
				require.NoError(t, err)
				weatherProps, ok := weatherSchema["properties"].(map[string]interface{})
				require.True(t, ok)

				// Check lat parameter
				lat, ok := weatherProps["lat"].(map[string]interface{})
				require.True(t, ok)
				require.Equal(t, "number", lat["type"])

				// Check unit parameter with enum
				unit, ok := weatherProps["unit"].(map[string]interface{})
				require.True(t, ok)
				require.Equal(t, "string", unit["type"])
				enumValues, ok := unit["enum"].([]interface{})
				require.True(t, ok)
				require.Contains(t, enumValues, "celsius")
				require.Contains(t, enumValues, "fahrenheit")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the test request data as raw JSON
			var anthropicReqData json.RawMessage
			err := xtest.LoadTestData(t, tt.requestFile, &anthropicReqData)
			require.NoError(t, err)

			// Create HTTP request with the loaded data
			httpReq := &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: anthropicReqData,
			}

			// Create transformer
			transformer := NewInboundTransformer()

			// Transform the request
			result, err := transformer.TransformRequest(t.Context(), httpReq)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Run validation
			tt.validate(t, result, httpReq)
		})
	}
}
