// internal/core/docker/client.go
package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
)

const doRegistry = "registry.digitalocean.com"

type Client struct {
	cli        *client.Client
	lg         zerolog.Logger
	authHeader string
	networks   []string
}

func New(lg zerolog.Logger) (*Client, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	c := &Client{cli: cli, lg: lg.With().Str("adapter", "docker").Logger()}

	if tok := os.Getenv("DO_REGISTRY_TOKEN"); tok != "" {
		if hdr, err := c.loginDO(context.Background(), tok); err == nil {
			c.authHeader = hdr
			c.lg.Info().Msg("logged in to DigitalOcean registry")
		} else {
			c.lg.Warn().Err(err).Msg("registry login failed; will try anonymous pulls")
		}
	}

	if nets, err := currentContainerNetworks(cli); err == nil {
		c.networks = nets
		c.lg.Debug().Strs("networks", nets).Msg("parent networks detected")
	} else {
		c.lg.Debug().Msg("running outside a container; using default bridge")
	}

	return c, nil
}

func (c *Client) RunAdapter(
	ctx context.Context,
	deviceID, image, natsURL string,
) (containerID string, err error) {
	name := "adapter-" + deviceID
	subject := "devices." + deviceID + ".telemetry"

	_ = c.cli.ContainerRemove(ctx, name,
		types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})

	if err := c.ensureImage(ctx, image); err != nil {
		return "", err
	}

	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Env: []string{
			"NATS_URL=" + natsURL,
			"SUBJECT=" + subject,
			"ENABLE_JETSTREAM=true",
		},
	}, nil, nil, nil, name)
	if err != nil {
		return "", err
	}

	for _, n := range c.networks {
		if err := c.cli.NetworkConnect(ctx, n, resp.ID, nil); err != nil {
			c.lg.Warn().Err(err).Str("network", n).Msg("connect adapter to network")
		}
	}

	if err := c.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", err
	}

	return resp.ID, nil
}

// StopAndRemoveContainer stops and removes a container. It's idempotent.
func (c *Client) StopAndRemoveContainer(ctx context.Context, containerIdentifier string) error {
	c.lg.Info().Str("container", containerIdentifier).Msg("stopping and removing container")

	// The `Force: true` option in ContainerRemove will stop the container first.
	// So we can simplify this to a single call.
	err := c.cli.ContainerRemove(ctx, containerIdentifier, types.ContainerRemoveOptions{
		Force:         true, // Stop the container if it's running.
		RemoveVolumes: true, // Remove anonymous volumes.
	})

	// If the container is already gone, that's okay.
	if err != nil && client.IsErrNotFound(err) {
		c.lg.Warn().Str("container", containerIdentifier).Msg("container not found, assuming it's already removed")
		return nil
	}

	if err == nil {
		c.lg.Info().Str("container", containerIdentifier).Msg("container removed successfully")
	}

	return err
}

func (c *Client) ensureImage(ctx context.Context, img string) error {
	_, _, err := c.cli.ImageInspectWithRaw(ctx, img)
	if err == nil {
		return nil
	}
	if client.IsErrNotFound(err) {
		opts := types.ImagePullOptions{}
		if c.authHeader != "" {
			opts.RegistryAuth = c.authHeader
		}
		rc, err := c.cli.ImagePull(ctx, img, opts)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, rc)
		rc.Close()
		return nil
	}
	return err
}

func (c *Client) loginDO(ctx context.Context, token string) (string, error) {
	cfg := registrytypes.AuthConfig{
		ServerAddress: doRegistry,
		Username:      "doctl",
		Password:      token,
	}
	if _, err := c.cli.RegistryLogin(ctx, cfg); err != nil {
		return "", err
	}
	raw, _ := json.Marshal(cfg)
	return base64.StdEncoding.EncodeToString(raw), nil
}

func currentContainerNetworks(cli *client.Client) ([]string, error) {
	contID, err := os.Hostname()
	if err != nil || len(contID) < 12 {
		return nil, err
	}
	ins, err := cli.ContainerInspect(context.Background(), contID)
	if err != nil {
		return nil, err
	}
	nets := make([]string, 0, len(ins.NetworkSettings.Networks))
	for n := range ins.NetworkSettings.Networks {
		nets = append(nets, n)
	}
	return nets, nil
}
