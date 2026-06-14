package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go-sniffer/internal/api"
	"go-sniffer/internal/storage"
	"go-sniffer/internal/worker"
	"go-sniffer/migrations"
)

type Config struct {
	ReaderWorkers   int
	SaverWorkers    int
	PreCheckWorkers int
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func main() {
	var programLevel slog.Level
	switch strings.ToUpper(os.Getenv("LOG_LEVEL")) {
	case "DEBUG":
		programLevel = slog.LevelDebug
	case "WARN":
		programLevel = slog.LevelWarn
	case "ERROR":
		programLevel = slog.LevelError
	default:
		programLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: programLevel}))
	slog.SetDefault(logger)

	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		connString = "postgres://postgres:postgres@localhost:5432/sniffer?sslmode=disable"
	}

	slog.Info("Connecting to TimescaleDB Pool...")
	DB, err := storage.InitDB(context.Background(), connString, migrations.Files)
	if err != nil {
		slog.Error("Failed to initialize database pool, terminating", "error", err)
		os.Exit(1)
	}

	cfg := Config{
		ReaderWorkers:   getEnvInt("WORKER_READERS", 2),
		SaverWorkers:    getEnvInt("WORKER_SAVERS", 2),
		PreCheckWorkers: getEnvInt("WORKER_PRECHECKS", 2),
	}

	myServer := &api.Server{
		Store:  &storage.Store{DB: DB},
		Packet: &api.PacketStore{DB: DB},
		Launcher: &worker.Launcher{
			DB:              DB,
			ReaderWorkers:   cfg.ReaderWorkers,
			SaverWorkers:    cfg.SaverWorkers,
			PreCheckWorkers: cfg.PreCheckWorkers,
		},
		Jobs:             make(map[string]context.CancelFunc),
		GlobFn:           filepath.Glob,
		DefaultSourceDir: os.Getenv("FILE_FOLDER"),
	}

	srv := &http.Server{
		Addr:    ":3000",
		Handler: myServer.Router(),
	}

	slog.Info("Starting local API server", "port", 3000)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err = <-serverErr:
		slog.Error("API server failed to start or crashed", "error", err)
		DB.Close()
		os.Exit(1)
	case <-ctx.Done():
		slog.Info("shutdown signal received, draining connections...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
		DB.Close()
		slog.Info("server stopped cleanly")
	}
}
