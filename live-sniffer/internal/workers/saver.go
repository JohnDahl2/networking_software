package workers

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"live-sniffer/internal/storage"
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
	totalSaved *int64,
	id int,
	packetStream <-chan []storage.PacketRow,
	results chan<- int,
	cancel context.CancelFunc,
	onFailure func(),
) {
	var localSaved int
	log := slog.With("saver_id", id)

	currentBackoff := minBackoff

	defer func() {
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
				return
			}

			batchSize := len(incomingBatch)

			writeStart := time.Now()

			if err := storage.BulkDatabaseCopy(ctx, DB, incomingBatch); err != nil {
				onFailure()
				log.Error("critical database batch write failure; triggering pipeline abort", "error", err.Error())
				cancel()
				return
			}

			writeDuration := time.Since(writeStart)

			localSaved += batchSize
			atomic.AddInt64(totalSaved, int64(batchSize))

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
