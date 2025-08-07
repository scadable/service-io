package devices

import "time"

// Device represents a single device adapter instance.
// It includes GORM tags for database mapping and JSON tags for API responses.
type Device struct {
	ID            string    `gorm:"primaryKey" json:"id" example:"EDIVRWCLGGPGCW7M"`
	DeviceType    string    `json:"type" example:"mqtt"`
	Image         string    `json:"image" example:"registry.digitalocean.com/scadable-container-registry/adapter-mqtt:latest"`
	NatsSubject   string    `json:"nats_subject" example:"devices.EDIVRWCLGGPGCW7M.telemetry"`
	ContainerID   string    `json:"container_id" example:"3518d34547496f2a8c4af44be3c71d7f..."`
	ContainerName string    `json:"container_name" example:"adapter-EDIVRWCLGGPGCW7M"`
	ContainerURL  string    `json:"container_url" example:"EDIVRWCLGGPGCW7M.localhost"`
	Status        string    `json:"status" example:"running"`
	CreatedAt     time.Time `json:"created_at"`

	// MQTT credentials, only populated for 'mqtt' type devices.
	MQTTUser string `json:"mqtt_user,omitempty" example:"EDIVRWCLGGPGCW7M"`
	// The JSON tag is changed from "-" to "mqtt_password,omitempty" to expose it.
	MQTTPassword string `json:"mqtt_password,omitempty" example:"aBc12DeF34gH56iJ"`
}
