// internal/adapters/traefik/client.go
package traefik

import (
	"fmt"
)

// Client handles the logic for configuring Traefik.
type Client struct {
	baseDomain   string
	entryPoint   string
	certResolver string
	network      string
}

// Config holds the configuration for the Traefik adapter.
type Config struct {
	BaseDomain   string // e.g., "io.scadable.com"
	EntryPoint   string // e.g., "websecure"
	CertResolver string // e.g., "myresolver" for Let's Encrypt
	Network      string // The name of the Docker network Traefik uses
}

// New creates a new Traefik client.
func New(cfg Config) *Client {
	return &Client{
		baseDomain:   cfg.BaseDomain,
		entryPoint:   cfg.EntryPoint,
		certResolver: cfg.CertResolver,
		network:      cfg.Network,
	}
}

// GenerateConfigForContainer returns the Docker labels and the public URL for a new adapter.
func (c *Client) GenerateConfigForContainer(containerName, deviceID, containerPort string) (labels map[string]string, url string) {
	hostRule := fmt.Sprintf("Host(`%s.%s`)", deviceID, c.baseDomain)
	url = fmt.Sprintf("%s.%s", deviceID, c.baseDomain)

	labels = map[string]string{
		"traefik.enable": "true",
		// Routers
		fmt.Sprintf("traefik.http.routers.%s.rule", containerName):             hostRule,
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", containerName):      c.entryPoint,
		fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", containerName): c.certResolver,
		// Services
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", containerName): containerPort,
		// Network
		"traefik.docker.network": c.network,
	}
	return labels, url
}
