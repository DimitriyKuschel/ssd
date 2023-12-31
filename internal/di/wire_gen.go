// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package di

import (
	"ssd/internal"
	"ssd/internal/controllers"
	"ssd/internal/providers"
	"ssd/internal/services"
	"ssd/internal/statistic"
	"ssd/internal/structures"
)

// Injectors from injectors.go:

func InitApp(cfg *structures.CliFlags) (*internal.App, error) {
	config, err := providers.NewConfigProvider(cfg)
	if err != nil {
		return nil, err
	}
	logger, err := providers.NewLogProvider(config)
	if err != nil {
		return nil, err
	}
	statisticServiceInterface := services.NewStatisticService()
	apiController := controllers.NewApiController(logger, statisticServiceInterface)
	compressorInterface := statistic.NewZstdCompressor()
	fileManager := statistic.NewFileManager(compressorInterface, statisticServiceInterface, logger)
	schedulerInterface := statistic.NewScheduler(config, logger, statisticServiceInterface, fileManager)
	routerProviderInterface := internal.InitRoutes(apiController, config)
	app, err := internal.NewApp(apiController, schedulerInterface, config, logger, routerProviderInterface)
	if err != nil {
		return nil, err
	}
	return app, nil
}
