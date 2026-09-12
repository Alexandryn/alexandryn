package jobs

import "time"

// Config tunes the worker pool.
type Config struct {
	// Concurrency is the number of poller goroutines.
	Concurrency int
	// PollInterval is how long a poller waits after finding no
	// claimable job before trying again.
	PollInterval time.Duration
	// LeaseDuration is how far ahead of now a claim or heartbeat sets
	// locked_until.
	LeaseDuration time.Duration
	// HeartbeatInterval is how often a running job's worker extends its
	// lease. Set to a third of LeaseDuration so two consecutive missed
	// heartbeats, not one, are needed before the lease expires.
	HeartbeatInterval time.Duration
	// ReaperInterval is the stale-job sweep period.
	ReaperInterval time.Duration
	// Backoff is the retry delay curve.
	Backoff Backoff
	// ShutdownGracePeriod bounds how long Shutdown waits for running
	// handlers to return before abandoning them to the reaper.
	ShutdownGracePeriod time.Duration
}

// Default worker-pool tuning.
const (
	defaultConcurrency         = 4
	defaultPollInterval        = 2 * time.Second
	defaultLeaseDuration       = 60 * time.Second
	defaultHeartbeatInterval   = 20 * time.Second
	defaultReaperInterval      = 30 * time.Second
	defaultBackoffBase         = 5 * time.Second
	defaultBackoffMax          = 5 * time.Minute
	defaultShutdownGracePeriod = 10 * time.Second
)

// DefaultConfig returns the default worker-pool tuning.
func DefaultConfig() Config {
	return Config{
		Concurrency:         defaultConcurrency,
		PollInterval:        defaultPollInterval,
		LeaseDuration:       defaultLeaseDuration,
		HeartbeatInterval:   defaultHeartbeatInterval,
		ReaperInterval:      defaultReaperInterval,
		Backoff:             Backoff{Base: defaultBackoffBase, Max: defaultBackoffMax},
		ShutdownGracePeriod: defaultShutdownGracePeriod,
	}
}

// withDefaults fills any zero field of c from DefaultConfig, so a caller
// can override only what it cares about.
func (c Config) withDefaults() Config {
	d := DefaultConfig()
	if c.Concurrency <= 0 {
		c.Concurrency = d.Concurrency
	}
	if c.PollInterval <= 0 {
		c.PollInterval = d.PollInterval
	}
	if c.LeaseDuration <= 0 {
		c.LeaseDuration = d.LeaseDuration
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = d.HeartbeatInterval
	}
	if c.ReaperInterval <= 0 {
		c.ReaperInterval = d.ReaperInterval
	}
	if c.Backoff.Base <= 0 {
		c.Backoff.Base = d.Backoff.Base
	}
	if c.Backoff.Max <= 0 {
		c.Backoff.Max = d.Backoff.Max
	}
	if c.ShutdownGracePeriod <= 0 {
		c.ShutdownGracePeriod = d.ShutdownGracePeriod
	}
	return c
}
