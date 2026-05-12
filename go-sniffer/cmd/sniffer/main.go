package main

import (
	"os"
	"fmt"
	"time"
	"path/filepath"

    // "go-sniffer/internal/db"
	"go-sniffer/internal/pcap" 
)

func worker(id int, jobs <-chan string, results chan<- int) {
    for path := range jobs {
        fmt.Printf("Worker %d: starting file %s\n", id, path)
        
        // Use your existing PcapReader logic here
        count := pcap.PcapReader(path)
        
        fmt.Printf("Worker %d: finished %s with %d packets\n", id, path, count)
        
        // Send the result back through the results channel
        results <- count
    }
}


func ProcessWithPool(workerCount int) {

	start := time.Now()

	pcapDir := os.Getenv("PCAP_SOURCE_DIR")
    if pcapDir == "" {
        pcapDir = "data/dumb_data"
    }

    absPath, _ := filepath.Abs(pcapDir)
    fmt.Printf("DEBUG: Looking for pcaps in: %s\n", absPath)

    filePaths, err := filepath.Glob(filepath.Join(pcapDir, "*.pcap"))
    if err != nil {
        fmt.Printf("Glob error: %v\n", err)
        return
    }

    if len(filePaths) == 0 {
        fmt.Printf("Warning: No files found in %s\n", pcapDir)
        return
    }
    // 1. Create the channels
    jobs := make(chan string, len(filePaths)) // Buffered to hold all files
    results := make(chan int, len(filePaths))

    // 2. Start the workers
    for w := 1; w <= workerCount; w++ {
        go worker(w, jobs, results)
    }

    // 3. Send the jobs (file paths) into the channel
    for _, path := range filePaths {
        jobs <- path
    }
    close(jobs) // This tells workers: "No more work coming!"

    // 4. Collect the results
    totalPackets := 0
    for a := 1; a <= len(filePaths); a++ {
        totalPackets += <-results
    }

    fmt.Printf("--- Total Packets across all files: %d ---\n", totalPackets)
	duration := time.Since(start)
    fmt.Printf("--- Processed %d files in %v ---\n", len(filePaths), duration)
}

func main() {
	// db.ConnectDB()
	// if db.DB != nil {
    //     defer db.DB.Close()
    // }

	demo_mode := os.Getenv("DEMO_MODE")
	if demo_mode == "true" {
		pcap.PackageGenerator()
	}
	pcap.ProcessAllPcaps()
	ProcessWithPool(2)
}