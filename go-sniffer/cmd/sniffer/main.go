package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
    "strconv"

	"go-sniffer/internal/api"
	"go-sniffer/internal/storage"
	"go-sniffer/internal/worker"
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
    var programLevel slog.Level // Need to set the logging level
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
    opts := &slog.HandlerOptions{
		Level: programLevel,
	}
    logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)
    connString := os.Getenv("DATABASE_URL")
    if connString == "" {
        connString = "postgres://postgres:postgres@localhost:5432/sniffer?sslmode=disable"
    }

    slog.Info("Connecting to TimescaleDB Pool...")
    DB, err := storage.InitDB(context.Background(), connString)
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
        DB:     DB,
        Store:  &storage.Store{DB: DB},
        Launcher: &worker.Launcher{
            DB:              DB,
            ReaderWorkers:   cfg.ReaderWorkers,
            SaverWorkers:    cfg.SaverWorkers,
            PreCheckWorkers: cfg.PreCheckWorkers,
        },
        Jobs:   make(map[string]context.CancelFunc),
        GlobFn: filepath.Glob,
    }

    slog.Info("Starting local API server", "port", 3000)

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    
    serverErr := make(chan error, 1)
    go func() {
        if err := http.ListenAndServe(":3000", myServer.Router()); err != nil {
            serverErr <- err
        }
    }()
    
    select {
    case err = <-serverErr:
        slog.Error("API server failed to start or crashed", "error", err)
        DB.Close()
        os.Exit(1)
    case <-ctx.Done():
        slog.Info("shutdown signal received, exiting cleanly")
        DB.Close()
    }
}