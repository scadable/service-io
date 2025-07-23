package devices

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"service-io/internal/core/docker"
	ncore "service-io/internal/core/nats"
	"service-io/pkg/rand"

	"github.com/rs/zerolog"
)

type Manager struct {
	kv         ncore.KeyValue
	nc         *ncore.Client
	docker     *docker.Client
	natsURL    string
	adapterMap map[string]string
	lg         zerolog.Logger
}

func New(
	nc *ncore.Client,
	bucketName string,
	natsURL string,
	adapterMap map[string]string,
	dcli *docker.Client,
	lg zerolog.Logger,
) (*Manager, error) {
	kv, err := nc.EnsureBucket(bucketName)
	if err != nil {
		return nil, err
	}
	return &Manager{
		kv:         kv,
		nc:         nc,
		docker:     dcli,
		natsURL:    natsURL,
		adapterMap: adapterMap,
		lg:         lg.With().Str("component", "manager").Logger(),
	}, nil
}

// AddDevice -> create ID, stream, registry entry, adapter container.
func (m *Manager) AddDevice(ctx context.Context, devType string) (*Device, error) {
	img, ok := m.adapterMap[devType]
	if !ok {
		return nil, fmt.Errorf("unsupported device type %q", devType)
	}

	id := rand.ID16()
	subject := fmt.Sprintf("devices.%s.telemetry", id)
	stream := "DEV_" + id

	if err := m.nc.EnsureStream(subject, stream); err != nil {
		return nil, err
	}

	dev := Device{
		ID:        id,
		Type:      devType,
		Subject:   subject,
		CreatedAt: time.Now().UTC(),
	}
	raw, _ := json.Marshal(dev)
	if _, err := m.kv.Put(id, raw); err != nil {
		return nil, err
	}

	if err := m.docker.RunAdapter(ctx, id, img, m.natsURL, subject); err != nil {
		_ = m.kv.Delete(id)
		return nil, fmt.Errorf("start adapter: %w", err)
	}
	return &dev, nil
}

func (m *Manager) ListDevices() ([]Device, error) {
	keys, err := m.kv.Keys()
	if err != nil {
		if err == ncore.ErrNoKeysFound {
			
			return []Device{}, nil // just empty
		}
		return nil, err
	}
	out := make([]Device, 0, len(keys))
	for _, k := range keys {
		entry, err := m.kv.Get(k)
		if err != nil && err != ncore.ErrKeyNotFound {
			return nil, err
		}
		var d Device
		_ = json.Unmarshal(entry.Value(), &d)
		out = append(out, d)
	}
	return out, nil
}
