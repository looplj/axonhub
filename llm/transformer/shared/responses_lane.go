package shared

import (
	"encoding/json"
	"sync"
)

type ResponsesBudget struct {
	mu    sync.Mutex
	used  int
	limit int
}

func NewResponsesBudget(limit int) *ResponsesBudget { return &ResponsesBudget{limit: limit} }

func (b *ResponsesBudget) Replace(previous, next int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if next > b.limit-(b.used-previous) {
		return false
	}
	b.used += next - previous
	return true
}

type ResponsesSnapshot struct {
	Input  []json.RawMessage
	Output []json.RawMessage
}

type ResponsesRecord struct {
	ID       string
	Origin   ResponsesOrigin
	Snapshot *ResponsesSnapshot
}

type ResponsesLane struct {
	id     string
	budget *ResponsesBudget
	mu     sync.Mutex
	record ResponsesRecord
	size   int
	closed bool
	retire map[ResponsesOrigin]func()
}

func NewResponsesLane(id string, budget *ResponsesBudget) *ResponsesLane {
	return &ResponsesLane{id: id, budget: budget, retire: make(map[ResponsesOrigin]func())}
}

func (l *ResponsesLane) ID() string               { return l.id }
func (l *ResponsesLane) Budget() *ResponsesBudget { return l.budget }

func (l *ResponsesLane) Record() ResponsesRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.record
}

func (l *ResponsesLane) Remember(record ResponsesRecord) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	size := 0
	if record.Snapshot != nil {
		for _, item := range record.Snapshot.Input {
			size += len(item)
		}
		for _, item := range record.Snapshot.Output {
			size += len(item)
		}
	}
	if !l.budget.Replace(l.size, size) {
		record.Snapshot = nil
		size = 0
		l.budget.Replace(l.size, 0)
	}
	l.record, l.size = record, size
}

func (l *ResponsesLane) RegisterLease(origin ResponsesOrigin, retire func()) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return false
	}
	l.retire[origin] = retire
	return true
}

func (l *ResponsesLane) ForgetLease(origin ResponsesOrigin) {
	l.mu.Lock()
	delete(l.retire, origin)
	l.mu.Unlock()
}

func (l *ResponsesLane) RetireLeases() {
	l.mu.Lock()
	retire := l.retire
	l.retire = make(map[ResponsesOrigin]func())
	l.mu.Unlock()
	for _, closeLease := range retire {
		closeLease()
	}
}

func (l *ResponsesLane) Close() {
	l.mu.Lock()
	l.closed = true
	l.budget.Replace(l.size, 0)
	l.size = 0
	l.record = ResponsesRecord{}
	l.mu.Unlock()
	l.RetireLeases()
}
