package devices

import "time"

type Device struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Subject   string    `json:"subject"`
	CreatedAt time.Time `json:"created_at"`
}
