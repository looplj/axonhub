package cmds

import (
	"testing"

	"github.com/looplj/axonhub/axon/api"
	"github.com/stretchr/testify/assert"
)

func TestIsValidModelType(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"chat", true},
		{"CHAT", true},
		{"Chat", true},
		{"embedding", true},
		{"EMBEDDING", true},
		{"rerank", true},
		{"image_generation", true},
		{"video_generation", true},
		{"IMAGE_GENERATION", true},
		{"image", false},
		{"video", false},
		{"audio", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidModelType(tt.input))
		})
	}
}

func TestModelFilterValidate(t *testing.T) {
	tests := []struct {
		name      string
		filter    ModelFilter
		expectErr bool
	}{
		{
			name:      "empty filter",
			filter:    ModelFilter{},
			expectErr: false,
		},
		{
			name:      "valid type chat",
			filter:    ModelFilter{Type: "chat"},
			expectErr: false,
		},
		{
			name:      "valid type embedding",
			filter:    ModelFilter{Type: "embedding"},
			expectErr: false,
		},
		{
			name:      "valid type rerank",
			filter:    ModelFilter{Type: "rerank"},
			expectErr: false,
		},
		{
			name:      "valid type image_generation",
			filter:    ModelFilter{Type: "image_generation"},
			expectErr: false,
		},
		{
			name:      "valid type video_generation",
			filter:    ModelFilter{Type: "video_generation"},
			expectErr: false,
		},
		{
			name:      "valid type case insensitive",
			filter:    ModelFilter{Type: "CHAT"},
			expectErr: false,
		},
		{
			name:      "invalid type image",
			filter:    ModelFilter{Type: "image"},
			expectErr: true,
		},
		{
			name:      "invalid type audio",
			filter:    ModelFilter{Type: "audio"},
			expectErr: true,
		},
		{
			name:      "keyword only no error",
			filter:    ModelFilter{Keyword: "gpt"},
			expectErr: false,
		},
		{
			name:      "valid type with keyword",
			filter:    ModelFilter{Type: "chat", Keyword: "gpt"},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filter.Validate()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFilterModels(t *testing.T) {
	descGPT := "GPT-4 model"
	descClaude := "Claude model"

	models := []*api.AvailableModelsAvailableModelsAvailableModel{
		{
			Id:          "gpt-4",
			Name:        "GPT-4",
			Type:        "chat",
			OwnedBy:     "openai",
			Description: &descGPT,
		},
		{
			Id:          "gpt-3.5-turbo",
			Name:        "GPT-3.5 Turbo",
			Type:        "chat",
			OwnedBy:     "openai",
			Description: nil,
		},
		{
			Id:          "claude-3-opus",
			Name:        "Claude 3 Opus",
			Type:        "chat",
			OwnedBy:     "anthropic",
			Description: &descClaude,
		},
		{
			Id:          "text-embedding-ada-002",
			Name:        "Text Embedding ADA",
			Type:        "embedding",
			OwnedBy:     "openai",
			Description: nil,
		},
		{
			Id:          "dall-e-3",
			Name:        "DALL-E 3",
			Type:        "image_generation",
			OwnedBy:     "openai",
			Description: nil,
		},
		{
			Id:          "cohere-rerank",
			Name:        "Cohere Rerank",
			Type:        "rerank",
			OwnedBy:     "cohere",
			Description: nil,
		},
		{
			Id:          "sora",
			Name:        "Sora",
			Type:        "video_generation",
			OwnedBy:     "openai",
			Description: nil,
		},
	}

	tests := []struct {
		name     string
		filter   ModelFilter
		expected []string
	}{
		{
			name:     "no filter",
			filter:   ModelFilter{},
			expected: []string{"gpt-4", "gpt-3.5-turbo", "claude-3-opus", "text-embedding-ada-002", "dall-e-3", "cohere-rerank", "sora"},
		},
		{
			name:     "filter by type chat",
			filter:   ModelFilter{Type: "chat"},
			expected: []string{"gpt-4", "gpt-3.5-turbo", "claude-3-opus"},
		},
		{
			name:     "filter by type embedding",
			filter:   ModelFilter{Type: "embedding"},
			expected: []string{"text-embedding-ada-002"},
		},
		{
			name:     "filter by type image_generation",
			filter:   ModelFilter{Type: "image_generation"},
			expected: []string{"dall-e-3"},
		},
		{
			name:     "filter by type rerank",
			filter:   ModelFilter{Type: "rerank"},
			expected: []string{"cohere-rerank"},
		},
		{
			name:     "filter by type video_generation",
			filter:   ModelFilter{Type: "video_generation"},
			expected: []string{"sora"},
		},
		{
			name:     "filter by type case insensitive",
			filter:   ModelFilter{Type: "CHAT"},
			expected: []string{"gpt-4", "gpt-3.5-turbo", "claude-3-opus"},
		},
		{
			name:     "filter by keyword gpt",
			filter:   ModelFilter{Keyword: "gpt"},
			expected: []string{"gpt-4", "gpt-3.5-turbo"},
		},
		{
			name:     "filter by keyword claude",
			filter:   ModelFilter{Keyword: "claude"},
			expected: []string{"claude-3-opus"},
		},
		{
			name:     "filter by keyword in name",
			filter:   ModelFilter{Keyword: "opus"},
			expected: []string{"claude-3-opus"},
		},
		{
			name:     "filter by keyword case insensitive",
			filter:   ModelFilter{Keyword: "GPT"},
			expected: []string{"gpt-4", "gpt-3.5-turbo"},
		},
		{
			name:     "filter by type and keyword",
			filter:   ModelFilter{Type: "chat", Keyword: "gpt"},
			expected: []string{"gpt-4", "gpt-3.5-turbo"},
		},
		{
			name:     "filter by type and keyword no match",
			filter:   ModelFilter{Type: "embedding", Keyword: "gpt"},
			expected: []string{},
		},
		{
			name:     "filter by keyword with whitespace",
			filter:   ModelFilter{Keyword: "  gpt  "},
			expected: []string{"gpt-4", "gpt-3.5-turbo"},
		},
		{
			name:     "filter by type with whitespace",
			filter:   ModelFilter{Type: "  chat  "},
			expected: []string{"gpt-4", "gpt-3.5-turbo", "claude-3-opus"},
		},
		{
			name:     "filter by non-existent type",
			filter:   ModelFilter{Type: "audio"},
			expected: []string{},
		},
		{
			name:     "filter by non-existent keyword",
			filter:   ModelFilter{Keyword: "nonexistent"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterModels(models, tt.filter)

			resultIDs := make([]string, len(result))
			for i, m := range result {
				resultIDs[i] = m.Id
			}

			assert.Equal(t, tt.expected, resultIDs)
		})
	}
}

func TestFilterModelsEmptyInput(t *testing.T) {
	var models []*api.AvailableModelsAvailableModelsAvailableModel

	result := FilterModels(models, ModelFilter{})
	assert.Empty(t, result)

	result = FilterModels(models, ModelFilter{Type: "chat"})
	assert.Empty(t, result)

	result = FilterModels(models, ModelFilter{Keyword: "gpt"})
	assert.Empty(t, result)
}

func TestFilterModelsPreservesOriginal(t *testing.T) {
	models := []*api.AvailableModelsAvailableModelsAvailableModel{
		{Id: "gpt-4", Name: "GPT-4", Type: "chat"},
		{Id: "claude-3", Name: "Claude 3", Type: "chat"},
	}

	result := FilterModels(models, ModelFilter{Type: "chat"})
	assert.Len(t, result, 2)

	assert.Equal(t, "gpt-4", models[0].Id)
	assert.Equal(t, "claude-3", models[1].Id)
}
