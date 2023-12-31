package internal

import (
	"context"
	"github.com/spf13/cast"
	"net/http"
	"os"
	"os/signal"
	"ssd/internal/controllers"
	"ssd/internal/providers"
	"ssd/internal/statistic/interfaces"
	"ssd/internal/structures"
	"syscall"
	"time"
)

type App struct {
	WebServer *http.Server
}

func NewApp(apiController *controllers.ApiController, scheduler interfaces.SchedulerInterface, conf *structures.Config, logger providers.Logger, router providers.RouterProviderInterface) (*App, error) {
	//routes
	for _, route := range router.GetRoutes() {
		http.Handle(route.Url, route.Handler)
	}

	//restore queues from file
	logger.Infof(providers.TypeApp, "Starting %s", conf.AppName)
	err := scheduler.Restore()
	if err != nil {
		logger.Errorf(providers.TypeApp, "Restore error: %s", err)
	}
	// Create a simple HTTP server
	app := &App{
		WebServer: &http.Server{
			Addr: conf.WebServer.Host + ":" + cast.ToString(conf.WebServer.Port),
		},
	}
	//run scheduler
	scheduler.Init()
	// Run our server in a goroutine so that it doesn't block
	go func() {
		logger.Infof(providers.TypeApp, "Listening HTTP clients on %s:%d", conf.WebServer.Host, conf.WebServer.Port)
		if err = app.WebServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	// Listen for SIGINT (aka Ctrl+C) signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT)

	// Block until we receive our signal
	<-stop

	// Create a deadline to wait for current connections to finish
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline before forcing a shutdown
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
