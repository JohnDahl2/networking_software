package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

func findInterface() string {
    devices, err := pcap.FindAllDevs()
    if err != nil {
        log.Fatal(err)
    }
    
    for _, device := range devices {
        if len(device.Addresses) > 0 {
            fmt.Printf("Found interface: %s\n", device.Name)
            return device.Name
        }
    }
    return "eth0" // Fallback
}

func main() {
	ConnectDB()
    defer DB.Close()

	device := findInterface()
	snapshotLen := int32(1024)
	promiscuous := false
	timeout := 30 * time.Second

	handle, err := pcap.OpenLive(device, snapshotLen, promiscuous, timeout)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	fmt.Printf("Sniffing on %s...\n", device)

	for packet := range packetSource.Packets() {
		// Just print and do a dummy insert for the first commit
		t := packet.Metadata().Timestamp
		length := packet.Metadata().Length

		_, err := DB.ExecContext(context.Background(), 
		"INSERT INTO packet_summary (time, length, info) VALUES ($1, $2, $3)",
		t, length, "Initial Commit Packet")
        
        if err != nil {
            log.Printf("DB Insert Error: %v\n", err)
        } else {
            fmt.Printf("Captured packet at %v, length %d stored.\n", t, length)
        }
	}
}