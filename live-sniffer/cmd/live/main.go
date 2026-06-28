package main

import (
	"github.com/spf13/cobra"
	"os"
)

var (
	iface      string
	workers       int
	sessionID string
)

var rootCmd = &cobra.Command{
	Use:   "network-live-saver",
	Aliases: []string{"nls"},
	Short: "A tool for live network capture",
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts a pipelines for capturing network data",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all network pipeline",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stops a network pipeline",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func init() {
    startCmd.Flags().StringVarP(&iface, "interface", "i", "en0", "network interface to capture on")
    startCmd.Flags().IntVarP(&workers, "workers", "w", 2, "number of saver workers")
    stopCmd.Flags().StringVarP(&sessionID, "session", "s", "", "session ID to stop")

    rootCmd.AddCommand(startCmd)
    rootCmd.AddCommand(stopCmd)
    rootCmd.AddCommand(listCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
