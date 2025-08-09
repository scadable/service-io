// internal/adapters/traefik/client.go
package traefik

import (
	"fmt"

	"github.com/rs/zerolog"
)

type Client struct {
	baseDomain   string
	entryPoint   string
	certResolver string
	network      string
	lg           zerolog.Logger // Add logger to the client
}

type Config struct {
	BaseDomain   string
	EntryPoint   string
	CertResolver string
	Network      string
	Logger       zerolog.Logger // Pass logger in config
}

func New(cfg Config) *Client {
	return &Client{
		baseDomain:   cfg.BaseDomain,
		entryPoint:   cfg.EntryPoint,
		certResolver: cfg.CertResolver,
		network:      cfg.Network,
		lg:           cfg.Logger.With().Str("component", "traefik-client").Logger(),
	}
}

// GenerateConfigForContainer creates Docker labels for routing to an adapter.
// It dynamically switches between secure (TLS) and insecure (TCP) configs.
func (c *Client) GenerateConfigForContainer(containerName, deviceID, containerPort string) (labels map[string]string, url string) {
	// --- Production Config (using a real domain) ---
	if c.baseDomain != "localhost" {
		host := fmt.Sprintf("%s.%s", deviceID, c.baseDomain)
		url = fmt.Sprintf("mqtts://%s:%s", host, containerPort)

		labels = map[string]string{
			"traefik.enable": "true",
			// TCP Router with TLS enabled
			fmt.Sprintf("traefik.tcp.routers.%s.rule", containerName):             fmt.Sprintf("HostSNI(`%s`)", host),
			fmt.Sprintf("traefik.tcp.routers.%s.entrypoints", containerName):      c.entryPoint,
			fmt.Sprintf("traefik.tcp.routers.%s.tls", containerName):              "true",
			fmt.Sprintf("traefik.tcp.routers.%s.tls.certresolver", containerName): c.certResolver,
			fmt.Sprintf("traefik.tcp.routers.%s.service", containerName):          containerName,
			// TCP Service
			fmt.Sprintf("traefik.tcp.services.%s.loadbalancer.server.port", containerName): containerPort,
			// Network
			"traefik.docker.network": c.network,
			// (optional) encourage wildcard cert issuance to avoid LE per-host limits:
			fmt.Sprintf("traefik.tcp.routers.%s.tls.domains[0].main", containerName): fmt.Sprintf("*.%s", c.baseDomain),
			fmt.Sprintf("traefik.tcp.routers.%s.tls.domains[0].sans", containerName): c.baseDomain,
		}
		labels[fmt.Sprintf("traefik.tcp.routers.%s.priority", containerName)] = "100"
		c.lg.Info().Str("mode", "production").Str("host", host).Msg("generated secure TLS routing labels")
		return labels, url
	}

	// --- Local Development Config (using localhost) ---
	url = fmt.Sprintf("mqtt://%s:1883", c.baseDomain) // Plain MQTT protocol

	labels = map[string]string{
		"traefik.enable": "true",
		// TCP Router with no TLS
		fmt.Sprintf("traefik.tcp.routers.%s.rule", containerName):        "HostSNI(`*`)",
		fmt.Sprintf("traefik.tcp.routers.%s.entrypoints", containerName): "mqtt",
		fmt.Sprintf("traefik.tcp.routers.%s.service", containerName):     containerName,
		// TCP Service
		fmt.Sprintf("traefik.tcp.services.%s.loadbalancer.server.port", containerName): containerPort,
		// Network
		"traefik.docker.network": c.network,
	}
	c.lg.Info().Str("mode", "local").Msg("generated insecure TCP routing labels")
	return labels, url
}
