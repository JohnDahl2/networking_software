package worker

import (
	"fmt"
    "context"
    "time"
    "go-sniffer/internal/db"
)

func PacketSaverWorker(ctx context.Context, id int, packetStream <-chan []string, results chan<- int, cancel context.CancelFunc) {
    const targetBatchSize = 1000
    currentBatch := make([]db.PacketRow, 0, targetBatchSize)
    totalSaved := 0

    defer func() {
        if len(currentBatch) > 0 {
            flushCtx, flushCancel := context.WithTimeout(context.Background(), 2*time.Second)
            defer flushCancel()

            if err := db.BulkDatabaseWrite(flushCtx, currentBatch); err == nil {
                totalSaved += len(currentBatch)
            } else {
                fmt.Printf("Saver %d final emergency flush error: %v\n", id, err)
            }
        }
        results <- totalSaved
    }()

    for {
        select {
        case <-ctx.Done():
            fmt.Printf("Saver %d: Context cancelled. Initiating graceful shutdown...\n", id)
            return
        case incomingSlice, ok := <-packetStream:
            if !ok {
                return
            }
            for _, pData := range incomingSlice {
                currentBatch = append(currentBatch, db.PacketRow{
                    Timestamp: time.Now(),
                    Length:    len(pData),
                })
                if len(currentBatch) >= targetBatchSize {
                    if err := db.BulkDatabaseWrite(ctx, currentBatch); err != nil {
                        fmt.Printf("Saver %d database write failed: %v\n", id, err)
                        cancel()
                        return   
                    }
                    totalSaved += len(currentBatch)
                    currentBatch = currentBatch[:0]
                }
            }
        }
    }
}