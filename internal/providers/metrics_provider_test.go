package providers

import (
	"io"
	"ssd/internal/models"
	"ssd/internal/structures"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// --- minimal mock for StatisticServiceInterface ---

type metricsTestService struct{}

func (m *metricsTestService) AddStats(_ *models.InputStats)                              {}
func (m *metricsTestService) AggregateStats()                                            {}
func (m *metricsTestService) GetStatistic(_ string) map[int]*models.StatRecord           { return nil }
func (m *metricsTestService) GetPersonalStatistic(_ string) map[string]*models.Statistic { return nil }
func (m *metricsTestService) GetByFingerprint(_, _ string) map[int]*models.StatRecord    { return nil }
func (m *metricsTestService) PutChannelData(_ string, _ map[int]*models.StatRecord, _ map[string]*models.Statistic) {
}
func (m *metricsTestService) PutChannelDataV4(_ string, _ map[int]*models.StatRecord, _ map[string]*models.FingerprintPersistence) {
}
func (m *metricsTestService) GetChannels() []string                        { return []string{"default"} }
func (m *metricsTestService) GetSnapshot() *models.StorageV4               { return nil }
func (m *metricsTestService) GetBufferSize() int                           { return 5 }
func (m *metricsTestService) GetRecordCount(_ string) int                  { return 0 }
func (m *metricsTestService) SetColdStorage(_ models.ColdStorageInterface) {}
func (m *metricsTestService) EvictExpiredFingerprints()                    {}
func (m *metricsTestService) WriteBinarySnapshot(_ io.Writer) error        { return nil }
func (m *metricsTestService) ReadBinarySnapshot(_ io.Reader) error         { return nil }

func TestNoopMetrics_WhenDisabled(t *testing.T) {
	conf := &structures.Config{
		Metrics: structures.MetricsConfig{Enabled: false},
	}
	m := NewMetricsProvider(conf, &metricsTestService{})
	_, ok := m.(*noopMetrics)
	assert.True(t, ok, "should return noopMetrics when disabled")

	// Ensure no-op methods don't panic
	m.IncRequestsTotal("/test", 200)
	m.ObserveRequestDuration("/test", time.Millisecond)
	m.IncCacheHits()
	m.IncCacheMisses()
	m.ObservePersistenceDuration(time.Millisecond)
	m.SetRecordsTotal("default", 10)
}

func TestMetricsProvider_WhenEnabled(t *testing.T) {
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg
	defer func() {
		prometheus.DefaultRegisterer = prometheus.NewRegistry()
		prometheus.DefaultGatherer = prometheus.DefaultRegisterer.(prometheus.Gatherer)
	}()

	conf := &structures.Config{
		Metrics: structures.MetricsConfig{Enabled: true},
	}
	m := NewMetricsProvider(conf, &metricsTestService{})
	_, ok := m.(*MetricsProvider)
	assert.True(t, ok, "should return MetricsProvider when enabled")
}

func TestMetricsProvider_IncrementCounters(t *testing.T) {
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg
	defer func() {
		prometheus.DefaultRegisterer = prometheus.NewRegistry()
		prometheus.DefaultGatherer = prometheus.DefaultRegisterer.(prometheus.Gatherer)
	}()

	conf := &structures.Config{
		Metrics: structures.MetricsConfig{Enabled: true},
	}
	m := NewMetricsProvider(conf, &metricsTestService{})

	// These should not panic
	m.IncRequestsTotal("/list", 200)
	m.IncRequestsTotal("/list", 404)
	m.ObserveRequestDuration("/list", 5*time.Millisecond)
	m.IncCacheHits()
	m.IncCacheMisses()
	m.ObservePersistenceDuration(100 * time.Millisecond)
	m.SetRecordsTotal("default", 42)
}

func TestHttpStatusBucket(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{100, "1xx"},
		{200, "2xx"},
		{201, "2xx"},
		{301, "3xx"},
		{400, "4xx"},
		{404, "4xx"},
		{500, "5xx"},
		{503, "5xx"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, httpStatusBucket(tt.code))
	}
}
