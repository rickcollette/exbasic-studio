package operations

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu     sync.Mutex
	events []Event
	done   chan Event
}

func newRecorder() *recorder { return &recorder{done: make(chan Event, 8)} }

func (r *recorder) sink(_ string, data any) {
	event := data.(Event)
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
	if event.Type == "completed" {
		r.done <- event
	}
}

func (r *recorder) wait(t *testing.T) Event {
	t.Helper()
	select {
	case event := <-r.done:
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("operation did not complete")
		return Event{}
	}
}

func TestOperationStreamsOrderedEventsAndCompletesOnce(t *testing.T) {
	rec := newRecorder()
	m := NewManager(rec.sink)
	info, err := m.Start("build", true, func(_ context.Context, op *Operation) (any, error) {
		op.Output("stdout", "one")
		op.Output("stdout", "")
		op.Output("stderr", "two")
		return "result", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	completed := rec.wait(t)
	if completed.Result != "result" || completed.Error != "" || completed.Cancelled {
		t.Fatalf("unexpected completion: %+v", completed)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.events) != 4 {
		t.Fatalf("got %d events, want started+2 outputs+completed: %+v", len(rec.events), rec.events)
	}
	for index, event := range rec.events {
		if event.ID != info.ID || event.Seq != uint64(index+1) {
			t.Fatalf("event %d has id=%q seq=%d", index, event.ID, event.Seq)
		}
	}
	if len(m.Running()) != 0 {
		t.Fatal("completed operation is still listed as running")
	}
}

func TestCancelStopsTaskAndReportsCancellation(t *testing.T) {
	rec := newRecorder()
	m := NewManager(rec.sink)
	info, err := m.Start("build", true, func(ctx context.Context, _ *Operation) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Start("build", true, func(context.Context, *Operation) (any, error) { return nil, nil }); err == nil {
		t.Fatal("a second exclusive operation of the same kind was accepted")
	}
	if !m.Cancel(info.ID) {
		t.Fatal("Cancel reported an unknown operation")
	}
	completed := rec.wait(t)
	if !completed.Cancelled || completed.Error == "" {
		t.Fatalf("unexpected completion: %+v", completed)
	}
	if m.Cancel(info.ID) {
		t.Fatal("cancelling a finished operation reported success")
	}
}

func TestPanicBecomesErrorAndShutdownWaits(t *testing.T) {
	rec := newRecorder()
	m := NewManager(rec.sink)
	if _, err := m.Start("x", false, func(context.Context, *Operation) (any, error) { panic("boom") }); err != nil {
		t.Fatal(err)
	}
	if completed := rec.wait(t); completed.Error == "" {
		t.Fatal("panic was not reported")
	}
	finished := false
	if _, err := m.Start("y", false, func(ctx context.Context, _ *Operation) (any, error) {
		<-ctx.Done()
		time.Sleep(20 * time.Millisecond)
		finished = true
		return nil, errors.New("stopped")
	}); err != nil {
		t.Fatal(err)
	}
	m.Shutdown()
	if !finished {
		t.Fatal("Shutdown returned before the task finished")
	}
	if _, err := m.Start("z", false, func(context.Context, *Operation) (any, error) { return nil, nil }); err == nil {
		t.Fatal("operation started after shutdown")
	}
}
