package models

type InputStats struct {
	Fingerprint string   `json:"f"`
	Clicks      []string `json:"c"`
	Views       []string `json:"v"`
}
