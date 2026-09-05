package domain

import (
	"time"
)

// NetworkSettings stores user-configurable host network options
// (backend-network-api.md FR-7).
type NetworkSettings struct {
	HostName           string
	RememberDeviceDays int
	UpdatedAt          time.Time
}
