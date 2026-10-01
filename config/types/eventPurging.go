package config

import "time"

// Retention clocks for EventPurgeConfig.RetentionClock.
const (
	// RetentionClockReceived ages an event from when this relay received it,
	// so an event that arrives already older than the keep window still gets
	// the whole window. The default.
	RetentionClockReceived = "received"
	// RetentionClockCreatedAt ages an event from its own created_at, the rule
	// before 0.8: an event that arrives already old goes at the next purge.
	RetentionClockCreatedAt = "created_at"
)

// DefaultLateArrivalMinutes is used when late_arrival_minutes is unset.
const DefaultLateArrivalMinutes = 10

type EventPurgeConfig struct {
	Enabled              bool            `yaml:"enabled" json:"enabled"`
	DisableAtStartup     bool            `yaml:"disable_at_startup" json:"disable_at_startup"`
	KeepIntervalHours    int             `yaml:"keep_interval_hours" json:"keep_interval_hours"`
	PurgeIntervalMinutes int             `yaml:"purge_interval_minutes" json:"purge_interval_minutes"`
	PurgeByCategory      map[string]bool `yaml:"purge_by_category" json:"purge_by_category"`
	PurgeByKindEnabled   bool            `yaml:"purge_by_kind_enabled" json:"purge_by_kind_enabled"`
	KindsToPurge         []int           `yaml:"kinds_to_purge" json:"kinds_to_purge"`
	// KeepKinds are never purged, regardless of category or kinds_to_purge —
	// a protective allow-list that overrides every other purge rule. Use it to
	// retain specific kinds (e.g. profiles/relay lists) while purging the rest.
	KeepKinds          []int `yaml:"keep_kinds" json:"keep_kinds"`
	ExcludeWhitelisted bool  `yaml:"exclude_whitelisted" json:"exclude_whitelisted"`
	// RetentionClock is RetentionClockReceived or RetentionClockCreatedAt.
	RetentionClock string `yaml:"retention_clock" json:"retention_clock"`
	// LateArrivalMinutes: an event received more than this long after its
	// created_at has its arrival time recorded, and the received clock ages
	// it from then. Anything sooner is aged from created_at, so it can go at
	// most this early.
	LateArrivalMinutes int `yaml:"late_arrival_minutes" json:"late_arrival_minutes"`
}

// AgesFromReceipt reports whether purge ages events from when they were
// received rather than from their created_at.
func (c EventPurgeConfig) AgesFromReceipt() bool {
	return c.RetentionClock != RetentionClockCreatedAt
}

// LateArrival returns the late-arrival threshold, defaulting when unset.
func (c EventPurgeConfig) LateArrival() time.Duration {
	m := c.LateArrivalMinutes
	if m <= 0 {
		m = DefaultLateArrivalMinutes
	}
	return time.Duration(m) * time.Minute
}
