package pcap

import (
	"bufio"
	"io"
	"log/slog" // Upgraded to structured logging
	"os"

	"github.com/google/gopacket/pcapgo"
)

// PcapReader cleanly opens a file, counts its packets, and logs telemetry metrics
func PcapReader(path string) int {
	f, err := os.Open(path)
	if err != nil {
		slog.Error("failed to open pcap file for tracking pass", "file_path", path, "error", err.Error())
		return 0
	}
	defer f.Close()

	bufferedReader := bufio.NewReader(f)
	reader, err := pcapgo.NewReader(bufferedReader)
	if err != nil {
		slog.Error("failed to instantiate pcap file reader engine", "file_path", path, "error", err.Error())
		return 0
	}

	count := 0
	for {
		_, _, err := reader.ReadPacketData()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Non-fatal skip for single corrupted packets within an otherwise valid file structure.
			// Set to DEBUG level so it stays quiet during standard runs.
			slog.Debug("skipping unreadable or corrupted packet data", "file_path", path, "error", err.Error())
			continue
		}
		count++
	}
	
	slog.Debug("isolated pcap file telemetry count complete", "file_path", path, "packet_count", count)
	return count
}