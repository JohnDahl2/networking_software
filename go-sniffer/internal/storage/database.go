package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	shared "networking_software/shared/storage"
)

// Re-export shared types so existing go-sniffer code needs no import changes.
type PacketRow = shared.PacketRow

var BulkDatabaseCopy = shared.BulkDatabaseCopy
var InitDB = shared.InitDB

// CheckAndInsertSourceFile is a worker goroutine that reads file paths from the files channel,
// computes a SHA256 checksum, and inserts into source_files if the file is new.
// New (unseen) file paths are sent to the results channel for the pipeline to process.
func CheckAndInsertSourceFile(ctx context.Context, DB DBStore, jobID pgtype.UUID, files <-chan string, results chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()

	for filePath := range files {
		f, err := os.Open(filePath)
		if err != nil {
			slog.Error("failed to open file for checksum", "file", filePath, "error", err)
			continue
		}

		h := sha256.New()
		_, err = io.CopyN(h, f, 65536)
		f.Close() //nolint:errcheck
		if err != nil && !errors.Is(err, io.EOF) {
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
