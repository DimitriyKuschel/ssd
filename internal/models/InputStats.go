package models

type InputStats struct {
	Fingerprint string   `json:"f"`
	Clicks      []string `json:"c"`
	Views       []string `json:"v"`
	Hits        []string `json:"h"`
	Engagements []string `json:"e"`
	Channel     string   `json:"ch"`
}
