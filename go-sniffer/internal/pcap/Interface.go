package pcap

import (
	"fmt"
	"log"
	"time"

	"go-sniffer/internal/db"

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
	return "eth0"
}

func DemoNetowrkReader() {
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
		t := packet.Metadata().Timestamp
		length := packet.Metadata().Length
		db.DatabaseWrite(t, length)
	}
}
