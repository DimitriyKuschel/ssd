package internal

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
	"os/signal"
	"ssd/internal/controllers"
	"ssd/internal/providers"
	"ssd/internal/statistic/interfaces"
	"ssd/internal/structures"
	"strconv"
	"syscall"
	"time"
)

type App struct {
	WebServer *http.Server
}

func NewApp(apiController *controllers.ApiController, healthController *controllers.HealthController, scheduler interfaces.SchedulerInterface, conf *structures.Config, logger providers.Logger, router providers.RouterProviderInterface, metrics providers.MetricsProviderInterface) (*App, error) {
	// Inner mux: API routes
	apiMux := http.NewServeMux()
	for _, route := range router.GetRoutes() {
		apiMux.Handle(route.Url, route.Handler)
	}

	// Wrap API routes with metrics middleware
	instrumentedAPI := providers.MetricsMiddleware(metrics, apiMux)

	// Outer mux: infrastructure + instrumented API
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthController.Health)
	if conf.Metrics.Enabled {
		mux.Handle("/metrics", promhttp.Handler())
	}
	mux.Handle("/", instrumentedAPI)

	logger.Infof(providers.TypeApp, "Starting %s", conf.AppName)
	err := scheduler.Restore()
	if err != nil {
		logger.Errorf(providers.TypeApp, "Restore error: %s", err)
	}

	app := &App{
		WebServer: &http.Server{
			Addr:         conf.WebServer.Host + ":" + strconv.Itoa(conf.WebServer.Port),
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	scheduler.Init()

	serverErr := make(chan error, 1)
	go func() {
		logger.Infof(providers.TypeApp, "Listening HTTP clients on %s:%d", conf.WebServer.Host, conf.WebServer.Port)
		if err := app.WebServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-stop:
		logger.Infof(providers.TypeApp, "Shutdown signal received")
	case err := <-serverErr:
		return nil, fmt.Errorf("server error: %w", err)
	}

	scheduler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = app.WebServer.Shutdown(ctx); err != nil {
		return nil, err
	}
	err = scheduler.Persist()
	if err != nil {
		return nil, err
	}
	logger.Infof(providers.TypeApp, "gracefully stopped")
	return app, nil
}
