package db

import (
	"fmt"
	"log"
	"os"
    "time"
    "context"
	"database/sql"

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
        return nil
    }

    numCols := 3
    placeholderCount := 1
    values := make([]interface{}, 0, len(rows)*numCols)
    query := "INSERT INTO packet_summary (time, length, info) VALUES "

    for i, row := range rows {
        if i > 0 {
            query += ","
        }
        query += fmt.Sprintf("($%d, $%d, $%d)", placeholderCount, placeholderCount+1, placeholderCount+2)
        
        values = append(values, row.Timestamp, row.Length, "Bulk Inserted Packet")
        placeholderCount += numCols
    }

    _, err := DB.ExecContext(ctx, query, values...)
    return err
}

func ConnectDB() {
    var err error
    connStr := os.Getenv("DATABASE_URL")

    DB, err = sql.Open("pgx", connStr)
    if err != nil {
        log.Fatalf("Unable to connect to database: %v\n", err)
    }

    fmt.Println("Connected to TimescaleDB!")

    if err := goose.SetDialect("postgres"); err != nil {
        log.Fatalf("Failed to set goose dialect: %v", err)
    }

    if err := goose.Up(DB, "migrations"); err != nil {
        log.Fatalf("Migration failed: %v", err)
    }
}
