package pcap

import (
	"bufio"
	"fmt"
	"log/slog"
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

type TrafficProfile struct {
    Name         string
    Proto        string 
    SrcIP        []byte
    DstIP        []byte
    SrcPort      int
    DstPort      int
    PayloadMin   int
    PayloadMax   int
    
    TCPFlags struct {
        SYN bool
        ACK bool
        RST bool
        FIN bool
    }
}

func ensureDir(dirName string) {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		slog.Info("target directory not found; creating it now", "directory", dirName)
		if err := os.MkdirAll(dirName, 0755); err != nil {
			slog.Error("failed to create directory path", "directory", dirName, "error", err.Error())
		}
	}
}

func GenerateDummyPcap(
    filename string, 
    packetCount int, 
    packetTime time.Time, 
    profiles []TrafficProfile, // Passed in from the outside like a Python list of dicts
    wg *sync.WaitGroup,
) {
    defer wg.Done()

    // -------------------------------------------------------------------------
    // STEP 1: INITIALIZE FILE STREAMING & BUFFERING
    // -------------------------------------------------------------------------
	f, err := os.Create(filename)
	if err != nil {
		slog.Error("failed to create dummy pcap target file", "file_path", filename, "error", err.Error())
		return
	}
	defer f.Close()

	// Use a large 1MB buffer to maximize SSD write speed
	bufferedWriter := bufio.NewWriterSize(f, 1024*1024)
	writer := pcapgo.NewWriter(bufferedWriter)
    
    // Write the mandatory PCAP global header at the top of the file
	_ = writer.WriteFileHeader(65536, layers.LinkTypeEthernet)

    // -------------------------------------------------------------------------
    // STEP 2: PRE-ALLOCATE BLANK PACKET LAYERS (The "Reusable Envelopes")
    // -------------------------------------------------------------------------
    // We create these once OUTSIDE the loop. Inside the loop, we just wipe 
    // and overwrite their keys so we don't stress the computer's memory.
    ethLayer := &layers.Ethernet{
        SrcMAC:       []byte{0x00, 0x0F, 0xAA, 0xBB, 0xCC, 0xDD},
        DstMAC:       []byte{0x00, 0x0F, 0x11, 0x22, 0x33, 0x44},
        EthernetType: layers.EthernetTypeIPv4,
    }

    tcpLayer := &layers.TCP{}

    // The conveyor belt where gopacket flattens the layers into binary ones and zeros
	buffer := gopacket.NewSerializeBuffer()
	options := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: false}
	
    // -------------------------------------------------------------------------
    // STEP 3: THE GENERATION LOOP
    // -------------------------------------------------------------------------
    for i := 0; i < packetCount; i++ {
        buffer.Clear()

        profile := profiles[rand.Intn(len(profiles))]
        ipLayer := &layers.IPv4{
            Version:  4, 
            TTL:      64, 
            SrcIP:    profile.SrcIP, 
            DstIP:    profile.DstIP,
        }

        // 3. Look at the protocol key ("TCP" or "UDP") and use the correct strategy switch
        switch profile.Proto {
        case "TCP":
            ipLayer.Protocol = layers.IPProtocolTCP

            *tcpLayer = layers.TCP{}
            tcpLayer.SrcPort = layers.TCPPort(profile.SrcPort)
            tcpLayer.DstPort = layers.TCPPort(profile.DstPort)
            tcpLayer.SYN = profile.TCPFlags.SYN
            tcpLayer.ACK = profile.TCPFlags.ACK
            tcpLayer.RST = profile.TCPFlags.RST
            tcpLayer.FIN = profile.TCPFlags.FIN

            if profile.PayloadMin == 0 {
                _ = gopacket.SerializeLayers(buffer, options, ethLayer, ipLayer, tcpLayer)
            } else {
                payloadBytes := make([]byte, profile.PayloadMin+rand.Intn(profile.PayloadMax-profile.PayloadMin+1))
                _ = gopacket.SerializeLayers(buffer, options, ethLayer, ipLayer, tcpLayer, gopacket.Payload(payloadBytes))
            }

        case "UDP":
            ipLayer.Protocol = layers.IPProtocolUDP

            udpLayer := &layers.UDP{
                SrcPort: layers.UDPPort(profile.SrcPort),
                DstPort: layers.UDPPort(profile.DstPort),
            }

            payloadSize := profile.PayloadMin
            if profile.PayloadMax > profile.PayloadMin {
                payloadSize = profile.PayloadMin + rand.Intn(profile.PayloadMax-profile.PayloadMin+1)
            }
            payloadBytes := make([]byte, payloadSize)
            _ = gopacket.SerializeLayers(buffer, options, ethLayer, ipLayer, udpLayer, gopacket.Payload(payloadBytes))
        }

        // ---------------------------------------------------------------------
        // STEP 4: WRITE ENCODED PACKET TO RAM BUFFER
        // ---------------------------------------------------------------------
        info := gopacket.CaptureInfo{
            Timestamp:      packetTime,
            CaptureLength:  len(buffer.Bytes()), // Size of the flattened binary data
            Length:         len(buffer.Bytes()),
        }

        _ = writer.WritePacket(info, buffer.Bytes())
        packetTime = packetTime.Add(10 * time.Millisecond)
    }

    // -------------------------------------------------------------------------
    // STEP 5: FLUSH FLUID TO DISK
    // -------------------------------------------------------------------------
    // Force whatever leftover data is chilling in the 1MB RAM buffer to write out to the SSD
	_ = bufferedWriter.Flush()
	_ = f.Sync()
	
	slog.Info("dummy pcap file generation complete", "file_name", filepath.Base(filename))
}

func PackageGenerator() {
    packet_time := time.Date(2026, time.March, 7, 14, 30, 0, 0, time.UTC)
    
    // -------------------------------------------------------------------------
    // STEP 1: DEFINE THE DICT-LIKE BLUEPRINTS (Once for the entire job)
    // -------------------------------------------------------------------------
    profiles := []TrafficProfile{
        // Profile 1: The Heavy Bulk Data Stream (Light Blue in Wireshark)
        {
            Name:       "Bulk Data Stream",
            Proto:      "UDP",
            SrcIP:      []byte{74, 125, 3, 199},  
            DstIP:      []byte{10, 5, 7, 113},    
            SrcPort:    443,
            DstPort:    61256,
            PayloadMin: 1200,
            PayloadMax: 1300, 
        },
    
        // Profile 2: The TCP Connection Reset Error (Red in Wireshark)
        {
            Name:       "TCP Connection Reset",
            Proto:      "TCP",
            SrcIP:      []byte{10, 5, 7, 113},
            DstIP:      []byte{54, 90, 39, 78},
            SrcPort:    63040,
            DstPort:    443,
            PayloadMin: 0, 
            PayloadMax: 0,
            TCPFlags: struct {
                SYN bool
                ACK bool
                RST bool
                FIN bool
            }{RST: true}, 
        },
    
        // Profile 3: Local Multicast Discovery Noise (Dark Blue in Wireshark)
        {
            Name:       "MDNS Local Discovery",
            Proto:      "UDP",
            SrcIP:      []byte{10, 5, 7, 182},
            DstIP:      []byte{224, 0, 0, 251}, 
            SrcPort:    5353,                   
            DstPort:    5353,
            PayloadMin: 150,
            PayloadMax: 250,
        },
    }

    // -------------------------------------------------------------------------
    // STEP 2: SETUP CONFIGURATION & ENV VARIABLES
    // -------------------------------------------------------------------------
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
    packetsPerFile := size * 1250

	slog.Info("starting dummy file generation job", 
		"files_remaining_to_generate", needed, 
		"target_file_size_mb", size,
	)
	
    // -------------------------------------------------------------------------
    // STEP 3: SPAWN WORKERS & PASS THE BLUEPRINTS
    // -------------------------------------------------------------------------
	var wg sync.WaitGroup
	for i := 1; i <= needed; i++ {
		wg.Add(1)
		fileName := filepath.Join(dumb_data_folder, fmt.Sprintf("test_batch_%d.pcap", len(matches)+i))
        
        // Pass the hourly timestamp offset AND the profiles list down to each worker
        calculatedTime := packet_time.Add(time.Hour * time.Duration(i))
        
        go GenerateDummyPcap(fileName, packetsPerFile, calculatedTime, profiles, &wg)
    }

	wg.Wait()
	slog.Info("all dummy data generation files are ready for pipeline stress testing")
}