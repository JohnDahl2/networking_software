
package pcap

import (
    "io"
    "time"
    "log"
	"os"
	"bufio"
	"fmt"
	"path/filepath"

	"github.com/google/gopacket/pcapgo"
)


func PcapReader(path string) int {
    f, err := os.Open(path)
    if err != nil {
        log.Printf("Error opening %s: %v", path, err)
        return 0
    }
    defer f.Close()

    bufferedReader := bufio.NewReader(f)
    reader, err := pcapgo.NewReader(bufferedReader)
    if err != nil {
        log.Printf("Error creating pcap reader for %s: %v", path, err)
        return 0
    }

    count := 0
    for {
        _, _, err := reader.ReadPacketData()
        if err == io.EOF {
            break
        }
        if err != nil {
            continue // Skip corrupted packets
        }
        count++
    }
    return count
}


func ProcessAllPcaps() {
    start := time.Now()
    pathPattern := filepath.Join("data", "dumb_data", "*.pcap")
    files, err := filepath.Glob(pathPattern)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("--- Found %d files to process ---\n", len(files))

    for _, file := range files {
        fmt.Printf("Processing %s... ", file)
        
        packetCount := PcapReader(file)
        
        fmt.Printf("Done. Found %d packets.\n", packetCount)
    }
    
    duration := time.Since(start)
    fmt.Printf("--- Processed %d files in %v ---\n", len(files), duration)

}
