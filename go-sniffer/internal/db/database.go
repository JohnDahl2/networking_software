package db

import (
	"fmt"
	"log"
	"os"
    "time"
    "context"
    "strings"
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
