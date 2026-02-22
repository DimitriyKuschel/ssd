package models

type ChannelData struct {
	TrendStats    map[int]*StatRecord   `json:"trend_stats"`
	PersonalStats map[string]*Statistic `json:"personal_stats"`
}

type Storage struct {
	Channels map[string]*ChannelData `json:"channels"`
}
