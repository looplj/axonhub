// 职责: 跟踪实时会话连接、配置确认、生成与关闭状态。
// 关联文件:
//
//	internal/server/orchestrator/realtime.go
//
// 注意事项:
//
//	仅 session.updated 确认配置；session.created 和 WebSocket 握手均不代表可发送媒体。

package realtime

import (
	"errors"
	"sync"
	"time"
)

type Phase string

const (
	Created    Phase = "CREATED"
	Connecting Phase = "CONNECTING"
	Configured Phase = "READY"
	Active     Phase = "ACTIVE"
	Closing    Phase = "CLOSING"
	Closed     Phase = "CLOSED"
)

var ErrUpdatePending = errors.New("session configuration update is awaiting acknowledgement")

type State struct {
	mu           sync.Mutex
	phase        Phase
	pending      bool
	seenCreated  bool
	active       int
	pendingSince time.Time
}

func NewState() *State       { return &State{phase: Created} }
func (s *State) Load() Phase { s.mu.Lock(); defer s.mu.Unlock(); return s.phase }
func (s *State) Ready() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.pending && (s.phase == Configured || s.phase == Active)
}

func (s *State) Connecting() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase != Created {
		return errors.New("invalid session transition")
	}
	s.phase = Connecting
	s.pending = true
	s.pendingSince = time.Now()
	return nil
}

func (s *State) Update() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase != Configured && s.phase != Active && (s.phase != Connecting || !s.seenCreated) {
		return errors.New("session cannot be updated before configuration acknowledgement or during generation")
	}
	if s.pending {
		return ErrUpdatePending
	}
	s.pending = true
	s.pendingSince = time.Now()
	return nil
}

func (s *State) ObserveServer(kind string, responseQueuedBeforeUpdate ...bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase == Closed || s.phase == Closing {
		return errors.New("session is closed")
	}
	switch kind {
	case "session.created":
		if s.seenCreated {
			return errors.New("duplicate upstream session")
		}
		s.seenCreated = true
	case "session.updated":
		if !s.seenCreated || !s.pending {
			return errors.New("unexpected configuration acknowledgement")
		}
		s.pending = false
		s.phase = Configured
		if s.active > 0 {
			s.phase = Active
		}
	case "response.created":
		queuedBeforeUpdate := len(responseQueuedBeforeUpdate) > 0 && responseQueuedBeforeUpdate[0]
		if (s.pending && s.active == 0 && !queuedBeforeUpdate) || (s.phase != Configured && s.phase != Active) {
			return errors.New("response before configuration or concurrent responses")
		}
		s.phase = Active
		s.active++
	case "response.done":
		if s.active > 0 {
			s.active--
		}
		if s.active == 0 && s.phase == Active {
			s.phase = Configured
		}
	}
	return nil
}

func (s *State) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = Closing
	s.pending = false
	s.phase = Closed
}

// AwaitClientConfiguration leaves adapter-configured sessions unconfigured until the client's first update.
func (s *State) AwaitClientConfiguration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase == Connecting && s.seenCreated {
		s.pending = false
	}
}

func (s *State) ConfigurationExpired(timeout time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending && !s.pendingSince.IsZero() && time.Since(s.pendingSince) > timeout
}
func (s *State) ConfigurationRejected() { s.mu.Lock(); defer s.mu.Unlock(); s.pending = false }

// ConfirmDefaultConfiguration applies only when the provider reports its full effective GA audio configuration.
func (s *State) ConfirmDefaultConfiguration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase == Connecting && s.seenCreated {
		s.pending = false
		s.phase = Configured
	}
}
