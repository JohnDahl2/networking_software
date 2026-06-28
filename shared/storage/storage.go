package storage

import (
	"context"
	"log/slog"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BulkWriter is the minimal interface needed to write packet batches.
// Both go-sniffer's DBStore and live-sniffer's pool satisfy this automatically.
type BulkWriter interface {
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}

// PacketRow represents a single packet to be written to the database.
type PacketRow struct {
	Time     time.Time   `db:"time"`
	SrcIP    netip.Addr  `db:"src_ip"`
	DstIP    netip.Addr  `db:"dst_ip"`
	SrcPort  int32       `db:"src_port"`
	DstPort  int32       `db:"dst_port"`
	Protocol string      `db:"protocol"`
	Length   int32       `db:"length"`
	TCPFlags int16       `db:"tcp_flags"`
	JobID    pgtype.UUID `db:"job_id"`
}

// InitDB creates and returns a configured connection pool.
func InitDB(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		slog.Error("Database config parsing failed", "error", err)
		return nil, err
	}

	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnIdleTime = 30 * time.Minute
	config.MaxConnLifetime = 1 * time.Hour

	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		slog.Error("Failed to create database pool", "error", err)
		return nil, err
	}

	if err = db.Ping(ctx); err != nil {
		slog.Error("Database ping failed", "error", err)
		return nil, err
	}

	return db, nil
}

// BulkDatabaseCopy writes a batch of packets to the database using COPY.
func BulkDatabaseCopy(ctx context.Context, db BulkWriter, rows []PacketRow) error {
	if len(rows) == 0 {
		return nil
	}

	inputRows := make([][]any, len(rows))
	for i, row := range rows {
		inputRows[i] = []any{
			row.Time, row.SrcIP.String(), row.DstIP.String(),
			row.SrcPort, row.DstPort, row.Protocol,
			row.Length, row.TCPFlags, row.JobID,
		}
	}

	_, err := db.CopyFrom(
		ctx,
		pgx.Identifier{"packet_logs"},
		[]string{"time", "src_ip", "dst_ip", "src_port", "dst_port", "protocol", "length", "tcp_flags", "job_id"},
		pgx.CopyFromRows(inputRows),
	)
	return err
}
