package main

import (
	"os"
    "time"
    "path/filepath"
    "context"
    "sync"
    "log/slog"
    "strings"
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
    packetStream := make(chan []string, 100) 
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
    slog.Debug("All done with packets", "Total packets", totalPackets, "Total files", len(filePaths), "Total Time for processesing", time.Since(start).String())
}


func main() {
    var programLevel slog.Level
    switch strings.ToUpper(os.Getenv("LOG_LEVEL")) {
	case "DEBUG":
		programLevel = slog.LevelDebug // Detailed troubleshooting info
	case "WARN":
		programLevel = slog.LevelWarn  // Warnings
	case "ERROR":
		programLevel = slog.LevelError // Critical issues
	default:
		programLevel = slog.LevelInfo  // Standard operational logs (Default)
	}
    opts := &slog.HandlerOptions{
		Level: programLevel,
	}
    logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)
    db.ConnectDB()
	demo_mode := os.Getenv("DEMO_MODE")
	if demo_mode == "true" {
		pcap.PackageGenerator()
	}
	ProcessWithPool(2, 2)
}