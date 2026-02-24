//go:build wireinject
// +build wireinject

package di

import (
	wire "github.com/google/wire"
	"ssd/internal"
	"ssd/internal/controllers"
	"ssd/internal/providers"
	"ssd/internal/services"
	"ssd/internal/statistic"
	"ssd/internal/structures"
)

func InitApp(cfg *structures.CliFlags) (*internal.App, error) {

	wire.Build(
		providers.NewConfigProvider,
		providers.NewLogProvider,
		providers.NewMetricsProvider,
		providers.NewInstrumentedCacheProvider,

		statistic.NewZstdCompressor,
		services.NewStatisticService,
		statistic.NewFileManager,
		statistic.NewScheduler,
		controllers.NewApiController,
		controllers.NewHealthController,
		internal.InitRoutes,
		internal.NewApp,
	)

	return nil, nil
}
