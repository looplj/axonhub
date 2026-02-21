package approval

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/looplj/axonhub/axon/permission/grant"
)

type Request struct {
	ID         string
	ThreadID   string
	Workspace  string
	ToolCallID string
	ToolName   string

	Capabilities []string
	Summary      string
	Reason       string
	RiskLevel    string

	// JSON-safe, redacted resources for UI display.
	Resources json.RawMessage
}

type Response struct {
	Granted bool
	Scope   grant.Scope
	Reason  string
}

// Service manages approval requests that require a UI subscriber to grant or deny.
// It supports publishing requests to subscribers and waiting for a resolution.
type Service interface {
	// Subscribe registers a subscriber (typically UI) to receive approval requests.
	// The returned channel will be closed when ctx is done.
	Subscribe(ctx context.Context) <-chan Request

	// Request publishes an approval request to subscribers and blocks until it is granted/denied,
	// or until ctx is done. It returns an error if req.ID is empty or if no subscribers exist.
	Request(ctx context.Context, req Request) (Response, error)

	// Grant resolves a pending approval request with the given scope.
	Grant(req Request, scope grant.Scope) error

	// Deny resolves a pending approval request as denied.
	Deny(req Request) error

	// Active returns the currently active request, if any.
	Active() (Request, bool)
}

type InProcessService struct {
	subsMu sync.RWMutex
	subs   []chan Request

	pendingMu sync.Mutex
	pending   map[string]chan Response

	requestMu sync.Mutex

	activeMu sync.RWMutex
	active   *Request
}

func NewInProcessService() *InProcessService {
	return &InProcessService{
		pending: make(map[string]chan Response),
	}
}

func (s *InProcessService) Subscribe(ctx context.Context) <-chan Request {
	ch := make(chan Request, 16)

	s.subsMu.Lock()
	s.subs = append(s.subs, ch)
	s.subsMu.Unlock()

	go func() {
		<-ctx.Done()
		s.subsMu.Lock()
		for i, c := range s.subs {
			if c == ch {
				s.subs = append(s.subs[:i], s.subs[i+1:]...)
				break
			}
		}
		s.subsMu.Unlock()
		close(ch)
	}()

	return ch
}

func (s *InProcessService) Request(ctx context.Context, req Request) (Response, error) {
	if req.ID == "" {
		return Response{}, errors.New("approval request ID is required")
	}

	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	s.subsMu.RLock()
	hasSubs := len(s.subs) > 0
	s.subsMu.RUnlock()
	if !hasSubs {
		return Response{}, errors.New("approval requested but no UI subscribers are available")
	}

	respCh := make(chan Response, 1)
	s.pendingMu.Lock()
	s.pending[req.ID] = respCh
	s.pendingMu.Unlock()

	s.activeMu.Lock()
	s.active = &req
	s.activeMu.Unlock()

	s.publish(req)

	select {
	case <-ctx.Done():
		s.pendingMu.Lock()
		delete(s.pending, req.ID)
		s.pendingMu.Unlock()

		s.activeMu.Lock()
		if s.active != nil && s.active.ID == req.ID {
			s.active = nil
		}
		s.activeMu.Unlock()

		return Response{}, ctx.Err()
	case resp := <-respCh:
		return resp, nil
	}
}

func (s *InProcessService) Grant(req Request, scope grant.Scope) error {
	return s.resolve(req, Response{Granted: true, Scope: scope})
}

func (s *InProcessService) Deny(req Request) error {
	return s.resolve(req, Response{Granted: false})
}

func (s *InProcessService) Active() (Request, bool) {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	if s.active == nil {
		return Request{}, false
	}
	return *s.active, true
}

func (s *InProcessService) resolve(req Request, resp Response) error {
	s.pendingMu.Lock()
	ch, ok := s.pending[req.ID]
	if ok {
		delete(s.pending, req.ID)
	}
	s.pendingMu.Unlock()
	if ok {
		ch <- resp
	}

	s.activeMu.Lock()
	if s.active != nil && s.active.ID == req.ID {
		s.active = nil
	}
	s.activeMu.Unlock()
	return nil
}

func (s *InProcessService) publish(req Request) {
	s.subsMu.RLock()
	subs := make([]chan Request, len(s.subs))
	copy(subs, s.subs)
	s.subsMu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- req:
		default:
		}
	}
}
