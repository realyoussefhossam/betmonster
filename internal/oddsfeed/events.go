package oddsfeed

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
)

type natsConn interface {
	Publish(subj string, data []byte) error
	Close()
}

// EventBus publishes feed events to NATS.
type EventBus struct {
	nc  natsConn
	log *log.Logger
}

// NewEventBus connects to a NATS server at the given URL.
func NewEventBus(url string, log *log.Logger) (*EventBus, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	return &EventBus{nc: nc, log: log}, nil
}

// Publish marshals the payload to JSON and publishes it to the given NATS subject.
func (b *EventBus) Publish(ctx context.Context, subject string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	if err := b.nc.Publish(subject, body); err != nil {
		return fmt.Errorf("publish event: %w", err)
	}
	if b.log != nil {
		b.log.Printf("published feed event: subject=%s", subject)
	}
	return nil
}

// Close closes the NATS connection.
func (b *EventBus) Close() { b.nc.Close() }
