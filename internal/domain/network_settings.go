package domain

import (
	"time"
)

// NetworkSettings stores user-configurable host network options.
type NetworkSettings struct {
	HostName           string
	RememberDeviceDays int
	UpdatedAt          time.Time
}
