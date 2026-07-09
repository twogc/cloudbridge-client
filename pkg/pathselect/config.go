package pathselect

import "time"

// Config is the path_select: block (mapstructure tags for viper).
// Defaults match docs/plans/GAP_CLOSURE_AND_IMPROVEMENTS.md Phase C example.
type Config struct {
	Enabled          bool          `mapstructure:"enabled"`
	Order            []string      `mapstructure:"order"`
	ProbeTimeout     time.Duration `mapstructure:"probe_timeout"`
	LadderTimeout    time.Duration `mapstructure:"ladder_timeout"`
	HealthInterval   time.Duration `mapstructure:"health_interval"`
	FailoverCooldown time.Duration `mapstructure:"failover_cooldown"`
	SoftFail         bool          `mapstructure:"soft_fail"`
}

// DefaultConfig returns library defaults (selector off until Phase C wiring).
func DefaultConfig() Config {
	return Config{
		Enabled:          false,
		Order:            []string{PathRelayQUIC, PathGRPCTunnel},
		ProbeTimeout:     5 * time.Second,
		LadderTimeout:    45 * time.Second,
		HealthInterval:   10 * time.Second,
		FailoverCooldown: 15 * time.Second,
		SoftFail:         false,
	}
}

// Normalize fills zero values with defaults (does not force Enabled).
func (c Config) Normalize() Config {
	d := DefaultConfig()
	if len(c.Order) == 0 {
		c.Order = append([]string(nil), d.Order...)
	}
	if c.ProbeTimeout <= 0 {
		c.ProbeTimeout = d.ProbeTimeout
	}
	if c.LadderTimeout <= 0 {
		c.LadderTimeout = d.LadderTimeout
	}
	if c.HealthInterval <= 0 {
		c.HealthInterval = d.HealthInterval
	}
	if c.FailoverCooldown <= 0 {
		c.FailoverCooldown = d.FailoverCooldown
	}
	return c
}
