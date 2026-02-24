package controllers

import (
	"fmt"
	json "github.com/goccy/go-json"
	"net/http"
	"ssd/internal/services"
	"time"
)

type HealthController struct {
	service   services.StatisticServiceInterface
	startTime time.Time
}

type healthResponse struct {
	Status        string  `json:"status"`
	Uptime        string  `json:"uptime"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	BufferSize    int     `json:"buffer_size"`
	Channels      int     `json:"channels"`
}

func (hc *HealthController) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(hc.startTime)
	resp := healthResponse{
		Status:        "ok",
		Uptime:        formatDuration(uptime),
		UptimeSeconds: uptime.Seconds(),
		BufferSize:    hc.service.GetBufferSize(),
		Channels:      len(hc.service.GetChannels()),
	}

	gson, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(gson)
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
}

func NewHealthController(service services.StatisticServiceInterface) *HealthController {
	return &HealthController{
		service:   service,
		startTime: time.Now(),
	}
}
