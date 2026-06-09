package main

import (
	"os"
	"go-sniffer/internal/pcap"
	"github.com/spf13/cobra"
)


var (
	count      int
	size       int
	dataFolder string
)

var rootCmd = &cobra.Command{
	Use:   "pcap-gen",
	Short: "A tool for generating and removing dummy pcap files",
}

var genCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate dummy pcap files in the specified output directory",
	Run: func(cmd *cobra.Command, args []string) {
		pcap.GenerateFiles(count, size, dataFolder)
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Remove all generated dummy pcap files from the output directory",
	Run: func(cmd *cobra.Command, args []string) {
		pcap.RemoveFiles(dataFolder)
	},
}

func init() {
	genCmd.Flags().IntVarP(&count, "count", "c", 10, "Number of pcap files to generate")
	genCmd.Flags().IntVarP(&size, "size", "s", 64, "Target size of each file in MB")
	genCmd.Flags().StringVarP(&dataFolder, "output", "o", pcap.DefaultDumbDataFolder, "Output directory for generated pcap files")
	deleteCmd.Flags().StringVarP(&dataFolder, "output", "o", pcap.DefaultDumbDataFolder, "Directory to remove dummy pcap files from")


	rootCmd.AddCommand(genCmd)
	rootCmd.AddCommand(deleteCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}