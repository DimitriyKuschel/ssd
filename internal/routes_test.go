package internal

import (
	"net/http"
	"net/http/httptest"
	"ssd/internal/controllers"
	"ssd/internal/models"
	"ssd/internal/providers"
	"ssd/internal/structures"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- minimal mocks for routes test ---

type routeTestLogger struct{}

func (m *routeTestLogger) Errorf(_ providers.TypeEnum, _ string, _ ...interface{}) {}
func (m *routeTestLogger) Warnf(_ providers.TypeEnum, _ string, _ ...interface{})  {}
func (m *routeTestLogger) Debugf(_ providers.TypeEnum, _ string, _ ...interface{}) {}
func (m *routeTestLogger) Infof(_ providers.TypeEnum, _ string, _ ...interface{})  {}
func (m *routeTestLogger) Fatalf(_ providers.TypeEnum, _ string, _ ...interface{}) {}
func (m *routeTestLogger) Close()                                                  {}

type routeTestCache struct{}

func (m *routeTestCache) Get(_ string) ([]byte, bool) { return nil, false }
func (m *routeTestCache) Set(_ string, _ []byte)      {}

type routeTestMockService struct{}

func (m *routeTestMockService) AddStats(_ *models.InputStats)                    {}
func (m *routeTestMockService) AggregateStats()                                  {}
func (m *routeTestMockService) GetStatistic(_ string) map[int]*models.StatRecord { return nil }
func (m *routeTestMockService) GetPersonalStatistic(_ string) map[string]*models.Statistic {
	return nil
}
func (m *routeTestMockService) GetByFingerprint(_, _ string) map[int]*models.StatRecord { return nil }
func (m *routeTestMockService) PutChannelData(_ string, _ map[int]*models.StatRecord, _ map[string]*models.Statistic) {
}
func (m *routeTestMockService) PutChannelDataV4(_ string, _ map[int]*models.StatRecord, _ map[string]*models.FingerprintPersistence) {
}
func (m *routeTestMockService) GetChannels() []string                        { return nil }
func (m *routeTestMockService) GetSnapshot() *models.StorageV4               { return nil }
func (m *routeTestMockService) GetBufferSize() int                           { return 0 }
func (m *routeTestMockService) GetRecordCount(_ string) int                  { return 0 }
func (m *routeTestMockService) SetColdStorage(_ models.ColdStorageInterface) {}
func (m *routeTestMockService) EvictExpiredFingerprints()                    {}

func TestInitRoutes_RegistersFiveRoutes(t *testing.T) {
	ac := controllers.NewApiController(&routeTestLogger{}, &routeTestMockService{}, &routeTestCache{})
	conf := &structures.Config{
		Statistic: structures.StatisticConfig{Interval: 10 * time.Second},
	}

	router := InitRoutes(ac, conf)
	routes := router.GetRoutes()

	require.Len(t, routes, 5)

	urls := make([]string, len(routes))
	for i, r := range routes {
		urls[i] = r.Url
	}

	assert.Contains(t, urls, "/list")
	assert.Contains(t, urls, "/")
	assert.Contains(t, urls, "/fingerprints")
	assert.Contains(t, urls, "/fingerprint")
	assert.Contains(t, urls, "/channels")
}

func TestInitRoutes_MethodEnforcement(t *testing.T) {
	ac := controllers.NewApiController(&routeTestLogger{}, &routeTestMockService{}, &routeTestCache{})
	conf := &structures.Config{
		Statistic: structures.StatisticConfig{Interval: 10 * time.Second},
	}

	router := InitRoutes(ac, conf)
	routes := router.GetRoutes()

	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Url, r.Handler)
	}

	// GET /list with POST should fail
	req := httptest.NewRequest(http.MethodPost, "/list", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)

	// POST / with GET should fail
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}
