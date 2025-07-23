package config

import (
	"encoding/json"
	"os"
	"strconv"
	"time"
)

type AdapterMap map[string]string

type Config struct {
	NATSURL        string
	ListenAddr     string
	DevBucket      string
	PublishTimeout time.Duration
	Adapters       AdapterMap
	DORegistryTok  string // digital ocean Personal Access Token
}

// MustLoad loads the required settings for the system to operate
func MustLoad() Config {
	url := getenv("NATS_URL", "nats://localhost:4222")
	addr := getenv("LISTEN_ADDR", ":9090")
	bucket := getenv("DEV_BUCKET", "devices")

	sec, _ := strconv.Atoi(getenv("PUBLISH_TIMEOUT_SEC", "5"))
	adapters := make(AdapterMap)
	_ = json.Unmarshal([]byte(getenv("ADAPTER_MAP_JSON", `{"random":"rand-adapter:latest"}`)), &adapters)

	return Config{
		NATSURL:        url,
		ListenAddr:     addr,
		DevBucket:      bucket,
		PublishTimeout: time.Duration(sec) * time.Second,
		Adapters:       adapters,
		DORegistryTok:  getenv("DO_REGISTRY_TOKEN", ""),
	}
}

// getenv fetches the env variables for the application to run
func getenv(k, d string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return d
}
