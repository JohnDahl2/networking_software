package db

import (
    "time"
    "context"
    "net/netip"

    "github.com/jackc/pgx/v5/pgtype"

	_ "github.com/jackc/pgx/v5/stdlib"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)


var DB *pgxpool.Pool

func InitDB(ctx context.Context, connString string) error {
    var err error
    // Connect to TimescaleDB using a connection pool
    DB, err = pgxpool.New(ctx, connString)
    if err != nil {
        return err
    }
    
    // Verify the connection works
    return DB.Ping(ctx)
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

    // Prepare the multi-row data matrix
    var inputRows [][]interface{}
    for _, row := range rows {
        inputRows = append(inputRows, []interface{}{
            row.Time, row.SrcIP.String(), row.DstIP.String(), 
            row.SrcPort, row.DstPort, row.Protocol, 
            row.Length, row.TCPFlags, row.StreamID,
        })
    }

    // Stream the binary matrix straight into the database engine
    _, err := DB.CopyFrom(
        ctx,
        pgx.Identifier{"packet_logs"},
        []string{"time", "src_ip", "dst_ip", "src_port", "dst_port", "protocol", "length", "tcp_flags", "stream_id"},
        pgx.CopyFromRows(inputRows),
    )
    return err
}

