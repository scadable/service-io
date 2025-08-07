package config

import (
	"encoding/json"
	"os"
	"strconv"
	"time"
)

type AdapterMap map[string]string

type Config struct {
	NATSURL             string
	DatabaseDSN         string
	ListenAddr          string
	DevBucket           string
	PublishTimeout      time.Duration
	Adapters            AdapterMap
	DORegistryTok       string // digital ocean Personal Access Token
	TraefikNetwork      string
	TraefikEntryPoint   string
	TraefikCertResolver string
	BaseDomain          string
}

// MustLoad loads the required settings for the system to operate
func MustLoad() Config {
	url := getenv("NATS_URL", "nats://localhost:4222")
	dsn := getenv("DATABASE_DSN", "postgres://user:password@localhost:5432/servicedb?sslmode=disable")
	addr := getenv("LISTEN_ADDR", ":9090")
	bucket := getenv("DEV_BUCKET", "devices")

	sec, _ := strconv.Atoi(getenv("PUBLISH_TIMEOUT_SEC", "5"))
	adapters := make(AdapterMap)
	_ = json.Unmarshal([]byte(getenv("ADAPTER_MAP_JSON", `{"random":"rand-adapter:latest"}`)), &adapters)

	return Config{
		NATSURL:             url,
		DatabaseDSN:         dsn,
		ListenAddr:          addr,
		DevBucket:           bucket,
		PublishTimeout:      time.Duration(sec) * time.Second,
		Adapters:            adapters,
		DORegistryTok:       getenv("DO_REGISTRY_TOKEN", "dop_v1_9669d538d70b521478b20088690d812ce75270151646a6add8bb4b4a22c6db8f"),
		TraefikNetwork:      getenv("TRAEFIK_NETWORK", "service-io_default"),
		TraefikEntryPoint:   getenv("TRAEFIK_ENTRYPOINT", "websecure"),
		TraefikCertResolver: getenv("TRAEFIK_CERT_RESOLVER", "myresolver"),
		BaseDomain:          getenv("BASE_DOMAIN", "io.scadable.com"),
	}
}

// getenv fetches the env variables for the application to run
func getenv(k, d string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return d
}
