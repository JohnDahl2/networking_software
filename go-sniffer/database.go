package main

import (
	"fmt"
	"log"
	"os"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

var DB *sql.DB

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