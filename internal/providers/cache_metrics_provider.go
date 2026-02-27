package providers

import "ssd/internal/structures"

// MetricsCacheProvider wraps a CacheProviderInterface and increments
// hit/miss counters on every Get call.
type MetricsCacheProvider struct {
	inner   CacheProviderInterface
	metrics MetricsProviderInterface
}

func (c *MetricsCacheProvider) Get(key string) ([]byte, bool) {
	val, ok := c.inner.Get(key)
	if ok {
		c.metrics.IncCacheHits()
	} else {
		c.metrics.IncCacheMisses()
	}
	return val, ok
}

func (c *MetricsCacheProvider) Set(key string, value []byte) {
	c.inner.Set(key, value)
}

// NewInstrumentedCacheProvider creates a cache provider wrapped with metrics instrumentation.
// When cache is disabled, returns the plain noopCache without metrics wrapping
// to avoid counting phantom cache misses.
func NewInstrumentedCacheProvider(conf *structures.Config, logger Logger, metrics MetricsProviderInterface) CacheProviderInterface {
	inner := NewCacheProvider(conf, logger)
	if !conf.Cache.Enabled {
		return inner
	}
	return &MetricsCacheProvider{
		inner:   inner,
		metrics: metrics,
	}
}
