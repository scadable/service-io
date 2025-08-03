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

	var devID string
	for {
		devID = rand.ID16()
		var count int64
		if err := m.db.Model(&Device{}).Where("id = ?", devID).Count(&count).Error; err != nil {
			return nil, fmt.Errorf("failed to check for existing device ID: %w", err)
		}
		if count == 0 {
			// The ID is unique, we can break the loop.
			break
		}
		m.lg.Warn().Str("device_id", devID).Msg("generated device ID already exists, retrying...")
	}

	dev := &Device{
		ID:            devID,
		DeviceType:    devType,
		Image:         img,
		NatsSubject:   fmt.Sprintf("devices.%s.telemetry", devID),
		ContainerName: "adapter-" + devID,
		Status:        "running",
		CreatedAt:     time.Now().UTC(),
	}

	streamName := "DEV_" + dev.ID
	if err := m.nc.EnsureStream(dev.NatsSubject, streamName); err != nil {
		return nil, fmt.Errorf("ensure nats stream: %w", err)
	}

	if err := m.db.Create(dev).Error; err != nil {
		return nil, fmt.Errorf("create device record in db: %w", err)
	}

	containerID, err := m.docker.RunAdapter(ctx, dev.ID, dev.Image, m.natsURL)
	if err != nil {
		m.lg.Error().Err(err).Str("device_id", dev.ID).Msg("failed to start container, rolling back")
		if delErr := m.nc.DeleteStream(streamName); delErr != nil {
			m.lg.Error().Err(delErr).Str("stream_name", streamName).Msg("rollback failed to delete nats stream")
		}
		if delErr := m.db.Delete(dev).Error; delErr != nil {
			m.lg.Error().Err(delErr).Str("device_id", dev.ID).Msg("rollback failed to delete db record")
		}
		return nil, fmt.Errorf("start adapter container: %w", err)
	}

	dev.ContainerID = containerID
	if err := m.db.Save(dev).Error; err != nil {
		m.lg.Error().Err(err).Str("device_id", dev.ID).Msg("failed to save container id to db")
	}

	return dev, nil
}

// RemoveDevice stops the container and marks the device as "stopped".
func (m *Manager) RemoveDevice(ctx context.Context, deviceID string) error {
	var dev Device
	if err := m.db.First(&dev, "id = ?", deviceID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("device with ID '%s' not found", deviceID)
		}
		return err
	}

	containerIdentifier := dev.ContainerID
	if containerIdentifier == "" {
		containerIdentifier = dev.ContainerName
	}

	if err := m.docker.StopAndRemoveContainer(ctx, containerIdentifier); err != nil {
		m.lg.Error().Err(err).Str("device_id", deviceID).Msg("failed to stop/remove container, proceeding to update status")
	}

	dev.Status = "stopped"
	if err := m.db.Save(&dev).Error; err != nil {
		return fmt.Errorf("failed to update device status to stopped: %w", err)
	}

	m.lg.Info().Str("device_id", deviceID).Msg("device stopped successfully")
	return nil
}

// RestartRunningDevices finds all devices with "running" status and starts them.
func (m *Manager) RestartRunningDevices(ctx context.Context) error {
	m.lg.Info().Msg("restarting any previously running devices...")
	var runningDevices []Device
	if err := m.db.Where("status = ?", "running").Find(&runningDevices).Error; err != nil {
		return fmt.Errorf("could not query running devices: %w", err)
	}

	for _, dev := range runningDevices {
		m.lg.Info().Str("device_id", dev.ID).Msg("restarting device")
		containerID, err := m.docker.RunAdapter(ctx, dev.ID, dev.Image, m.natsURL)
		if err != nil {
			m.lg.Error().Err(err).Str("device_id", dev.ID).Msg("failed to restart device container")
			dev.Status = "stopped"
		} else {
			dev.ContainerID = containerID
		}

		if err := m.db.Save(&dev).Error; err != nil {
			m.lg.Error().Err(err).Str("device_id", dev.ID).Msg("failed to update device record after restart attempt")
		}
	}
	m.lg.Info().Int("count", len(runningDevices)).Msg("device restart process complete")
	return nil
}

// CleanupAdapters stops all managed containers.
func (m *Manager) CleanupAdapters(ctx context.Context) error {
	m.lg.Info().Msg("cleaning up all adapter containers")
	devices, err := m.ListDevices()
	if err != nil {
		return fmt.Errorf("could not list devices for cleanup: %w", err)
	}

	for _, dev := range devices {
		if dev.Status == "running" {
			containerIdentifier := dev.ContainerID
			if containerIdentifier == "" {
				containerIdentifier = dev.ContainerName
			}
			if err := m.docker.StopAndRemoveContainer(ctx, containerIdentifier); err != nil {
				m.lg.Error().Err(err).Str("device_id", dev.ID).Msg("failed during cleanup")
			}
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
