package worker

import (
	"os"
	"time"
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"go-sniffer/internal/storage"
)




func ProcessWithPool(
	ctx context.Context,
	DB *pgxpool.Pool,
	workerReaderCount int, 
	workerSaverCount int,
	) {
	start := time.Now()

    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

	pcapDir := os.Getenv("PCAP_SOURCE_DIR")
    if pcapDir == "" {
        pcapDir = "data/dumb_data"
    }

    absPath, _ := filepath.Abs(pcapDir)
    slog.Debug("Looking for pcaps in", "File:", absPath)

    filePaths, _ := filepath.Glob(filepath.Join(pcapDir, "*.pcap"))
    jobs := make(chan string, len(filePaths)) 
    packetStream := make(chan []storage.PacketRow, 100) 
    finalCounts := make(chan int, workerSaverCount)

    var wg sync.WaitGroup

    for w := 1; w <= workerSaverCount; w++ {
        go PacketSaverWorker(ctx, DB, w, packetStream, finalCounts, cancel)
    }
    for w := 1; w <= workerReaderCount; w++ {
        wg.Add(1)
        go PcapWorker(ctx, w, jobs, packetStream, &wg)
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
    finalReadCount  := atomic.LoadInt64(&TotalPacketsRead)
    finalSavedCount := atomic.LoadInt64(&TotalSavedPackets)

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
