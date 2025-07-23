package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"service-io/internal/config"
	"service-io/internal/core/devices"
	dockercli "service-io/internal/core/docker"
	ncore "service-io/internal/core/nats"
	api "service-io/internal/delivery/http"

	"github.com/rs/zerolog"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().
		Str("svc", "service-io").Logger()

	cfg := config.MustLoad()
	log.Info().Interface("cfg", cfg).Msg("boot")

	nc, err := ncore.New(cfg.NATSURL, log)
	if err != nil {
		log.Fatal().Err(err).Msg("nats connect")
	}
	defer nc.Close()

	dcli, err := dockercli.New(log)
	if err != nil {
		log.Fatal().Err(err).Msg("docker connect")
	}

	mgr, err := devices.New(nc, cfg.DevBucket, cfg.NATSURL, cfg.Adapters, dcli, log)
	if err != nil {
		log.Fatal().Err(err).Msg("manager init")
	}

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
	_ = srv.Shutdown(context.Background())
	log.Info().Msg("bye")
}
