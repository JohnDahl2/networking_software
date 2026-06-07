package pcap

import (
	"log/slog" // Upgraded to structured logging
	"os"

	"github.com/google/gopacket/pcap"
)

func FindInterface() string {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		slog.Error("failed to scan system network interfaces", "error", err.Error())
		os.Exit(1) // Cleanly halts the application if the lookup completely fails
	}

	for _, device := range devices {
		if len(device.Addresses) > 0 {
			slog.Info("active network interface discovered", "interface_name", device.Name)
			return device.Name
		}
	}

	// Non-fatal fallback alert
	slog.Warn("no active network interfaces detected; using fallback default", "fallback_interface", "eth0")
	return "eth0"
}