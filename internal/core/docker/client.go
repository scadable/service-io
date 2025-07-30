// internal/core/docker/client.go
// --------------------------------
// Docker wrapper that
//
//	· logs in to DO Container Registry (if DO_REGISTRY_TOKEN present)
//	· spawns adapter containers in the same networks as service-io
package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/docker/docker/api/types/container"
	"io"
	"os"

	types "github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
)

const doRegistry = "registry.digitalocean.com"

// ------------------------------
// Client
// ------------------------------

type Client struct {
	cli        *client.Client
	lg         zerolog.Logger
	authHeader string // base64-encoded JSON for ImagePull
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

	// --- Registry login (optional) ---
	if tok := os.Getenv("DO_REGISTRY_TOKEN"); tok != "" {
		if hdr, err := c.loginDO(context.Background(), tok); err == nil {
			c.authHeader = hdr
			c.lg.Info().Msg("logged in to DigitalOcean registry")
		} else {
			c.lg.Warn().Err(err).Msg("registry login failed; will try anonymous pulls")
		}
	}

	// --- Discover parent networks (if running in a container) ---
	if nets, err := currentContainerNetworks(cli); err == nil {
		c.networks = nets
		c.lg.Debug().Strs("networks", nets).Msg("parent networks detected")
	} else {
		c.lg.Debug().Msg("running outside a container; using default bridge")
	}

	return c, nil
}

// ------------------------------
// Public API
// ------------------------------

// RunAdapter starts a new container for a device and returns the container ID.
func (c *Client) RunAdapter(
	ctx context.Context,
	deviceID, image, natsURL, subject string,
) (containerID string, err error) {
	name := "adapter-" + deviceID

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

	// Join the same Docker networks as service-io.
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

// ------------------------------
// internals
// ------------------------------

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

// currentContainerNetworks returns the names of every Docker network
// the *service-io* container itself is attached to.
// Returns nil slice when running outside a container.
func currentContainerNetworks(cli *client.Client) ([]string, error) {
	contID, err := os.Hostname() // inside Docker -> container ID
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
