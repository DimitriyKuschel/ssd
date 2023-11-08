package internal

import (
	"net/http"
	"ssd/internal/controllers"
	"ssd/internal/providers"
	"ssd/internal/structures"
)

func InitRoutes(apiController *controllers.ApiController, conf *structures.Config) providers.RouterProviderInterface {
	routers := providers.NewRouterProvider()

	routers.Get("/list", http.HandlerFunc(apiController.GetStats))
	routers.Post("/", http.HandlerFunc(apiController.ReceiveStats))
	routers.Get("/fingerprints", http.HandlerFunc(apiController.GetPersonalStats))
	routers.Get("/fingerprint", http.HandlerFunc(apiController.GetByFingerprint))
	return routers
}
