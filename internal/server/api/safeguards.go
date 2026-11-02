package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/looplj/axonhub/internal/server/safeguards"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// Remove only the locally handled safeguard so pass-through channels do not
// independently review the same tool calls. Keep other provider extensions.
func prepareSafeguardsRequest(request *httpclient.Request, config safeguards.Config, call safeguards.ClassifierFunc) (*safeguards.Evaluator, *httpclient.Request, error) {
	evaluator, err := safeguards.New(request.Body, config, call)
	if err != nil || evaluator == nil {
		return evaluator, request, err
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(request.Body, &body); err != nil {
		return nil, request, err
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(body["safeguards"], &entries); err != nil {
		return nil, request, err
	}
	remaining := make([]json.RawMessage, 0, len(entries))
	for _, entry := range entries {
		var kind struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(entry, &kind); err != nil {
			return nil, request, err
		}
		if kind.Type != "dangerous_tool_use" {
			remaining = append(remaining, entry)
		}
	}
	if len(remaining) == 0 {
		delete(body, "safeguards")
	} else {
		body["safeguards"], _ = json.Marshal(remaining)
	}
	prepared := *request
	prepared.Body, err = json.Marshal(body)
	// The body changed; do not forward an obsolete content length.
	prepared.Headers = request.Headers.Clone()
	prepared.Headers.Del("Content-Length")
	return evaluator, &prepared, err
}

type safeguardsStream struct {
	streams.Stream[*httpclient.StreamEvent]

	ctx           context.Context
	reviewer      *safeguards.Stream
	current       *httpclient.StreamEvent
	err           error
	pending       []*httpclient.StreamEvent
	exhausted     bool
	abortUpstream context.CancelFunc
}

// wrapSafeguardsStream adds per-request review state while preserving downstream
// delivery acknowledgement. A nil evaluator leaves the source unchanged.
func wrapSafeguardsStream(ctx context.Context, source streams.Stream[*httpclient.StreamEvent], evaluator *safeguards.Evaluator, abortUpstream context.CancelFunc) streams.Stream[*httpclient.StreamEvent] {
	if evaluator == nil {
		return source
	}
	return preserveStreamDelivery(&safeguardsStream{Stream: source, ctx: ctx, reviewer: safeguards.NewStream(evaluator), abortUpstream: abortUpstream}, source)
}

// Next withholds the terminal delta until the upstream has completed and released
// its capacity, then reviews it before replaying the buffered terminal events.
// Tail validation or review-observation errors stop delivery.
func (s *safeguardsStream) Next() bool {
	if s.err != nil {
		return false
	}
	var event *httpclient.StreamEvent
	if len(s.pending) > 0 {
		event, s.pending = s.pending[0], s.pending[1:]
	} else {
		if s.exhausted || !s.Stream.Next() {
			return false
		}
		event = s.Stream.Current()
	}
	if event == nil || len(event.Data) == 0 {
		s.current = event
		return true
	}
	var envelope struct {
		Type  string `json:"type"`
		Delta struct {
			StopReason string `json:"stop_reason"`
		} `json:"delta"`
	}
	if json.Unmarshal(event.Data, &envelope) == nil && envelope.Type == "message_delta" && envelope.Delta.StopReason != "" && !s.exhausted {
		// Finish consuming the upstream before invoking the reviewer. Otherwise
		// a reviewer sharing a channel with concurrency=1 waits on its own main
		// request's slot. Delivery acknowledgement still happens only after the
		// annotated terminal event has actually been sent to the client.
		copy := *event
		copy.Data = append([]byte(nil), event.Data...)
		event = &copy
		if err := s.drainTerminalTail(); err != nil {
			s.err = err
			return false
		}
	}
	data, err := s.reviewer.Observe(s.ctx, event.Data)
	if err != nil {
		s.err = err
		return false
	}
	copy := *event
	copy.Data = data
	copy.Size = len(data)
	s.current = &copy
	return true
}

// drainTerminalTail buffers a bounded tail and requires message_stop before a
// clean upstream EOF. Copies protect buffered events from reused provider buffers;
// the optional abort callback bounds a stalled tail to one second.
func (s *safeguardsStream) drainTerminalTail() error {
	// Bound incomplete terminal tails. Completed Anthropic transports close at
	// message_stop before capture fan-out, without canceling downstream delivery.
	if s.abortUpstream != nil {
		timer := time.AfterFunc(time.Second, s.abortUpstream)
		defer timer.Stop()
	}
	bytes, stopped := 0, false
	for s.Stream.Next() {
		event := s.Stream.Current()
		if event == nil {
			continue
		}
		bytes += len(event.Data)
		if len(s.pending) >= 256 || bytes > 1<<20 {
			return fmt.Errorf("safeguards terminal stream tail exceeds limit")
		}
		var envelope struct {
			Type string `json:"type"`
		}
		if len(event.Data) != 0 {
			if err := json.Unmarshal(event.Data, &envelope); err != nil {
				return err
			}
			switch envelope.Type {
			case "message_stop":
				stopped = true
			case "ping":
			default:
				return fmt.Errorf("unexpected safeguards terminal stream event %q", envelope.Type)
			}
		}
		copy := *event
		copy.Data = append([]byte(nil), event.Data...)
		s.pending = append(s.pending, &copy)
	}
	s.exhausted = true
	if err := s.Stream.Err(); err != nil {
		return err
	}
	if !stopped {
		return fmt.Errorf("safeguards stream ended without message_stop")
	}
	return nil
}

func (s *safeguardsStream) Current() *httpclient.StreamEvent { return s.current }

// Err prioritizes local review or terminal-tail failures over the source error
// so consumers do not mistake a rejected response for successful completion.
func (s *safeguardsStream) Err() error {
	if s.err != nil {
		return s.err
	}
	return s.Stream.Err()
}
