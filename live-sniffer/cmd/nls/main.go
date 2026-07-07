package main

import (
	"context"
	"fmt"
	"live-sniffer/internal/config"
	"live-sniffer/internal/daemon"
	"live-sniffer/internal/pipeline"
	"live-sniffer/internal/proto"
	"live-sniffer/internal/storage"
	"live-sniffer/migrations"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var (
	iface        string
	workers      int
	sessionID    string
)

var rootCmd = &cobra.Command{
	Use:   "network-live-saver",
	Aliases: []string{"nls"},
	Short: "A tool for live network capture",
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage nls configuration",
}

var daemonCmd = &cobra.Command{
    Use:   "daemon",
    Short: "Manage the nls daemon",
}

var daemonStartCmd = &cobra.Command{
    Use:   "start",
    Short: "Start the nls daemon",
    Run: func(cmd *cobra.Command, args []string) {
        cfg, err := config.ReadConfig()
        if err != nil {
            slog.Error("failed to read config", "error", err)
            os.Exit(1)
        }
        db, err := storage.InitDB(cmd.Context(), cfg.DBUrl)
        if err != nil {
            slog.Error("failed to connect to db", "error", err)
            os.Exit(1)
        }
        l := &pipeline.Launcher{
            DB:       db,
            Sessions: make(map[string]context.CancelFunc),
        }
        d := &daemon.Daemon{
            SocketPath: config.SockPath(),
            Launcher:   l,
        }
        if err := d.Start(cmd.Context()); err != nil {
            slog.Error("daemon error", "error", err)
            os.Exit(1)
        }
    },
}

var daemonStopCmd = &cobra.Command{
    Use:   "stop",
    Short: "Stop the nls daemon",
    Run: func(cmd *cobra.Command, args []string) {
        resp, err := daemon.Send(config.SockPath(), proto.Request{Command: proto.CmdShutdown})
        if err != nil {
            slog.Error("failed to contact daemon", "error", err)
            os.Exit(1)
        }
        if !resp.OK {
            slog.Error("daemon error", "error", resp.Error)
            os.Exit(1)
        }
        slog.Info("daemon stopped")
    },
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
			if err = migrations.RunMigrations(cfg.DBUrl); err != nil {
				slog.Error("migrations failed", "error", err)
				os.Exit(1)
			}
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
    Short: "Start a capture pipeline",
    Run: func(cmd *cobra.Command, args []string) {
        resp, err := daemon.Send(config.SockPath(), proto.Request{
            Command:   proto.CmdStart,
            Interface: iface,
            Workers:   workers,
        })
        if err != nil {
            slog.Error("failed to contact daemon", "error", err)
            os.Exit(1)
        }
        if !resp.OK {
            slog.Error("failed to start", "error", resp.Error)
            os.Exit(1)
        }
        slog.Info("pipeline started", "session_id", resp.SessionID)
    },
}

var stopCmd = &cobra.Command{
    Use:   "stop",
    Short: "Stop a capture pipeline",
    Run: func(cmd *cobra.Command, args []string) {
        resp, err := daemon.Send(config.SockPath(), proto.Request{
            Command:   proto.CmdStop,
            SessionID: sessionID,
        })
        if err != nil {
            slog.Error("failed to contact daemon", "error", err)
            os.Exit(1)
        }
        if !resp.OK {
            slog.Error("failed to start", "error", resp.Error)
            os.Exit(1)
        }
        slog.Info("pipeline stoped", "session_id", resp.SessionID)
    },
}

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List all pipeline",
    Run: func(cmd *cobra.Command, args []string) {
        resp, err := daemon.Send(config.SockPath(), proto.Request{
            Command:   proto.CmdStatus,
        })
        if err != nil {
            slog.Error("failed to contact daemon", "error", err)
            os.Exit(1)
        }
        if !resp.OK {
            slog.Error("failed to get session information", "error", resp.Error)
            os.Exit(1)
        }
		for _, s := range resp.Sessions {
			fmt.Printf("id=%s  iface=%s  status=%s  started=%s\n", s.ID, s.Interface, s.Status, s.StartedAt)
		}
        slog.Info("pipeline listed")
    },
}

func init() {
	startCmd.Flags().StringVarP(&iface, "interface", "i", "", "network interface to capture on (defaults to config value)")
	startCmd.Flags().IntVarP(&workers, "workers", "w", 2, "number of saver workers")
	stopCmd.Flags().StringVarP(&sessionID, "session", "s", "", "session ID to stop")

	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)

	configCmd.AddCommand(configSetCmd)

	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(configCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}