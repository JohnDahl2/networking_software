package worker

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"go-sniffer/internal/db"
)

// Global atomic counter tracking packets successfully committed to the database.
// This can be safely read concurrently by your upcoming Gin API routes.
var TotalSavedPackets int64

func PacketSaverWorker(ctx context.Context, id int, packetStream <-chan []string, results chan<- int, cancel context.CancelFunc) {
	const targetBatchSize = 1000
	currentBatch := make([]db.PacketRow, 0, targetBatchSize)
	var localSaved int // Tracks this specific worker's contribution for the final results channel

	log := slog.With("saver_id", id)

	defer func() {
		// Final emergency flush handling if channels close or context is cut
		if len(currentBatch) > 0 {
			flushCtx, flushCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer flushCancel()

			if err := db.BulkDatabaseWrite(flushCtx, currentBatch); err == nil {
				batchSize := len(currentBatch)
				localSaved += batchSize
				
				// Atomic increment for the emergency batch flush
				atomic.AddInt64(&TotalSavedPackets, int64(batchSize))
				
				log.Debug("emergency database flush succeeded during shutdown", "flushed_count", batchSize)
			} else {
				log.Error("emergency database flush failed during shutdown", "error", err.Error())
			}
		}
		results <- localSaved
	}()

	for {
		select {
		case <-ctx.Done():
			log.Warn("pipeline context cancelled; initiating graceful shutdown sequence")
			return
		case incomingSlice, ok := <-packetStream:
			if !ok {
				log.Debug("packet data stream closed; saver exiting normally")
				return
			}
        
			for _, pData := range incomingSlice {
				currentBatch = append(currentBatch, db.PacketRow{
					Timestamp: time.Now(),
					Length:    len(pData),
				})

				if len(currentBatch) >= targetBatchSize {
					if err := db.BulkDatabaseWrite(ctx, currentBatch); err != nil {
						log.Error("critical database batch write failure; triggering pipeline abort", "error", err.Error())
						cancel() // Hit the emergency brake for all other goroutines
						return
					}
					
					batchSize := len(currentBatch)
					localSaved += batchSize
					
					// FIXED: Thread-safe atomic addition directly adding the batch size to global stats
					atomic.AddInt64(&TotalSavedPackets, int64(batchSize))

					currentBatch = currentBatch[:0] // Clear slice memory buffer without reallocating
				}
			}
		}
	}
}