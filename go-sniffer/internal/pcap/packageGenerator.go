package pcap

import (
	"bufio"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"time"

     "golang.org/x/sync/errgroup"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

const DefaultDumbDataFolder = "./data/dumb_data"

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

func ensureDir(dirName string) error {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		slog.Info("target directory not found; creating it now", "directory", dirName)
		if err := os.MkdirAll(dirName, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dirName, err)
		}
	}
    return nil
}

func GenerateDummyPcap(
    filename string, 
    packetCount int, 
    packetTime time.Time, 
    profiles []TrafficProfile,
) error{
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create pcap file %s: %w", filename, err)
	}
	defer f.Close()

	bufferedWriter := bufio.NewWriterSize(f, 1024*1024)
	writer := pcapgo.NewWriter(bufferedWriter)
    
	if err := writer.WriteFileHeader(65536, layers.LinkTypeEthernet); err != nil {
        return fmt.Errorf("write pcap file header: %w", err)
    }

    ethLayer := &layers.Ethernet{
        SrcMAC:       []byte{0x00, 0x0F, 0xAA, 0xBB, 0xCC, 0xDD},
        DstMAC:       []byte{0x00, 0x0F, 0x11, 0x22, 0x33, 0x44},
        EthernetType: layers.EthernetTypeIPv4,
    }

    tcpLayer := &layers.TCP{}

    // The conveyor belt where gopacket flattens the layers into binary ones and zeros
	buffer := gopacket.NewSerializeBuffer()
	options := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: false}

    for i := 0; i < packetCount; i++ {
        buffer.Clear()

        profile := profiles[rand.Intn(len(profiles))]
        ipLayer := &layers.IPv4{
            Version:  4, 
            TTL:      64, 
            SrcIP:    profile.SrcIP, 
            DstIP:    profile.DstIP,
        }

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
                if err := gopacket.SerializeLayers(buffer, options, ethLayer, ipLayer, tcpLayer); err != nil {
                    return fmt.Errorf("serialize packet %d: %w", i, err)
                }
            } else {
                payloadBytes := make([]byte, profile.PayloadMin+rand.Intn(profile.PayloadMax-profile.PayloadMin+1))
                if err := gopacket.SerializeLayers(buffer, options, ethLayer, ipLayer, tcpLayer, gopacket.Payload(payloadBytes)); err != nil {
                    return fmt.Errorf("serialize packet %d: %w", i, err)
                } 
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
            if err := gopacket.SerializeLayers(buffer, options, ethLayer, ipLayer, udpLayer, gopacket.Payload(payloadBytes)); err != nil {
                return fmt.Errorf("serialize packet %d: %w", i, err)
            } 
        }

        info := gopacket.CaptureInfo{
            Timestamp:      packetTime,
            CaptureLength:  len(buffer.Bytes()), // Size of the flattened binary data
            Length:         len(buffer.Bytes()),
        }

        if err := writer.WritePacket(info, buffer.Bytes()); err != nil {
            return fmt.Errorf("write packet %d: %w", i, err)
        }
        packetTime = packetTime.Add(10 * time.Millisecond)
    }

    if err := bufferedWriter.Flush(); err != nil {
        return fmt.Errorf("flush buffer to file: %w", err)
    }
    if err := f.Sync(); err != nil {
        return fmt.Errorf("fsync pcap file: %w", err)
    }
	
	slog.Info("dummy pcap file generation complete", "file_name", filepath.Base(filename))
    return nil
}

func PackageDumbGenerator(count int, size int, dataFolder string) {
    packetTime := time.Date(2026, time.March, 7, 14, 30, 0, 0, time.UTC)
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

	if count == 0 { count = 3 }
	if size == 0 { size = 10 }

	if err := ensureDir(dataFolder); err != nil {
        slog.Error("failed to set up data directory", "error", err)
        return
    }

	matches, _ := filepath.Glob(filepath.Join(dataFolder, "*.pcap"))
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

	g := new(errgroup.Group)
	for i := 1; i <= needed; i++ {
        i := i
		fileName := filepath.Join(dataFolder, fmt.Sprintf("test_batch_%d.pcap", len(matches)+i))
        calculatedTime := packetTime.Add(time.Hour * time.Duration(i))
        g.Go(func() error {
            return GenerateDummyPcap(fileName, packetsPerFile, calculatedTime, profiles)
        })
    }
    if err := g.Wait(); err != nil {
        slog.Error("dummy pcap generation failed", "error", err)
        return
    }
	slog.Info("all dummy data generation files are ready for pipeline stress testing")
}


func PackageDumbRemoveFiles(dataFolder string) {
    filePath := filepath.Join(dataFolder, "test_batch_*.pcap")
    matches, err := filepath.Glob(filePath)
    if err != nil {
        slog.Error("Issue with the folder path")
        return
    }
    if len(matches) == 0 {
        slog.Info("no dummy pcap files found to remove")
        return
    }
    for _, file := range(matches){
        err := os.Remove(file)
        if err != nil {
            slog.Error("Could not remove file", "file", file, "error", err)
        }
    }
    slog.Info("removed dummy pcap files", "count", len(matches))
}


