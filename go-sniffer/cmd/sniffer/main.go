package main

import (
    "go-sniffer/internal/db"
	"go-sniffer/internal/pcap" 
)

func main() {
	db.ConnectDB()
    defer db.DB.Close()

	pcap.DemoNetowrkReader()
}