package internal

import (
	"net/http"
	"ssd/internal/controllers"
)

func InitRoutes(apiController *controllers.ApiController) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /list", apiController.GetStats)
	mux.HandleFunc("POST /", apiController.ReceiveStats)
	mux.HandleFunc("GET /fingerprints", apiController.GetPersonalStats)
	mux.HandleFunc("GET /fingerprint", apiController.GetByFingerprint)
	mux.HandleFunc("GET /channels", apiController.GetChannels)
	return mux
}
