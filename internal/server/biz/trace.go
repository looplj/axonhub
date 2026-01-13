package biz

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/samber/lo"
	"go.uber.org/fx"
	"golang.org/x/sync/errgroup"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/trace"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

type TraceServiceParams struct {
	fx.In

	RequestService *RequestService
	Ent            *ent.Client
}

func NewTraceService(params TraceServiceParams) *TraceService {
	return &TraceService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		requestService: params.RequestService,
	}
}

type TraceService struct {
	*AbstractService

	requestService *RequestService
}

// GetOrCreateTrace retrieves an existing trace by trace_id and project_id,
// or creates a new one if it doesn't exist.
func (s *TraceService) GetOrCreateTrace(ctx context.Context, projectID int, traceID string, threadID *int) (*ent.Trace, error) {
	client := s.entFromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	// Try to find existing trace
	existingTrace, err := client.Trace.Query().
		Where(
			trace.TraceIDEQ(traceID),
			trace.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err == nil {
		// Trace found
		return existingTrace, nil
	}

	// If error is not "not found", return the error
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to query trace: %w", err)
	}

	// Trace not found, create new one
	newTrace, err := client.Trace.Create().
		SetTraceID(traceID).
		SetProjectID(projectID).
		SetNillableThreadID(threadID).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return client.Trace.Query().
				Where(
					trace.TraceIDEQ(traceID),
					trace.ProjectIDEQ(projectID),
				).
				Only(ctx)
		}

		return nil, fmt.Errorf("failed to create trace: %w", err)
	}

	return newTrace, nil
}

// GetTraceByID retrieves a trace by its trace_id and project_id.
func (s *TraceService) GetTraceByID(ctx context.Context, traceID string, projectID int) (*ent.Trace, error) {
	client := s.entFromContext(ctx)
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

// GetThreadFirstTrace retrieves the first trace for a thread by thread ID.
func (s *TraceService) GetThreadFirstTrace(ctx context.Context, threadID int) (*ent.Trace, error) {
	client := s.entFromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	trace, err := client.Trace.Query().
		Where(trace.ThreadIDEQ(threadID)).
		Order(ent.Asc(trace.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get first trace for thread: %w", err)
	}

	return trace, nil
}

func (s *TraceService) GetFirstSegment(ctx context.Context, traceID int) (*Segment, error) {
	return s.requestService.GetTraceFirstSegment(ctx, traceID)
}

func (s *TraceService) GetFirstUserQuery(ctx context.Context, traceID int) (*string, error) {
	segment, err := s.GetFirstSegment(ctx, traceID)
	if err != nil {
		return nil, err
	}

	if segment == nil {
		return nil, nil
	}

	return segment.FirstUserQuery(), nil
}

func (s *TraceService) GetFirstText(ctx context.Context, traceID int) (*string, error) {
	segment, err := s.GetFirstSegment(ctx, traceID)
	if err != nil {
		return nil, err
	}

	if segment == nil {
		return nil, nil
	}

	return segment.FirstText(), nil
}

// Segment represents a segment in a trace.
// A trace contains multiple segments, and each segment contains multiple spans.
type Segment struct {
	ID            int              `json:"id"`
	ParentID      *int             `json:"parentId,omitempty"`
	Model         string           `json:"model"`
	Children      []*Segment       `json:"children,omitempty"`
	RequestSpans  []Span           `json:"requestSpans,omitempty"`
	ResponseSpans []Span           `json:"responseSpans,omitempty"`
	Metadata      *RequestMetadata `json:"metadata,omitempty"`
	StartTime     time.Time        `json:"startTime"`
	EndTime       time.Time        `json:"endTime"`
	Duration      int64            `json:"duration"` // Duration in milliseconds
}

func (s *Segment) FirstUserQuery() *string {
	if s == nil {
		return nil
	}

	// Search in request spans first
	for _, span := range s.RequestSpans {
		if span.Type == "user_query" && span.Value != nil && span.Value.UserQuery != nil {
			return lo.ToPtr(span.Value.UserQuery.Text)
		}
	}

	// If not found in current segment, search in children
	for _, child := range s.Children {
		if query := child.FirstUserQuery(); query != nil {
			return query
		}
	}

	return nil
}

func (s *Segment) FirstText() *string {
	if s == nil {
		return nil
	}

	// Search in request spans first
	for _, span := range s.ResponseSpans {
		if span.Type == "text" && span.Value != nil && span.Value.Text != nil {
			return lo.ToPtr(span.Value.Text.Text)
		}
	}

	// If not found in current segment, search in children
	for _, child := range s.Children {
		if text := child.FirstText(); text != nil {
			return text
		}
	}

	return nil
}

// Span represents a trace span with timing and metadata information.
type Span struct {
	ID string `json:"id"`
	// Type of the span.
	// "system_instruction": system instruction.
	// "user_query": the query from user.
	// "user_image_url": the image url from user.
	// "text": llm responsed text.
	// "thinking": llm responsed thinking.
	// "image_url": User image url
	// "tool_use": llm responsed tool use.
	// "tool_result": result of tool running.
	Type      string     `json:"type"`
	StartTime time.Time  `json:"startTime"`
	EndTime   time.Time  `json:"endTime"`
	Value     *SpanValue `json:"value,omitempty"`
}

type SpanValue struct {
	SystemInstruction *SpanSystemInstruction `json:"systemInstruction,omitempty"`
	UserQuery         *SpanUserQuery         `json:"userQuery,omitempty"`
	UserImageURL      *SpanUserImageURL      `json:"userImageUrl,omitempty"`
	Text              *SpanText              `json:"text,omitempty"`
	Thinking          *SpanThinking          `json:"thinking,omitempty"`
	ImageURL          *SpanImageURL          `json:"imageUrl,omitempty"`
	ToolUse           *SpanToolUse           `json:"toolUse,omitempty"`
	ToolResult        *SpanToolResult        `json:"toolResult,omitempty"`
}

type SpanSystemInstruction struct {
	Instruction string `json:"instruction,omitempty"`
}

type SpanUserQuery struct {
	Text string `json:"text,omitempty"`
}

type SpanUserImageURL struct {
	URL string `json:"url,omitempty"`
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

type SpanToolUse struct {
	ID        string  `json:"id,omitempty"`
	Name      string  `json:"name"`
	Arguments *string `json:"arguments,omitempty"`
}

type SpanToolResult struct {
	ToolCallID string `json:"id,omitempty"`
	IsError    bool   `json:"error,omitempty"`
	// Text or image_url
	// Type string  `json:"type,omitempty"`
	Text *string `json:"text,omitempty"`
}

// RequestMetadata contains additional metadata for a segment.
type RequestMetadata struct {
	ItemCount    *int   `json:"itemCount,omitempty"`
	InputTokens  *int64 `json:"inputTokens,omitempty"`
	OutputTokens *int64 `json:"outputTokens,omitempty"`
	TotalTokens  *int64 `json:"totalTokens,omitempty"`
	CachedTokens *int64 `json:"cachedTokens,omitempty"`
}

// GetRootSegment retrieves the hierarchical segments for a trace ID.
func (s *TraceService) GetRootSegment(ctx context.Context, traceID int) (*Segment, error) {
	client := s.entFromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	requests, err := client.Request.Query().
		Where(request.TraceIDEQ(traceID), request.StatusEQ(request.StatusCompleted)).
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

	segments := make([]*Segment, len(requests))
	// Store the original spans for each segment (including request and response) for later comparison
	var originSegmentSpans [][]Span

	for i, req := range requests {
		segments[i], err = requestToSegment(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to build segment: %w", err)
		}

		// Preserve the original spans (request + response) for subsequent comparison
		segmentSpans := segments[i].RequestSpans
		segmentSpans = append(segmentSpans, segments[i].ResponseSpans...)
		originSegmentSpans = append(originSegmentSpans, segmentSpans)

		if i == 0 {
			continue
		}

		segments[i].ParentID = &requests[i-1].ID
		segments[i-1].Children = append(segments[i-1].Children, segments[i])

		// Deduplicate requestSpans because later requests carry previous context messages as a prefix
		// Remove spans that duplicate those in the parent segment
		segments[i].RequestSpans = deduplicateSpansWithParent(segments[i].RequestSpans, originSegmentSpans[i-1])
	}

	return segments[0], nil
}

// FirstUserQuery is the resolver for the firstUserQuery field.
func (s *TraceService) FirstUserQuery(ctx context.Context, id int) (*string, error) {
	segment, err := s.GetFirstSegment(ctx, id)
	if err != nil {
		return nil, err
	}

	if segment == nil {
		return nil, nil
	}

	return segment.FirstUserQuery(), nil
}

// FirstText is the resolver for the firstText field.
func (s *TraceService) FirstText(ctx context.Context, id int) (*string, error) {
	segment, err := s.GetFirstSegment(ctx, id)
	if err != nil {
		return nil, err
	}

	if segment == nil {
		return nil, nil
	}

	return segment.FirstText(), nil
}

// requestToSegment converts a request entity to a Segment.
func requestToSegment(ctx context.Context, req *ent.Request) (*Segment, error) {
	segment := &Segment{
		ID:        req.ID,
		Model:     req.ModelID,
		StartTime: req.CreatedAt,
		EndTime:   req.UpdatedAt,
		Duration:  req.UpdatedAt.Sub(req.CreatedAt).Milliseconds(),
	}

	var (
		requestSpans  []Span
		responseSpans []Span
	)

	if len(req.RequestBody) > 0 {
		httpReq := &httpclient.Request{
			Body: req.RequestBody,
			// Ensure the gemini path format.
			Path: fmt.Sprintf("%s:generateContent", req.ModelID),
			Headers: map[string][]string{
				"Content-Type": {"application/json"},
			},
			TransformerMetadata: map[string]any{},
		}

		inbound, err := getInboundTransformer(llm.APIFormat(req.Format))
		if err != nil {
			return nil, fmt.Errorf("failed to get inbound transformer: %w", err)
		}

		llmReq, err := inbound.TransformRequest(ctx, httpReq)
		if err != nil {
			return nil, fmt.Errorf("failed to transform request body: %w", err)
		}

		requestSpans = append(requestSpans, extractSpansFromMessages(llmReq.Messages, fmt.Sprintf("request-%d", req.ID))...)
	}

	if len(req.ResponseBody) > 0 {
		outbound, err := getOutboundTransformer(llm.APIFormat(req.Format))
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

		segment.Metadata = extractMetadataFromResponse(unifiedResp)
		if len(unifiedResp.Choices) > 0 && unifiedResp.Choices[0].Message != nil {
			responseSpans = append(responseSpans, extractSpansFromMessage(unifiedResp.Choices[0].Message, fmt.Sprintf("response-%d", req.ID))...)
		}
	}

	segment.RequestSpans = requestSpans
	segment.ResponseSpans = responseSpans

	return segment, nil
}

func extractSpansFromMessages(messages []llm.Message, idPrefix string) []Span {
	var spans []Span

	for i, msg := range messages {
		msgSpans := extractSpansFromMessage(&msg, fmt.Sprintf("%s-%d", idPrefix, i))
		spans = append(spans, msgSpans...)
	}

	return spans
}

// extractSpansFromMessage converts a single message to spans.
func extractSpansFromMessage(msg *llm.Message, idPrefix string) []Span {
	var spans []Span

	now := time.Now()

	// Handle reasoning content
	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		spans = append(spans, Span{
			ID:        fmt.Sprintf("%s-reasoning-%d", idPrefix, len(spans)),
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
		switch msg.Role {
		case "system":
			spans = append(spans, Span{
				ID:        fmt.Sprintf("%s-system_instruction-%d", idPrefix, len(spans)),
				Type:      "system_instruction",
				StartTime: now,
				EndTime:   now,
				Value: &SpanValue{
					SystemInstruction: &SpanSystemInstruction{
						Instruction: *msg.Content.Content,
					},
				},
			})
		case "user":
			spans = append(spans, Span{
				ID:        fmt.Sprintf("%s-text-%d", idPrefix, len(spans)),
				Type:      "user_query",
				StartTime: now,
				EndTime:   now,
				Value: &SpanValue{
					UserQuery: &SpanUserQuery{
						Text: *msg.Content.Content,
					},
				},
			})
		case "tool":
			spans = append(spans, Span{
				ID:        fmt.Sprintf("%s-text-%d", idPrefix, len(spans)),
				Type:      "tool_result",
				StartTime: now,
				EndTime:   now,
				Value: &SpanValue{
					ToolResult: &SpanToolResult{
						Text: msg.Content.Content,
					},
				},
			})
		default:
			spans = append(spans, Span{
				ID:        fmt.Sprintf("%s-text-%d", idPrefix, len(spans)),
				Type:      "text",
				StartTime: now,
				EndTime:   now,
				Value: &SpanValue{
					Text: &SpanText{
						Text: *msg.Content.Content,
					},
				},
			})
		}
	}

	// Handle multiple content parts
	for _, part := range msg.Content.MultipleContent {
		switch part.Type {
		case "text":
			if part.Text == nil {
				continue
			}

			switch msg.Role {
			case "system":
				spans = append(spans, Span{
					ID:        fmt.Sprintf("%s-system_instruction-%d", idPrefix, len(spans)),
					Type:      "system_instruction",
					StartTime: now,
					EndTime:   now,
					Value: &SpanValue{
						SystemInstruction: &SpanSystemInstruction{
							Instruction: *part.Text,
						},
					},
				})
			case "user":
				spans = append(spans, Span{
					ID:        fmt.Sprintf("%s-text-%d", idPrefix, len(spans)),
					Type:      "user_query",
					StartTime: now,
					EndTime:   now,
					Value: &SpanValue{
						UserQuery: &SpanUserQuery{
							Text: *part.Text,
						},
					},
				})
			case "tool":
				spans = append(spans, Span{
					ID:        fmt.Sprintf("%s-tool_result-%d", idPrefix, len(spans)),
					Type:      "tool_result",
					StartTime: now,
					EndTime:   now,
					Value: &SpanValue{
						ToolResult: &SpanToolResult{
							Text: part.Text,
						},
					},
				})
			default:
				spans = append(spans, Span{
					ID:        fmt.Sprintf("%s-text-%d", idPrefix, len(spans)),
					Type:      "text",
					StartTime: now,
					EndTime:   now,
					Value: &SpanValue{
						Text: &SpanText{
							Text: *part.Text,
						},
					},
				})
			}
		case "image_url":
			if part.ImageURL == nil {
				continue
			}

			switch msg.Role {
			case "user":
				spans = append(spans, Span{
					ID:        fmt.Sprintf("%s-image_url-%d", idPrefix, len(spans)),
					Type:      "user_image_url",
					StartTime: now,
					EndTime:   now,
					Value: &SpanValue{
						UserImageURL: &SpanUserImageURL{
							URL: part.ImageURL.URL,
						},
					},
				})
			case "tool":
				spans = append(spans, Span{
					ID:        fmt.Sprintf("%s-image_url-%d", idPrefix, len(spans)),
					Type:      "tool_result",
					StartTime: now,
					EndTime:   now,
					Value: &SpanValue{
						ToolResult: &SpanToolResult{
							// Image URL in tool result - store as output
							Text: &part.ImageURL.URL,
						},
					},
				})
			default:
				spans = append(spans, Span{
					ID:        fmt.Sprintf("%s-image_url-%d", idPrefix, len(spans)),
					Type:      "image_url",
					StartTime: now,
					EndTime:   now,
					Value: &SpanValue{
						ImageURL: &SpanImageURL{
							URL: part.ImageURL.URL,
						},
					},
				})
			}

		default:
			// ignore for now.
		}
	}

	// Handle tool calls
	for _, toolCall := range msg.ToolCalls {
		args := toolCall.Function.Arguments
		toolSpan := Span{
			ID:        fmt.Sprintf("%s-tool-%d", idPrefix, len(spans)),
			Type:      "tool_use",
			StartTime: now,
			EndTime:   now,
			Value: &SpanValue{
				ToolUse: &SpanToolUse{
					ID:        toolCall.ID,
					Name:      toolCall.Function.Name,
					Arguments: &args,
				},
			},
		}
		spans = append(spans, toolSpan)
	}

	return spans
}

// getInboundTransformer returns the appropriate inbound transformer based on format.
func getInboundTransformer(format llm.APIFormat) (transformer.Inbound, error) {
	//nolint:exhaustive // Checked
	switch format {
	case llm.APIFormatOpenAIChatCompletion:
		return openai.NewInboundTransformer(), nil
	case llm.APIFormatOpenAIResponse:
		return responses.NewInboundTransformer(), nil
	case llm.APIFormatAnthropicMessage:
		return anthropic.NewInboundTransformer(), nil
	case llm.APIFormatGeminiContents:
		return gemini.NewInboundTransformer(), nil
	default:
		return nil, fmt.Errorf("unsupported format for inbound transformation: %s", format)
	}
}

func getOutboundTransformer(format llm.APIFormat) (transformer.Outbound, error) {
	//nolint:exhaustive // Checked
	switch format {
	case llm.APIFormatOpenAIChatCompletion:
		config := &openai.Config{
			PlatformType: openai.PlatformOpenAI,
			BaseURL:      "https://api.openai.com/v1",
			APIKey:       "dummy",
		}

		return openai.NewOutboundTransformerWithConfig(config)
	case llm.APIFormatOpenAIResponse:
		config := &openai.Config{
			PlatformType: openai.PlatformOpenAI,
			BaseURL:      "https://api.openai.com/v1",
			APIKey:       "dummy",
		}

		return responses.NewOutboundTransformer(config.BaseURL, config.APIKey)
	case llm.APIFormatAnthropicMessage:
		config := &anthropic.Config{
			Type:    anthropic.PlatformDirect,
			BaseURL: "https://api.anthropic.com",
			APIKey:  "dummy",
		}

		return anthropic.NewOutboundTransformerWithConfig(config)
	case llm.APIFormatGeminiContents:
		config := gemini.Config{
			BaseURL: "https://generativelanguage.googleapis.com",
			APIKey:  "dummy",
		}

		return gemini.NewOutboundTransformerWithConfig(config)
	default:
		return nil, fmt.Errorf("unsupported format for outbound transformation: %s", format)
	}
}

// extractMetadataFromResponse extracts metadata from the unified response.
func extractMetadataFromResponse(resp *llm.Response) *RequestMetadata {
	if resp == nil || resp.Usage == nil {
		return nil
	}

	usage := resp.Usage
	metadata := &RequestMetadata{
		TotalTokens: &usage.TotalTokens,
	}

	if usage.PromptTokens > 0 {
		metadata.InputTokens = &usage.PromptTokens
	}

	if usage.CompletionTokens > 0 {
		metadata.OutputTokens = &usage.CompletionTokens
	}

	if usage.PromptTokensDetails != nil && usage.PromptTokensDetails.CachedTokens > 0 {
		metadata.CachedTokens = &usage.PromptTokensDetails.CachedTokens
	}

	return metadata
}

// deduplicateSpansWithParent removes spans from current that already exist in parent.
// This is needed because subsequent requests in a trace carry previous context messages as prefix.
func deduplicateSpansWithParent(current, parent []Span) []Span {
	if len(current) == 0 {
		return current
	}

	if len(parent) == 0 {
		return current
	}

	capacity := max(len(current)-len(parent), 0)

	result := make([]Span, 0, capacity)

	for i, span := range current {
		if i >= len(parent) {
			result = append(result, span)
			continue
		}

		currentKey := spanToKey(span)

		parentKey := spanToKey(parent[i])
		if currentKey == parentKey {
			continue
		}

		result = append(result, span)
	}

	return result
}

// spanToKey generates a unique key for a span based on its content.
func spanToKey(span Span) string {
	if span.Value == nil {
		return fmt.Sprintf("%s:", span.Type)
	}

	switch span.Type {
	case "user_query":
		if span.Value.UserQuery != nil {
			return fmt.Sprintf("%s:%s", span.Type, span.Value.UserQuery.Text)
		}
	case "user_image_url":
		if span.Value.UserImageURL != nil {
			return fmt.Sprintf("%s:%s", span.Type, span.Value.UserImageURL.URL)
		}
	case "text":
		if span.Value.Text != nil {
			return fmt.Sprintf("%s:%s", span.Type, span.Value.Text.Text)
		}
	case "thinking":
		if span.Value.Thinking != nil {
			return fmt.Sprintf("%s:%s", span.Type, span.Value.Thinking.Thinking)
		}
	case "image_url":
		if span.Value.ImageURL != nil {
			return fmt.Sprintf("%s:%s", span.Type, span.Value.ImageURL.URL)
		}
	case "tool_use":
		if span.Value.ToolUse != nil {
			args := ""
			if span.Value.ToolUse.Arguments != nil {
				args = *span.Value.ToolUse.Arguments
			}

			return fmt.Sprintf("%s:%s:%s:%s", span.Type, span.Value.ToolUse.ID, span.Value.ToolUse.Name, args)
		}
	case "tool_result":
		if span.Value.ToolResult != nil {
			output := ""
			if span.Value.ToolResult.Text != nil {
				output = *span.Value.ToolResult.Text
			}

			return fmt.Sprintf("%s:%s:%v:%s", span.Type, span.Value.ToolResult.ToolCallID, span.Value.ToolResult.IsError, output)
		}
	}

	return fmt.Sprintf("%s:", span.Type)
}
