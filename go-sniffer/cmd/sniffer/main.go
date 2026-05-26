package main

import (
	"os"
    "time"
    "path/filepath"
    "context"
    "sync"
    "log/slog"
    "sync/atomic"
    "strings"
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib" 
    "github.com/pressly/goose/v3"
    "go-sniffer/internal/db"
	"go-sniffer/internal/pcap"
    "go-sniffer/internal/worker"
)


func ProcessWithPool(workerReaderCount int, workerSaverCount int) {
	start := time.Now()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

	pcapDir := os.Getenv("PCAP_SOURCE_DIR")
    if pcapDir == "" {
        pcapDir = "data/dumb_data"
    }

    absPath, _ := filepath.Abs(pcapDir)
    slog.Debug("Looking for pcaps in", "File:", absPath)

    filePaths, _ := filepath.Glob(filepath.Join(pcapDir, "*.pcap"))
    jobs := make(chan string, len(filePaths)) 
    packetStream := make(chan []db.PacketRow, 100) 
    finalCounts := make(chan int, workerSaverCount)

    var wg sync.WaitGroup

    for w := 1; w <= workerSaverCount; w++ {
        go worker.PacketSaverWorker(ctx, w, packetStream, finalCounts, cancel)
    }
    for w := 1; w <= workerReaderCount; w++ {
        wg.Add(1)
        go worker.PcapWorker(ctx, w, jobs, packetStream, &wg)
    }
    for _, path := range filePaths {
        jobs <- path
    }
    close(jobs)

    go func() {
        wg.Wait()
        close(packetStream)
    }()

    totalPackets := 0
    for i := 0; i < workerSaverCount; i++ {
        totalPackets += <-finalCounts
    }
    
    durationSeconds := time.Since(start).Seconds()
    finalReadCount  := atomic.LoadInt64(&worker.TotalPacketsRead)
    finalSavedCount := atomic.LoadInt64(&worker.TotalSavedPackets)

    slog.Debug("All done with packets", "Total packets", totalPackets, "Total files", len(filePaths), "Total Time for processesing", time.Since(start).String())

    var readPPS, writePPS float64
    if durationSeconds > 0 {
        readPPS  = float64(finalReadCount) / durationSeconds
        writePPS = float64(finalSavedCount) / durationSeconds
    }

    slog.Info("pipeline processing performance report",
        "status",                  "success",
        "total_files",             len(filePaths),
        "duration_seconds",        durationSeconds,
        "total_packets_read",      finalReadCount,
        "packets_read_per_sec",    readPPS,       
        "total_packets_inserted",  finalSavedCount,
        "packets_saved_per_sec",   writePPS,      
    )
}

func RunMigrations(connString string) error {
    // Goose needs a standard *sql.DB connection just to execute the migration SQL
    db, err := sql.Open("pgx", connString)
    if err != nil {
        return err
    }
    defer db.Close()

    // Tell goose where your SQL files live and run them
    slog.Info("Running migrations")
    return goose.Up(db, "./migrations")
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

    // CREATE A CONTEXT FOR STARTUP
    ctx := context.Background()

    // 1. Swap db.ConnectDB() out for your new thread-safe connection pool!
    slog.Info("Connecting to TimescaleDB Pool...")
    if err := db.InitDB(ctx, connString); err != nil {
        slog.Error("Failed to initialize database pool, terminating", "error", err)
        os.Exit(1)
    }

    // 2. Run Goose migrations using the same connection string
    if err := RunMigrations(connString); err != nil {
        slog.Error("Failed running schema migrations, terminating application", "error", err)
        os.Exit(1)
    }

    demoMode := os.Getenv("DEMO_MODE")
	if demoMode == "true" {
		pcap.PackageGenerator()
	}
	
	ProcessWithPool(2, 2)
}