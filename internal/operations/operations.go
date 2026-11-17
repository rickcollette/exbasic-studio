// Package operations runs long-lived backend work (builds, runs, Git network
// commands) with a context, an identifier, streamed output and exactly one
// completion event.
package operations

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// EventName is the single frontend event channel used for operation updates.
const EventName = "operation"

// Sink receives events for delivery to the frontend. Implementations must be
// safe for concurrent use.
type Sink func(name string, data any)

// Event is one ordered update about an operation. Seq increases by one for
// every event of the same operation, so the frontend can discard duplicates
// and detect gaps.
type Event struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Seq       uint64 `json:"seq"`
	Type      string `json:"type"` // started | output | completed
	Stream    string `json:"stream,omitempty"`
	Text      string `json:"text,omitempty"`
	Result    any    `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
}

// Info identifies a started operation.
type Info struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

// Operation is handed to the running task so it can stream output.
type Operation struct {
	Info
	manager *Manager
	seq     atomic.Uint64
	cancel  context.CancelFunc
	done    chan struct{}
}

// Output streams a chunk of text to the frontend.
func (o *Operation) Output(stream, text string) {
	if text == "" {
		return
	}
	o.emit(Event{Type: "output", Stream: stream, Text: text})
}

func (o *Operation) emit(event Event) {
	event.ID, event.Kind, event.Seq = o.ID, o.Kind, o.seq.Add(1)
	if sink := o.manager.sink; sink != nil {
		sink(EventName, event)
	}
}

// Task is the body of an operation. Its result is delivered with the
// completion event; a context cancellation is reported as Cancelled.
type Task func(ctx context.Context, op *Operation) (any, error)

// Manager tracks running operations.
type Manager struct {
	sink   Sink
	mu     sync.Mutex
	next   uint64
	active map[string]*Operation
	closed bool
}

func NewManager(sink Sink) *Manager {
	return &Manager{sink: sink, active: make(map[string]*Operation)}
}

// SetSink replaces the event sink. It is called once the Wails application
// exists; operations cannot start before that.
func (m *Manager) SetSink(sink Sink) {
	m.mu.Lock()
	m.sink = sink
	m.mu.Unlock()
}

// Start launches a task in its own goroutine and returns immediately.
// With exclusive set, a second operation of the same kind is refused while
// the first is still running.
func (m *Manager) Start(kind string, exclusive bool, task Task) (Info, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Info{}, errors.New("the application is shutting down")
	}
	if exclusive {
		for _, running := range m.active {
			if running.Kind == kind {
				m.mu.Unlock()
				return Info{}, fmt.Errorf("a %s operation is already running", kind)
			}
		}
	}
	m.next++
	ctx, cancel := context.WithCancel(context.Background())
	op := &Operation{Info: Info{ID: fmt.Sprintf("op-%d", m.next), Kind: kind}, manager: m, cancel: cancel, done: make(chan struct{})}
	m.active[op.ID] = op
	m.mu.Unlock()

	op.emit(Event{Type: "started"})
	go func() {
		defer close(op.done)
		defer cancel()
		var result any
		var err error
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					err = fmt.Errorf("internal error: %v", recovered)
				}
			}()
			result, err = task(ctx, op)
		}()
		completed := Event{Type: "completed", Result: result}
		if ctx.Err() != nil {
			completed.Cancelled = true
		}
		if err != nil {
			completed.Error = err.Error()
		}
		m.mu.Lock()
		delete(m.active, op.ID)
		m.mu.Unlock()
		op.emit(completed)
	}()
	return op.Info, nil
}

// Cancel requests cancellation. Cancelling an unknown or finished operation
// is not an error: the completion event has already been (or will be) sent.
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	op := m.active[id]
	m.mu.Unlock()
	if op == nil {
		return false
	}
	op.cancel()
	return true
}

// Running lists operations that have not completed.
func (m *Manager) Running() []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Info, 0, len(m.active))
	for _, op := range m.active {
		out = append(out, op.Info)
	}
	return out
}

// CancelAll cancels everything and waits for the tasks to return. New
// operations may be started afterwards.
func (m *Manager) CancelAll() { m.stop(false) }

// Shutdown cancels everything, waits, and refuses further operations.
func (m *Manager) Shutdown() { m.stop(true) }

func (m *Manager) stop(final bool) {
	m.mu.Lock()
	if final {
		m.closed = true
	}
	pending := make([]*Operation, 0, len(m.active))
	for _, op := range m.active {
		pending = append(pending, op)
	}
	m.mu.Unlock()
	for _, op := range pending {
		op.cancel()
	}
	for _, op := range pending {
		<-op.done
	}
}
