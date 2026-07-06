package oddsfeed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
)

// EventBus publishes feed events to NATS.
type EventBus struct {
	conn   *nats.Conn
	logger *slog.Logger
}

// NewEventBus connects to a NATS server at the given URL.
func NewEventBus(url string, logger *slog.Logger) (*EventBus, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	return &EventBus{conn: nc, logger: logger}, nil
}

// Publish marshals the payload to JSON and publishes it to the given NATS subject.
func (b *EventBus) Publish(ctx context.Context, subject string, payload map[string]string) error {
	body, _ := json.Marshal(payload)
	if err := b.conn.Publish(subject, body); err != nil {
		return err
	}
	if b.logger != nil {
		b.logger.Debug("published feed event", slog.String("subject", subject))
	}
	return nil
}

// Close closes the NATS connection.
func (b *EventBus) Close() { b.conn.Close() }
