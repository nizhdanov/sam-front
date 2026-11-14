package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"sam/pkg/natsx"
	"sam/services/processor/internal/domain"
)

var backoff = []time.Duration{0, time.Second, 3 * time.Second}

type Publisher struct{ js jetstream.JetStream }

func New(js jetstream.JetStream) *Publisher { return &Publisher{js: js} }

func (p *Publisher) Publish(ctx context.Context, s domain.Snapshot, attempt int) error {
	event := natsx.SnapshotCreated{
		EventID:       uuid.NewString(),
		EventType:     natsx.SnapshotEventType,
		SchemaVersion: natsx.SnapshotSchemaVersion,
		InventoryID:   s.ID.String(),
		ImportID:      s.ImportID.String(),
		SerialNumber:  s.SerialNumber,
		CreatedAt:     time.Now().UTC(),
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	msgID := fmt.Sprintf("%s:%d", s.ID, attempt)

	var lastErr error
	for _, wait := range backoff {
		if wait > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, lastErr = p.js.Publish(pubCtx, natsx.SnapshotSubject, body, jetstream.WithMsgID(msgID))
		cancel()
		if lastErr == nil {
			return nil
		}
	}
	return fmt.Errorf("publish snapshot %s: %w", s.ID, lastErr)
}
