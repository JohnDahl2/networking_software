package main

import (
	"os"
    "context"
    "log/slog"
    "strings"
    "net/http"

    "go-sniffer/internal/api"
    "go-sniffer/internal/storage"
)

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

    ctx := context.Background()

    slog.Info("Connecting to TimescaleDB Pool...")
    DB, err := storage.InitDB(ctx, connString)
    if err != nil {
        slog.Error("Failed to initialize database pool, terminating", "error", err)
        os.Exit(1)
    }

    myServer := &api.Server{
        DB:    DB,
        Store: &storage.Store{DB: DB},
        Ctx: ctx,
        Jobs: make(map[string]context.CancelFunc),
    }

    slog.Info("Starting local API server", "port", 3000)


    serverErr := make(chan error, 1)
    go func () {
        if err := http.ListenAndServe(":3000", myServer.Router()); err != nil {
            serverErr <- err
        }
    } ()
    err = <-serverErr
    slog.Error("API server failed to start or crashed", "error", err)
    os.Exit(1)
}