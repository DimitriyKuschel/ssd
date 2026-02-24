package providers

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"ssd/internal/services"
	"ssd/internal/structures"
	"time"
)

type MetricsProviderInterface interface {
	IncRequestsTotal(endpoint string, status int)
	ObserveRequestDuration(endpoint string, duration time.Duration)
	IncCacheHits()
	IncCacheMisses()
	ObservePersistenceDuration(duration time.Duration)
	SetRecordsTotal(channel string, count int)
}

type MetricsProvider struct {
	requestsTotal       *prometheus.CounterVec
	requestDuration     *prometheus.HistogramVec
	cacheHits           prometheus.Counter
	cacheMisses         prometheus.Counter
	persistenceDuration prometheus.Histogram
	recordsTotal        *prometheus.GaugeVec
}

func (m *MetricsProvider) IncRequestsTotal(endpoint string, status int) {
	m.requestsTotal.WithLabelValues(endpoint, httpStatusBucket(status)).Inc()
}

func (m *MetricsProvider) ObserveRequestDuration(endpoint string, duration time.Duration) {
	m.requestDuration.WithLabelValues(endpoint).Observe(duration.Seconds())
}

func (m *MetricsProvider) IncCacheHits() {
	m.cacheHits.Inc()
}

func (m *MetricsProvider) IncCacheMisses() {
	m.cacheMisses.Inc()
}

func (m *MetricsProvider) ObservePersistenceDuration(duration time.Duration) {
	m.persistenceDuration.Observe(duration.Seconds())
}

func (m *MetricsProvider) SetRecordsTotal(channel string, count int) {
	m.recordsTotal.WithLabelValues(channel).Set(float64(count))
}

func httpStatusBucket(code int) string {
	switch {
	case code < 200:
		return "1xx"
	case code < 300:
		return "2xx"
	case code < 400:
		return "3xx"
	case code < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

func NewMetricsProvider(conf *structures.Config, service services.StatisticServiceInterface) MetricsProviderInterface {
	if !conf.Metrics.Enabled {
		return &noopMetrics{}
	}

	m := &MetricsProvider{
		requestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "ssd_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"endpoint", "status"}),

		requestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "ssd_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"endpoint"}),

		cacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "ssd_cache_hits_total",
			Help: "Total number of cache hits",
		}),

		cacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "ssd_cache_misses_total",
			Help: "Total number of cache misses",
		}),

		persistenceDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "ssd_persistence_duration_seconds",
			Help:    "Duration of persistence operations in seconds",
			Buckets: prometheus.DefBuckets,
		}),

		recordsTotal: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ssd_records_total",
			Help: "Total number of stat records per channel",
		}, []string{"channel"}),
	}

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "ssd_buffer_size",
		Help: "Current number of items in the active buffer",
	}, func() float64 {
		return float64(service.GetBufferSize())
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "ssd_channels_total",
		Help: "Total number of channels",
	}, func() float64 {
		return float64(len(service.GetChannels()))
	})

	return m
}

// noopMetrics is a no-op implementation for when metrics are disabled.
type noopMetrics struct{}

func (n *noopMetrics) IncRequestsTotal(_ string, _ int)                 {}
func (n *noopMetrics) ObserveRequestDuration(_ string, _ time.Duration) {}
func (n *noopMetrics) IncCacheHits()                                    {}
func (n *noopMetrics) IncCacheMisses()                                  {}
func (n *noopMetrics) ObservePersistenceDuration(_ time.Duration)       {}
func (n *noopMetrics) SetRecordsTotal(_ string, _ int)                  {}
