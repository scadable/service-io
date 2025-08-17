// internal/adapters/traefik/client.go
package traefik

import (
	"fmt"

	"github.com/rs/zerolog"
)

type Client struct {
	baseDomain string
	// entryPoint is no longer needed here as it's determined by the router
	certResolver string
	network      string
	lg           zerolog.Logger
}

type Config struct {
	BaseDomain string
	// EntryPoint can be removed from here
	CertResolver string
	Network      string
	Logger       zerolog.Logger
}

func New(cfg Config) *Client {
	return &Client{
		baseDomain:   cfg.BaseDomain,
		certResolver: cfg.CertResolver,
		network:      cfg.Network,
		lg:           cfg.Logger.With().Str("component", "traefik-client").Logger(),
	}
}

// GenerateConfigForContainer creates Docker labels for routing to an adapter.
// It dynamically switches between secure (TLS) and insecure (TCP) configs.
func (c *Client) GenerateConfigForContainer(containerName, deviceID, containerPort string) (labels map[string]string, url string) {
	if c.baseDomain != "localhost" {
		host := fmt.Sprintf("%s.%s", deviceID, c.baseDomain)
		// ✅ FIX: The public URL must point to Traefik's public MQTTS port (8883).
		url = fmt.Sprintf("mqtts://%s:8883", host)

		labels = map[string]string{
			"traefik.enable": "true",

			// TCP Router with TLS enabled
			fmt.Sprintf("traefik.tcp.routers.%s.rule", containerName): fmt.Sprintf("HostSNI(`%s`)", host),
			// ✅ FIX: The secure router MUST only listen on the secure 'mqtts' entrypoint.
			fmt.Sprintf("traefik.tcp.routers.%s.entrypoints", containerName):      "mqtts",
			fmt.Sprintf("traefik.tcp.routers.%s.tls", containerName):              "true",
			fmt.Sprintf("traefik.tcp.routers.%s.tls.certresolver", containerName): c.certResolver,
			// This assumes you have a TLS option named 'mqtt' defined in your traefik_mqtt.yaml.
			fmt.Sprintf("traefik.tcp.routers.%s.tls.options", containerName): "mqtt@file",
			fmt.Sprintf("traefik.tcp.routers.%s.service", containerName):     containerName,

			// TCP Service
			fmt.Sprintf("traefik.tcp.services.%s.loadbalancer.server.port", containerName): containerPort,

			// Network
			"traefik.docker.network": c.network,

			// Wildcard certificate configuration (this part is good)
			fmt.Sprintf("traefik.tcp.routers.%s.tls.domains[0].main", containerName): fmt.Sprintf("*.%s", c.baseDomain),
			fmt.Sprintf("traefik.tcp.routers.%s.tls.domains[0].sans", containerName): c.baseDomain,
		}

		c.lg.Info().
			Str("mode", "production").
			Str("host", host).
			Msg("generated secure TLS routing labels")
		return labels, url
	}

	// --- Local Development Config (using localhost) ---
	// ✅ FIX: The client connects to Traefik's 'mqtt' port, not the container's port.
	url = fmt.Sprintf("mqtt://localhost:1883")

	labels = map[string]string{
		"traefik.enable": "true",
		// TCP Router with no TLS
		// ✅ FIX: Plain TCP has no SNI. Use HostSNI(`*`) to catch all traffic on the entrypoint.
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
