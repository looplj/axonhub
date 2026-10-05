package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/samber/lo"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

const imageCountMetadataKey = "image_count"

// ImageStreamEvent is the wire format shared by Images generation/edit SSE.
// PartialImageIndex is a pointer so the first partial image (index zero) survives
// serialization, while completed events omit the field.
type ImageStreamEvent struct {
	Type              string               `json:"type"`
	B64JSON           string               `json:"b64_json"`
	CreatedAt         int64                `json:"created_at"`
	Background        string               `json:"background,omitempty"`
	OutputFormat      string               `json:"output_format,omitempty"`
	Quality           string               `json:"quality,omitempty"`
	Size              string               `json:"size,omitempty"`
	PartialImageIndex *int                 `json:"partial_image_index,omitempty"`
	Usage             *ImagesResponseUsage `json:"usage,omitempty"`
}

// Streaming multipart options must reject malformed integers. Non-streaming
// requests retain the existing behavior of omitting malformed optional values.
func parseOptionalImageInt64(name, value string, strict bool) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		if strict {
			return nil, fmt.Errorf("%w: invalid %s: %q", transformer.ErrInvalidRequest, name, value)
		}
		return nil, nil
	}
	return lo.ToPtr(parsed), nil
}

func validateImageStreamOptions(useStream bool, model string, n, partialImages *int64) error {
	if !useStream {
		return nil
	}
	if model == "" || model == "dall-e-2" || model == "dall-e-3" {
		return fmt.Errorf("%w: DALL-E image models do not support streaming; specify an image model that supports SSE", transformer.ErrInvalidRequest)
	}
	if n != nil && (*n < 1 || *n > 10) {
		return fmt.Errorf("%w: n must be between 1 and 10", transformer.ErrInvalidRequest)
	}
	if partialImages != nil && (*partialImages < 0 || *partialImages > 3) {
		return fmt.Errorf("%w: partial_images must be between 0 and 3", transformer.ErrInvalidRequest)
	}
	return nil
}

func imageStreamPrefix(format llm.APIFormat) string {
	if format == llm.APIFormatOpenAIImageEdit {
		return "image_edit."
	}
	return "image_generation."
}

func decodeImageStreamEvent(req *httpclient.Request, event *httpclient.StreamEvent) (*llm.Response, error) {
	// Some compatible providers flatten error fields instead of wrapping them
	// in an error object. Preserve their message before the shared fallback.
	if event != nil && event.Type == "error" && len(event.Data) > 0 {
		var detail llm.ErrorDetail
		if json.Unmarshal(event.Data, &detail) == nil && detail.Message != "" {
			if detail.Type == "" {
				detail.Type = "stream_error"
			}
			return nil, &llm.ResponseError{Detail: detail}
		}
	}
	if err := parseStreamErrorEvent(event); err != nil {
		return nil, err
	}
	if event == nil || len(event.Data) == 0 || string(event.Data) == "[DONE]" {
		return nil, nil
	}
	var data ImageStreamEvent
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return nil, fmt.Errorf("%w: invalid image stream event: %w", transformer.ErrInvalidResponse, err)
	}
	if data.Type == "" {
		data.Type = event.Type
	}
	if data.Type == "error" {
		var detail llm.ErrorDetail
		if err := json.Unmarshal(event.Data, &detail); err != nil {
			return nil, fmt.Errorf("%w: invalid image stream error: %w", transformer.ErrInvalidResponse, err)
		}
		if detail.Message == "" {
			detail.Message = "stream error"
		}
		return nil, &llm.ResponseError{Detail: detail}
	}
	prefix := imageStreamPrefix(llm.APIFormat(req.APIFormat))
	kind := strings.TrimPrefix(data.Type, prefix)
	if data.Type != prefix+kind || (kind != llm.ImageEventPartial && kind != llm.ImageEventCompleted) {
		return nil, fmt.Errorf("%w: unsupported image stream event %q", transformer.ErrInvalidResponse, data.Type)
	}
	if data.B64JSON == "" || (kind == llm.ImageEventPartial && (data.PartialImageIndex == nil || *data.PartialImageIndex < 0)) {
		return nil, fmt.Errorf("%w: missing image data or partial_image_index", transformer.ErrInvalidResponse)
	}
	// Reuse the JSON mapping without copying large base64 payloads through JSON.
	resp := imageResponseFromPayload(&ImagesResponse{
		Created: data.CreatedAt, Data: []ImageData{{B64JSON: data.B64JSON}},
		Background: data.Background, OutputFormat: data.OutputFormat,
		Quality: data.Quality, Size: data.Size, Usage: data.Usage,
	}, req)
	resp.Image.EventType = kind
	resp.Image.PartialImageIndex = data.PartialImageIndex
	return resp, nil
}

// ImageStreamProgress tracks the requested number of native Images results.
// Persistence observes raw events before the decoder, so it uses the same
// request-aware completion rule as the decoder and SSE writer.
type ImageStreamProgress struct {
	prefix    string
	expected  int64
	completed int64
}

func (t *OutboundTransformer) NewStreamEventObserver(req *httpclient.Request) transformer.StreamEventObserver {
	progress := NewImageStreamProgress(req)
	if progress == nil {
		return nil
	}
	return progress
}

func NewImageStreamProgress(req *httpclient.Request) *ImageStreamProgress {
	if req == nil || (req.APIFormat != llm.APIFormatOpenAIImageGeneration.String() && req.APIFormat != llm.APIFormatOpenAIImageEdit.String()) {
		return nil
	}
	n := int64(1)
	if count, ok := req.TransformerMetadata[imageCountMetadataKey].(int64); ok && count > 0 {
		n = count
	} else {
		body := req.Body
		if len(req.JSONBody) > 0 {
			body = req.JSONBody
		}
		// Multipart logging bodies store form values as strings.
		value := gjson.GetBytes(body, "n")
		if count, err := strconv.ParseInt(value.String(), 10, 64); err == nil && count > 0 {
			n = count
		}
	}
	return &ImageStreamProgress{prefix: imageStreamPrefix(llm.APIFormat(req.APIFormat)), expected: n}
}

func (p *ImageStreamProgress) Observe(event *httpclient.StreamEvent) *httpclient.StreamEvent {
	if p == nil || event == nil || event.ImageStreamCompleted != nil {
		// Existing metadata comes from an earlier validated Images observer,
		// such as pass-through capture before the concurrent consumers split.
		return event
	}
	// Attach request-specific completion to all events, including [DONE] and
	// unrelated protocol markers. Clone because events can be shared or global.
	annotated := *event
	annotated.ImageStreamCompleted = lo.ToPtr(false)
	typeName := gjson.GetBytes(event.Data, "type").String()
	if typeName == "" {
		typeName = event.Type
	}
	if event.Type == "error" || typeName != p.prefix+llm.ImageEventCompleted {
		return &annotated
	}
	// Validate the typed payload before persistence can mark success. A valid
	// JSON object with malformed metadata (e.g. created_at as a string) still
	// fails the decoder and must not establish completion in the raw layer.
	var image ImageStreamEvent
	if json.Unmarshal(event.Data, &image) != nil || image.B64JSON == "" || gjson.GetBytes(event.Data, "error").Exists() {
		return &annotated
	}
	annotated.ImageStreamCompleted = lo.ToPtr(p.observeCompleted(event))
	return &annotated
}

// Call only after decoding and validating a completed image. The decoder and
// aggregator already have that payload, so counting needs no second JSON decode.
func (p *ImageStreamProgress) observeCompleted(event *httpclient.StreamEvent) bool {
	p.completed++
	if event.ImageStreamCompleted != nil {
		return *event.ImageStreamCompleted
	}
	return p.completed >= p.expected
}

// imageOutboundStream requires a semantic completed event. EOF or [DONE] after
// partial images cannot be mistaken for successful generation.
type imageOutboundStream struct {
	ctx       context.Context
	req       *httpclient.Request
	source    streams.Stream[*httpclient.StreamEvent]
	current   *llm.Response
	err       error
	completed bool
	done      bool
	closed    bool
	progress  *ImageStreamProgress
}

func newImageOutboundStream(ctx context.Context, req *httpclient.Request, source streams.Stream[*httpclient.StreamEvent]) streams.Stream[*llm.Response] {
	return &imageOutboundStream{ctx: ctx, req: req, source: source, progress: NewImageStreamProgress(req)}
}

func (s *imageOutboundStream) Next() bool {
	if s.closed || s.err != nil || s.done {
		return false
	}
	if s.completed {
		s.current = llm.DoneResponse
		s.done = true
		return true
	}
	if err := s.ctx.Err(); err != nil {
		s.err = err
		return false
	}
	for s.source.Next() {
		if err := s.ctx.Err(); err != nil {
			s.err = err
			return false
		}
		raw := s.source.Current()
		resp, err := decodeImageStreamEvent(s.req, raw)
		if err != nil {
			s.err = err
			return false
		}
		if resp == nil {
			continue
		}
		s.current = resp
		s.completed = resp.Image.EventType == llm.ImageEventCompleted && s.progress.observeCompleted(raw)
		resp.Image.StreamCompleted = s.completed
		if s.completed {
			resp.Choices = []llm.Choice{{Index: 0, FinishReason: lo.ToPtr("stop")}}
		}
		return true
	}
	s.err = s.source.Err()
	if s.err == nil {
		s.err = s.ctx.Err()
	}
	if s.err == nil {
		s.err = llm.ErrStreamIncomplete
	}
	return false
}

func (s *imageOutboundStream) Current() *llm.Response { return s.current }
func (s *imageOutboundStream) Err() error             { return s.err }
func (s *imageOutboundStream) Close() error {
	s.closed = true
	return s.source.Close()
}

func (t *ImageInboundTransformer) transformImageStream(ctx context.Context, source streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	if t.apiFormat == llm.APIFormatOpenAIImageVariation {
		return nil, fmt.Errorf("%w: image variations do not support streaming", transformer.ErrInvalidRequest)
	}
	return streams.NoNil(streams.MapErr(source, func(resp *llm.Response) (*httpclient.StreamEvent, error) {
		if resp == nil || resp == llm.DoneResponse || resp.Object == "[DONE]" {
			return nil, nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		img := resp.Image
		if img == nil || len(img.Data) != 1 || img.Data[0].B64JSON == "" ||
			(img.EventType != llm.ImageEventPartial && img.EventType != llm.ImageEventCompleted) {
			return nil, fmt.Errorf("%w: upstream did not provide an Images SSE event", transformer.ErrInvalidResponse)
		}
		wire := ImageStreamEvent{
			Type:    imageStreamPrefix(t.apiFormat) + img.EventType,
			B64JSON: img.Data[0].B64JSON, CreatedAt: img.Created,
			Background: img.Background, OutputFormat: img.OutputFormat,
			Quality: img.Quality, Size: img.Size, PartialImageIndex: img.PartialImageIndex,
		}
		if img.EventType == llm.ImageEventCompleted {
			wire.PartialImageIndex = nil
			wire.Usage = imageUsageFromLLM(resp.Usage)
		}
		body, err := json.Marshal(wire)
		if err != nil {
			return nil, err
		}
		event := &httpclient.StreamEvent{Type: wire.Type, Data: body}
		if img.EventType == llm.ImageEventCompleted {
			event.ImageStreamCompleted = lo.ToPtr(img.StreamCompleted)
		}
		return event, nil
	})), nil
}

// Aggregate all final images, never the intermediate previews. This body is
// used for persisted request responses and stream-to-JSON adaptation.
func aggregateImageStream(ctx context.Context, req *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	progress := NewImageStreamProgress(req)
	var final *llm.Response
	completed := false
	for _, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return nil, llm.ResponseMeta{}, err
		}
		resp, err := decodeImageStreamEvent(req, chunk)
		if err != nil {
			return nil, llm.ResponseMeta{}, err
		}
		if resp == nil || resp.Image.EventType != llm.ImageEventCompleted {
			continue
		}
		// Inbound events already carry request-level completion from the unified
		// decoder. Raw outbound chunks instead use the original request count.
		completed = progress.observeCompleted(chunk)
		if final == nil {
			final = resp
		} else {
			final.Image.Data = append(final.Image.Data, resp.Image.Data...)
			final.Usage = addImageUsage(final.Usage, resp.Usage)
		}
		if completed {
			break
		}
	}
	if final == nil || !completed {
		return nil, llm.ResponseMeta{}, llm.ErrStreamIncomplete
	}
	response, err := (&ImageInboundTransformer{apiFormat: llm.APIFormat(req.APIFormat)}).TransformResponse(ctx, final)
	if err != nil {
		return nil, llm.ResponseMeta{}, err
	}
	return response.Body, llm.ResponseMeta{ID: final.ID, Usage: final.Usage, Completed: true}, nil
}

// Native Images completed events report usage for each generated image. Keep
// the event usage unchanged on the wire and sum it for the persisted request.
func addImageUsage(total, next *llm.Usage) *llm.Usage {
	if next == nil {
		return total
	}
	if total == nil {
		return next
	}
	total.PromptTokens += next.PromptTokens
	total.CompletionTokens += next.CompletionTokens
	total.TotalTokens += next.TotalTokens
	if next.PromptTokensDetails != nil {
		if total.PromptTokensDetails == nil {
			total.PromptTokensDetails = &llm.PromptTokensDetails{}
		}
		total.PromptTokensDetails.ImageTokens += next.PromptTokensDetails.ImageTokens
		total.PromptTokensDetails.TextTokens += next.PromptTokensDetails.TextTokens
		total.PromptTokensDetails.CachedTokens += next.PromptTokensDetails.CachedTokens
	}
	if next.CompletionTokensDetails != nil {
		if total.CompletionTokensDetails == nil {
			total.CompletionTokensDetails = &llm.CompletionTokensDetails{}
		}
		total.CompletionTokensDetails.ReasoningTokens += next.CompletionTokensDetails.ReasoningTokens
	}
	if next.Cost != nil {
		if total.Cost == nil {
			total.Cost = lo.ToPtr(*next.Cost)
		} else {
			*total.Cost += *next.Cost
		}
	}
	return total
}
