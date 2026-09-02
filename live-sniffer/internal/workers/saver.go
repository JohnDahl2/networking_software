package workers

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"live-sniffer/internal/storage"

	"github.com/confluentinc/confluent-kafka-go/kafka"
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
	consumer *kafka.Consumer, 
	cancel context.CancelFunc,
	onFailure func(),
) {
	var localSaved int
	log := slog.With("saver_id", id)

	currentBackoff := minBackoff

	for {
		select {
		case <-ctx.Done():
			log.Warn("pipeline context cancelled; initiating graceful shutdown sequence")
			return
		default: 
			msg, err := consumer.ReadMessage(100 * time.Millisecond)
			if err != nil {
				if kafkaErr, ok := err.(kafka.Error); ok && kafkaErr.Code() == kafka.ErrTimedOut {
					continue
				}
				log.Warn("kafka read error", "error", err)
				continue  // don't return on read errors, keep trying
			}
			var batch []storage.PacketRow
			if err := json.Unmarshal(msg.Value, &batch); err != nil {
				log.Warn("failed to unmarshal batch", "error", err)
				continue
			}
			batchSize := len(batch)

			writeStart := time.Now()

			if err := storage.BulkDatabaseCopy(ctx, DB, batch); err != nil {
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
