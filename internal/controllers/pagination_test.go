package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"ssd/internal/models"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parsePagination tests ---

func TestParsePagination_NoParams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/list", nil)
	limit, offset := parsePagination(r)
	assert.Equal(t, 0, limit)
	assert.Equal(t, 0, offset)
}

func TestParsePagination_LimitOnly(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/list?limit=10", nil)
	limit, offset := parsePagination(r)
	assert.Equal(t, 10, limit)
	assert.Equal(t, 0, offset)
}

func TestParsePagination_OffsetOnly(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/list?offset=5", nil)
	limit, offset := parsePagination(r)
	assert.Equal(t, 0, limit)
	assert.Equal(t, 5, offset)
}

func TestParsePagination_BothParams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/list?limit=10&offset=20", nil)
	limit, offset := parsePagination(r)
	assert.Equal(t, 10, limit)
	assert.Equal(t, 20, offset)
}

func TestParsePagination_MaxLimitCap(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/list?limit=99999", nil)
	limit, _ := parsePagination(r)
	assert.Equal(t, maxLimit, limit)
}

func TestParsePagination_NegativeValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/list?limit=-5&offset=-10", nil)
	limit, offset := parsePagination(r)
	assert.Equal(t, 0, limit)
	assert.Equal(t, 0, offset)
}

func TestParsePagination_InvalidValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/list?limit=abc&offset=xyz", nil)
	limit, offset := parsePagination(r)
	assert.Equal(t, 0, limit)
	assert.Equal(t, 0, offset)
}

// --- hasPagination tests ---

func TestHasPagination(t *testing.T) {
	assert.False(t, hasPagination(0, 0))
	assert.True(t, hasPagination(10, 0))
	assert.True(t, hasPagination(0, 5))
	assert.True(t, hasPagination(10, 5))
}

// --- paginatedCacheKey tests ---

func TestPaginatedCacheKey_NoPagination(t *testing.T) {
	assert.Equal(t, "list:default", paginatedCacheKey("list:default", 0, 0))
}

func TestPaginatedCacheKey_WithPagination(t *testing.T) {
	assert.Equal(t, "list:default:10:20", paginatedCacheKey("list:default", 10, 20))
}

// --- paginateIntMap tests ---

func TestPaginateIntMap_NilData(t *testing.T) {
	result, total := paginateIntMap(nil, 10, 0)
	assert.Nil(t, result)
	assert.Equal(t, 0, total)
}

func TestPaginateIntMap_LimitOnly(t *testing.T) {
	data := map[int]*models.StatRecord{
		1: {Views: 10}, 2: {Views: 20}, 3: {Views: 30}, 4: {Views: 40}, 5: {Views: 50},
	}
	result, total := paginateIntMap(data, 3, 0)
	assert.Equal(t, 5, total)
	assert.Len(t, result, 3)
	// Should return keys 1, 2, 3 (sorted ascending)
	assert.Contains(t, result, 1)
	assert.Contains(t, result, 2)
	assert.Contains(t, result, 3)
}

func TestPaginateIntMap_OffsetOnly(t *testing.T) {
	data := map[int]*models.StatRecord{
		1: {Views: 10}, 2: {Views: 20}, 3: {Views: 30},
	}
	result, total := paginateIntMap(data, 0, 1)
	assert.Equal(t, 3, total)
	assert.Len(t, result, 2)
	assert.Contains(t, result, 2)
	assert.Contains(t, result, 3)
}

func TestPaginateIntMap_LimitAndOffset(t *testing.T) {
	data := map[int]*models.StatRecord{
		1: {Views: 10}, 2: {Views: 20}, 3: {Views: 30}, 4: {Views: 40}, 5: {Views: 50},
	}
	result, total := paginateIntMap(data, 2, 2)
	assert.Equal(t, 5, total)
	assert.Len(t, result, 2)
	assert.Contains(t, result, 3)
	assert.Contains(t, result, 4)
}

func TestPaginateIntMap_OffsetBeyondData(t *testing.T) {
	data := map[int]*models.StatRecord{
		1: {Views: 10}, 2: {Views: 20},
	}
	result, total := paginateIntMap(data, 10, 100)
	assert.Equal(t, 2, total)
	assert.Empty(t, result)
}

func TestPaginateIntMap_LimitExceedsRemaining(t *testing.T) {
	data := map[int]*models.StatRecord{
		1: {Views: 10}, 2: {Views: 20}, 3: {Views: 30},
	}
	result, total := paginateIntMap(data, 100, 1)
	assert.Equal(t, 3, total)
	assert.Len(t, result, 2)
}

// --- paginateStringMap tests ---

func TestPaginateStringMap_NilData(t *testing.T) {
	result, total := paginateStringMap(nil, 10, 0)
	assert.Nil(t, result)
	assert.Equal(t, 0, total)
}

func TestPaginateStringMap_LimitAndOffset(t *testing.T) {
	data := map[string]*models.Statistic{
		"a": {Data: map[int]*models.StatRecord{1: {Views: 1}}},
		"b": {Data: map[int]*models.StatRecord{2: {Views: 2}}},
		"c": {Data: map[int]*models.StatRecord{3: {Views: 3}}},
		"d": {Data: map[int]*models.StatRecord{4: {Views: 4}}},
	}
	result, total := paginateStringMap(data, 2, 1)
	assert.Equal(t, 4, total)
	assert.Len(t, result, 2)
	assert.Contains(t, result, "b")
	assert.Contains(t, result, "c")
}

func TestPaginateStringMap_OffsetBeyondData(t *testing.T) {
	data := map[string]*models.Statistic{
		"a": {Data: map[int]*models.StatRecord{1: {Views: 1}}},
	}
	result, total := paginateStringMap(data, 10, 100)
	assert.Equal(t, 1, total)
	assert.Empty(t, result)
}

// --- paginateSlice tests ---

func TestPaginateSlice_NilData(t *testing.T) {
	result, total := paginateSlice(nil, 10, 0)
	assert.Nil(t, result)
	assert.Equal(t, 0, total)
}

func TestPaginateSlice_LimitAndOffset(t *testing.T) {
	data := []string{"a", "b", "c", "d", "e"}
	result, total := paginateSlice(data, 2, 1)
	assert.Equal(t, 5, total)
	assert.Equal(t, []string{"b", "c"}, result)
}

func TestPaginateSlice_OffsetBeyondData(t *testing.T) {
	data := []string{"a", "b"}
	result, total := paginateSlice(data, 10, 100)
	assert.Equal(t, 2, total)
	assert.Empty(t, result)
}

// --- Handler pagination integration tests ---

func TestGetStats_WithPagination(t *testing.T) {
	svc := &mockService{
		statisticData: map[int]*models.StatRecord{
			1: {Views: 10}, 2: {Views: 20}, 3: {Views: 30}, 4: {Views: 40}, 5: {Views: 50},
		},
	}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/list?limit=2&offset=1", nil)
	rr := httptest.NewRecorder()
	ac.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result paginatedResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 2, result.Limit)
	assert.Equal(t, 1, result.Offset)

	dataMap, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Len(t, dataMap, 2)
}

func TestGetStats_WithoutPagination_RawResponse(t *testing.T) {
	svc := &mockService{
		statisticData: map[int]*models.StatRecord{
			1: {Views: 10}, 2: {Views: 20},
		},
	}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	rr := httptest.NewRecorder()
	ac.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Should be a flat map, not wrapped
	var result map[string]*models.StatRecord
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Len(t, result, 2)
}

func TestGetPersonalStats_WithPagination(t *testing.T) {
	svc := &mockService{
		personalData: map[string]*models.Statistic{
			"fp1": {Data: map[int]*models.StatRecord{1: {Views: 5}}},
			"fp2": {Data: map[int]*models.StatRecord{2: {Views: 10}}},
			"fp3": {Data: map[int]*models.StatRecord{3: {Views: 15}}},
		},
	}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/fingerprints?limit=2", nil)
	rr := httptest.NewRecorder()
	ac.GetPersonalStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result paginatedResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 2, result.Limit)

	dataMap, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Len(t, dataMap, 2)
}

func TestGetByFingerprint_WithPagination(t *testing.T) {
	svc := &mockService{
		fpData: map[int]*models.StatRecord{
			1: {Views: 10}, 2: {Views: 20}, 3: {Views: 30},
		},
	}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/fingerprint?f=fp1&limit=2", nil)
	rr := httptest.NewRecorder()
	ac.GetByFingerprint(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result paginatedResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 2, result.Limit)
}

func TestGetChannels_WithPagination(t *testing.T) {
	svc := &mockService{channelsList: []string{"alpha", "beta", "gamma", "delta"}}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/channels?limit=2&offset=1", nil)
	rr := httptest.NewRecorder()
	ac.GetChannels(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result paginatedResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, 4, result.Total)
	assert.Equal(t, 2, result.Limit)
	assert.Equal(t, 1, result.Offset)

	data, ok := result.Data.([]any)
	require.True(t, ok)
	assert.Len(t, data, 2)
	assert.Equal(t, "beta", data[0])
	assert.Equal(t, "gamma", data[1])
}

func TestGetChannels_WithoutPagination_RawResponse(t *testing.T) {
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

// --- Cache key with pagination ---

func TestCacheKey_PaginationSeparateKeys(t *testing.T) {
	cache := newMockCache()
	svc := &mockService{
		statisticData: map[int]*models.StatRecord{
			1: {Views: 10}, 2: {Views: 20}, 3: {Views: 30},
		},
	}
	ac := newTestController(svc, cache)

	// Request without pagination
	req1 := httptest.NewRequest(http.MethodGet, "/list", nil)
	rr1 := httptest.NewRecorder()
	ac.GetStats(rr1, req1)

	// Request with pagination
	req2 := httptest.NewRequest(http.MethodGet, "/list?limit=1", nil)
	rr2 := httptest.NewRecorder()
	ac.GetStats(rr2, req2)

	// Both should be cached under different keys
	_, ok1 := cache.Get("list:default")
	assert.True(t, ok1)
	_, ok2 := cache.Get("list:default:1:0")
	assert.True(t, ok2)

	// Responses should differ
	assert.NotEqual(t, rr1.Body.String(), rr2.Body.String())
}

func TestCacheHit_WithPagination(t *testing.T) {
	cache := newMockCache()
	cachedData, _ := json.Marshal(paginatedResponse{
		Data:   map[string]int{"1": 10},
		Total:  5,
		Limit:  1,
		Offset: 0,
	})
	cache.Set("list:default:1:0", cachedData)

	svc := &mockService{
		statisticData: map[int]*models.StatRecord{99: {Views: 999}},
	}
	ac := newTestController(svc, cache)

	req := httptest.NewRequest(http.MethodGet, "/list?limit=1", nil)
	rr := httptest.NewRecorder()
	ac.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, string(cachedData), rr.Body.String())
}

func TestPagination_MaxLimitEnforced(t *testing.T) {
	data := make(map[int]*models.StatRecord)
	for i := range 20000 {
		data[i] = &models.StatRecord{Views: i}
	}
	svc := &mockService{statisticData: data}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/list?limit=20000", nil)
	rr := httptest.NewRecorder()
	ac.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result paginatedResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, 20000, result.Total)
	assert.Equal(t, maxLimit, result.Limit)

	dataMap, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Len(t, dataMap, maxLimit)
}

// --- Edge cases ---

func TestPagination_EmptyData(t *testing.T) {
	svc := &mockService{statisticData: map[int]*models.StatRecord{}}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/list?limit=10", nil)
	rr := httptest.NewRecorder()
	ac.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result paginatedResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, 0, result.Total)
	assert.Equal(t, 10, result.Limit)

	dataMap, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Empty(t, dataMap)
}

func TestPagination_NilData_WithPagination(t *testing.T) {
	svc := &mockService{statisticData: nil}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/list?limit=10", nil)
	rr := httptest.NewRecorder()
	ac.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result paginatedResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, 0, result.Total)
	assert.Nil(t, result.Data)
}

func TestPagination_OffsetOnlyWithoutLimit(t *testing.T) {
	svc := &mockService{
		statisticData: map[int]*models.StatRecord{
			1: {Views: 10}, 2: {Views: 20}, 3: {Views: 30}, 4: {Views: 40}, 5: {Views: 50},
		},
	}
	ac := newTestController(svc, newMockCache())

	req := httptest.NewRequest(http.MethodGet, "/list?offset=3", nil)
	rr := httptest.NewRecorder()
	ac.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result paginatedResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 0, result.Limit)
	assert.Equal(t, 3, result.Offset)

	// offset=3 without limit → returns all remaining (keys 4, 5)
	dataMap, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Len(t, dataMap, 2)
}

func TestPagination_LimitZeroExplicit(t *testing.T) {
	svc := &mockService{
		statisticData: map[int]*models.StatRecord{
			1: {Views: 10}, 2: {Views: 20},
		},
	}
	ac := newTestController(svc, newMockCache())

	// ?limit=0 is treated as "no pagination" → raw response
	req := httptest.NewRequest(http.MethodGet, "/list?limit=0", nil)
	rr := httptest.NewRecorder()
	ac.GetStats(rr, req)

	var result map[string]*models.StatRecord
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Len(t, result, 2)
}

func TestPagination_ConsecutivePages(t *testing.T) {
	svc := &mockService{
		statisticData: map[int]*models.StatRecord{
			1: {Views: 10}, 2: {Views: 20}, 3: {Views: 30}, 4: {Views: 40}, 5: {Views: 50},
		},
	}
	ac := newTestController(svc, newMockCache())

	// Page 1: offset=0, limit=2 → keys 1,2
	req1 := httptest.NewRequest(http.MethodGet, "/list?limit=2&offset=0", nil)
	rr1 := httptest.NewRecorder()
	ac.GetStats(rr1, req1)

	var page1 paginatedResponse
	require.NoError(t, json.Unmarshal(rr1.Body.Bytes(), &page1))
	p1Data := page1.Data.(map[string]any)
	assert.Len(t, p1Data, 2)
	assert.Contains(t, p1Data, "1")
	assert.Contains(t, p1Data, "2")

	// Page 2: offset=2, limit=2 → keys 3,4
	req2 := httptest.NewRequest(http.MethodGet, "/list?limit=2&offset=2", nil)
	rr2 := httptest.NewRecorder()
	ac.GetStats(rr2, req2)

	var page2 paginatedResponse
	require.NoError(t, json.Unmarshal(rr2.Body.Bytes(), &page2))
	p2Data := page2.Data.(map[string]any)
	assert.Len(t, p2Data, 2)
	assert.Contains(t, p2Data, "3")
	assert.Contains(t, p2Data, "4")

	// Page 3: offset=4, limit=2 → key 5 only
	req3 := httptest.NewRequest(http.MethodGet, "/list?limit=2&offset=4", nil)
	rr3 := httptest.NewRecorder()
	ac.GetStats(rr3, req3)

	var page3 paginatedResponse
	require.NoError(t, json.Unmarshal(rr3.Body.Bytes(), &page3))
	p3Data := page3.Data.(map[string]any)
	assert.Len(t, p3Data, 1)
	assert.Contains(t, p3Data, "5")

	// All pages report same total
	assert.Equal(t, 5, page1.Total)
	assert.Equal(t, 5, page2.Total)
	assert.Equal(t, 5, page3.Total)

	// No overlap between pages
	for k := range p1Data {
		assert.NotContains(t, p2Data, k)
		assert.NotContains(t, p3Data, k)
	}
	for k := range p2Data {
		assert.NotContains(t, p3Data, k)
	}
}
