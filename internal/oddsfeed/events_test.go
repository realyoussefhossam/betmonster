package oddsfeed

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"testing"
)

type mockNatsConn struct {
	published  []struct {
		subj string
		data []byte
	}
	closed     bool
	publishErr error
}

func (m *mockNatsConn) Publish(subj string, data []byte) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.published = append(m.published, struct {
		subj string
		data []byte
	}{subj: subj, data: data})
	return nil
}

func (m *mockNatsConn) Close() { m.closed = true }

func TestEventBusPublish(t *testing.T) {
	mock := &mockNatsConn{}
	bus := &EventBus{nc: mock, log: log.New(io.Discard, "", 0)}
	payload := map[string]string{"id": "event-123"}
	if err := bus.Publish(context.Background(), "feed.event.updated", payload); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(mock.published) != 1 {
		t.Fatalf("expected 1 publish, got %d", len(mock.published))
	}
	if mock.published[0].subj != "feed.event.updated" {
		t.Fatalf("expected subject feed.event.updated, got %s", mock.published[0].subj)
	}
	var got map[string]string
	if err := json.Unmarshal(mock.published[0].data, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got["id"] != "event-123" {
		t.Fatalf("expected payload id event-123, got %s", got["id"])
	}
}

func TestEventBusPublishReturnsUnderlyingError(t *testing.T) {
	mock := &mockNatsConn{publishErr: errors.New("nats down")}
	bus := &EventBus{nc: mock, log: log.New(io.Discard, "", 0)}
	if err := bus.Publish(context.Background(), "feed.event.updated", map[string]string{"id": "x"}); err == nil {
		t.Fatalf("expected publish error")
	}
}

func TestEventBusClose(t *testing.T) {
	mock := &mockNatsConn{}
	bus := &EventBus{nc: mock, log: log.New(io.Discard, "", 0)}
	bus.Close()
	if !mock.closed {
		t.Fatalf("expected close to be called")
	}
}
