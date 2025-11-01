package biz

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/trace"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/anthropic"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

type TraceService struct {
	requestService *RequestService
}

func NewTraceService(requestService *RequestService) *TraceService {
	return &TraceService{
		requestService: requestService,
	}
}

// GetOrCreateTrace retrieves an existing trace by trace_id and project_id,
// or creates a new one if it doesn't exist.
func (s *TraceService) GetOrCreateTrace(ctx context.Context, projectID int, traceID string, threadID *int) (*ent.Trace, error) {
	client := ent.FromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	// Try to find existing trace
	trace, err := client.Trace.Query().
		Where(
			trace.TraceIDEQ(traceID),
			trace.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err == nil {
		// Trace found
		return trace, nil
	}

	// If error is not "not found", return the error
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to query trace: %w", err)
	}

	// Trace not found, create new one
	createTrace := client.Trace.Create().
		SetTraceID(traceID).
		SetProjectID(projectID).
		SetNillableThreadID(threadID)

	return createTrace.Save(ctx)
}

// GetTraceByID retrieves a trace by its trace_id and project_id.
func (s *TraceService) GetTraceByID(ctx context.Context, traceID string, projectID int) (*ent.Trace, error) {
	client := ent.FromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	trace, err := client.Trace.Query().
		Where(
			trace.TraceIDEQ(traceID),
			trace.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get trace: %w", err)
	}

	return trace, nil
}

type RequestTrace struct {
	ID            int              `json:"id"`
	ParentID      *int             `json:"parentId,omitempty"`
	Model         string           `json:"model"`
	Children      []*RequestTrace  `json:"children,omitempty"`
	RequestSpans  []Span           `json:"requestSpans,omitempty"`
	ResponseSpans []Span           `json:"responseSpans,omitempty"`
	Metadata      *RequestMetadata `json:"metadata,omitempty"`
	StartTime     time.Time        `json:"startTime"`
	EndTime       time.Time        `json:"endTime"`
	Duration      string           `json:"duration"`
}

// Span represents a trace span with timing and metadata information.
type Span struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	// "query" | "image_url" | "embedding" | "tool_use" | "tool_result" | "text" | "thinking"
	Type      string     `json:"type"`
	StartTime time.Time  `json:"startTime,omitempty"`
	EndTime   time.Time  `json:"endTime,omitempty"`
	Value     *SpanValue `json:"value,omitempty"`
}

type SpanValue struct {
	Query          *SpanQuery          `json:"query,omitempty"`
	Text           *SpanText           `json:"text,omitempty"`
	ImageURL       *SpanImageURL       `json:"imageUrl,omitempty"`
	Thinking       *SpanThinking       `json:"thinking,omitempty"`
	FunctionCall   *SpanFunctionCall   `json:"functionCall,omitempty"`
	FunctionResult *SpanFunctionResult `json:"functionResult,omitempty"`
}

type SpanQuery struct {
	ModelID string `json:"modelId,omitempty"`
	Prompt  string `json:"prompt,omitempty"`
}

type SpanThinking struct {
	Thinking string `json:"thinking,omitempty"`
}

type SpanText struct {
	Text string `json:"text,omitempty"`
}

type SpanImageURL struct {
	URL string `json:"url,omitempty"`
}

type SpanFunctionCall struct {
	ID        string  `json:"id,omitempty"`
	Name      string  `json:"name"`
	Arguments *string `json:"arguments,omitempty"`
}

type SpanFunctionResult struct {
	ID      string `json:"id,omitempty"`
	IsError bool   `json:"error,omitempty"`
	// Text
	Type string  `json:"type,omitempty"`
	Text *string `json:"text,omitempty"`
}

// RequestMetadata contains additional metadata for a span.
type RequestMetadata struct {
	Cost      *float64 `json:"cost,omitempty"`
	ItemCount *int     `json:"itemCount,omitempty"`
	Tokens    *int64   `json:"tokens,omitempty"`
}

type SpanToolCall struct {
	ID    string  `json:"id,omitempty"`
	Name  string  `json:"name"`
	Input *string `json:"input,omitempty"`
}

type SpanToolResult struct {
	ID      string  `json:"id,omitempty"`
	IsError bool    `json:"error,omitempty"`
	Output  *string `json:"output,omitempty"`
}

// GetRootRequestTrace retrieves the hierarchical request traces for a trace ID.
func (s *TraceService) GetRootRequestTrace(ctx context.Context, traceID int) (*RequestTrace, error) {
	client := ent.FromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	requests, err := client.Request.Query().
		Where(request.TraceIDEQ(traceID)).
		Order(ent.Asc(request.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query requests: %w", err)
	}

	if len(requests) == 0 {
		return nil, nil
	}

	eg, ctx := errgroup.WithContext(ctx)
	for _, req := range requests {
		eg.Go(func() (err error) {
			req.RequestBody, err = s.requestService.LoadRequestBody(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to load request body: %w", err)
			}

			req.ResponseBody, err = s.requestService.LoadResponseBody(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to load request response body: %w", err)
			}

			return err
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("failed to load request body: %w", err)
	}

	traces := make([]*RequestTrace, len(requests))
	for i, req := range requests {
		traces[i], err = s.requestToTrace(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to build request trace: %w", err)
		}

		if i == 0 {
			continue
		}

		traces[i].ParentID = &requests[i-1].ID
		traces[i-1].Children = append(traces[i-1].Children, traces[i])
	}

	return traces[0], nil
}

// requestToTrace converts a request entity to a RequestTrace.
func (s *TraceService) requestToTrace(ctx context.Context, req *ent.Request) (*RequestTrace, error) {
	trace := &RequestTrace{
		ID:        req.ID,
		Model:     req.ModelID,
		StartTime: req.CreatedAt,
		EndTime:   req.UpdatedAt,
		Duration:  req.UpdatedAt.Sub(req.CreatedAt).String(),
	}

	var (
		requestSpans  []Span
		responseSpans []Span
	)

	if len(req.RequestBody) > 0 {
		httpReq := &httpclient.Request{
			Body: req.RequestBody,
			Headers: map[string][]string{
				"Content-Type": {"application/json"},
			},
		}

		inbound, err := s.getInboundTransformer(llm.APIFormat(req.Format))
		if err != nil {
			return nil, fmt.Errorf("failed to get inbound transformer: %w", err)
		}

		llmReq, err := inbound.TransformRequest(ctx, httpReq)
		if err != nil {
			return nil, fmt.Errorf("failed to transform request body: %w", err)
		}

		requestSpans = append(requestSpans, s.extractSpansFromMessages(llmReq.Messages, "request")...)
	}

	if len(req.ResponseBody) > 0 {
		outbound, err := s.getOutboundTransformer(llm.APIFormat(req.Format))
		if err != nil {
			return nil, fmt.Errorf("failed to get outbound transformer: %w", err)
		}

		httpResp := &httpclient.Response{
			Body:       req.ResponseBody,
			StatusCode: http.StatusOK,
			Headers: http.Header{
				"Content-Type": {"application/json"},
			},
		}

		unifiedResp, err := outbound.TransformResponse(ctx, httpResp)
		if err != nil {
			return nil, fmt.Errorf("failed to transform response body: %w", err)
		}

		// trace.Metadata = s.extractMetadataFromResponse(unifiedResp)
		if len(unifiedResp.Choices) > 0 && unifiedResp.Choices[0].Message != nil {
			responseSpans = append(responseSpans, s.extractSpansFromMessage(unifiedResp.Choices[0].Message, "response")...)
		}
	}

	trace.RequestSpans = requestSpans
	trace.ResponseSpans = responseSpans

	return trace, nil
}

func (s *TraceService) extractSpansFromMessages(messages []llm.Message, idPrefix string) []Span {
	var spans []Span

	for i, msg := range messages {
		msgSpans := s.extractSpansFromMessage(&msg, fmt.Sprintf("%s-%d", idPrefix, i))
		spans = append(spans, msgSpans...)
	}

	return spans
}

// extractSpansFromMessage converts a single message to spans.
func (s *TraceService) extractSpansFromMessage(msg *llm.Message, idPrefix string) []Span {
	var spans []Span

	now := time.Now()

	// Handle reasoning content
	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		spans = append(spans, Span{
			ID:        fmt.Sprintf("%s-reasoning", idPrefix),
			Name:      "Reasoning",
			Type:      "thinking",
			StartTime: now,
			EndTime:   now,
			Value: &SpanValue{
				Thinking: &SpanThinking{
					Thinking: *msg.ReasoningContent,
				},
			},
		})
	}

	// Handle text content
	if msg.Content.Content != nil && *msg.Content.Content != "" {
		spans = append(spans, Span{
			ID:        fmt.Sprintf("%s-text", idPrefix),
			Name:      fmt.Sprintf("%s message", msg.Role),
			Type:      lo.Ternary(msg.Role == "user", "query", "text"),
			StartTime: now,
			EndTime:   now,
			Value: &SpanValue{
				Text: &SpanText{
					Text: *msg.Content.Content,
				},
			},
		})
	}

	// Handle multiple content parts
	for i, part := range msg.Content.MultipleContent {
		partSpan := Span{
			ID:        fmt.Sprintf("%s-part-%d", idPrefix, i),
			StartTime: now,
			EndTime:   now,
		}

		switch part.Type {
		case "text":
			partSpan.Name = "Text content"

			partSpan.Type = "text"
			if part.Text != nil {
				partSpan.Value = &SpanValue{
					Text: &SpanText{
						Text: *part.Text,
					},
				}
			}
		case "image_url":
			partSpan.Name = "Image"

			partSpan.Type = "image_url"
			if part.ImageURL != nil {
				partSpan.Value = &SpanValue{
					ImageURL: &SpanImageURL{
						URL: part.ImageURL.URL,
					},
				}
			}
		default:
			// ignore for now.
		}

		spans = append(spans, partSpan)
	}

	// Handle tool calls
	for i, toolCall := range msg.ToolCalls {
		args := toolCall.Function.Arguments
		toolSpan := Span{
			ID:        fmt.Sprintf("%s-tool-%d", idPrefix, i),
			Name:      fmt.Sprintf("Tool: %s", toolCall.Function.Name),
			Type:      "tool_use",
			StartTime: now,
			EndTime:   now,
			Value: &SpanValue{
				FunctionCall: &SpanFunctionCall{
					ID:        toolCall.ID,
					Name:      toolCall.Function.Name,
					Arguments: &args,
				},
			},
		}
		spans = append(spans, toolSpan)
	}

	// Handle tool results (when role is "tool")
	if msg.Role == "tool" && msg.ToolCallID != nil {
		var text *string
		if msg.Content.Content != nil {
			text = msg.Content.Content
		}

		isError := false
		if msg.ToolCallIsError != nil {
			isError = *msg.ToolCallIsError
		}

		toolResultSpan := Span{
			ID:        fmt.Sprintf("%s-tool-result", idPrefix),
			Name:      "Tool Result",
			Type:      "tool_result",
			StartTime: now,
			EndTime:   now,
			Value: &SpanValue{
				FunctionResult: &SpanFunctionResult{
					ID:      *msg.ToolCallID,
					IsError: isError,
					Type:    "text",
					Text:    text,
				},
			},
		}
		spans = append(spans, toolResultSpan)
	}

	return spans
}

// getInboundTransformer returns the appropriate inbound transformer based on format.
func (s *TraceService) getInboundTransformer(format llm.APIFormat) (transformer.Inbound, error) {
	//nolint:exhaustive // TODO: add more formats.
	switch format {
	case llm.APIFormatOpenAIChatCompletion:
		return openai.NewInboundTransformer(), nil
	case llm.APIFormatAnthropicMessage:
		return anthropic.NewInboundTransformer(), nil
	default:
		return nil, fmt.Errorf("unsupported format for inbound transformation: %s", format)
	}
}

func (s *TraceService) getOutboundTransformer(format llm.APIFormat) (transformer.Outbound, error) {
	//nolint:exhaustive // TODO: add more formats.
	switch format {
	case llm.APIFormatOpenAIChatCompletion:
		config := &openai.Config{
			Type:    openai.PlatformOpenAI,
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "dummy",
		}

		return openai.NewOutboundTransformerWithConfig(config)
	case llm.APIFormatAnthropicMessage:
		config := &anthropic.Config{
			Type:    anthropic.PlatformDirect,
			BaseURL: "https://api.anthropic.com",
			APIKey:  "dummy",
		}

		return anthropic.NewOutboundTransformerWithConfig(config)
	default:
		return nil, fmt.Errorf("unsupported format for outbound transformation: %s", format)
	}
}
