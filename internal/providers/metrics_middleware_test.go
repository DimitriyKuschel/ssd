package providers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockMetrics struct {
	requestEndpoint string
	requestStatus   int
	requestCalls    int
	durationCalls   int
}

func (m *mockMetrics) IncRequestsTotal(endpoint string, status int) {
	m.requestEndpoint = endpoint
	m.requestStatus = status
	m.requestCalls++
}
func (m *mockMetrics) ObserveRequestDuration(_ string, _ time.Duration) { m.durationCalls++ }
func (m *mockMetrics) IncCacheHits()                                    {}
func (m *mockMetrics) IncCacheMisses()                                  {}
func (m *mockMetrics) ObservePersistenceDuration(_ time.Duration)       {}
func (m *mockMetrics) SetRecordsTotal(_ string, _ int)                  {}

func TestMetricsMiddleware_CapturesStatusAndEndpoint(t *testing.T) {
	metrics := &mockMetrics{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	mw := MetricsMiddleware(metrics, handler)

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	assert.Equal(t, 1, metrics.requestCalls)
	assert.Equal(t, "/list", metrics.requestEndpoint)
	assert.Equal(t, http.StatusCreated, metrics.requestStatus)
	assert.Equal(t, 1, metrics.durationCalls)
}

func TestMetricsMiddleware_DefaultStatus200(t *testing.T) {
	metrics := &mockMetrics{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	mw := MetricsMiddleware(metrics, handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, metrics.requestStatus)
}

func TestStatusWriter_WriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rr, status: http.StatusOK}

	sw.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, sw.status)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
