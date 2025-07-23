package nats

import (
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

type Client struct {
	nc *nats.Conn
	js nats.JetStreamContext
	lg zerolog.Logger
}

// New makes a new connection to the nats service
func New(url string, lg zerolog.Logger) (*Client, error) {
	nc, err := nats.Connect(url, nats.Name("service-io"))
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		_ = nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}

	return &Client{nc: nc, js: js, lg: lg.With().Str("adapter", "nats").Logger()}, nil
}

// EnsureStream will add a stream to teh nats service ready to be used
func (c *Client) EnsureStream(subject, stream string) error {
	_, err := c.js.AddStream(&nats.StreamConfig{
		Name:     stream,
		Subjects: []string{subject},
		Storage:  nats.FileStorage,
		Replicas: 1,
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		return err
	}
	return nil
}

// KV bucket (devices registry) ----------

func (c *Client) Bucket(name string) (nats.KeyValue, error) {
	return c.js.KeyValue(name)
}

func (c *Client) EnsureBucket(name string) (nats.KeyValue, error) {
	kv, err := c.js.KeyValue(name)
	if err == nil {
		return kv, nil
	}
	if err != nats.ErrBucketNotFound {
		return nil, err
	}
	return c.js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket:      name,
		Description: "Device registry",
		History:     1,
		TTL:         0,
		Replicas:    1,
	})
}

// Close the connection
func (c *Client) Close() { _ = c.nc.Drain() }
