package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/chunkbuffer"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type RequestPreviewHandlersParams struct {
	fx.In

	RequestService     *biz.RequestService
	LiveStreamRegistry *biz.LiveStreamRegistry
}

type RequestPreviewHandlers struct {
	RequestService     *biz.RequestService
	LiveStreamRegistry *biz.LiveStreamRegistry
	StreamWriter       StreamWriter
}

type RequestPreviewFallbackResponse struct {
	Mode           string                   `json:"mode"`
	ResponseChunks []objects.JSONRawMessage `json:"responseChunks"`
}

type RequestDetailPreviewContract struct {
	SingleInstanceOnly                           bool
	SupportsDistributedReplay                    bool
	AllowsDatabaseSchemaChanges                  bool
	ExecutionLevelPreview                        bool
	EventOrder                                   []string
	Scope                                        string
	ReuseInMemoryChunkBuffer                     bool
	FinalBatchPersistenceUnchanged               bool
	FallbackMode                                 string
	FallbackBehavior                             string
	FallbackUsesExecutionPreview                 bool
	FallbackStartsSecondaryLivePollingLoop       bool
	EndpointPath                                 string
	ContentType                                  string
	EventTypes                                   []string
	ReplayOmitsTerminalDoneEvent                 bool
	IncrementalOmitsTerminalDoneEvent            bool
	ConnectAfterCompletionFallsBackToStaticFetch bool
}

func RequestDetailSSEContract() RequestDetailPreviewContract {
	return RequestDetailPreviewContract{
		SingleInstanceOnly:                           true,
		SupportsDistributedReplay:                    false,
		AllowsDatabaseSchemaChanges:                  false,
		ExecutionLevelPreview:                        false,
		EventOrder:                                   []string{"replay", "incremental"},
		Scope:                                        "request",
		ReuseInMemoryChunkBuffer:                     true,
		FinalBatchPersistenceUnchanged:               true,
		FallbackMode:                                 "static-fetch",
		FallbackBehavior:                             "load persisted request detail once when SSE cannot connect",
		FallbackUsesExecutionPreview:                 false,
		FallbackStartsSecondaryLivePollingLoop:       false,
		EndpointPath:                                 "/admin/requests/:request_id/preview",
		ContentType:                                  "text/event-stream",
		EventTypes:                                   []string{"preview.replay", "preview.chunk", "preview.completed"},
		ReplayOmitsTerminalDoneEvent:                 true,
		IncrementalOmitsTerminalDoneEvent:            true,
		ConnectAfterCompletionFallsBackToStaticFetch: true,
	}
}

func NewRequestPreviewHandlers(params RequestPreviewHandlersParams) *RequestPreviewHandlers {
	return &RequestPreviewHandlers{
		RequestService:     params.RequestService,
		LiveStreamRegistry: params.LiveStreamRegistry,
		StreamWriter:       WriteSSEStream,
	}
}

func (h *RequestPreviewHandlers) PreviewRequest(c *gin.Context) {
	ctx := c.Request.Context()

	projectID, ok := contexts.GetProjectID(ctx)
	if !ok || projectID <= 0 {
		JSONError(c, http.StatusBadRequest, errors.New("Project ID not found in context"))
		return
	}

	var uri DownloadContentRequest
	if err := c.ShouldBindUri(&uri); err != nil {
		JSONError(c, http.StatusBadRequest, fmt.Errorf("Invalid request body: %w", err))
		return
	}

	req, err := ent.FromContext(ctx).Request.Get(ctx, uri.RequestID)
	if err != nil {
		if ent.IsNotFound(err) {
			JSONError(c, http.StatusNotFound, errors.New("Request not found"))
			return
		}
		JSONError(c, http.StatusInternalServerError, errors.New("Failed to load request"))
		return
	}

	if req.ProjectID != projectID {
		JSONError(c, http.StatusNotFound, errors.New("Request not found"))
		return
	}

	if req.Status != request.StatusProcessing || !req.Stream {
		h.writeStaticPreview(c, req)
		return
	}

	buffer := h.LiveStreamRegistry.GetRequestBuffer(req.ID)
	if buffer == nil {
		h.writeStaticPreview(c, req)
		return
	}

	stream := newRequestPreviewStream(ctx, buffer)
	defer func() { _ = stream.Close() }()

	streamWriter := h.StreamWriter
	if streamWriter == nil {
		streamWriter = WriteSSEStream
	}
	streamWriter(c, stream)
}

func (h *RequestPreviewHandlers) writeStaticPreview(c *gin.Context, req *ent.Request) {
	chunks := req.ResponseChunks
	if len(chunks) == 0 {
		loadedChunks, err := h.RequestService.LoadResponseChunks(c.Request.Context(), req)
		if err != nil {
			JSONError(c, http.StatusInternalServerError, errors.New("Failed to load request preview"))
			return
		}
		chunks = loadedChunks
	}

	c.JSON(http.StatusOK, RequestPreviewFallbackResponse{
		Mode:           "static-fetch",
		ResponseChunks: chunks,
	})
}

type requestPreviewStream struct {
	done        <-chan struct{}
	buffer      *chunkbuffer.Buffer
	notifyCh    <-chan struct{}
	unsubscribe func()
	index       int
	replayUntil int
	completed   bool
	current     *httpclient.StreamEvent
}

var _ streams.Stream[*httpclient.StreamEvent] = (*requestPreviewStream)(nil)

func newRequestPreviewStream(ctx context.Context, buffer *chunkbuffer.Buffer) *requestPreviewStream {
	notifyCh, replayUntil, unsubscribe := buffer.SubscribeFromCurrent()
	return &requestPreviewStream{
		done:        ctx.Done(),
		buffer:      buffer,
		notifyCh:    notifyCh,
		unsubscribe: unsubscribe,
		replayUntil: replayUntil,
	}
}

const previewIdleTimeout = 3 * time.Minute

func (s *requestPreviewStream) Next() bool {
	for {
		bufferClosed := false

		for {
			chunkIndex := s.index

			chunk, nextIndex, closed, ok := s.buffer.Read(s.index)
			bufferClosed = closed

			if !ok {
				break
			}

			s.index = nextIndex

			if chunk == nil || isPreviewTerminalChunk(chunk) {
				continue
			}

			eventType := "preview.chunk"
			if chunkIndex < s.replayUntil {
				eventType = "preview.replay"
			}

			s.current = &httpclient.StreamEvent{
				Type: eventType,
				Data: json.RawMessage(chunk.Data),
			}

			return true
		}

		if bufferClosed && !s.completed {
			s.completed = true
			s.current = &httpclient.StreamEvent{
				Type: "preview.completed",
				Data: previewCompletedEventData,
			}

			return true
		}

		if s.completed {
			s.current = nil
			return false
		}

		idleTimer := time.NewTimer(previewIdleTimeout)
		select {
		case <-s.done:
			idleTimer.Stop()
			s.current = nil
			return false
		case <-s.notifyCh:
			idleTimer.Stop()
		case <-idleTimer.C:
			log.Warn(context.Background(), "request preview stream idle timeout", log.Duration("timeout", previewIdleTimeout))
			s.current = nil
			return false
		}
	}
}

func (s *requestPreviewStream) Current() *httpclient.StreamEvent {
	return s.current
}

func (s *requestPreviewStream) Err() error {
	return nil
}

func (s *requestPreviewStream) Close() error {
	if s.unsubscribe != nil {
		s.unsubscribe()
		s.unsubscribe = nil
	}
	return nil
}

var previewCompletedEventData = mustMarshalPreviewEventData(gin.H{"status": "completed"})

func mustMarshalPreviewEventData(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func isPreviewTerminalChunk(chunk *httpclient.StreamEvent) bool {
	return chunk != nil && bytes.Equal(chunk.Data, llm.DoneStreamEvent.Data)
}
