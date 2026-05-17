package pcap

import (
	"fmt"
	"log"
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
