package providers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type cacheMetricsTestMetrics struct {
	hits   int
	misses int
}

func (m *cacheMetricsTestMetrics) IncRequestsTotal(_ string, _ int)                 {}
func (m *cacheMetricsTestMetrics) ObserveRequestDuration(_ string, _ time.Duration) {}
func (m *cacheMetricsTestMetrics) IncCacheHits()                                    { m.hits++ }
func (m *cacheMetricsTestMetrics) IncCacheMisses()                                  { m.misses++ }
func (m *cacheMetricsTestMetrics) ObservePersistenceDuration(_ time.Duration)       {}
func (m *cacheMetricsTestMetrics) SetRecordsTotal(_ string, _ int)                  {}

type cacheMetricsTestInner struct {
	data map[string][]byte
}

func (c *cacheMetricsTestInner) Get(key string) ([]byte, bool) {
	v, ok := c.data[key]
	return v, ok
}
func (c *cacheMetricsTestInner) Set(key string, value []byte) {
	c.data[key] = value
}

func TestMetricsCacheProvider_Hit(t *testing.T) {
	inner := &cacheMetricsTestInner{data: map[string][]byte{"key1": []byte("val1")}}
	metrics := &cacheMetricsTestMetrics{}
	cache := &MetricsCacheProvider{inner: inner, metrics: metrics}

	val, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, []byte("val1"), val)
	assert.Equal(t, 1, metrics.hits)
	assert.Equal(t, 0, metrics.misses)
}

func TestMetricsCacheProvider_Miss(t *testing.T) {
	inner := &cacheMetricsTestInner{data: map[string][]byte{}}
	metrics := &cacheMetricsTestMetrics{}
	cache := &MetricsCacheProvider{inner: inner, metrics: metrics}

	val, ok := cache.Get("missing")
	assert.False(t, ok)
	assert.Nil(t, val)
	assert.Equal(t, 0, metrics.hits)
	assert.Equal(t, 1, metrics.misses)
}

func TestMetricsCacheProvider_SetDelegates(t *testing.T) {
	inner := &cacheMetricsTestInner{data: map[string][]byte{}}
	metrics := &cacheMetricsTestMetrics{}
	cache := &MetricsCacheProvider{inner: inner, metrics: metrics}

	cache.Set("key2", []byte("val2"))

	val, ok := inner.Get("key2")
	assert.True(t, ok)
	assert.Equal(t, []byte("val2"), val)
}

func TestMetricsCacheProvider_MultipleOperations(t *testing.T) {
	inner := &cacheMetricsTestInner{data: map[string][]byte{"a": []byte("1")}}
	metrics := &cacheMetricsTestMetrics{}
	cache := &MetricsCacheProvider{inner: inner, metrics: metrics}

	cache.Get("a") // hit
	cache.Get("b") // miss
	cache.Get("a") // hit
	cache.Get("c") // miss

	assert.Equal(t, 2, metrics.hits)
	assert.Equal(t, 2, metrics.misses)
}
