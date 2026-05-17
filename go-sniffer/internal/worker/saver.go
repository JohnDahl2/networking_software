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
    totalSaved := 0 // We'll use this name consistently

    for incomingSlice := range packetStream {
        for _, pData := range incomingSlice {
            currentBatch = append(currentBatch, db.PacketRow{
                Timestamp: time.Now(),
                Length:    len(pData),
            })

            if len(currentBatch) >= targetBatchSize {
                if err := db.BulkDatabaseWrite(context.Background(), currentBatch); err != nil {
                    fmt.Printf("Saver %d error: %v\n", id, err)
                } else {
                    totalSaved += len(currentBatch)
                }
                currentBatch = currentBatch[:0]
            }
        }
    }

    // Handle the final "leftover" packets after the channel closes
    if len(currentBatch) > 0 {
        err := db.BulkDatabaseWrite(context.Background(), currentBatch)
        if err == nil {
            totalSaved += len(currentBatch) // Use totalSaved here
        } else {
            fmt.Printf("Saver %d final batch error: %v\n", id, err)
        }
    }
    
    results <- totalSaved // Send the consistent variable name
}