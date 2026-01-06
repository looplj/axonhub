package aisdk

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTransformer(t *testing.T) {
	tests := []struct {
		name         string
		headers      http.Header
		expectedType string
	}{
		{
			name: "data stream header present",
			headers: http.Header{
				"X-Vercel-Ai-Ui-Message-Stream": []string{"v1"},
			},
			expectedType: "*aisdk.DataStreamTransformer",
		},
		{
			name: "data stream header with different value",
			headers: http.Header{
				"X-Vercel-Ai-Ui-Message-Stream": []string{"v2"},
			},
			expectedType: "*aisdk.TextTransformer",
		},
		{
			name:         "no data stream header",
			headers:      http.Header{},
			expectedType: "*aisdk.TextTransformer",
		},
		{
			name: "other headers present",
			headers: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"test-agent"},
			},
			expectedType: "*aisdk.TextTransformer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := NewTransformer(tt.headers)
			require.NotNil(t, transformer)

			// Check the type of transformer returned
			transformerType := getTransformerType(transformer)
			require.Equal(t, tt.expectedType, transformerType)
		})
	}
}

func TestNewTransformerByType(t *testing.T) {
	tests := []struct {
		name            string
		transformerType TransformerType
		expectedType    string
	}{
		{
			name:            "data stream type",
			transformerType: TransformerTypeDataStream,
			expectedType:    "*aisdk.DataStreamTransformer",
		},
		{
			name:            "text type",
			transformerType: TransformerTypeText,
			expectedType:    "*aisdk.TextTransformer",
		},
		{
			name:            "unknown type defaults to text",
			transformerType: TransformerType("unknown"),
			expectedType:    "*aisdk.TextTransformer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := NewTransformerByType(tt.transformerType)
			require.NotNil(t, transformer)

			// Check the type of transformer returned
			transformerType := getTransformerType(transformer)
			require.Equal(t, tt.expectedType, transformerType)
		})
	}
}

func TestTransformerTypeConstants(t *testing.T) {
	require.Equal(t, TransformerType("text"), TransformerTypeText)
	require.Equal(t, TransformerType("datastream"), TransformerTypeDataStream)
}

// Helper function to get the type name of a transformer.
func getTransformerType(transformer interface{}) string {
	switch transformer.(type) {
	case *DataStreamTransformer:
		return "*aisdk.DataStreamTransformer"
	case *TextTransformer:
		return "*aisdk.TextTransformer"
	default:
		return "unknown"
	}
}
