package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"ssd/internal/models"
	"ssd/internal/providers"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- local mocks (scoped to controller tests) ---

type mockLogger struct{}

func (m *mockLogger) Errorf(_ providers.TypeEnum, _ string, _ ...interface{}) {}
func (m *mockLogger) Warnf(_ providers.TypeEnum, _ string, _ ...interface{})  {}
func (m *mockLogger) Debugf(_ providers.TypeEnum, _ string, _ ...interface{}) {}
func (m *mockLogger) Infof(_ providers.TypeEnum, _ string, _ ...interface{})  {}
func (m *mockLogger) Fatalf(_ providers.TypeEnum, _ string, _ ...interface{}) {}
func (m *mockLogger) Close()                                                  {}

type mockService struct {
	addCalls      []*models.InputStats
	statisticData map[int]*models.StatRecord
	personalData  map[string]*models.Statistic
	fpData        map[int]*models.StatRecord
	channelsList  []string
}

func (m *mockService) AddStats(data *models.InputStats)                 { m.addCalls = append(m.addCalls, data) }
func (m *mockService) AggregateStats()                                  {}
func (m *mockService) GetStatistic(_ string) map[int]*models.StatRecord { return m.statisticData }
func (m *mockService) GetPersonalStatistic(_ string) map[string]*models.Statistic {
	return m.personalData
}
func (m *mockService) GetByFingerprint(_, _ string) map[int]*models.StatRecord { return m.fpData }
func (m *mockService) PutChannelData(_ string, _ map[int]*models.StatRecord, _ map[string]*models.Statistic) {
}
func (m *mockService) GetChannels() []string { return m.channelsList }

type mockCache struct {
	data map[string][]byte
}

func newMockCache() *mockCache                     { return &mockCache{data: make(map[string][]byte)} }
func (m *mockCache) Get(key string) ([]byte, bool) { v, ok := m.data[key]; return v, ok }
func (m *mockCache) Set(key string, value []byte)  { m.data[key] = value }

// --- helpers ---

func newTestController(svc *mockService, cache *mockCache) *ApiController {
	return NewApiController(&mockLogger{}, svc, cache)
}

// --- ReceiveStats tests ---

func TestReceiveStats_ValidPayload(t *testing.T) {
	svc := &mockService{}
	ac := newTestController(svc, newMockCache())

	payload := `{"v":["1","2"],"c":["1"],"f":"fp1","ch":"news"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	rr := httptest.NewRecorder()

	ac.ReceiveStats(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	require.Len(t, svc.addCalls, 1)
	assert.Equal(t, "news", svc.addCalls[0].Channel)
	assert.Equal(t, "fp1", svc.addCalls[0].Fingerprint)
	assert.Equal(t, []string{"1", "2"}, svc.addCalls[0].Views)
}

func TestReceiveStats_InvalidJSON(t *testing.T) {
	svc := &mockService{}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json"))
	rr := httptest.NewRecorder()

	ac.ReceiveStats(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Empty(t, svc.addCalls)
}

func TestReceiveStats_EmptyBody(t *testing.T) {
	svc := &mockService{}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	rr := httptest.NewRecorder()

	ac.ReceiveStats(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestReceiveStats_OversizedBody(t *testing.T) {
	svc := &mockService{}
	ac := newTestController(svc, newMockCache())

	big := strings.Repeat("x", maxRequestBodySize+1)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	rr := httptest.NewRecorder()

	ac.ReceiveStats(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestReceiveStats_DefaultChannel(t *testing.T) {
	svc := &mockService{}
	ac := newTestController(svc, newMockCache())

	payload := `{"v":["1"]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	rr := httptest.NewRecorder()

	ac.ReceiveStats(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	require.Len(t, svc.addCalls, 1)
	assert.Equal(t, "default", svc.addCalls[0].Channel)
}

// --- GetStats tests ---

func TestGetStats_ReturnsJSON(t *testing.T) {
	svc := &mockService{
		statisticData: map[int]*models.StatRecord{
			1: {Views: 10, Clicks: 2},
		},
	}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	rr := httptest.NewRecorder()

	ac.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var result map[string]*models.StatRecord
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, 10, result["1"].Views)
}

func TestGetStats_WithChannelParam(t *testing.T) {
	svc := &mockService{
		statisticData: map[int]*models.StatRecord{},
	}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/list?ch=news", nil)
	rr := httptest.NewRecorder()

	ac.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// --- GetPersonalStats tests ---

func TestGetPersonalStats_ReturnsJSON(t *testing.T) {
	svc := &mockService{
		personalData: map[string]*models.Statistic{
			"fp1": {Data: map[int]*models.StatRecord{1: {Views: 5}}},
		},
	}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/fingerprints", nil)
	rr := httptest.NewRecorder()

	ac.GetPersonalStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}

// --- GetByFingerprint tests ---

func TestGetByFingerprint_ReturnsJSON(t *testing.T) {
	svc := &mockService{
		fpData: map[int]*models.StatRecord{1: {Views: 7}},
	}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/fingerprint?f=fp1", nil)
	rr := httptest.NewRecorder()

	ac.GetByFingerprint(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}

func TestGetByFingerprint_NilData(t *testing.T) {
	svc := &mockService{fpData: nil}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/fingerprint?f=unknown", nil)
	rr := httptest.NewRecorder()

	ac.GetByFingerprint(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "null", strings.TrimSpace(rr.Body.String()))
}

// --- GetChannels tests ---

func TestGetChannels_ReturnsJSON(t *testing.T) {
	svc := &mockService{channelsList: []string{"default", "news"}}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/channels", nil)
	rr := httptest.NewRecorder()

	ac.GetChannels(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var result []string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, []string{"default", "news"}, result)
}

// --- Cache behavior tests ---

func TestCacheHit_ServiceNotCalled(t *testing.T) {
	cache := newMockCache()
	cachedData, _ := json.Marshal(map[string]int{"1": 10})
	cache.Set("list:default", cachedData)

	svc := &mockService{
		statisticData: map[int]*models.StatRecord{99: {Views: 999}},
	}
	ac := newTestController(svc, cache)

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	rr := httptest.NewRecorder()

	ac.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, string(cachedData), rr.Body.String())
}

func TestCacheMiss_SavesResult(t *testing.T) {
	cache := newMockCache()
	svc := &mockService{
		statisticData: map[int]*models.StatRecord{1: {Views: 10}},
	}
	ac := newTestController(svc, cache)

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	rr := httptest.NewRecorder()

	ac.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	val, ok := cache.Get("list:default")
	assert.True(t, ok)
	assert.NotEmpty(t, val)
}

func TestCacheKey_Channels(t *testing.T) {
	cache := newMockCache()
	svc := &mockService{channelsList: []string{"default"}}
	ac := newTestController(svc, cache)

	req := httptest.NewRequest(http.MethodGet, "/channels", nil)
	rr := httptest.NewRecorder()

	ac.GetChannels(rr, req)

	_, ok := cache.Get("channels")
	assert.True(t, ok)
}

func TestCacheKey_FingerprintIncludesFP(t *testing.T) {
	cache := newMockCache()
	svc := &mockService{fpData: map[int]*models.StatRecord{1: {Views: 1}}}
	ac := newTestController(svc, cache)

	req := httptest.NewRequest(http.MethodGet, "/fingerprint?f=abc&ch=news", nil)
	rr := httptest.NewRecorder()

	ac.GetByFingerprint(rr, req)

	_, ok := cache.Get("fp:news:abc")
	assert.True(t, ok)
}

// --- Content-Type tests ---

func TestContentType_AllGetEndpoints(t *testing.T) {
	svc := &mockService{
		statisticData: map[int]*models.StatRecord{},
		personalData:  map[string]*models.Statistic{},
		fpData:        map[int]*models.StatRecord{},
		channelsList:  []string{},
	}
	cache := newMockCache()
	ac := newTestController(svc, cache)

	endpoints := []struct {
		path    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"/list", ac.GetStats},
		{"/fingerprints", ac.GetPersonalStats},
		{"/fingerprint?f=x", ac.GetByFingerprint},
		{"/channels", ac.GetChannels},
	}

	for _, ep := range endpoints {
		t.Run(ep.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, ep.path, nil)
			rr := httptest.NewRecorder()
			ep.handler(rr, req)
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
		})
	}
}

// --- getChannel helper tests ---

func TestGetChannel_Default(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	assert.Equal(t, "default", getChannel(req))
}

func TestGetChannel_Custom(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?ch=news", nil)
	assert.Equal(t, "news", getChannel(req))
}

func TestGetChannel_Empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?ch=", nil)
	assert.Equal(t, "default", getChannel(req))
}
