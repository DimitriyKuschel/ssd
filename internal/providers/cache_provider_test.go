package providers

import (
	"ssd/internal/structures"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// local mock logger to avoid import cycle with testutil
type cacheTestLogger struct{}

func (m *cacheTestLogger) Errorf(_ TypeEnum, _ string, _ ...interface{}) {}
func (m *cacheTestLogger) Warnf(_ TypeEnum, _ string, _ ...interface{})  {}
func (m *cacheTestLogger) Debugf(_ TypeEnum, _ string, _ ...interface{}) {}
func (m *cacheTestLogger) Infof(_ TypeEnum, _ string, _ ...interface{})  {}
func (m *cacheTestLogger) Fatalf(_ TypeEnum, _ string, _ ...interface{}) {}
func (m *cacheTestLogger) Close()                                        {}

func cacheConfig(enabled bool, size int, interval time.Duration) *structures.Config {
	return &structures.Config{
		Cache: structures.CacheConfig{
			Enabled: enabled,
			Size:    size,
		},
		Statistic: structures.StatisticConfig{
			Interval: interval,
		},
	}
}

func TestCacheProvider_DisabledReturnsNoop(t *testing.T) {
	logger := &cacheTestLogger{}
	c := NewCacheProvider(cacheConfig(false, 10, 5*time.Second), logger)
	_, ok := c.Get("any")
	assert.False(t, ok)
	assert.IsType(t, &noopCache{}, c)
}

func TestCacheProvider_ZeroSizeReturnsNoop(t *testing.T) {
	logger := &cacheTestLogger{}
	c := NewCacheProvider(cacheConfig(true, 0, 5*time.Second), logger)
	assert.IsType(t, &noopCache{}, c)
}

func TestCacheProvider_EnabledReturnsCacheProvider(t *testing.T) {
	logger := &cacheTestLogger{}
	c := NewCacheProvider(cacheConfig(true, 1, 5*time.Second), logger)
	assert.IsType(t, &CacheProvider{}, c)
}

func TestCacheProvider_SetAndGet(t *testing.T) {
	logger := &cacheTestLogger{}
	c := NewCacheProvider(cacheConfig(true, 1, 5*time.Second), logger)

	c.Set("key1", []byte("value1"))
	val, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, []byte("value1"), val)
}

func TestCacheProvider_Miss(t *testing.T) {
	logger := &cacheTestLogger{}
	c := NewCacheProvider(cacheConfig(true, 1, 5*time.Second), logger)

	val, ok := c.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestCacheProvider_Overwrite(t *testing.T) {
	logger := &cacheTestLogger{}
	c := NewCacheProvider(cacheConfig(true, 1, 5*time.Second), logger)

	c.Set("key1", []byte("v1"))
	c.Set("key1", []byte("v2"))

	val, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, []byte("v2"), val)
}

func TestNoopCache_AlwaysMiss(t *testing.T) {
	c := &noopCache{}
	c.Set("key1", []byte("value1"))

	val, ok := c.Get("key1")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestCacheProvider_TTLExpiry(t *testing.T) {
	logger := &cacheTestLogger{}
	// TTL = interval + 1 = 2s
	c := NewCacheProvider(cacheConfig(true, 1, 1*time.Second), logger)

	c.Set("key1", []byte("value1"))
	val, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, []byte("value1"), val)

	time.Sleep(2100 * time.Millisecond)

	_, ok = c.Get("key1")
	assert.False(t, ok)
}
