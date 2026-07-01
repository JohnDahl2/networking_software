package main

import (
	"context"
	"live-sniffer/internal/pipeline"
	"live-sniffer/internal/storage"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var (
	iface      string
	workers       int
	sessionID string
)
var launcher *pipeline.Launcher

var rootCmd = &cobra.Command{
	Use:   "network-live-saver",
	Aliases: []string{"nls"},
	Short: "A tool for live network capture",
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts a pipeline for capturing network data",
	Run: func(cmd *cobra.Command, args []string) {
		if err := launcher.StartPipeline(cmd.Context(), iface, workers); err != nil {
			slog.Error("failed to start pipeline", "error", err)
			os.Exit(1)
		}
		<-cmd.Context().Done() // block until Ctrl+C or DB stop signal
		slog.Info("shutting down")
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all network pipeline",
	Run: func(cmd *cobra.Command, args []string) {
		if err := launcher.ViewPipelines(cmd.Context()); err != nil {
            slog.Error("failed to view pipeline", "error", err)
            os.Exit(1)
        }
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stops a network pipeline",
	Run: func(cmd *cobra.Command, args []string) {
		if err := launcher.StopPipeline(cmd.Context(), sessionID); err != nil {
            slog.Error("failed to stop pipeline", "error", err)
            os.Exit(1)
        }
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
    rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
        db, err := storage.InitDB(cmd.Context(), os.Getenv("DATABASE_URL"))
        if err != nil {
            return err
        }
        launcher = &pipeline.Launcher{
            DB:       db,
            Sessions: make(map[string]context.CancelFunc),
        }
        return nil
    }

    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}