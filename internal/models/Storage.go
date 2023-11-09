package models

type Storage struct {
	TrendStats    map[int]*StatRecord   `json:"trend_stats"`
	PersonalStats map[string]*Statistic `json:"personal_stats"`
}
