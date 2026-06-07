package storage

import (
	"context"
	"os"
	"io"
	"errors"
	"fmt"
	"crypto/sha256"
	"log/slog"
	"net/netip"
	"time"
	"sync"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)


func InitDB(ctx context.Context, connString string)(*pgxpool.Pool, error){
	var err error
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		slog.Error("Database config parsing failed", "Error", err)
		return nil, err
	}

	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnIdleTime = 30 * time.Minute
	config.MaxConnLifetime = 1 * time.Hour

	DB, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		slog.Error("Failed to create database pool", "Error", err)
		return nil, err
	}

	if err = DB.Ping(ctx); err != nil {
		slog.Error("Database ping failed, host may be down", "Error", err)
		return nil, err
	}

	if err = RunMigrations(connString); err != nil {
		slog.Error("Schema migrations failed", "Error", err)
		return nil, err
	}

	return DB, nil
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

func BulkDatabaseCopy(ctx context.Context, DB DBStore, rows []PacketRow) error {
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

// CheckAndInsertSourceFile is a worker goroutine that reads file paths from the files channel,
// computes a SHA256 checksum, and inserts into source_files if the file is new.
// New (unseen) file paths are sent to the results channel for the pipeline to process.
func CheckAndInsertSourceFile(ctx context.Context, DB DBStore, jobID pgtype.UUID, files <-chan string, results chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()

	for filePath := range files {
		// Open and hash the file
		f, err := os.Open(filePath)
		if err != nil {
			slog.Error("failed to open file for checksum", "file", filePath, "error", err)
			continue
		}

		h := sha256.New()
		_, err = io.CopyN(h, f, 65536)
		f.Close()
		if err != nil && err != io.EOF {
			slog.Error("failed to compute checksum", "file", filePath, "error", err)
			continue
		}
		checksum := fmt.Sprintf("%x", h.Sum(nil))

		// Try to insert — unique constraint rejects duplicates
		_, err = DB.Exec(ctx, `
			INSERT INTO source_files (job_id, file_path, checksum)
			VALUES ($1, $2, $3)
		`, jobID, filePath, checksum)

		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				slog.Debug("skipping duplicate file", "file", filePath)
				continue
			}
			slog.Error("failed to insert source file record", "file", filePath, "error", err)
			continue
		}

		// File is new — send it to the pipeline
		select {
		case results <- filePath:
		case <-ctx.Done():
			return
		}
	}
}

