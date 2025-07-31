package devices

import (
	"context"
	"errors"
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

	dev := &Device{
		ID:            rand.ID16(),
		DeviceType:    devType,
		Image:         img,
		NatsSubject:   fmt.Sprintf("devices.%s.telemetry", rand.ID16()),
		ContainerName: "adapter-" + rand.ID16(),
		CreatedAt:     time.Now().UTC(),
	}

	streamName := "DEV_" + dev.ID
	if err := m.nc.EnsureStream(dev.NatsSubject, streamName); err != nil {
		return nil, fmt.Errorf("ensure nats stream: %w", err)
	}

	if err := m.db.Create(dev).Error; err != nil {
		return nil, fmt.Errorf("create device record in db: %w", err)
	}

	containerID, err := m.docker.RunAdapter(ctx, dev.ID, dev.Image, m.natsURL, dev.NatsSubject)
	if err != nil {
		m.db.Delete(dev)
		return nil, fmt.Errorf("start adapter container: %w", err)
	}

	dev.ContainerID = containerID
	if err := m.db.Save(dev).Error; err != nil {
		m.lg.Error().Err(err).Str("device_id", dev.ID).Msg("failed to save container id to db")
	}

	return dev, nil
}

// RemoveDevice stops the container and deletes the record from the database.
func (m *Manager) RemoveDevice(ctx context.Context, deviceID string) error {
	var dev Device
	if err := m.db.First(&dev, "id = ?", deviceID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("device with ID '%s' not found", deviceID)
		}
		return err
	}

	if err := m.docker.StopAndRemoveContainer(ctx, dev.ContainerName); err != nil {
		// Log the error but proceed to delete the DB record anyway
		m.lg.Error().Err(err).Str("device_id", deviceID).Msg("failed to stop/remove container")
	}

	if err := m.db.Delete(&dev).Error; err != nil {
		return fmt.Errorf("failed to delete device record from db: %w", err)
	}

	return nil
}

// CleanupAdapters stops all managed containers. Intended for graceful shutdown.
func (m *Manager) CleanupAdapters(ctx context.Context) error {
	m.lg.Info().Msg("cleaning up all adapter containers")
	devices, err := m.ListDevices()
	if err != nil {
		return fmt.Errorf("could not list devices for cleanup: %w", err)
	}

	for _, dev := range devices {
		if err := m.docker.StopAndRemoveContainer(ctx, dev.ContainerName); err != nil {
			m.lg.Error().Err(err).Str("device_id", dev.ID).Msg("failed during cleanup")
		}
	}
	m.lg.Info().Msg("adapter cleanup complete")
	return nil
}

func (m *Manager) ListDevices() ([]Device, error) {
	var devices []Device
	if err := m.db.Find(&devices).Error; err != nil {
		return nil, err
	}
	return devices, nil
}
