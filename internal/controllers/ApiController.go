package controllers

import (
	json "github.com/goccy/go-json"
	"net/http"
	"ssd/internal/models"
	"ssd/internal/providers"
	"ssd/internal/services"
)

const maxRequestBodySize = 1 << 20 // 1 MB

type ApiController struct {
	logger  providers.Logger
	service services.StatisticServiceInterface
	cache   providers.CacheProviderInterface
}

func NewApiController(logger providers.Logger, service services.StatisticServiceInterface, cache providers.CacheProviderInterface) *ApiController {
	return &ApiController{
		logger:  logger,
		service: service,
		cache:   cache,
	}
}

func getChannel(r *http.Request) string {
	ch := r.URL.Query().Get("ch")
	if ch == "" {
		return services.DefaultChannel
	}
	return ch
}

func (ac *ApiController) serveFromCacheOrCompute(w http.ResponseWriter, cacheKey string, compute func() (any, error)) {
	if data, ok := ac.cache.Get(cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	result, err := compute()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	gson, err := json.Marshal(result)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	ac.cache.Set(cacheKey, gson)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(gson)
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
	ac.service.AddStats(&payload)
	w.WriteHeader(http.StatusCreated)
}

func (ac *ApiController) GetStats(w http.ResponseWriter, r *http.Request) {
	ch := getChannel(r)
	ac.serveFromCacheOrCompute(w, "list:"+ch, func() (any, error) {
		return ac.service.GetStatistic(ch), nil
	})
}

func (ac *ApiController) GetPersonalStats(w http.ResponseWriter, r *http.Request) {
	ch := getChannel(r)
	ac.serveFromCacheOrCompute(w, "fps:"+ch, func() (any, error) {
		return ac.service.GetPersonalStatistic(ch), nil
	})
}

func (ac *ApiController) GetByFingerprint(w http.ResponseWriter, r *http.Request) {
	ch := getChannel(r)
	fp := r.URL.Query().Get("f")
	ac.serveFromCacheOrCompute(w, "fp:"+ch+":"+fp, func() (any, error) {
		return ac.service.GetByFingerprint(ch, fp), nil
	})
}

func (ac *ApiController) GetChannels(w http.ResponseWriter, r *http.Request) {
	ac.serveFromCacheOrCompute(w, "channels", func() (any, error) {
		return ac.service.GetChannels(), nil
	})
}
