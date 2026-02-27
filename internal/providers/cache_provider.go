package providers

import (
	"github.com/coocood/freecache"
	"ssd/internal/structures"
	"unsafe"
)

type CacheProviderInterface interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte)
}

type CacheProvider struct {
	cache *freecache.Cache
	ttl   int
}

func NewCacheProvider(conf *structures.Config, logger Logger) CacheProviderInterface {
	if !conf.Cache.Enabled || conf.Cache.Size <= 0 {
		logger.Infof(TypeApp, "Cache disabled")
		return &noopCache{}
	}

	sizeBytes := conf.Cache.Size * 1024 * 1024
	ttl := max(int(conf.Statistic.Interval.Seconds()), 1) + 1

	logger.Infof(TypeApp, "Cache initialized: %dMB, TTL=%ds", conf.Cache.Size, ttl)

	return &CacheProvider{
		cache: freecache.NewCache(sizeBytes),
		ttl:   ttl,
	}
}

// unsafeStringToBytes converts string to []byte without allocation.
// Safe when the result is only read (not modified), which is the case
// for freecache — it copies keys internally.
func unsafeStringToBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func (c *CacheProvider) Get(key string) ([]byte, bool) {
	val, err := c.cache.Get(unsafeStringToBytes(key))
	if err != nil {
		return nil, false
	}
	return val, true
}

func (c *CacheProvider) Set(key string, value []byte) {
	_ = c.cache.Set(unsafeStringToBytes(key), value, c.ttl)
}

type noopCache struct{}

func (n *noopCache) Get(_ string) ([]byte, bool) { return nil, false }
func (n *noopCache) Set(_ string, _ []byte)      {}
