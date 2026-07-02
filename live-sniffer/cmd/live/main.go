package main

import (
	"context"
	"fmt"
	"live-sniffer/internal/config"
	"live-sniffer/internal/pipeline"
	"live-sniffer/internal/storage"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var (
	iface     string
	workers   int
	sessionID string
)
var launcher *pipeline.Launcher

var rootCmd = &cobra.Command{
	Use:   "network-live-saver",
	Aliases: []string{"nls"},
	Short: "A tool for live network capture",
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage nls configuration",
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a config value (keys: db-url, interface)",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key, val := args[0], args[1]
		cfg, err := config.ReadConfig()
		if err != nil && !os.IsNotExist(err) {
			slog.Error("failed to read config", "error", err)
			os.Exit(1)
		}
		switch key {
		case "db-url":
			cfg.DBUrl = val
		case "interface":
			cfg.DefaultInterface = val
		default:
			slog.Error("unknown config key", "key", key, "valid_keys", "db-url, interface")
			os.Exit(1)
		}
		if err := config.SaveConfig(cfg.DBUrl, cfg.DefaultInterface); err != nil {
			slog.Error("failed to save config", "error", err)
			os.Exit(1)
		}
		slog.Info("config updated", "key", key, "value", val)
	},
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
	startCmd.Flags().StringVarP(&iface, "interface", "i", "", "network interface to capture on (defaults to config value)")
	startCmd.Flags().IntVarP(&workers, "workers", "w", 2, "number of saver workers")
	stopCmd.Flags().StringVarP(&sessionID, "session", "s", "", "session ID to stop")

	configCmd.AddCommand(configSetCmd)

	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(configCmd)
}

func main() {
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// config set doesn't need a DB connection
		if cmd.Name() == "set" {
			return nil
		}
		cfg, err := config.ReadConfig()
		if err != nil {
			return fmt.Errorf("could not read config: run 'nls config set db-url <your-db-url>' first")
		}
		// use flag value if provided, otherwise fall back to config default
		if iface == "" {
			iface = cfg.DefaultInterface
		}
		db, err := storage.InitDB(cmd.Context(), cfg.DBUrl)
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