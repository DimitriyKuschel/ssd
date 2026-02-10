package controllers

import (
	"encoding/json"
	"net/http"
	"ssd/internal/models"
	"ssd/internal/providers"
	"ssd/internal/services"
)

const maxRequestBodySize = 1 << 20 // 1 MB

type ApiController struct {
	logger  providers.Logger
	service services.StatisticServiceInterface
}

func NewApiController(logger providers.Logger, service services.StatisticServiceInterface) *ApiController {
	return &ApiController{
		logger:  logger,
		service: service,
	}
}

func (ac *ApiController) ReceiveStats(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	var payload models.InputStats
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	ac.service.AddStats(&payload)
	w.WriteHeader(http.StatusCreated)
}

func (ac *ApiController) GetStats(w http.ResponseWriter, r *http.Request) {
	gson, e := json.Marshal(ac.service.GetStatistic())
	if e != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(gson)
}

func (ac *ApiController) GetPersonalStats(w http.ResponseWriter, r *http.Request) {
	gson, e := json.Marshal(ac.service.GetPersonalStatistic())
	if e != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(gson)
}

func (ac *ApiController) GetByFingerprint(w http.ResponseWriter, r *http.Request) {
	gson, e := json.Marshal(ac.service.GetByFingerprint(r.URL.Query().Get("f")))
	if e != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(gson)
}
