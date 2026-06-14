package worker

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"go-sniffer/internal/storage"
)

// Launcher implements api.PipelineLauncher using the real worker pool.
type Launcher struct {
	DB              storage.DBStore
	ReaderWorkers   int
	SaverWorkers    int
	PreCheckWorkers int
}

func (l *Launcher) Launch(ctx context.Context, jobID pgtype.UUID, paths []string) {
	ProcessWithPool(ctx, l.DB, jobID, paths, l.ReaderWorkers, l.SaverWorkers, l.PreCheckWorkers)
}

func ProcessWithPool(
	ctx context.Context,
	DB storage.DBStore,
	jobID pgtype.UUID,
	filePaths []string,
	workerReaderCount int,
	workerSaverCount int,
	workerPreCheckCount int,
) {
	start := time.Now()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var totalRead, totalSaved int64

	slog.Debug("Starting pipeline", "total_files", len(filePaths))

	files := make(chan string, len(filePaths))
	validFiles := make(chan string, len(filePaths))
	packetStream := make(chan []storage.PacketRow, 100)
	finalCounts := make(chan int, workerSaverCount)

	var checksumWg sync.WaitGroup
	for w := 1; w <= workerPreCheckCount; w++ {
		checksumWg.Add(1)
		go storage.CheckAndInsertSourceFile(ctx, DB, jobID, files, validFiles, &checksumWg)
	}

	// Feed all file paths into the checksum workers.
	for _, path := range filePaths {
		files <- path
	}
	close(files)

	go func() {
		checksumWg.Wait()
		close(validFiles)
	}()

	// Start saver and reader workers.
	var readerWg sync.WaitGroup
	for w := 1; w <= workerSaverCount; w++ {
		go PacketSaverWorker(ctx, DB, jobID, &totalSaved, w, packetStream, finalCounts, cancel)
	}
	for w := 1; w <= workerReaderCount; w++ {
		readerWg.Add(1)
		go PcapWorker(ctx, DB, jobID, &totalRead, w, validFiles, packetStream, &readerWg)
	}

	go func() {
		readerWg.Wait()
		close(packetStream)
	}()

	totalPackets := 0
	for i := 0; i < workerSaverCount; i++ {
		totalPackets += <-finalCounts
	}

	now := time.Now()
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer updateCancel()
	storage.UpdateJobStatus(updateCtx, DB, jobID, "COMPLETED", &now) //nolint:errcheck

	durationSeconds := time.Since(start).Seconds()
	finalReadCount := atomic.LoadInt64(&totalRead)
	finalSavedCount := atomic.LoadInt64(&totalSaved)

	slog.Debug("pipeline complete", "total_packets", totalPackets, "total_files", len(filePaths), "total_duration", time.Since(start).String())

	var readPPS, writePPS float64
	if durationSeconds > 0 {
		readPPS = float64(finalReadCount) / durationSeconds
		writePPS = float64(finalSavedCount) / durationSeconds
	}

	slog.Info("pipeline processing performance report",
		"status", "success",
		"total_files", len(filePaths),
		"duration_seconds", durationSeconds,
		"total_packets_read", finalReadCount,
		"packets_read_per_sec", readPPS,
		"total_packets_inserted", finalSavedCount,
		"packets_saved_per_sec", writePPS,
	)
}
