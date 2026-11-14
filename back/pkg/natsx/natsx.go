package natsx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	InventoryStream = "INVENTORY"
	SnapshotSubject = "inventory.snapshot.created.v1"
	WorkerConsumer  = "inventory-worker"
)

type Config struct {
	URL           string        `env:"URL" envDefault:"nats://localhost:4222"`
	StreamMaxAge  time.Duration `env:"STREAM_MAX_AGE" envDefault:"168h"`
	StreamMaxSize int64         `env:"STREAM_MAX_BYTES" envDefault:"268435456"`
}

func Connect(cfg Config, name string, log *slog.Logger) (*nats.Conn, jetstream.JetStream, error) {
	nc, err := nats.Connect(cfg.URL,
		nats.Name(name),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Warn("nats disconnected", "err", err)
			}
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			log.Info("nats reconnected", "url", c.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to nats: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("jetstream: %w", err)
	}
	return nc, js, nil
}

func EnsureInventoryStream(ctx context.Context, js jetstream.JetStream, cfg Config) (jetstream.Stream, error) {
	return js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       InventoryStream,
		Subjects:   []string{SnapshotSubject},
		Storage:    jetstream.FileStorage,
		Retention:  jetstream.WorkQueuePolicy,
		Replicas:   1,
		MaxAge:     cfg.StreamMaxAge,
		MaxBytes:   cfg.StreamMaxSize,
		Duplicates: 2 * time.Minute,
	})
}

func Ready(nc *nats.Conn) func(context.Context) error {
	return func(context.Context) error {
		if !nc.IsConnected() {
			return errors.New("nats is " + nc.Status().String())
		}
		return nil
	}
}
