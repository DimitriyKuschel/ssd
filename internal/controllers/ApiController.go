package controllers

import (
	"cmp"
	"net/http"

	json "github.com/goccy/go-json"
	"golang.org/x/sync/singleflight"

	"ssd/internal/models"
	"ssd/internal/providers"
	"ssd/internal/services"
)

const maxRequestBodySize = 1 << 20 // 1 MB

type ApiController struct {
	logger  providers.Logger
	service services.StatisticServiceInterface
	cache   providers.CacheProviderInterface
	sf      singleflight.Group
}

func NewApiController(logger providers.Logger, service services.StatisticServiceInterface, cache providers.CacheProviderInterface) *ApiController {
	return &ApiController{
		logger:  logger,
		service: service,
		cache:   cache,
	}
}

func getChannel(r *http.Request) string {
	return cmp.Or(r.URL.Query().Get("ch"), services.DefaultChannel)
}

func (ac *ApiController) writeJSON(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (ac *ApiController) serveFromCacheOrCompute(w http.ResponseWriter, cacheKey string, compute func() (any, error)) {
	if data, ok := ac.cache.Get(cacheKey); ok {
		ac.writeJSON(w, data)
		return
	}

	// Collapse concurrent misses for the same key (e.g. the burst right after the
	// cache is cleared on aggregation) into a single compute+marshal; waiters share
	// the resulting bytes. The bytes are only read, so sharing across goroutines is
	// safe.
	v, err, _ := ac.sf.Do(cacheKey, func() (any, error) {
		result, err := compute()
		if err != nil {
			return nil, err
		}
		gson, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		ac.cache.Set(cacheKey, gson)
		return gson, nil
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	ac.writeJSON(w, v.([]byte))
}

func (ac *ApiController) ReceiveStats(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	var payload models.InputStats
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if payload.Channel == "" {
		payload.Channel = services.DefaultChannel
	}
	if !isValidChannel(payload.Channel) || !isValidFingerprint(payload.Fingerprint) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	ac.service.AddStats(&payload)
	w.WriteHeader(http.StatusCreated)
}

func (ac *ApiController) GetStats(w http.ResponseWriter, r *http.Request) {
	ch := getChannel(r)
	limit, offset := parsePagination(r)
	cacheKey := paginatedCacheKey("list:"+ch, limit, offset)
	ac.serveFromCacheOrCompute(w, cacheKey, func() (any, error) {
		if hasPagination(limit, offset) {
			page, total := ac.service.GetStatisticPage(ch, limit, offset)
			return paginatedResponse{Data: page, Total: total, Limit: limit, Offset: offset}, nil
		}
		return ac.service.GetStatistic(ch), nil
	})
}

func (ac *ApiController) GetPersonalStats(w http.ResponseWriter, r *http.Request) {
	ch := getChannel(r)
	limit, offset := parsePagination(r)
	cacheKey := paginatedCacheKey("fps:"+ch, limit, offset)
	ac.serveFromCacheOrCompute(w, cacheKey, func() (any, error) {
		if hasPagination(limit, offset) {
			page, total := ac.service.GetPersonalStatisticPage(ch, limit, offset)
			return paginatedResponse{Data: page, Total: total, Limit: limit, Offset: offset}, nil
		}
		return ac.service.GetPersonalStatistic(ch), nil
	})
}

func (ac *ApiController) GetByFingerprint(w http.ResponseWriter, r *http.Request) {
	ch := getChannel(r)
	fp := r.URL.Query().Get("f")
	limit, offset := parsePagination(r)
	cacheKey := paginatedCacheKey("fp:"+ch+":"+fp, limit, offset)
	ac.serveFromCacheOrCompute(w, cacheKey, func() (any, error) {
		data := ac.service.GetByFingerprint(ch, fp)
		if hasPagination(limit, offset) {
			page, total := paginateIntMap(data, limit, offset)
			return paginatedResponse{Data: page, Total: total, Limit: limit, Offset: offset}, nil
		}
		return data, nil
	})
}

func (ac *ApiController) GetChannels(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	cacheKey := paginatedCacheKey("channels", limit, offset)
	ac.serveFromCacheOrCompute(w, cacheKey, func() (any, error) {
		data := ac.service.GetChannels()
		if hasPagination(limit, offset) {
			page, total := paginateSlice(data, limit, offset)
			return paginatedResponse{Data: page, Total: total, Limit: limit, Offset: offset}, nil
		}
		return data, nil
	})
}
