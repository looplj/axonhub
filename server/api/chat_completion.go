package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/llm/client"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/types"
	"github.com/looplj/axonhub/log"
)

type ChatCompletionProcessor struct {
	InboundTransformer  transformer.InboundTransformer
	OutboundTransformer transformer.OutboundTransformer
	HTTPClient          client.HttpClient
}

// NewChatCompletionProcessor creates a new ChatCompletionProcessor
func NewChatCompletionProcessor(
	inboundTransformer transformer.InboundTransformer,
	outboundTransformer transformer.OutboundTransformer,
	httpClient client.HttpClient,
) *ChatCompletionProcessor {
	return &ChatCompletionProcessor{
		InboundTransformer:  inboundTransformer,
		OutboundTransformer: outboundTransformer,
		HTTPClient:          httpClient,
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

	// Step 3: Outbound transformation - Convert to provider-specific format
	genericReq, err := processor.OutboundTransformer.Transform(ctx, chatReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to transform request: %v", err)})
		return err
	}

	log.Info(ctx, "Generic request", log.Any("request", genericReq))

	// Step 4: Execute HTTP request to provider
	genericResp, err := processor.executeRequest(ctx, genericReq)
	if err != nil {
		log.Error(ctx, "Failed to execute request", log.Cause(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to execute request: %v", err)})
		return err
	}

	log.Info(ctx, "Generic resp", log.Any("response", genericResp))

	// Step 5: Handle streaming vs non-streaming responses
	if chatReq.Stream {
		return processor.handleStreamingResponse(c, genericResp, chatReq)
	} else {
		return processor.handleNonStreamingResponse(c, genericResp, chatReq)
	}
}

func (processor *ChatCompletionProcessor) executeRequest(ctx context.Context, req *types.GenericHttpRequest) (*types.GenericHttpResponse, error) {
	// Use the dedicated HTTP client for streaming or non-streaming requests
	if req.Streaming {
		return processor.HTTPClient.DoStream(ctx, req)
	} else {
		return processor.HTTPClient.Do(ctx, req)
	}
}

func (processor *ChatCompletionProcessor) handleNonStreamingResponse(c *gin.Context, genericResp *types.GenericHttpResponse, originalReq *types.ChatCompletionRequest) error {
	// Step 6: Outbound response transformation
	chatResp, err := processor.OutboundTransformer.TransformResponse(c.Request.Context(), genericResp, originalReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to transform response: %v", err)})
		return err
	}

	// Step 7: Inbound response transformation (final formatting)
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

func (processor *ChatCompletionProcessor) handleStreamingResponse(c *gin.Context, genericResp *types.GenericHttpResponse, originalReq *types.ChatCompletionRequest) error {
	// Step 6: Outbound streaming response transformation
	streamChan, err := processor.OutboundTransformer.TransformStreamResponse(c.Request.Context(), genericResp, originalReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to transform streaming response: %v", err)})
		return err
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Stream responses
	for streamResp := range streamChan {
		// Step 7: Format as SSE
		data, err := json.Marshal(streamResp)
		if err != nil {
			continue
		}

		// Write SSE format
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()
	}

	// Send done signal
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()

	return nil
}
