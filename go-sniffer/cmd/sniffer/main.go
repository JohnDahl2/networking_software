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
    "net/http"


    _ "github.com/jackc/pgx/v5/stdlib" 
    "go-sniffer/internal/api"
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
    if err := db.InitDB(ctx, connString); err != nil {
        slog.Error("Failed to initialize database pool, terminating", "error", err)
        os.Exit(1)
    }

    demoMode := os.Getenv("DEMO_MODE")
	if demoMode == "true" {
		pcap.PackageGenerator()
	}
    myServer := &api.Server{
        DB: db.DB, 
    }

    slog.Info("Starting local API server", "port", 3000)

    // 2. Let ListenAndServe block main() naturally. No go func, no select{} needed!
    if err := http.ListenAndServe(":3000", myServer.Router()); err != nil {
        slog.Error("API server failed to start or crashed", "error", err)
        os.Exit(1)
    }
	//ProcessWithPool(2, 2)
    select {}
}