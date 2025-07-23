// Thin wrapper over Docker Engine SDK.
package docker

import (
	"context"

	types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
)

type Client struct {
	cli *client.Client
	lg  zerolog.Logger
}

func New(lg zerolog.Logger) (*Client, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli, lg: lg.With().Str("adapter", "docker").Logger()}, nil
}

// RunAdapter launches (or replaces) a container named adapter-<deviceID>.
func (c *Client) RunAdapter(
	ctx context.Context,
	deviceID string,
	image string,
	natsURL string,
	subject string,
) error {
	name := "adapter-" + deviceID

	// Remove any previous incarnation (idempotent).
	_ = c.cli.ContainerRemove(ctx, name,
		types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})

	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Env: []string{
			"NATS_URL=" + natsURL,
			"SUBJECT=" + subject,
			"ENABLE_JETSTREAM=true",
		},
	}, nil, nil, nil, name)
	if err != nil {
		return err
	}
	return c.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
}
