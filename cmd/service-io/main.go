package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	gormadapter "service-io/internal/adapters/gorm"
	ncore "service-io/internal/adapters/nats"
	"syscall"

	"service-io/internal/config"
	"service-io/internal/core/devices"
	dockercli "service-io/internal/core/docker"
	api "service-io/internal/delivery/http"

	_ "service-io/docs" // Import the generated docs

	"github.com/rs/zerolog"
)

// @title           service-io API
// @version         1.0
// @description     This is the API for the service-io device onboarding microservice.
// @host            localhost:9090
// @BasePath        /
func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().
		Str("svc", "service-io").Logger()

	cfg := config.MustLoad()
	log.Info().Interface("cfg", cfg).Msg("boot")

	db, err := gormadapter.New(cfg.DatabaseDSN, log)
	if err != nil {
		log.Fatal().Err(err).Msg("gorm connect")
	}

	nc, err := ncore.New(cfg.NATSURL, log)
	if err != nil {
		log.Fatal().Err(err).Msg("nats connect")
	}
	defer nc.Close()

	dcli, err := dockercli.New(log)
	if err != nil {
		log.Fatal().Err(err).Msg("docker connect")
	}

	mgr, err := devices.New(db, nc, cfg.NATSURL, cfg.Adapters, dcli, log)
	if err != nil {
		log.Fatal().Err(err).Msg("manager init")
	}

	// --- ADD THIS BLOCK ---
	// Restart any devices that were running before shutdown.
	if err := mgr.RestartRunningDevices(context.Background()); err != nil {
		log.Error().Err(err).Msg("error during device restart")
	}
	// ----------------------

	handler := api.New(mgr, log)
	srv := &http.Server{Addr: cfg.ListenAddr, Handler: handler}

	// graceful-shutdown
	ctx, stop := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info().Str("listen", cfg.ListenAddr).Msg("HTTP up")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http")
		}
	}()

	<-ctx.Done()

	// --- Shutdown Logic ---
	log.Info().Msg("shutting down server...")
	_ = srv.Shutdown(context.Background())

	// --- Cleanup Logic ---
	if err := mgr.CleanupAdapters(context.Background()); err != nil {
		log.Error().Err(err).Msg("error during adapter cleanup")
	}

	log.Info().Msg("bye")
}
