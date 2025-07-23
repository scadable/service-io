// Light wrapper around Docker Engine SDK (local socket by default).
package docker

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
)

type Client struct {
	cli *client.Client
	lg  zerolog.Logger
}

func New(lg zerolog.Logger) (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli, lg: lg.With().Str("adapter", "docker").Logger()}, nil
}

// RunAdapter starts (or replaces) a container named adapter-<id>.
func (c *Client) RunAdapter(
	ctx context.Context,
	id string,
	image string,
	natsURL, subject string,
) error {
	name := "adapter-" + id

	// Delete previous container with same name (idempotent).
	_ = c.cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true})

	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Env: []string{
			"NATS_URL=" + natsURL,
			"SUBJECT=" + subject,
			"ENABLE_JETSTREAM=" + "true",
		},
	}, nil, nil, nil, name)
	if err != nil {
		return err
	}
	return c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
}
