package pcap

import (
	"os"
	"bufio"
	"fmt"
	"time"
	"path/filepath"
	"strconv" 

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcapgo"
	"github.com/google/gopacket/layers"
)

var dumb_data_folder string = "./data/dumb_data"

func ensureDir(dirName string) {
	_, err := os.Stat(dirName)

	if os.IsNotExist(err) {
		fmt.Printf("Folder %s not found. Creating it...\n", dirName)
		
		err := os.MkdirAll(dirName, 0755)
		if err != nil {
			fmt.Printf("Failed to create folder: %v\n", err)
			return
		}
	} else {
		fmt.Printf("Folder %s already exists.\n", dirName)
	}
}

func GenerateDummyPcap(filename string, packetCount int) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	bufferedWriter := bufio.NewWriter(f)

	writer := pcapgo.NewWriter(bufferedWriter)
	writer.WriteFileHeader(65536, layers.LinkTypeEthernet)

	ethernetLayer := &layers.Ethernet{
		SrcMAC:       []byte{0x00, 0x0F, 0xAA, 0xBB, 0xCC, 0xDD},
		DstMAC:       []byte{0x00, 0x0F, 0x11, 0x22, 0x33, 0x44},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ipLayer := &layers.IPv4{
		SrcIP: []byte{127, 0, 0, 1},
		DstIP: []byte{127, 0, 0, 1},
	}
	
	buffer := gopacket.NewSerializeBuffer()
	options := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	fmt.Printf("Generating %s with %d packets...\n", filename, packetCount)

	for i := 0; i < packetCount; i++ {
		gopacket.SerializeLayers(buffer, options, ethernetLayer, ipLayer, gopacket.Payload([]byte("dummy data")))
		
		info := gopacket.CaptureInfo{
			Timestamp:      time.Now(),
			CaptureLength:  len(buffer.Bytes()),
			Length:         len(buffer.Bytes()),
		}

		if err := writer.WritePacket(info, buffer.Bytes()); err != nil {
			return err
		}
	}
	if err := bufferedWriter.Flush(); err != nil {
        return err
    }

	return f.Sync()
}

func generatePcapFiles(number int, sizeMb int, dataDir string) error {
    packetsPerMb := 1250
    totalPackets := sizeMb * packetsPerMb

    ensureDir(dataDir)
    for i := 1; i <= number; i++ {
        fileName := filepath.Join(dataDir, fmt.Sprintf("test_batch_%d.pcap", i))

        err := GenerateDummyPcap(fileName, totalPackets)
        if err != nil {
            return fmt.Errorf("failed on file %d: %w", i, err)
        }
        
        fmt.Printf("Successfully generated %s (%d Mb)\n", fileName, sizeMb)
    }
    return nil
}

func PackageGenerator() {
    countStr := os.Getenv("GENERATOR_COUNT")
    sizeStr := os.Getenv("GENERATOR_SIZE_MB")

    count, _ := strconv.Atoi(countStr)
    if count == 0 { count = 3 } 

    size, _ := strconv.Atoi(sizeStr)
    if size == 0 { size = 10 } 

    fmt.Println("--- Starting PCAP Generation Job ---")
    err := generatePcapFiles(count, size, dumb_data_folder)
    if err != nil {
        fmt.Printf("Generator Error: %v\n", err)
        return
    }
    fmt.Println("--- Generation Complete ---")
}