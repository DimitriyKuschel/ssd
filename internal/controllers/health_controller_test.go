package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth_ReturnsOK(t *testing.T) {
	svc := &mockService{channelsList: []string{"default", "news"}}
	hc := NewHealthController(svc)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	hc.Health(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["status"])
	assert.Contains(t, resp, "uptime")
	assert.Contains(t, resp, "uptime_seconds")
	assert.Contains(t, resp, "buffer_size")
	assert.Contains(t, resp, "channels")
	assert.Equal(t, float64(2), resp["channels"])
}

func TestHealth_MethodNotAllowed(t *testing.T) {
	svc := &mockService{}
	hc := NewHealthController(svc)

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rr := httptest.NewRecorder()
	hc.Health(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHealth_BufferSizeReflected(t *testing.T) {
	svc := &mockService{channelsList: []string{"default"}}
	hc := NewHealthController(svc)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	hc.Health(rr, req)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["buffer_size"])
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"zero", 0, "0h0m0s"},
		{"one minute", 60 * time.Second, "0h1m0s"},
		{"one hour", time.Hour, "1h0m0s"},
		{"mixed", time.Hour + time.Minute + time.Second, "1h1m1s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatDuration(tt.duration))
		})
	}
}
