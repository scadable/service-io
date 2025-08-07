package devices

import "time"

// Device represents a single device adapter instance.
// It includes GORM tags for database mapping and JSON tags for API responses.
type Device struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	DeviceType    string    `json:"type"`
	Image         string    `json:"image"`
	NatsSubject   string    `json:"nats_subject"`
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	ContainerURL  string    `json:"container_url"` // Add this line
	Status        string    `json:"status"`        // e.g., "running", "stopped"
	CreatedAt     time.Time `json:"created_at"`
}
