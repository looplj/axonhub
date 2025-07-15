package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/samber/lo"
	openai "github.com/sashabaranov/go-openai"
	"github.com/looplj/axonhub/llm/provider"
	"github.com/looplj/axonhub/llm/types"
	"github.com/looplj/axonhub/pkg/streams"
)

// Provider implements the provider.Provider interface for OpenAI
type Provider struct {
	name          string
	client        *openai.Client
	config        *provider.ProviderConfig
	modelMappings map[string]provider.ModelMapping
}

// NewProvider creates a new OpenAI provider
func NewProvider(config *provider.ProviderConfig) provider.Provider {
	clientConfig := openai.DefaultConfig(config.APIKey)
	clientConfig.BaseURL = config.BaseURL

	modelMappings := lo.SliceToMap(config.ModelMappings, func(item provider.ModelMapping) (string, provider.ModelMapping) {
		return item.From, item
	})
	client := openai.NewClientWithConfig(clientConfig)
	return &Provider{
		name:          config.Name,
		client:        client,
		modelMappings: modelMappings,
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return p.name
}

// ChatCompletion sends a chat completion request and returns the response
func (p *Provider) ChatCompletion(ctx context.Context, request *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error) {
	openaiReq, err := p.convertToOpenAIRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	resp, err := p.client.CreateChatCompletion(ctx, *openaiReq)
	if err != nil {
		return nil, fmt.Errorf("openai API error: %w", err)
	}

	return convertFromOpenAIResponse(&resp)
}

// ChatCompletionStream sends a streaming chat completion request
func (p *Provider) ChatCompletionStream(ctx context.Context, request *types.ChatCompletionRequest) (streams.Stream[*types.ChatCompletionResponse], error) {
	// Convert internal request to OpenAI request
	openaiReq, err := p.convertToOpenAIRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	openaiReq.Stream = true
	stream, err := p.client.CreateChatCompletionStream(ctx, *openaiReq)
	if err != nil {
		return nil, fmt.Errorf("openai stream API error: %w", err)
	}

	return &streamAdapter{
		ChatCompletionStream: stream,
	}, nil
}

// SupportsModel checks if the provider supports a specific model
func (p *Provider) SupportsModel(model string) bool {
	// OpenAI model patterns
	openaiModels := []string{
		"gpt-4", "gpt-4-turbo", "gpt-4o", "gpt-4o-mini",
		"gpt-3.5-turbo", "gpt-3.5",
		"o1-preview", "o1-mini",
	}

	for _, supportedModel := range openaiModels {
		if strings.HasPrefix(model, supportedModel) {
			return true
		}
	}

	return false
}

// GetConfig returns the provider configuration
func (p *Provider) GetConfig() *provider.ProviderConfig {
	return p.config
}

// SetConfig updates the provider configuration
func (p *Provider) SetConfig(config *provider.ProviderConfig) {
	p.config = config

	// Recreate client with new config
	p.client = openai.NewClient(config.APIKey)
	if config.BaseURL != "" {
		clientConfig := openai.DefaultConfig(config.APIKey)
		clientConfig.BaseURL = config.BaseURL
		p.client = openai.NewClientWithConfig(clientConfig)
	}
}

// convertToOpenAIRequest converts internal request to OpenAI request
func (p *Provider) convertToOpenAIRequest(req *types.ChatCompletionRequest) (*openai.ChatCompletionRequest, error) {
	openaiReq := &openai.ChatCompletionRequest{
		Model: req.Model,
	}

	// Convert messages
	for _, msg := range req.Messages {
		openaiMsg := openai.ChatCompletionMessage{
			Role: msg.Role,
		}

		// Handle content
		if msg.Content.Content != nil {
			openaiMsg.Content = *msg.Content.Content
		} else if len(msg.Content.MultipleContent) > 0 {
			// Handle multi-modal content
			var parts []openai.ChatMessagePart
			for _, part := range msg.Content.MultipleContent {
				switch part.Type {
				case "text":
					if part.Text != nil {
						parts = append(parts, openai.ChatMessagePart{
							Type: openai.ChatMessagePartTypeText,
							Text: *part.Text,
						})
					}
				case "image_url":
					if part.ImageURL != nil {
						parts = append(parts, openai.ChatMessagePart{
							Type: openai.ChatMessagePartTypeImageURL,
							ImageURL: &openai.ChatMessageImageURL{
								URL:    part.ImageURL.URL,
								Detail: openai.ImageURLDetail(part.ImageURL.Detail),
							},
						})
					}
				}
			}
			openaiMsg.MultiContent = parts
		}

		// Handle other fields
		if msg.Name != nil {
			openaiMsg.Name = *msg.Name
		}
		if msg.ToolCallID != nil {
			openaiMsg.ToolCallID = *msg.ToolCallID
		}

		openaiReq.Messages = append(openaiReq.Messages, openaiMsg)
	}

	// Set optional parameters
	if req.MaxTokens != nil {
		openaiReq.MaxTokens = int(*req.MaxTokens)
	}
	if req.MaxCompletionTokens != nil {
		openaiReq.MaxCompletionTokens = int(*req.MaxCompletionTokens)
	}
	if req.Temperature != nil {
		openaiReq.Temperature = float32(*req.Temperature)
	}
	if req.TopP != nil {
		openaiReq.TopP = float32(*req.TopP)
	}
	if req.N != nil {
		openaiReq.N = int(*req.N)
	}
	if req.Stream != nil {
		openaiReq.Stream = *req.Stream
	}
	if req.Stop != nil {
		if req.Stop.Stop != nil {
			openaiReq.Stop = []string{*req.Stop.Stop}
		} else {
			openaiReq.Stop = req.Stop.MultipleStop
		}
	}
	if req.PresencePenalty != nil {
		openaiReq.PresencePenalty = float32(*req.PresencePenalty)
	}
	if req.FrequencyPenalty != nil {
		openaiReq.FrequencyPenalty = float32(*req.FrequencyPenalty)
	}
	if req.User != nil {
		openaiReq.User = *req.User
	}

	return openaiReq, nil
}

// convertFromOpenAIResponse converts OpenAI response to internal response
func convertFromOpenAIResponse(oaiResp *openai.ChatCompletionResponse) (*types.ChatCompletionResponse, error) {
	resp, err := convertToResponse(oaiResp)
	if err != nil {
		return nil, err
	}
	resp.SetHeader(oaiResp.Header())
	return resp, nil
}

func convertFromOpenAIStreamResponse(oaiResp *openai.ChatCompletionStreamResponse) (*types.ChatCompletionResponse, error) {
	return convertToResponse(oaiResp)
}

func convertToResponse(oaiResp any) (*types.ChatCompletionResponse, error) {
	data, err := json.Marshal(oaiResp)
	if err != nil {
		return nil, err
	}

	resp := &types.ChatCompletionResponse{}
	err = json.Unmarshal(data, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
