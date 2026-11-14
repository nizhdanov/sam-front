package events

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"sam/pkg/natsx"
	"sam/services/worker/internal/service"
)

type Config struct {
	Concurrency   int           `env:"CONCURRENCY" envDefault:"2"`
	MaxAckPending int           `env:"MAX_ACK_PENDING" envDefault:"8"`
	MaxDeliver    int           `env:"MAX_RETRIES" envDefault:"5"`
	AckWait       time.Duration `env:"ACK_WAIT" envDefault:"120s"`
	RetryDelay    time.Duration `env:"RETRY_DELAY" envDefault:"10s"`
}

type Consumer struct {
	cfg      Config
	consumer jetstream.Consumer
	svc      *service.Service
	log      *slog.Logger
}

func NewConsumer(ctx context.Context, js jetstream.JetStream, cfg Config, svc *service.Service, log *slog.Logger) (*Consumer, error) {
	c, err := js.CreateOrUpdateConsumer(ctx, natsx.InventoryStream, jetstream.ConsumerConfig{
		Durable:       natsx.WorkerConsumer,
		FilterSubject: natsx.SnapshotSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       cfg.AckWait,
		MaxDeliver:    cfg.MaxDeliver,
		MaxAckPending: cfg.MaxAckPending,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return nil, err
	}
	return &Consumer{cfg: cfg, consumer: c, svc: svc, log: log}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	iter, err := c.consumer.Messages(jetstream.PullMaxMessages(c.cfg.Concurrency))
	if err != nil {
		return err
	}
	defer iter.Stop()

	jobs := make(chan jetstream.Msg)
	done := make(chan struct{})
	for range c.cfg.Concurrency {
		go func() {
			defer func() { done <- struct{}{} }()
			for msg := range jobs {
				c.handle(ctx, msg)
			}
		}()
	}

	go func() {
		<-ctx.Done()
		iter.Stop()
	}()
	for {
		msg, err := iter.Next()
		if err != nil {
			if !errors.Is(err, jetstream.ErrMsgIteratorClosed) {
				c.log.Error("consume messages", "err", err)
			}
			break
		}
		jobs <- msg
	}
	close(jobs)
	for range c.cfg.Concurrency {
		<-done
	}
	return nil
}

func (c *Consumer) handle(ctx context.Context, msg jetstream.Msg) {
	var event natsx.SnapshotCreated
	if err := json.Unmarshal(msg.Data(), &event); err != nil || event.SchemaVersion != natsx.SnapshotSchemaVersion {
		c.log.Error("invalid inventory event, dropping", "err", err, "payload", string(msg.Data()))
		_ = msg.Term()
		return
	}
	inventoryID, err := uuid.Parse(event.InventoryID)
	if err != nil {
		c.log.Error("invalid inventory id, dropping", "inventory_id", event.InventoryID)
		_ = msg.Term()
		return
	}
	log := c.log.With("event_id", event.EventID, "inventory_id", inventoryID, "import_id", event.ImportID)

	meta, _ := msg.Metadata()
	delivered := uint64(1)
	if meta != nil {
		delivered = meta.NumDelivered
	}

	err = c.svc.Process(ctx, inventoryID, event.SerialNumber, func() { _ = msg.InProgress() })
	switch {
	case err == nil:
		if err := msg.Ack(); err != nil {
			log.Warn("ack failed", "err", err)
		}
	case service.IsPermanent(err):
		log.Error("snapshot failed permanently", "err", err)
		_ = msg.Term()
	case ctx.Err() != nil:
		_ = msg.Nak()
	case delivered >= uint64(c.cfg.MaxDeliver):
		log.Error("snapshot exhausted delivery attempts", "attempts", delivered, "err", err)
		c.svc.GiveUp(context.WithoutCancel(ctx), inventoryID, err)
		_ = msg.Term()
	default:
		log.Warn("snapshot processing will be retried", "attempt", delivered, "err", err)
		_ = msg.NakWithDelay(c.cfg.RetryDelay * time.Duration(delivered))
	}
}
