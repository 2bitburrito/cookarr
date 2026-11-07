package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hpalmer/cookarr/internal/api"
	"github.com/hpalmer/cookarr/internal/config"
	"github.com/hpalmer/cookarr/internal/fetch"
	"github.com/hpalmer/cookarr/internal/store"
)

func main() {
	if err := serve(); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := cfg.Logger
	slog.SetDefault(log)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	db, err := store.Open(ctx, cfg.DBDriver, cfg.DBPath, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return err
	}

	defer func() {
		if err := db.Close(); err != nil {
			log.Error("close database", "err", err)
		}
	}()

	if cfg.AuthDisabled {
		log.Warn("AUTH_DISABLED is set: every request is treated as the owner")
	}
	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: api.New(api.Config{
			Store:        db,
			Fetcher:      fetch.New(cfg.MaxBodyBytes, false),
			Logger:       log,
			AuthDisabled: cfg.AuthDisabled,
			MaxBodyBytes: cfg.MaxBodyBytes,
		}),
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)

	go func() {
		log.Info("listening", "addr", cfg.Addr, "db_driver", cfg.DBDriver)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	log.Info("shutdown complete")
	return nil
}
