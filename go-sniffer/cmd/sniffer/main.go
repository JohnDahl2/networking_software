package main

import (
	"os"

    // "go-sniffer/internal/db"
	"go-sniffer/internal/pcap" 
)

func main() {
	// db.ConnectDB()
	// if db.DB != nil {
    //     defer db.DB.Close()
    // }

	demo_mode := os.Getenv("DEMO_MODE")
	if demo_mode == "true" {
		pcap.PackageGenerator()
	}
}