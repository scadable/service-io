package devices

import (
	"context"
	"fmt"
	ncore "service-io/internal/adapters/nats"
	"time"

	"service-io/internal/core/docker"
	"service-io/pkg/rand"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type Manager struct {
	db         *gorm.DB
	nc         *ncore.Client
	docker     *docker.Client
	natsURL    string
	adapterMap map[string]string
	lg         zerolog.Logger
}

func New(
	db *gorm.DB,
	nc *ncore.Client,
	natsURL string,
	adapterMap map[string]string,
	dcli *docker.Client,
	lg zerolog.Logger,
) (*Manager, error) {
	return &Manager{
		db:         db,
		nc:         nc,
		docker:     dcli,
		natsURL:    natsURL,
		adapterMap: adapterMap,
		lg:         lg.With().Str("component", "manager").Logger(),
	}, nil
}

// AddDevice -> create DB record, NATS stream, and then the adapter container.
func (m *Manager) AddDevice(ctx context.Context, devType string) (*Device, error) {
	img, ok := m.adapterMap[devType]
	if !ok {
		return nil, fmt.Errorf("unsupported device type %q", devType)
	}

	// 1. Prepare device details
	dev := &Device{
		ID:            rand.ID16(),
		DeviceType:    devType,
		Image:         img,
		NatsSubject:   fmt.Sprintf("devices.%s.telemetry", rand.ID16()),
		ContainerName: "adapter-" + rand.ID16(),
		CreatedAt:     time.Now().UTC(),
	}

	// 2. Create the NATS stream for telemetry
	streamName := "DEV_" + dev.ID
	if err := m.nc.EnsureStream(dev.NatsSubject, streamName); err != nil {
		return nil, fmt.Errorf("ensure nats stream: %w", err)
	}

	// 3. Create the record in the database
	if err := m.db.Create(dev).Error; err != nil {
		return nil, fmt.Errorf("create device record in db: %w", err)
	}

	// 4. Run the adapter container
	containerID, err := m.docker.RunAdapter(ctx, dev.ID, dev.Image, m.natsURL, dev.NatsSubject)
	if err != nil {
		// If container fails to start, delete the DB record to keep state consistent.
		m.db.Delete(dev)
		return nil, fmt.Errorf("start adapter container: %w", err)
	}

	// 5. Update the record with the final ContainerID
	dev.ContainerID = containerID
	if err := m.db.Save(dev).Error; err != nil {
		// This is less critical, but we should log it.
		m.lg.Error().Err(err).Str("device_id", dev.ID).Msg("failed to save container id to db")
	}

	return dev, nil
}

func (m *Manager) ListDevices() ([]Device, error) {
	var devices []Device
	if err := m.db.Find(&devices).Error; err != nil {
		return nil, err
	}
	// If no devices are found, GORM returns an empty slice and no error.
	return devices, nil
}
