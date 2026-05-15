package worker

import (
	"fmt"
    "context"
    "time"
    "go-sniffer/internal/db"
)

func PacketSaverWorker(id int, packetStream <-chan []string, results chan<- int) {
    const targetBatchSize = 1000
    currentBatch := make([]db.PacketRow, 0, targetBatchSize) 
    totalSavedByWorker := 0

    for incomingSlice := range packetStream {
        for _, pData := range incomingSlice {
            currentBatch = append(currentBatch, db.PacketRow{
                Timestamp: time.Now(), 
                Length:    len(pData),
            })
        }

        if len(currentBatch) >= targetBatchSize {
            err := db.BulkDatabaseWrite(context.Background(), currentBatch)
            if err != nil {
                fmt.Printf("Saver %d: DB Error: %v\n", id, err)
            } else {
                totalSavedByWorker += len(currentBatch)
                currentBatch = currentBatch[:0] 
            }
        }
    }

    if len(currentBatch) > 0 {
        err := db.BulkDatabaseWrite(context.Background(), currentBatch)
        if err == nil {
            totalSavedByWorker += len(currentBatch)
        }
    }
    results <- totalSavedByWorker
}