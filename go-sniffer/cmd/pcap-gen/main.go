package main

import (
	"os"
	"fmt"
	"go-sniffer/internal/pcap"
	"github.com/spf13/cobra"
)


var (
	count int
	size  int
)

var rootCmd = &cobra.Command{
	Use:   "pcap-gen",
	Short: "A tool for generating dummy pcap files",
}

var genCmd = &cobra.Command{
	Use:   "generate",
	Short: fmt.Sprintf("Generate dummy pcap packages in %s", pcap.DumbDataFolder),
	Run: func(cmd *cobra.Command, args []string) {
		pcap.PackageDumbGenerator(count, size)
	},
}

func init() {
	genCmd.Flags().IntVarP(&count, "count", "c", 10, "Number of packets to generate")
	genCmd.Flags().IntVarP(&size, "size", "s", 64, "Size of each packet in bytes")


	rootCmd.AddCommand(genCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}