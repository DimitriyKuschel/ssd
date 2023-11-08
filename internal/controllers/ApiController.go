package controllers

import (
	"encoding/json"
	"net/http"
	"ssd/internal/models"
	"ssd/internal/providers"
	"ssd/internal/services"
)

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
	var payload models.InputStats
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ac.service.AddStats(&payload)
	w.WriteHeader(201)
}

func (ac *ApiController) GetStats(w http.ResponseWriter, r *http.Request) {
	gson, e := json.Marshal(ac.service.GetStatistic())
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(200)
	w.Write(gson)
}

func (ac *ApiController) GetPersonalStats(w http.ResponseWriter, r *http.Request) {
	gson, e := json.Marshal(ac.service.GetPersonalStatistic())
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(200)
	w.Write(gson)
}

func (ac *ApiController) GetByFingerprint(w http.ResponseWriter, r *http.Request) {
	gson, e := json.Marshal(ac.service.GetByFingerprint(r.URL.Query().Get("f")))
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(200)
	w.Write(gson)
}
