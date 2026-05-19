package pcap

import (
	"bufio"
	"fmt"
	"log/slog" // Upgraded to structured logging
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

var dumb_data_folder string = "./data/dumb_data"

func ensureDir(dirName string) {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		slog.Info("target directory not found; creating it now", "directory", dirName)
		if err := os.MkdirAll(dirName, 0755); err != nil {
			slog.Error("failed to create directory path", "directory", dirName, "error", err.Error())
		}
	}
}

// GenerateDummyPcap handles the heavy lifting for a single file
func GenerateDummyPcap(filename string, packetCount int, wg *sync.WaitGroup) {
	defer wg.Done()

	f, err := os.Create(filename)
	if err != nil {
		slog.Error("failed to create dummy pcap target file", "file_path", filename, "error", err.Error())
		return
	}
	defer f.Close()

	// Use a large 1MB buffer to maximize SSD write speed
	bufferedWriter := bufio.NewWriterSize(f, 1024*1024)
	writer := pcapgo.NewWriter(bufferedWriter)
	_ = writer.WriteFileHeader(65536, layers.LinkTypeEthernet)

	buffer := gopacket.NewSerializeBuffer()
	options := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	
	// Pre-set time to avoid calling time.Now() millions of times
	timestamp := time.Now()

	for i := 0; i < packetCount; i++ {
		buffer.Clear() // Crucial: prevents memory growth

		// Randomize IPs so TimescaleDB builds real indexes
		srcIP := []byte{192, 168, 1, byte(rand.Intn(254))}
		dstIP := []byte{10, 0, 0, byte(rand.Intn(254))}

		eth := &layers.Ethernet{
			SrcMAC:       []byte{0x00, 0x0F, 0xAA, 0xBB, 0xCC, 0xDD},
			DstMAC:       []byte{0x00, 0x0F, 0x11, 0x22, 0x33, 0x44},
			EthernetType: layers.EthernetTypeIPv4,
		}
		ip := &layers.IPv4{
			Version: 4, TTL: 64, Protocol: layers.IPProtocolTCP,
			SrcIP: srcIP, DstIP: dstIP,
		}

		_ = gopacket.SerializeLayers(buffer, options, eth, ip, gopacket.Payload([]byte("rugged-refined-data")))
		
		info := gopacket.CaptureInfo{
			Timestamp:      timestamp,
			CaptureLength:  len(buffer.Bytes()),
			Length:         len(buffer.Bytes()),
		}
		
		_ = writer.WritePacket(info, buffer.Bytes())
		timestamp = timestamp.Add(time.Microsecond)
	}

	_ = bufferedWriter.Flush()
	_ = f.Sync()
	
	// Clean structural feedback when a worker finishes its disk writes
	slog.Info("dummy pcap file generation complete", "file_name", filepath.Base(filename))
}

func PackageGenerator() {
	count, _ := strconv.Atoi(os.Getenv("GENERATOR_COUNT"))
	size, _ := strconv.Atoi(os.Getenv("GENERATOR_SIZE_MB"))

	if count == 0 { count = 3 }
	if size == 0 { size = 10 }

	ensureDir(dumb_data_folder)
	
	matches, _ := filepath.Glob(filepath.Join(dumb_data_folder, "*.pcap"))
	if len(matches) >= count {
		slog.Info("existing dummy data meets requirements; skipping mock generation", 
			"found_files", len(matches), 
			"required_count", count,
		)
		return
	}

	needed := count - len(matches)
	packetsPerFile := size * 1250 // Roughly 1MB = 1250 packets with overhead

	slog.Info("starting dummy file generation job", 
		"files_remaining_to_generate", needed, 
		"target_file_size_mb", size,
	)
	
	var wg sync.WaitGroup
	for i := 1; i <= needed; i++ {
		wg.Add(1)
		fileName := filepath.Join(dumb_data_folder, fmt.Sprintf("test_batch_%d.pcap", len(matches)+i))
		// Launch each file generation in its own goroutine
		go GenerateDummyPcap(fileName, packetsPerFile, &wg)
	}

	wg.Wait()
	slog.Info("all dummy data generation files are ready for pipeline stress testing")
}