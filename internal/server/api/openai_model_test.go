package api

import (
	"testing"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/stretchr/testify/assert"
)

func TestConvertModelToOpenAIExtended_NilModelCard(t *testing.T) {
	remark := "Test description"
	m := &ent.Model{
		ModelID:   "gpt-4",
		Name:      "GPT-4",
		Developer: "openai",
		Type:      model.TypeChat,
		Icon:      "openai",
		Remark:    &remark,
		CreatedAt: time.Unix(1686935002, 0),
		ModelCard: nil,
	}

	result := convertModelToOpenAIExtended(m, nil)

	assert.Equal(t, "gpt-4", result.ID)
	assert.Equal(t, "GPT-4", result.Name)
	assert.Equal(t, "Test description", result.Description)
	assert.Equal(t, "openai", result.OwnedBy)
	assert.Equal(t, "chat", result.Type)
	assert.Equal(t, "openai", result.Icon)
	assert.Equal(t, int64(1686935002), result.Created)
	assert.Nil(t, result.Capabilities)
	assert.Nil(t, result.Pricing)
}

func TestConvertModelToOpenAIExtended_CompleteData(t *testing.T) {
	remark := "GPT-4 is a large multimodal model"
	m := &ent.Model{
		ModelID:   "gpt-4",
		Name:      "GPT-4",
		Developer: "openai",
		Type:      model.TypeChat,
		Icon:      "openai",
		Remark:    &remark,
		CreatedAt: time.Unix(1686935002, 0),
		ModelCard: &objects.ModelCard{
			Vision:    true,
			ToolCall:  true,
			Reasoning: objects.ModelCardReasoning{Supported: true},
			Limit:     objects.ModelCardLimit{Context: 8192, Output: 4096},
			Cost:      objects.ModelCardCost{Input: 0.03, Output: 0.06, CacheRead: 0.015, CacheWrite: 0.03},
		},
	}

	result := convertModelToOpenAIExtended(m, nil)

	assert.Equal(t, "gpt-4", result.ID)
	assert.Equal(t, "GPT-4", result.Name)
	assert.Equal(t, "GPT-4 is a large multimodal model", result.Description)
	assert.NotNil(t, result.Capabilities)
	assert.True(t, result.Capabilities.Vision)
	assert.True(t, result.Capabilities.ToolCall)
	assert.True(t, result.Capabilities.Reasoning)
	assert.Equal(t, 8192, result.ContextLength)
	assert.Equal(t, 4096, result.MaxOutputTokens)
	assert.NotNil(t, result.Pricing)
	assert.Equal(t, 0.03, result.Pricing.Input)
	assert.Equal(t, 0.06, result.Pricing.Output)
	assert.Equal(t, 0.015, result.Pricing.CacheRead)
	assert.Equal(t, 0.03, result.Pricing.CacheWrite)
	assert.Equal(t, "per_1m_tokens", result.Pricing.Unit)
	assert.Equal(t, "USD", result.Pricing.Currency)
}

func TestConvertModelToOpenAIExtended_NilRemark(t *testing.T) {
	m := &ent.Model{
		ModelID:   "gpt-4",
		Name:      "GPT-4",
		Developer: "openai",
		Type:      model.TypeChat,
		Icon:      "openai",
		Remark:    nil,
		CreatedAt: time.Now(),
		ModelCard: nil,
	}

	result := convertModelToOpenAIExtended(m, nil)
	assert.Equal(t, "", result.Description)
	assert.Nil(t, result.Capabilities)
	assert.Nil(t, result.Pricing)
}
