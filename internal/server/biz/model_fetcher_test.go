package biz

import (
	"testing"
)

func TestExtractJSONArray(t *testing.T) {
	tests := []struct {
		name        string
		body        []byte
		target      interface{}
		expectError bool
	}{
		{
			name:        "valid JSON array with data field",
			body:        []byte(`{"data":[{"id":"model1"},{"id":"model2"}]}`),
			target:      &[]ModelIdentify{},
			expectError: false,
		},
		{
			name:        "valid JSON array without object wrapper",
			body:        []byte(`[{"id":"model1"},{"id":"model2"},{"id":"model3"}]`),
			target:      &[]ModelIdentify{},
			expectError: false,
		},
		{
			name:        "JSON array with nested arrays",
			body:        []byte(`{"data":[],"models":[{"name":"models/gemini-pro"}]}`),
			target:      &[]ModelIdentify{},
			expectError: false,
		},
		{
			name:        "no JSON array in body",
			body:        []byte(`{"object":"value"}`),
			target:      &[]ModelIdentify{},
			expectError: true,
		},
		{
			name:        "empty JSON array",
			body:        []byte(`[]`),
			target:      &[]ModelIdentify{},
			expectError: false,
		},
		{
			name:        "JSON array with single element",
			body:        []byte(`[{"id":"single-model"}]`),
			target:      &[]ModelIdentify{},
			expectError: false,
		},
		{
			name:        "complex response with multiple arrays",
			body:        []byte(`{"data":[{"id":"model1"}],"extra":[{"key":"value"}],"models":[]}`),
			target:      &[]ModelIdentify{},
			expectError: false,
		},
		{
			name:        "invalid JSON array structure",
			body:        []byte(`[invalid json]`),
			target:      &[]ModelIdentify{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExtractJSONArray(tt.body, tt.target)

			if tt.expectError {
				if err == nil {
					t.Errorf("extractJSONArray() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("extractJSONArray() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestExtractJSONArrayWithModelIdentify(t *testing.T) {
	t.Run("extract and verify models", func(t *testing.T) {
		body := []byte(`{"data":[{"id":"gpt-4"},{"id":"gpt-3.5-turbo"},{"id":"claude-3"}]}`)

		var models []ModelIdentify

		err := ExtractJSONArray(body, &models)
		if err != nil {
			t.Fatalf("extractJSONArray() error: %v", err)
		}

		if len(models) != 3 {
			t.Errorf("expected 3 models, got %d", len(models))
		}

		expectedIDs := []string{"gpt-4", "gpt-3.5-turbo", "claude-3"}
		for i, model := range models {
			if model.ID != expectedIDs[i] {
				t.Errorf("model[%d].ID = %q, want %q", i, model.ID, expectedIDs[i])
			}
		}
	})

	t.Run("extract from plain array", func(t *testing.T) {
		body := []byte(`[{"id":"model-a"},{"id":"model-b"}]`)

		var models []ModelIdentify

		err := ExtractJSONArray(body, &models)
		if err != nil {
			t.Fatalf("extractJSONArray() error: %v", err)
		}

		if len(models) != 2 {
			t.Errorf("expected 2 models, got %d", len(models))
		}
	})

	t.Run("extract empty array", func(t *testing.T) {
		body := []byte(`[]`)

		var models []ModelIdentify

		err := ExtractJSONArray(body, &models)
		if err != nil {
			t.Fatalf("extractJSONArray() error: %v", err)
		}

		if len(models) != 0 {
			t.Errorf("expected 0 models, got %d", len(models))
		}
	})
}

func TestExtractJSONArrayWithGeminiModels(t *testing.T) {
	t.Run("extract gemini models array", func(t *testing.T) {
		body := []byte(`{"models":[{"name":"models/gemini-pro","displayName":"Gemini Pro"}]}`)

		var models []GeminiModelResponse

		err := ExtractJSONArray(body, &models)
		if err != nil {
			t.Fatalf("extractJSONArray() error: %v", err)
		}

		if len(models) != 1 {
			t.Errorf("expected 1 model, got %d", len(models))
		}

		if models[0].Name != "models/gemini-pro" {
			t.Errorf("model.Name = %q, want %q", models[0].Name, "models/gemini-pro")
		}
	})
}
