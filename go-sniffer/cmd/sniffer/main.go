package main

import (
	"os"
    "time"
    "path/filepath"
    "fmt"
    "sync"
    "go-sniffer/internal/db"
	"go-sniffer/internal/pcap"
    "go-sniffer/internal/worker"
)


func ProcessWithPool(workerReaderCount int, workerSaverCount int) {

	start := time.Now()

	pcapDir := os.Getenv("PCAP_SOURCE_DIR")
    if pcapDir == "" {
        pcapDir = "data/dumb_data"
    }

    absPath, _ := filepath.Abs(pcapDir)
    fmt.Printf("DEBUG: Looking for pcaps in: %s\n", absPath)

    filePaths, _ := filepath.Glob(filepath.Join(pcapDir, "*.pcap"))
    jobs := make(chan string, len(filePaths)) 
    packetStream := make(chan []string, 100) 
    finalCounts := make(chan int, workerSaverCount)

    var wg sync.WaitGroup

    for w := 1; w <= workerSaverCount; w++ {
        go worker.PacketSaverWorker(w, packetStream, finalCounts)
    }
    for w := 1; w <= workerReaderCount; w++ {
        wg.Add(1)
        go worker.PcapWorker(w, jobs, packetStream, &wg)
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

    fmt.Printf("--- Total Packets across all files: %d ---\n", totalPackets)
    duration := time.Since(start)
    fmt.Printf("--- Processed %d files in %v ---\n", len(filePaths), duration)
}


func main() {
    db.ConnectDB()
	demo_mode := os.Getenv("DEMO_MODE")
	if demo_mode == "true" {
		pcap.PackageGenerator()
	}
	ProcessWithPool(2, 2)
}