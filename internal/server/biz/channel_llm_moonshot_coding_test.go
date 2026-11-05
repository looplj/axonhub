package biz

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestMoonshotCodingFiltersNativeToolsWithoutChangingDirectReasoning(t *testing.T) {
	tests := []struct {
		name        string
		endpoints   []objects.ChannelEndpoint
		expectedURL string
	}{
		{
			name:        "default endpoint",
			expectedURL: "https://api.kimi.com/coding/v1/messages",
		},
		{
			name: "custom endpoint",
			endpoints: []objects.ChannelEndpoint{{
				APIFormat: llm.APIFormatAnthropicMessage.String(),
				BaseURL:   "https://custom-kimi.example.com/coding",
			}},
			expectedURL: "https://custom-kimi.example.com/coding/v1/messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
			t.Cleanup(func() { _ = client.Close() })
			ctx := authz.WithTestBypass(context.Background())

			create := client.Channel.Create().
				SetName("Kimi Coding").
				SetType(channel.TypeMoonshotCoding).
				SetBaseURL("https://api.kimi.com/coding").
				SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
				SetSupportedModels([]string{"k3.1-pro"}).
				SetDefaultTestModel("k3.1-pro")
			if tt.endpoints != nil {
				create = create.SetEndpoints(tt.endpoints)
			}
			entity := create.SaveX(ctx)

			built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entity)
			require.NoError(t, err)
			outbound := built.Outbound
			if tt.endpoints != nil {
				outbound, err = BuildOutboundByAPIFormat(built, llm.APIFormatAnthropicMessage.String())
				require.NoError(t, err)
			}

			request, err := outbound.TransformRequest(ctx, moonshotCodingDirectRequest(nil, moonshotCodingTestTools()))
			require.NoError(t, err)
			require.Equal(t, tt.expectedURL, request.URL)

			var payload anthropic.MessageRequest
			require.NoError(t, json.Unmarshal(request.Body, &payload))
			require.Equal(t, int64(8192), payload.MaxTokens)
			require.Len(t, payload.Tools, 1)
			require.Equal(t, "get_weather", payload.Tools[0].Name)
			require.NotNil(t, payload.OutputConfig)
			require.Equal(t, llm.ReasoningEffortMedium, payload.OutputConfig.Effort)
			require.NotNil(t, payload.Thinking)
			require.Equal(t, "adaptive", payload.Thinking.Type)
			require.Zero(t, payload.Thinking.BudgetTokens)
		})
	}
}

func TestMoonshotCodingRejectsForcedChoiceWhenNativeToolsFiltered(t *testing.T) {
	tests := []struct {
		name       string
		toolChoice *llm.ToolChoice
		tools      []llm.Tool
	}{
		{
			name:       "required",
			toolChoice: &llm.ToolChoice{ToolChoice: lo.ToPtr("required")},
			tools: []llm.Tool{{
				Type:      llm.ToolTypeWebSearch,
				WebSearch: &llm.WebSearch{},
			}},
		},
		{
			name: "hosted web search",
			toolChoice: &llm.ToolChoice{
				NamedToolChoice: &llm.NamedToolChoice{
					Type: llm.ToolTypeWebSearch,
				},
			},
			tools: []llm.Tool{
				{
					Type: "function",
					Function: llm.Function{
						Name:       anthropic.WebSearchFunctionName,
						Parameters: json.RawMessage(`{"type":"object","properties":{}}`),
					},
				},
				{
					Type:      llm.ToolTypeWebSearch,
					WebSearch: &llm.WebSearch{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
			t.Cleanup(func() { _ = client.Close() })
			ctx := authz.WithTestBypass(context.Background())

			entity := client.Channel.Create().
				SetName("Kimi Coding").
				SetType(channel.TypeMoonshotCoding).
				SetBaseURL("https://api.kimi.com/coding").
				SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
				SetSupportedModels([]string{"k3.1-pro"}).
				SetDefaultTestModel("k3.1-pro").
				SaveX(ctx)

			built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entity)
			require.NoError(t, err)

			_, err = built.Outbound.TransformRequest(ctx, moonshotCodingDirectRequest(tt.toolChoice, tt.tools))
			require.ErrorIs(t, err, transformer.ErrUnsupportedConversion)
		})
	}
}

func moonshotCodingDirectRequest(toolChoice *llm.ToolChoice, tools []llm.Tool) *llm.Request {
	return &llm.Request{
		Model:           "k3.1-pro",
		ReasoningEffort: llm.ReasoningEffortMedium,
		Messages:        []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}},
		Tools:           tools,
		ToolChoice:      toolChoice,
	}
}

func moonshotCodingTestTools() []llm.Tool {
	return []llm.Tool{
		{
			Type: "function",
			Function: llm.Function{
				Name:       "get_weather",
				Parameters: json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type:      llm.ToolTypeWebSearch,
			WebSearch: &llm.WebSearch{},
		},
	}
}
