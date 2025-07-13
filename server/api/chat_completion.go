package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/llm/provider"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/types"
	"github.com/looplj/axonhub/log"
)

type ChatCompletionProcessor struct {
	InboundTransformer transformer.Transformer
	ProviderRegistry   provider.ProviderRegistry
}

// NewChatCompletionProcessor creates a new ChatCompletionProcessor
func NewChatCompletionProcessor(
	inboundTransformer transformer.Transformer,
	providerRegistry provider.ProviderRegistry,
) *ChatCompletionProcessor {
	return &ChatCompletionProcessor{
		InboundTransformer: inboundTransformer,
		ProviderRegistry:   providerRegistry,
	}
}

func (processor *ChatCompletionProcessor) Process(c *gin.Context) error {
	ctx := c.Request.Context()

	// Step 1: Inbound transformation - Convert HTTP request to ChatCompletionRequest
	chatReq, err := processor.InboundTransformer.Transform(ctx, c.Request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to parse request: %v", err)})
		return err
	}

	log.Info(ctx, "Inbound request", log.Any("request", chatReq))

	// Step 2: TODO - Apply decorators (rate limiting, authentication, etc.)
	// This would be where decorator chain processing happens

	// Step 3: Get provider for the model
	prov, err := processor.ProviderRegistry.GetProviderForModel(chatReq.Model)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("No provider found for model %s: %v", chatReq.Model, err)})
		return err
	}

	log.Info(ctx, "Using provider", log.Any("provider", prov.GetConfig().Name), log.Any("model", chatReq.Model))

	// Step 4: Handle streaming vs non-streaming responses
	if chatReq.Stream != nil && *chatReq.Stream {
		return processor.handleStreamingResponse(c, prov, chatReq)
	} else {
		return processor.handleNonStreamingResponse(c, prov, chatReq)
	}
}

func (processor *ChatCompletionProcessor) handleNonStreamingResponse(c *gin.Context, prov provider.Provider, originalReq *types.ChatCompletionRequest) error {
	// Step 5: Call provider for chat completion
	chatResp, err := prov.ChatCompletion(c.Request.Context(), originalReq)
	if err != nil {
		log.Error(c.Request.Context(), "Provider chat completion failed", log.Cause(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Provider error: %v", err)})
		return err
	}

	log.Info(c.Request.Context(), "Chat completion response", log.Any("response", chatResp))

	// Step 6: Inbound response transformation (final formatting)
	// Create a mock HTTP response for the inbound transformer
	mockResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}
	mockResp.Header.Set("Content-Type", "application/json")

	finalResp, err := processor.InboundTransformer.TransformResponse(c.Request.Context(), chatResp, mockResp)
	if err != nil {
		// If inbound transformation fails, return the chat response directly
		c.Header("Content-Type", "application/json")
		c.JSON(http.StatusOK, chatResp)
		return nil
	}

	// Return the transformed response
	for key, value := range finalResp.Headers {
		c.Header(key, value)
	}
	c.Data(finalResp.StatusCode, finalResp.Headers["Content-Type"], finalResp.Body)
	return nil
}

func (processor *ChatCompletionProcessor) handleStreamingResponse(c *gin.Context, prov provider.Provider, originalReq *types.ChatCompletionRequest) error {
	stream, err := prov.ChatCompletionStream(c.Request.Context(), originalReq)
	if err != nil {
		log.Error(c.Request.Context(), "Provider streaming failed", log.Cause(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Provider streaming error: %v", err)})
		return err
	}

	log.Info(c.Request.Context(), "Started streaming response")

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// TODO Handle error
	for stream.Next() {
		cur := stream.Current()
		data, err := json.Marshal(cur)
		if err != nil {
			log.Error(c.Request.Context(), "Failed to marshal stream response", log.Cause(err))
			continue
		}

		log.Info(c.Request.Context(), "Stream response", log.Any("current", cur))

		// Write SSE format
		_, err = fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		if err != nil {
			return err
		}
		c.Writer.Flush()
	}

	_, err = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	if err != nil {
		log.Error(c.Request.Context(), "Failed to write stream end", log.Cause(err))
		return err
	}
	c.Writer.Flush()
	return nil
}
