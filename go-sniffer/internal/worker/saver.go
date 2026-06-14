package worker

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"go-sniffer/internal/storage"
)

const (
	baseBackoffThreshold = 100 * time.Millisecond
	minBackoff           = 25 * time.Millisecond
	maxBackoff           = 3 * time.Second
	multiplier           = 3
)

func PacketSaverWorker(
	ctx context.Context,
	DB storage.DBStore,
	jobID pgtype.UUID,
	totalSaved *int64,
	id int,
	packetStream <-chan []storage.PacketRow,
	results chan<- int,
	cancel context.CancelFunc,
) {
	var localSaved int
	log := slog.With("saver_id", id)

	currentBackoff := minBackoff

	var lastActiveBatch []storage.PacketRow
	defer func() {
		// Final emergency flush handling if channels close or context is cut
		if len(lastActiveBatch) > 0 {
			flushCtx, flushCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer flushCancel()
			now := time.Now()
			if err := storage.BulkDatabaseCopy(flushCtx, DB, lastActiveBatch); err == nil {
				batchSize := len(lastActiveBatch)
				localSaved += batchSize
				atomic.AddInt64(totalSaved, int64(batchSize))
				log.Debug("emergency database flush succeeded during shutdown", "flushed_count", batchSize)
			} else {
				storage.UpdateJobStatus(ctx, DB, jobID, "FAILED", &now) //nolint:errcheck
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
		case incomingBatch, ok := <-packetStream:
			if !ok {
				log.Debug("packet data stream closed; saver exiting normally")
				lastActiveBatch = nil
				return
			}

			lastActiveBatch = incomingBatch
			batchSize := len(incomingBatch)

			writeStart := time.Now()

			if err := storage.BulkDatabaseCopy(ctx, DB, incomingBatch); err != nil {
				now := time.Now()
				storage.UpdateJobStatus(ctx, DB, jobID, "FAILED", &now) //nolint:errcheck
				log.Error("critical database batch write failure; triggering pipeline abort", "error", err.Error())
				lastActiveBatch = nil
				cancel()
				return
			}

			writeDuration := time.Since(writeStart)

			localSaved += batchSize
			atomic.AddInt64(totalSaved, int64(batchSize))

			lastActiveBatch = nil
			if writeDuration > baseBackoffThreshold {
				currentBackoff = currentBackoff * multiplier
				if currentBackoff > maxBackoff {
					currentBackoff = maxBackoff
				}
				log.Debug("hardware I/O bottleneck detected; applying backpressure throttle",
					"db_write_time", writeDuration.Milliseconds(),
					"next_throttle_duration", currentBackoff.Milliseconds(),
				)
				select {
				case <-time.After(currentBackoff):
				case <-ctx.Done():
					return
				}
			} else {
				currentBackoff = minBackoff
			}
		}
	}
}
