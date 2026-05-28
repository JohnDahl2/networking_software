package db

import (
	"context"
	"log/slog"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
    "github.com/pressly/goose/v3"
    "github.com/jackc/pgx/v5/stdlib"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)


var DB *pgxpool.Pool


func InitDB(ctx context.Context, connString string) error {
	var err error
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		slog.Error("Database config parsing failed", "Error", err)
		return err
	}

	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnIdleTime = 30 * time.Minute
	config.MaxConnLifetime = 1 * time.Hour

	DB, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		slog.Error("Failed to create database pool", "Error", err)
		return err
	}

	if err = DB.Ping(ctx); err != nil {
		slog.Error("Database ping failed, host may be down", "Error", err)
		return err
	}

	if err = RunMigrations(connString); err != nil {
		slog.Error("Schema migrations failed", "Error", err)
		return err
	}

	return nil
}



func RunMigrations(connString string) error {
	pgxConfig, err := pgx.ParseConfig(connString)
	if err != nil {
		return err
	}

	dbConn := stdlib.OpenDB(*pgxConfig)
	defer dbConn.Close()

	slog.Info("Running database schema migrations via Goose...")
	
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(dbConn, "./migrations")
}

type PacketRow struct {
    Time         time.Time  `db:"time"`
    SrcIP        netip.Addr `db:"src_ip"`
    DstIP        netip.Addr `db:"dst_ip"`
    SrcPort      int32      `db:"src_port"`
    DstPort      int32      `db:"dst_port"`
    Protocol     string     `db:"protocol"`
    Length       int32      `db:"length"`
    TCPFlags     int16      `db:"tcp_flags"`
    StreamID     pgtype.UUID `db:"stream_id"`
}

func BulkDatabaseCopy(ctx context.Context, rows []PacketRow) error {
    if len(rows) == 0 {
        return nil
    }

    var inputRows [][]interface{}
    for _, row := range rows {
        inputRows = append(inputRows, []interface{}{
            row.Time, row.SrcIP.String(), row.DstIP.String(), 
            row.SrcPort, row.DstPort, row.Protocol, 
            row.Length, row.TCPFlags, row.StreamID,
        })
    }

    _, err := DB.CopyFrom(
        ctx,
        pgx.Identifier{"packet_logs"},
        []string{"time", "src_ip", "dst_ip", "src_port", "dst_port", "protocol", "length", "tcp_flags", "stream_id"},
        pgx.CopyFromRows(inputRows),
    )
    return err
}
