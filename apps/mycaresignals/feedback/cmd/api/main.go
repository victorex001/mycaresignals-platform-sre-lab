package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qr-feedback-api/internal/config"
	"qr-feedback-api/internal/integration"
	"qr-feedback-api/internal/server"
	"qr-feedback-api/internal/storage"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("load configuration")
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("open db")
	}
	defer db.Close()
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)

	if err := db.Ping(); err != nil {
		log.Fatal().Err(err).Msg("ping db")
	}

	engine := server.NewRouter(cfg, db)
	dispatcher := integration.NewDispatcher(storage.NewRepo(db, cfg.SecretKey), cfg.TherapeuticIntakeURL, cfg.TherapeuticIntakeToken)
	dispatchContext, cancelDispatch := context.WithCancel(context.Background())
	defer cancelDispatch()
	go dispatcher.Run(dispatchContext)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Info().Str("port", cfg.Port).Msg("listening")
		serverErrors <- srv.ListenAndServe()
	}()

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-shutdownSignal.Done():
		log.Info().Msg("shutdown signal received")
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("listen")
		}
		return
	}
	cancelDispatch()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed")
		if closeErr := srv.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("forced server close failed")
		}
	}
}
