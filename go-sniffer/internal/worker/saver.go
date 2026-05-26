package worker

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"go-sniffer/internal/db"
)

// Global atomic counter tracking packets successfully committed to the database.
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
                    // 1. Start the stopwatch right before diving into the database
                    writeStart := time.Now()

					if err := db.BulkDatabaseWrite(ctx, currentBatch); err != nil {
						log.Error("critical database batch write failure; triggering pipeline abort", "error", err.Error())
						cancel() // Hit the emergency brake for all other goroutines
						return
					}
					
                    // 2. Calculate exactly how long the database storage engine took to respond
                    writeDuration := time.Since(writeStart)
                    
					batchSize := len(currentBatch)
					localSaved += batchSize
					
					atomic.AddInt64(&TotalSavedPackets, int64(batchSize))
                    currentBatch = currentBatch[:0] // Clear slice buffer safely

                    // 3. ADAPTIVE BRAKE: If the database write took longer than 25ms,
                    // the host storage driver (Docker/macOS) is starting to bottleneck.
                    // Sleep for the exact write duration to apply hardware-safe backpressure.
                    if writeDuration > 25*time.Millisecond {
                        log.Warn("hardware I/O bottleneck detected; applying backpressure throttle", 
                            "db_write_time", writeDuration,
                        )
                        
                        // We use a select block to ensure the sleep can be aborted if the app shuts down mid-nap
                        select {
                        case <-time.After(writeDuration):
                        case <-ctx.Done():
                            return
                        }
                    }
                }
            }
        }
    }
}