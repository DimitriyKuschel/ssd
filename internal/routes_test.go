package internal

import (
	"io"
	"net/http"
	"net/http/httptest"
	"ssd/internal/controllers"
	"ssd/internal/models"
	"ssd/internal/providers"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- minimal mocks for routes test ---

type routeTestLogger struct{}

func (m *routeTestLogger) Errorf(_ providers.TypeEnum, _ string, _ ...any) {}
func (m *routeTestLogger) Warnf(_ providers.TypeEnum, _ string, _ ...any)  {}
func (m *routeTestLogger) Debugf(_ providers.TypeEnum, _ string, _ ...any) {}
func (m *routeTestLogger) Infof(_ providers.TypeEnum, _ string, _ ...any)  {}
func (m *routeTestLogger) Fatalf(_ providers.TypeEnum, _ string, _ ...any) {}
func (m *routeTestLogger) Close()                                          {}

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
func (m *routeTestMockService) WriteBinarySnapshot(_ io.Writer) error        { return nil }
func (m *routeTestMockService) ReadBinarySnapshot(_ io.Reader) error         { return nil }

func TestInitRoutes_MethodEnforcement(t *testing.T) {
	ac := controllers.NewApiController(&routeTestLogger{}, &routeTestMockService{}, &routeTestCache{})
	mux := InitRoutes(ac)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"GET /list OK", http.MethodGet, "/list", http.StatusOK},
		{"DELETE /list rejected", http.MethodDelete, "/list", http.StatusMethodNotAllowed},
		{"POST / empty body", http.MethodPost, "/", http.StatusBadRequest},
		{"GET / rejected", http.MethodGet, "/", http.StatusMethodNotAllowed},
		{"GET /fingerprints OK", http.MethodGet, "/fingerprints", http.StatusOK},
		{"GET /fingerprint OK", http.MethodGet, "/fingerprint", http.StatusOK},
		{"GET /channels OK", http.MethodGet, "/channels", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)
			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}
