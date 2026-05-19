package db

import (
	"fmt"
	"os"
    "time"
    "context"
    "strings"
	"database/sql"
    "log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

var DB *sql.DB

type PacketRow struct {
    Timestamp time.Time
    Length    int
}

func BulkDatabaseWrite(ctx context.Context, rows []PacketRow) error {
    if len(rows) == 0 {
        slog.Debug("There was no rows to upload")
        return nil
    }

    const numCols = 3
    // Use a Builder to avoid massive memory allocations
    var queryBuilder strings.Builder
    queryBuilder.WriteString("INSERT INTO packet_summary (time, length, info) VALUES ")
    
    values := make([]interface{}, 0, len(rows)*numCols)

    for i, row := range rows {
        if i > 0 {
            queryBuilder.WriteString(",")
        }
        // Manually calculate placeholders without Sprintf for speed
        p1, p2, p3 := i*numCols+1, i*numCols+2, i*numCols+3
        queryBuilder.WriteString(fmt.Sprintf("($%d, $%d, $%d)", p1, p2, p3))
        
        values = append(values, row.Timestamp, row.Length, "Bulk Inserted Packet")
    }

    _, err := DB.ExecContext(ctx, queryBuilder.String(), values...)
    if err != nil {
        slog.Error("bulk database write failed", "error", err.Error())
        return err
    }
    return nil
}

func ConnectDB() {
    var err error
    connStr := os.Getenv("DATABASE_URL")

    DB, err = sql.Open("pgx", connStr)
    if err != nil {
        slog.Error("unable to initialize database pool", "error", err.Error())
    }

    slog.Info("Db connected")

    if err := goose.SetDialect("postgres"); err != nil {
        slog.Error("Unable to set goose:","error", err.Error())
    }

    if err := goose.Up(DB, "migrations"); err != nil {
        slog.Error("Unable to migrate:","error", err.Error())
    }
    slog.Info("database migrations applied successfully")
}
