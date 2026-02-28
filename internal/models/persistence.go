package models

import "time"

// FingerprintPersistence is the V4 persistence format for a single fingerprint.
// It is a JSON superset of Statistic{Data} — V3 files unmarshal into this struct
// with LastSeen as zero-value, enabling seamless migration.
type FingerprintPersistence struct {
	Data     map[int]*StatRecord `json:"data"`
	LastSeen time.Time           `json:"last_seen"`
}

// ChannelDataV4 is the V4 persistence format for a single channel.
type ChannelDataV4 struct {
	TrendStats    map[int]*StatRecord                `json:"trend_stats"`
	PersonalStats map[string]*FingerprintPersistence `json:"personal_stats"`
}

// StorageV4 is the V4 persistence envelope with an explicit version field.
type StorageV4 struct {
	Version  int                       `json:"version"`
	Channels map[string]*ChannelDataV4 `json:"channels"`
}
