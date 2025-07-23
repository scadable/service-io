package nats

import (
	"fmt"

	natsgo "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

// Alias external types so callers import only our package.
type KeyValue = natsgo.KeyValue

var ErrKeyNotFound = natsgo.ErrKeyNotFound
var ErrNoKeysFound = natsgo.ErrNoKeysFound

type Client struct {
	nc *natsgo.Conn
	js natsgo.JetStreamContext
	lg zerolog.Logger
}

func New(url string, lg zerolog.Logger) (*Client, error) {
	nc, err := natsgo.Connect(url, natsgo.Name("service-io"))
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	return &Client{nc: nc, js: js, lg: lg.With().Str("adapter", "nats").Logger()}, nil
}

// EnsureStream idempotently creates a dedicated stream for one device.
func (c *Client) EnsureStream(subject, name string) error {
	_, err := c.js.AddStream(&natsgo.StreamConfig{
		Name:     name,
		Subjects: []string{subject},
		Storage:  natsgo.FileStorage,
		Replicas: 1,
	})
	if err != nil && err != natsgo.ErrStreamNameAlreadyInUse {
		return err
	}
	return nil
}

// -------- Key-value bucket (device registry) --------

func (c *Client) EnsureBucket(name string) (KeyValue, error) {
	kv, err := c.js.KeyValue(name)
	if err == nil {
		return kv, nil
	}
	if err != natsgo.ErrBucketNotFound {
		return nil, err
	}
	return c.js.CreateKeyValue(&natsgo.KeyValueConfig{
		Bucket:      name,
		Description: "Device registry",
		History:     1,
		Replicas:    1,
	})
}

func (c *Client) Close() { _ = c.nc.Drain() }
