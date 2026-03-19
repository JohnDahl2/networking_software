package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/jackc/pgx/v5"
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
	// 1. Database Connection
	// Note: In 'host' network mode, use 'localhost'. 
	// If in standard Docker bridge mode, use the service name 'db'.
	dbUrl := os.Getenv("DATABASE_URL")
	conn, err := pgx.Connect(context.Background(), dbUrl)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer conn.Close(context.Background())
	fmt.Println("Successfully connected to TimescaleDB!")

		// 1. Bootstrap the Database Schema
	fmt.Println("Ensuring schema exists...")

	// Create the standard table if it doesn't exist
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS packet_summary (
			time        TIMESTAMPTZ       NOT NULL,
			length      INTEGER,
			info        TEXT
		);
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Enable TimescaleDB Hypertable (wrapped in a check to avoid errors if already enabled)
	_, err = conn.Exec(context.Background(), `
		SELECT create_hypertable('packet_summary', 'time', if_not_exists => TRUE);
	`)
	if err != nil {
		// We log this but don't necessarily 'Fatal' because 
		// it might just mean the extension isn't loaded yet
		fmt.Printf("Note: Hypertable setup skip/fail: %v\n", err)
	}

	fmt.Println("Schema is ready!")

	// 2. Setup Packet Capture
	device := findInterface()
	snapshotLen := int32(1024)
	promiscuous := false
	timeout := 30 * time.Second

	handle, err := pcap.OpenLive(device, snapshotLen, promiscuous, timeout)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	// 3. Simple Capture Loop
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	fmt.Printf("Sniffing on %s...\n", device)

	for packet := range packetSource.Packets() {
		// Just print and do a dummy insert for the first commit
		t := packet.Metadata().Timestamp
		length := packet.Metadata().Length

		_, err := conn.Exec(context.Background(), 
			"INSERT INTO packet_summary (time, length, info) VALUES ($1, $2, $3)",
			t, length, "Initial Commit Packet")
		
		if err != nil {
			log.Printf("DB Insert Error: %v\n", err)
		} else {
			fmt.Printf("Captured packet at %v, length %d stored.\n", t, length)
		}
	}
}